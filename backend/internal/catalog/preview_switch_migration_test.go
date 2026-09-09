package catalog

import (
	"context"
	"testing"
)

func TestOpenRetiresPerDrivePreviewConfiguration(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/catalog.db"
	cat, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cat.Close() })
	if err := cat.UpsertDrive(ctx, &Drive{
		ID: "drive", Kind: "onedrive", RootID: "root",
		Credentials: map[string]string{"refresh_token": "keep"}, SkipDirIDs: []string{"skip"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := cat.db.ExecContext(ctx, "ALTER TABLE drives ADD COLUMN teaser_enabled INTEGER NOT NULL DEFAULT 0"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"preview.enabled", "drives.teaser_enabled.default_open_migrated"} {
		if err := cat.SetSetting(ctx, key, "0"); err != nil {
			t.Fatal(err)
		}
	}
	if err := cat.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		reopened, err := Open(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		if hasColumn(t, reopened, "drives", "teaser_enabled") {
			t.Fatal("per-drive preview column was retained")
		}
		drive, err := reopened.GetDrive(ctx, "drive")
		if err != nil || drive.RootID != "root" || drive.Credentials["refresh_token"] != "keep" || len(drive.SkipDirIDs) != 1 {
			t.Fatalf("migration lost drive configuration: %+v, %v", drive, err)
		}
		for _, key := range []string{"preview.enabled", "drives.teaser_enabled.default_open_migrated"} {
			if value, err := reopened.GetSetting(ctx, key, "missing"); err != nil || value != "missing" {
				t.Fatalf("legacy setting %s remains: %q, %v", key, value, err)
			}
		}
		if err := reopened.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
