package catalog

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func TestSQLiteWriteTransactionsReserveWriterBeforeReading(t *testing.T) {
	for _, commit := range []bool{true, false} {
		name := "rollback"
		if commit {
			name = "commit"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			path := filepath.Join(t.TempDir(), "catalog.db")
			cat, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer cat.Close()
			seedTagMaintenanceVideoRaw(t, cat, "video", "original", "video.mp4")

			// A separate pool also covers writers outside this Catalog instance.
			other, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(0)")
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			writer, err := cat.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer writer.Rollback()
			var title string
			if err := writer.QueryRowContext(ctx, `SELECT title FROM videos WHERE id = 'video'`).Scan(&title); err != nil {
				t.Fatal(err)
			}
			if _, err := other.ExecContext(ctx, `UPDATE videos SET title = 'competing' WHERE id = 'video'`); err == nil {
				t.Fatal("another writer committed after the transaction read its snapshot")
			} else {
				var sqliteErr *sqlite.Error
				if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_BUSY {
					t.Fatalf("competing write error = %v, want SQLITE_BUSY", err)
				}
			}

			// Read-only transactions must retain WAL snapshot reads without taking
			// the writer reservation, even while the first transaction commits.
			reader, err := cat.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatalf("begin concurrent reader: %v", err)
			}
			defer reader.Rollback()
			if err := reader.QueryRowContext(ctx, `SELECT title FROM videos WHERE id = 'video'`).Scan(&title); err != nil || title != "original" {
				t.Fatalf("read before write: title=%q error=%v", title, err)
			}
			if _, err := writer.ExecContext(ctx, `UPDATE videos SET title = 'updated' WHERE id = 'video'`); err != nil {
				t.Fatalf("write after reading: %v", err)
			}
			want := "original"
			if commit {
				err = writer.Commit()
				want = "updated"
			} else {
				err = writer.Rollback()
			}
			if err != nil {
				t.Fatalf("finish writer: %v", err)
			}
			if err := reader.QueryRowContext(ctx, `SELECT title FROM videos WHERE id = 'video'`).Scan(&title); err != nil || title != "original" {
				t.Fatalf("read snapshot after write: title=%q error=%v", title, err)
			}
			if err := other.QueryRowContext(ctx, `SELECT title FROM videos WHERE id = 'video'`).Scan(&title); err != nil || title != want {
				t.Fatalf("read committed state: title=%q error=%v, want %q", title, err, want)
			}
			if _, err := other.ExecContext(ctx, `UPDATE videos SET title = 'next writer' WHERE id = 'video'`); err != nil {
				t.Fatalf("writer reservation was not released: %v", err)
			}
		})
	}
}

func TestReplaceAutoVideoTagsWaitsForConcurrentWriter(t *testing.T) {
	cat, _ := openTagMaintenanceTestCatalog(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	seedTagMaintenanceVideoRaw(t, cat, "video", "ordinary", "video.mp4")
	for _, label := range []string{"old-auto", "new-auto"} {
		if _, err := cat.EnsureTag(ctx, label, "user"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cat.ReplaceAutoVideoTags(ctx, "video", []TagAssignment{{Label: "old-auto", Source: "auto"}}); err != nil {
		t.Fatal(err)
	}
	barrier, err := cat.BeginWriteBarrier(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer barrier.Close()

	type outcome struct {
		changed bool
		err     error
	}
	finished := make(chan outcome, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		changed, err := cat.ReplaceAutoVideoTags(ctx, "video", []TagAssignment{{Label: "new-auto", Source: "auto", Evidence: "title"}})
		finished <- outcome{changed, err}
	}()
	defer func() {
		cancel()
		_ = barrier.Close()
		<-exited
	}()

	select {
	case result := <-finished:
		t.Fatalf("tag replacement finished while the writer lock was held: changed=%t error=%v", result.changed, result.err)
	case <-time.After(100 * time.Millisecond):
	}
	video, err := cat.GetVideo(ctx, "video")
	if err != nil || !sameStrings(video.Tags, []string{"old-auto"}) {
		t.Fatalf("read during pending tag write: video=%+v error=%v", video, err)
	}
	if err := barrier.Close(); err != nil {
		t.Fatal(err)
	}
	result := <-finished
	if result.err != nil || !result.changed {
		t.Fatalf("tag replacement after writer released: changed=%t error=%v", result.changed, result.err)
	}
	video, err = cat.GetVideo(ctx, "video")
	if err != nil || !sameStrings(video.Tags, []string{"new-auto"}) {
		t.Fatalf("tags after replacement: video=%+v error=%v", video, err)
	}
	metadata, err := cat.ListVideoTagMetadata(ctx, []string{"video"})
	if err != nil {
		t.Fatal(err)
	}
	if got := metadata["video"]["new-auto"]; got.Source != "auto" || got.Evidence != "title" {
		t.Fatalf("tag metadata = %+v, want auto with title evidence", got)
	}
}

func TestReplaceAutoVideoTagsCancellationReleasesConnection(t *testing.T) {
	cat, ctx := openTagMaintenanceTestCatalog(t)
	seedTagMaintenanceVideoRaw(t, cat, "video", "ordinary", "video.mp4")
	if _, err := cat.EnsureTag(ctx, "new-auto", "user"); err != nil {
		t.Fatal(err)
	}
	cat.db.SetMaxOpenConns(2)
	barrier, err := cat.BeginWriteBarrier(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer barrier.Close()

	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	assignments := []TagAssignment{{Label: "new-auto", Source: "auto"}}
	if changed, err := cat.ReplaceAutoVideoTags(waitCtx, "video", assignments); err == nil || changed || !errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("canceled tag replacement: changed=%t error=%v context=%v", changed, err, waitCtx.Err())
	}
	if err := barrier.Close(); err != nil {
		t.Fatal(err)
	}
	video, err := cat.GetVideo(ctx, "video")
	if err != nil || len(video.Tags) != 0 {
		t.Fatalf("canceled replacement changed tags: video=%+v error=%v", video, err)
	}
	if changed, err := cat.ReplaceAutoVideoTags(ctx, "video", assignments); err != nil || !changed {
		t.Fatalf("tag replacement after cancellation: changed=%t error=%v", changed, err)
	}
}

func TestReplaceAutoVideoTagsTxChecksManualLockInTransaction(t *testing.T) {
	cat, ctx := openTagMaintenanceTestCatalog(t)
	seedTagMaintenanceVideoRaw(t, cat, "video", "ordinary", "video.mp4")
	if _, err := cat.EnsureTag(ctx, "new-auto", "user"); err != nil {
		t.Fatal(err)
	}
	if cat.hasManualTags(ctx, "video") {
		t.Fatal("video was already manually locked")
	}
	tx, err := cat.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// A pre-transaction check cannot see a manual lock in this write snapshot.
	if _, err := tx.ExecContext(ctx, `UPDATE videos SET tags_manual = 1 WHERE id = 'video'`); err != nil {
		t.Fatal(err)
	}
	if changed, err := replaceAutoVideoTagsTx(ctx, tx, "video", []TagAssignment{{Label: "new-auto", Source: "auto"}}); err != nil || changed {
		t.Fatalf("replacement ignored the transaction's manual lock: changed=%t error=%v", changed, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var assignments int
	if err := cat.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_tags WHERE video_id = 'video'`).Scan(&assignments); err != nil || assignments != 0 {
		t.Fatalf("manual video gained automatic assignments: count=%d error=%v", assignments, err)
	}
	if !cat.hasManualTags(ctx, "video") {
		t.Fatal("manual lock was removed")
	}
}
