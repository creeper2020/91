package catalog

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestDeleteDriveRemovesCatalogVideosAndKeepsOtherDrives(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	for _, d := range []*Drive{
		{ID: "drive-delete", Kind: "googledrive", Name: "Delete", RootID: "root"},
		{ID: "drive-keep", Kind: "p115", Name: "Keep", RootID: "root"},
	} {
		if err := cat.UpsertDrive(ctx, d); err != nil {
			t.Fatalf("upsert drive %s: %v", d.ID, err)
		}
	}
	for _, v := range []*Video{
		{ID: "delete-video", DriveID: "drive-delete", FileID: "delete-file", Title: "Delete", Tags: []string{"tag-a"}, PublishedAt: now},
		{ID: "keep-video", DriveID: "drive-keep", FileID: "keep-file", Title: "Keep", Tags: []string{"tag-a"}, PublishedAt: now},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("upsert video %s: %v", v.ID, err)
		}
	}
	if _, err := cat.db.ExecContext(ctx, `INSERT INTO scans (drive_id, started_at, scanned, added) VALUES (?, ?, 1, 1)`, "drive-delete", now.UnixMilli()); err != nil {
		t.Fatalf("seed scan: %v", err)
	}

	if err := cat.DeleteDrive(ctx, "drive-delete"); err != nil {
		t.Fatalf("delete drive: %v", err)
	}

	if _, err := cat.GetDrive(ctx, "drive-delete"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted drive err = %v, want sql.ErrNoRows", err)
	}
	if _, err := cat.GetVideo(ctx, "delete-video"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted video err = %v, want sql.ErrNoRows", err)
	}
	if _, err := cat.GetDrive(ctx, "drive-keep"); err != nil {
		t.Fatalf("kept drive missing: %v", err)
	}
	if _, err := cat.GetVideo(ctx, "keep-video"); err != nil {
		t.Fatalf("kept video missing: %v", err)
	}
	if got := countRows(t, cat, `SELECT COUNT(*) FROM video_tags WHERE video_id = 'delete-video'`); got != 0 {
		t.Fatalf("deleted video tag refs = %d, want 0", got)
	}
	if got := countRows(t, cat, `SELECT COUNT(*) FROM scans WHERE drive_id = 'drive-delete'`); got != 0 {
		t.Fatalf("deleted drive scans = %d, want 0", got)
	}
}

func TestDeleteVideosForMissingDrivesKeepsLocalUpload(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	if err := cat.UpsertDrive(ctx, &Drive{ID: "drive-keep", Kind: "p115", Name: "Keep", RootID: "root"}); err != nil {
		t.Fatalf("upsert drive: %v", err)
	}
	for _, v := range []*Video{
		{ID: "orphan-video", DriveID: "deleted-google-drive", FileID: "orphan-file", Title: "Orphan", Tags: []string{"tag-a"}, PublishedAt: now},
		{ID: "local-video", DriveID: "local-upload", FileID: "local-file", Title: "Local", PublishedAt: now},
		{ID: "keep-video", DriveID: "drive-keep", FileID: "keep-file", Title: "Keep", PublishedAt: now},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("upsert video %s: %v", v.ID, err)
		}
	}

	deleted, err := cat.DeleteVideosForMissingDrives(ctx, []string{"local-upload"})
	if err != nil {
		t.Fatalf("delete missing-drive videos: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := cat.GetVideo(ctx, "orphan-video"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("orphan video err = %v, want sql.ErrNoRows", err)
	}
	for _, id := range []string{"local-video", "keep-video"} {
		if _, err := cat.GetVideo(ctx, id); err != nil {
			t.Fatalf("video %s missing: %v", id, err)
		}
	}
}

func countRows(t *testing.T, cat *Catalog, query string) int {
	t.Helper()
	var count int
	if err := cat.db.QueryRow(query).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
