package catalog

import (
	"context"
	"testing"
	"time"
)

func TestListVideosHidesMissingDriveVideosWhenDrivesExist(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	if err := cat.UpsertDrive(ctx, &Drive{
		ID:            "active-drive",
		Kind:          "pikpak",
		Name:          "Active",
		RootID:        "root",
		TeaserEnabled: true,
	}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	now := time.Now()
	for _, v := range []*Video{
		{
			ID:          "visible-video",
			DriveID:     "active-drive",
			FileID:      "visible-file",
			Title:       "Visible",
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "orphan-video",
			DriveID:     "deleted-drive",
			FileID:      "orphan-file",
			Title:       "Orphan",
			PublishedAt: now.Add(time.Second),
			CreatedAt:   now.Add(time.Second),
			UpdatedAt:   now.Add(time.Second),
		},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	items, total, err := cat.ListVideos(ctx, ListParams{Page: 1, PageSize: 10, Sort: "latest"})
	if err != nil {
		t.Fatalf("list videos: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ID != "visible-video" {
		t.Fatalf("items total=%d items=%v, want only visible-video", total, items)
	}
}
