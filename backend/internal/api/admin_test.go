package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/video-site/backend/internal/auth"
	"github.com/video-site/backend/internal/catalog"
)

func TestHandleLoginReturnsForbiddenForBannedIP(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.BanLoginIP(ctx, "203.0.113.20", "test"); err != nil {
		t.Fatalf("ban ip: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	req.RemoteAddr = "203.0.113.20:12345"
	rr := httptest.NewRecorder()

	(&AdminServer{
		Catalog: cat,
		Auth:    &auth.Authenticator{Username: "admin", Password: "secret", Catalog: cat},
	}).handleLogin(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandleLoginRequiresSetupBeforeDefaultLogin(t *testing.T) {
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", strings.NewReader(`{"username":"admin","password":"admin123"}`))
	rr := httptest.NewRecorder()
	(&AdminServer{
		Catalog:       cat,
		Auth:          &auth.Authenticator{Username: "admin", Password: "admin123", Catalog: cat},
		SetupRequired: func() bool { return true },
	}).handleLogin(rr, req)

	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428; body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupStoresCredentialsAndCreatesSession(t *testing.T) {
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	authr := &auth.Authenticator{Username: "admin", Password: "admin123", Catalog: cat}
	setupRequired := true
	var savedUser, savedPass string
	req := httptest.NewRequest(http.MethodPost, "/admin/api/setup", strings.NewReader(`{"username":"owner","password":"secret123"}`))
	rr := httptest.NewRecorder()

	(&AdminServer{
		Catalog:       cat,
		Auth:          authr,
		SetupRequired: func() bool { return setupRequired },
		OnSetup: func(username, password string) error {
			savedUser, savedPass = username, password
			authr.SetCredentials(username, password)
			setupRequired = false
			return nil
		},
	}).handleSetup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rr.Code, rr.Body.String())
	}
	if savedUser != "owner" || savedPass != "secret123" {
		t.Fatalf("saved credentials = %q/%q, want owner/secret123", savedUser, savedPass)
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("setup did not set a session cookie")
	}
	ok, err := cat.ValidateSession(context.Background(), cookies[0].Value)
	if err != nil || !ok {
		t.Fatalf("setup session valid=%v err=%v", ok, err)
	}
}

func TestHandleCheckUpdateReportsNewRelease(t *testing.T) {
	dir := t.TempDir()
	versionFile := filepath.Join(dir, ".version")
	if err := os.WriteFile(versionFile, []byte("v0.1.0\n2026-05-29 12:00:00\n"), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			http.Error(w, "missing user agent", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/nianzhibai/91/releases/tag/v0.2.0",
		})
	}))
	t.Cleanup(releaseServer.Close)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/update/check", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		VersionFilePath: versionFile,
		ReleaseAPIURL:   releaseServer.URL,
	}).handleCheckUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got updateCheckDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CurrentVersion != "v0.1.0" {
		t.Fatalf("currentVersion = %q, want v0.1.0", got.CurrentVersion)
	}
	if got.LatestVersion != "v0.2.0" {
		t.Fatalf("latestVersion = %q, want v0.2.0", got.LatestVersion)
	}
	if !got.HasUpdate {
		t.Fatalf("hasUpdate = false, want true")
	}
	if got.ReleaseURL == "" {
		t.Fatalf("releaseUrl is empty")
	}
}

func TestHandleCheckUpdateReportsUpToDate(t *testing.T) {
	dir := t.TempDir()
	versionFile := filepath.Join(dir, ".version")
	if err := os.WriteFile(versionFile, []byte("v0.2.0\n"), 0o644); err != nil {
		t.Fatalf("write version file: %v", err)
	}
	releaseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"tag_name": "v0.2.0",
			"html_url": "https://github.com/nianzhibai/91/releases/tag/v0.2.0",
		})
	}))
	t.Cleanup(releaseServer.Close)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/update/check", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		VersionFilePath: versionFile,
		ReleaseAPIURL:   releaseServer.URL,
	}).handleCheckUpdate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got updateCheckDTO
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.HasUpdate {
		t.Fatalf("hasUpdate = true, want false")
	}
}

func TestHandleUpsertDrivePreservesExistingCredentialsWhenRequestCredentialsEmpty(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:         "quark-main",
		Kind:       "quark",
		Name:       "Old name",
		RootID:     "0",
		ScanRootID: "0",
		Credentials: map[string]string{
			"cookie": "existing-cookie",
		},
		Status: "ok",
	}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives", strings.NewReader(`{
		"id": "quark-main",
		"kind": "quark",
		"name": "New name",
		"rootId": "0",
		"scanRootId": "scan-root",
		"credentials": {}
	}`))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleUpsertDrive(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetDrive(ctx, "quark-main")
	if err != nil {
		t.Fatalf("get drive: %v", err)
	}
	if got.Name != "New name" {
		t.Fatalf("name = %q, want New name", got.Name)
	}
	if got.ScanRootID != "scan-root" {
		t.Fatalf("scanRootId = %q, want scan-root", got.ScanRootID)
	}
	if got.Credentials["cookie"] != "existing-cookie" {
		t.Fatalf("cookie credential = %q, want existing-cookie", got.Credentials["cookie"])
	}
}

func TestHandleUpsertDrivePreservesExistingMinScanFileSizeWhenOmitted(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID: "p115-main", Kind: "p115", Name: "115", RootID: "0", Status: "ok",
		MinScanFileSizeBytes: 150 * 1024 * 1024,
		ScanDirIDs:           []string{"keep-dir"},
	}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives", strings.NewReader(`{
		"id": "p115-main",
		"kind": "p115",
		"name": "115 renamed",
		"rootId": "0",
		"scanRootId": "0",
		"credentials": {}
	}`))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleUpsertDrive(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetDrive(ctx, "p115-main")
	if err != nil {
		t.Fatalf("get drive: %v", err)
	}
	if got.MinScanFileSizeBytes != 150*1024*1024 {
		t.Fatalf("min scan size = %d, want 150MiB", got.MinScanFileSizeBytes)
	}
	if len(got.ScanDirIDs) != 1 || got.ScanDirIDs[0] != "keep-dir" {
		t.Fatalf("scan dir ids = %#v, want preserved scan dirs", got.ScanDirIDs)
	}
}

func TestHandleUpsertDriveReplacesExistingCredentialsWhenProvided(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:         "quark-main",
		Kind:       "quark",
		Name:       "Old name",
		RootID:     "0",
		ScanRootID: "0",
		Credentials: map[string]string{
			"cookie": "existing-cookie",
		},
		Status: "ok",
	}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives", bytes.NewBufferString(`{
		"id": "quark-main",
		"kind": "quark",
		"name": "New name",
		"rootId": "0",
		"scanRootId": "0",
		"credentials": {"cookie": "new-cookie"}
	}`))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleUpsertDrive(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetDrive(ctx, "quark-main")
	if err != nil {
		t.Fatalf("get drive: %v", err)
	}
	if got.Credentials["cookie"] != "new-cookie" {
		t.Fatalf("cookie credential = %q, want new-cookie", got.Credentials["cookie"])
	}
}

func TestHandleSetDriveScanFilter(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:         "p115-main",
		Kind:       "p115",
		Name:       "115",
		RootID:     "0",
		SkipDirIDs: []string{"legacy-skip"},
	}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/p115-main/scan-filter", strings.NewReader(`{"minFileSizeBytes":104857600,"skipFileNameKeywords":["广告"," telegram ","广告"]}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p115-main")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleSetDriveScanFilter(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetDrive(ctx, "p115-main")
	if err != nil {
		t.Fatalf("get drive: %v", err)
	}
	if got.MinScanFileSizeBytes != 104857600 {
		t.Fatalf("min scan size = %d, want 104857600", got.MinScanFileSizeBytes)
	}
	if len(got.SkipFileNameKeywords) != 2 || got.SkipFileNameKeywords[0] != "广告" || got.SkipFileNameKeywords[1] != "telegram" {
		t.Fatalf("skip keywords = %#v, want cleaned keywords", got.SkipFileNameKeywords)
	}
	var body struct {
		MinFileSizeBytes     int64    `json:"minFileSizeBytes"`
		SkipFileNameKeywords []string `json:"skipFileNameKeywords"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.MinFileSizeBytes != 104857600 || len(body.SkipFileNameKeywords) != 2 {
		t.Fatalf("response = %#v, want filter values", body)
	}
}

func TestHandleSetDriveScanDirs(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.UpsertDrive(ctx, &catalog.Drive{ID: "p115-main", Kind: "p115", Name: "115", RootID: "0"}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/p115-main/scan-dirs", strings.NewReader(`{"dirIds":[" dir-a ","dir-b","dir-a",""]}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p115-main")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleSetDriveScanDirs(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetDrive(ctx, "p115-main")
	if err != nil {
		t.Fatalf("get drive: %v", err)
	}
	if len(got.ScanDirIDs) != 2 || got.ScanDirIDs[0] != "dir-a" || got.ScanDirIDs[1] != "dir-b" {
		t.Fatalf("scan dir ids = %#v, want cleaned scan dirs", got.ScanDirIDs)
	}
	if len(got.SkipDirIDs) != 0 {
		t.Fatalf("legacy skip dir ids = %#v, want cleared when saving scan dirs", got.SkipDirIDs)
	}
	var body struct {
		ScanDirIDs []string `json:"scanDirIds"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.ScanDirIDs) != 2 || body.ScanDirIDs[0] != "dir-a" || body.ScanDirIDs[1] != "dir-b" {
		t.Fatalf("response = %#v, want scan dir ids", body)
	}
}

func TestHandleSetSpider91Sources(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.UpsertDrive(ctx, &catalog.Drive{
		ID:     "91Spider",
		Kind:   "spider91",
		Name:   "91",
		RootID: "/",
		Credentials: map[string]string{
			"last_crawl_at": "1780212269",
		},
		Status:        "ok",
		TeaserEnabled: true,
	}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/91Spider/spider91-sources", strings.NewReader(`{
		"sources": [
			{"url":"https://www.91porn.com/v.php?category=top&viewtype=basic","targetNew":15},
			{"url":"https://91porn.com/v.php?category=mf&viewtype=basic","targetNew":50}
		]
	}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "91Spider")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleSetSpider91Sources(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := cat.GetDrive(ctx, "91Spider")
	if err != nil {
		t.Fatalf("get drive: %v", err)
	}
	if got.Credentials["last_crawl_at"] != "1780212269" {
		t.Fatalf("last_crawl_at = %q, want preserved", got.Credentials["last_crawl_at"])
	}
	if got.Credentials["target_new"] != "65" {
		t.Fatalf("target_new = %q, want 65", got.Credentials["target_new"])
	}
	var stored []spider91SourceConfig
	if err := json.Unmarshal([]byte(got.Credentials[spider91ListSourcesCredentialKey]), &stored); err != nil {
		t.Fatalf("stored sources json: %v", err)
	}
	if len(stored) != 2 || stored[0].TargetNew != 15 || stored[1].TargetNew != 50 {
		t.Fatalf("stored sources = %#v, want top=15 mf=50", stored)
	}
	var body struct {
		Sources   []spider91SourceConfig `json:"sources"`
		TargetNew int                    `json:"targetNew"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TargetNew != 65 || len(body.Sources) != 2 {
		t.Fatalf("response = %#v, want target 65 and 2 sources", body)
	}
}

func TestHandleSetSpider91SourcesRejectsNonSpider91Drive(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})
	if err := cat.UpsertDrive(ctx, &catalog.Drive{ID: "p115-main", Kind: "p115", Name: "115", RootID: "0"}); err != nil {
		t.Fatalf("seed drive: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/p115-main/spider91-sources", strings.NewReader(`{"sources":[{"url":"https://91porn.com/v.php?category=mf&viewtype=basic","targetNew":50}]}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p115-main")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	(&AdminServer{Catalog: cat}).handleSetSpider91Sources(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDriveCleanupPreview(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/admin/api/drives/p115-main/cleanup-preview", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p115-main")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()

	called := false
	(&AdminServer{
		GetDriveCleanupPreview: func(ctx context.Context, driveID string) (DriveCleanupPreview, error) {
			called = true
			if driveID != "p115-main" {
				t.Fatalf("driveID = %q, want p115-main", driveID)
			}
			return DriveCleanupPreview{
				DriveID:       driveID,
				Scanned:       3,
				FullDriveScan: true,
				SafeToClean:   true,
				Total:         1,
				Items: []DriveCleanupPreviewItem{{
					ID:       "video-1",
					Title:    "Ad",
					FileName: "广告.mp4",
					FileID:   "file-1",
					Reason:   "filename_keyword",
				}},
			}, nil
		},
	}).handleDriveCleanupPreview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatalf("preview callback was not called")
	}
	var body DriveCleanupPreview
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].Reason != "filename_keyword" {
		t.Fatalf("response = %#v, want cleanup preview item", body)
	}
}

func TestHandleListDrivesIncludesTeaserCounts(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	for _, d := range []*catalog.Drive{
		{ID: "OneDrive", Kind: "onedrive", Name: "OneDrive", RootID: "root", Status: "ok"},
		{ID: "PikPak", Kind: "pikpak", Name: "PikPak", RootID: "", Status: "ok"},
	} {
		if err := cat.UpsertDrive(ctx, d); err != nil {
			t.Fatalf("seed drive %s: %v", d.ID, err)
		}
	}

	now := time.Now()
	videos := []*catalog.Video{
		{ID: "od-ready-1", DriveID: "OneDrive", FileID: "od-file-1", Title: "OD Ready 1", ThumbnailURL: "/p/thumb/od-ready-1", PreviewStatus: "ready", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "od-ready-2", DriveID: "OneDrive", FileID: "od-file-2", Title: "OD Ready 2", PreviewStatus: "ready", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "od-pending", DriveID: "OneDrive", FileID: "od-file-3", Title: "OD Pending", PreviewStatus: "pending", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "od-skipped", DriveID: "OneDrive", FileID: "od-file-4", Title: "OD Skipped", PreviewStatus: "skipped", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "pp-pending", DriveID: "PikPak", FileID: "pp-file-1", Title: "PP Pending", PreviewStatus: "pending", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "pp-failed", DriveID: "PikPak", FileID: "pp-file-2", Title: "PP Failed", ThumbnailURL: "/p/thumb/pp-failed", PreviewStatus: "failed", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
	}
	for _, v := range videos {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}
	if err := cat.UpdateVideoMeta(ctx, "od-ready-2", catalog.VideoMetaPatch{ThumbnailStatus: "failed"}); err != nil {
		t.Fatalf("mark thumbnail failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/drives", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{
		Catalog: cat,
		GetDriveGenerationStatuses: func() map[string]DriveGenerationStatuses {
			return map[string]DriveGenerationStatuses{
				"OneDrive": {
					Thumbnail: GenerationStatus{State: "cooling", QueueLength: 3, CooldownUntil: "2026-05-16T21:00:00+08:00"},
					Preview:   GenerationStatus{State: "generating", CurrentTitle: "OD Pending"},
				},
			}
		},
	}).handleListDrives(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got []struct {
		ID                        string           `json:"id"`
		ThumbnailGenerationStatus GenerationStatus `json:"thumbnailGenerationStatus"`
		PreviewGenerationStatus   GenerationStatus `json:"previewGenerationStatus"`
		ThumbnailReadyCount       int              `json:"thumbnailReadyCount"`
		ThumbnailPendingCount     int              `json:"thumbnailPendingCount"`
		ThumbnailFailedCount      int              `json:"thumbnailFailedCount"`
		TeaserReadyCount          int              `json:"teaserReadyCount"`
		TeaserPendingCount        int              `json:"teaserPendingCount"`
		TeaserFailedCount         int              `json:"teaserFailedCount"`
		TeaserSkippedCount        int              `json:"teaserSkippedCount"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byID := map[string]struct {
		TeaserReady      int
		TeaserPending    int
		TeaserFailed     int
		TeaserSkipped    int
		ThumbnailReady   int
		ThumbnailPending int
		ThumbnailFailed  int
		Thumbnail        GenerationStatus
		Preview          GenerationStatus
	}{}
	for _, d := range got {
		byID[d.ID] = struct {
			TeaserReady      int
			TeaserPending    int
			TeaserFailed     int
			TeaserSkipped    int
			ThumbnailReady   int
			ThumbnailPending int
			ThumbnailFailed  int
			Thumbnail        GenerationStatus
			Preview          GenerationStatus
		}{
			TeaserReady:      d.TeaserReadyCount,
			TeaserPending:    d.TeaserPendingCount,
			TeaserFailed:     d.TeaserFailedCount,
			TeaserSkipped:    d.TeaserSkippedCount,
			ThumbnailReady:   d.ThumbnailReadyCount,
			ThumbnailPending: d.ThumbnailPendingCount,
			ThumbnailFailed:  d.ThumbnailFailedCount,
			Thumbnail:        d.ThumbnailGenerationStatus,
			Preview:          d.PreviewGenerationStatus,
		}
	}
	if byID["OneDrive"].TeaserReady != 2 || byID["OneDrive"].TeaserPending != 1 || byID["OneDrive"].TeaserFailed != 0 || byID["OneDrive"].TeaserSkipped != 1 {
		t.Fatalf("OneDrive counts = %#v, want ready=2 pending=1 failed=0 skipped=1", byID["OneDrive"])
	}
	if byID["OneDrive"].ThumbnailReady != 1 || byID["OneDrive"].ThumbnailPending != 2 || byID["OneDrive"].ThumbnailFailed != 1 {
		t.Fatalf("OneDrive thumbnail counts = %#v, want ready=1 pending=2 failed=1", byID["OneDrive"])
	}
	if byID["OneDrive"].Thumbnail.State != "cooling" || byID["OneDrive"].Preview.State != "generating" {
		t.Fatalf("OneDrive generation statuses = %#v, want thumbnail cooling and preview generating", byID["OneDrive"])
	}
	if byID["PikPak"].TeaserReady != 0 || byID["PikPak"].TeaserPending != 1 || byID["PikPak"].TeaserFailed != 1 {
		t.Fatalf("PikPak counts = %#v, want ready=0 pending=1 failed=1", byID["PikPak"])
	}
	if byID["PikPak"].ThumbnailReady != 1 || byID["PikPak"].ThumbnailPending != 1 || byID["PikPak"].ThumbnailFailed != 0 {
		t.Fatalf("PikPak thumbnail counts = %#v, want ready=1 pending=1 failed=0", byID["PikPak"])
	}
	if byID["PikPak"].Thumbnail.State != "idle" || byID["PikPak"].Preview.State != "idle" {
		t.Fatalf("PikPak generation statuses = %#v, want idle defaults", byID["PikPak"])
	}
}

func TestHandleDriveStorageReportsLocalMediaUsage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cat, err := catalog.Open(filepath.Join(root, "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	localDir := filepath.Join(root, "previews")
	thumbDir := filepath.Join(localDir, "thumbs")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatalf("mkdir thumbs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "drive-one-video.mp4"), []byte("teaser-one"), 0o644); err != nil {
		t.Fatalf("write teaser one: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "drive-two-video.mp4"), []byte("teaser-two!!"), 0o644); err != nil {
		t.Fatalf("write teaser two: %v", err)
	}
	if err := os.WriteFile(filepath.Join(thumbDir, "drive-one-video.jpg"), []byte("jpg-one"), 0o644); err != nil {
		t.Fatalf("write thumb one: %v", err)
	}
	if err := os.WriteFile(filepath.Join(thumbDir, "drive-two-video.jpg"), []byte("jpg-two!!"), 0o644); err != nil {
		t.Fatalf("write thumb two: %v", err)
	}

	for _, d := range []*catalog.Drive{
		{ID: "drive-one", Kind: "onedrive", Name: "Drive One", RootID: "root", Status: "ok"},
		{ID: "drive-two", Kind: "pikpak", Name: "Drive Two", RootID: "", Status: "ok"},
	} {
		if err := cat.UpsertDrive(ctx, d); err != nil {
			t.Fatalf("seed drive %s: %v", d.ID, err)
		}
	}
	now := time.Now()
	for _, v := range []*catalog.Video{
		{
			ID:            "drive-one-video",
			DriveID:       "drive-one",
			FileID:        "file-one",
			Title:         "Video One",
			PreviewLocal:  filepath.Join(localDir, "drive-one-video.mp4"),
			PreviewStatus: "ready",
			PublishedAt:   now,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "drive-two-video",
			DriveID:       "drive-two",
			FileID:        "file-two",
			Title:         "Video Two",
			PreviewLocal:  filepath.Join(localDir, "drive-two-video.mp4"),
			PreviewStatus: "ready",
			PublishedAt:   now,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/drives/storage", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat, LocalPreviewDir: localDir}).handleDriveStorage(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		ThumbnailBytes int64 `json:"thumbnailBytes"`
		TeaserBytes    int64 `json:"teaserBytes"`
		TotalBytes     int64 `json:"totalBytes"`
		AvailableBytes int64 `json:"availableBytes"`
		Drives         map[string]struct {
			ThumbnailBytes int64 `json:"thumbnailBytes"`
			TeaserBytes    int64 `json:"teaserBytes"`
			TotalBytes     int64 `json:"totalBytes"`
		} `json:"drives"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ThumbnailBytes != int64(len("jpg-one")+len("jpg-two!!")) {
		t.Fatalf("thumbnail bytes = %d, want %d", got.ThumbnailBytes, len("jpg-one")+len("jpg-two!!"))
	}
	if got.TeaserBytes != int64(len("teaser-one")+len("teaser-two!!")) {
		t.Fatalf("teaser bytes = %d, want %d", got.TeaserBytes, len("teaser-one")+len("teaser-two!!"))
	}
	if got.TotalBytes != got.ThumbnailBytes+got.TeaserBytes {
		t.Fatalf("total bytes = %d, want thumbnail + teaser", got.TotalBytes)
	}
	if got.AvailableBytes <= 0 {
		t.Fatalf("available bytes = %d, want positive", got.AvailableBytes)
	}
	if got.Drives["drive-one"].ThumbnailBytes != int64(len("jpg-one")) ||
		got.Drives["drive-one"].TeaserBytes != int64(len("teaser-one")) {
		t.Fatalf("drive-one usage = %#v", got.Drives["drive-one"])
	}
	if got.Drives["drive-two"].TotalBytes != int64(len("jpg-two!!")+len("teaser-two!!")) {
		t.Fatalf("drive-two total = %d, want %d", got.Drives["drive-two"].TotalBytes, len("jpg-two!!")+len("teaser-two!!"))
	}
}

func TestHandleCreateTagClassifiesExistingVideos(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	if err := cat.UpsertVideo(ctx, &catalog.Video{
		ID:          "video-1",
		DriveID:     "drive",
		FileID:      "file-1",
		Title:       "清纯短发",
		PublishedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("seed video: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/tags", strings.NewReader(`{"label":"清纯"}`))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleCreateTag(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Label      string `json:"label"`
		Classified int    `json:"classified"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Label != "清纯" || got.Classified != 1 {
		t.Fatalf("response = %#v, want 清纯 classified 1", got)
	}

	video, err := cat.GetVideo(ctx, "video-1")
	if err != nil {
		t.Fatalf("get video: %v", err)
	}
	if len(video.Tags) != 1 || video.Tags[0] != "清纯" {
		t.Fatalf("video tags = %#v, want 清纯", video.Tags)
	}
}

func TestHandleAdminListVideosFiltersByDriveID(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	videos := []*catalog.Video{
		{
			ID:          "od-video",
			DriveID:     "OneDrive",
			FileID:      "od-file",
			Title:       "OneDrive video",
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "pp-video",
			DriveID:     "PikPak",
			FileID:      "pp-file",
			Title:       "PikPak video",
			PublishedAt: now.Add(-time.Hour),
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	for _, v := range videos {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos?driveId=OneDrive", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 {
		t.Fatalf("response total/items = %d/%d, want 1/1: %#v", got.Total, len(got.Items), got.Items)
	}
	if got.Items[0].DriveID != "OneDrive" || got.Items[0].ID != "od-video" {
		t.Fatalf("item = %#v, want OneDrive od-video", got.Items[0])
	}
}

func TestHandleAdminListVideosFiltersByKeywordAndTag(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	if _, err := cat.CreateTagAndClassify(ctx, "精选", nil, "user"); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	now := time.Now()
	for _, v := range []*catalog.Video{
		{
			ID:          "match-video",
			DriveID:     "OneDrive",
			FileID:      "match-file",
			Title:       "alpha title",
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "tag-only-video",
			DriveID:     "OneDrive",
			FileID:      "tag-only-file",
			Title:       "beta title",
			PublishedAt: now.Add(-time.Hour),
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
		if err := cat.SetManualVideoTags(ctx, v.ID, []string{"精选"}); err != nil {
			t.Fatalf("seed tags for %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos?keyword=alpha&tag=%E7%B2%BE%E9%80%89", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != "match-video" {
		t.Fatalf("response = total:%d items:%#v, want only match-video", got.Total, got.Items)
	}
}

func TestHandleAdminListVideosPaginates(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	now := time.Now()
	for i, title := range []string{"first", "second", "third"} {
		v := &catalog.Video{
			ID:          title,
			DriveID:     "OneDrive",
			FileID:      title + "-file",
			Title:       title,
			PublishedAt: now.Add(-time.Duration(i) * time.Hour),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed video %s: %v", v.ID, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/videos?driveId=OneDrive&page=2&size=2", nil)
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleAdminListVideos(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Items []catalog.Video `json:"items"`
		Total int             `json:"total"`
		Page  int             `json:"page"`
		Size  int             `json:"size"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Total != 3 || got.Page != 2 || got.Size != 2 {
		t.Fatalf("pagination meta = total:%d page:%d size:%d, want 3/2/2", got.Total, got.Page, got.Size)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "third" {
		t.Fatalf("items = %#v, want only third", got.Items)
	}
}

func TestHandleBulkVideoTagsAddsRemovesAndReplaces(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	for _, label := range []string{"整理", "精选"} {
		if _, err := cat.CreateTagAndClassify(ctx, label, nil, "user"); err != nil {
			t.Fatalf("seed tag %s: %v", label, err)
		}
	}
	now := time.Now()
	for _, id := range []string{"video-a", "video-b"} {
		if err := cat.UpsertVideo(ctx, &catalog.Video{
			ID:          id,
			DriveID:     "OneDrive",
			FileID:      id + "-file",
			Title:       id,
			PublishedAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("seed video %s: %v", id, err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/videos/bulk-tags", strings.NewReader(`{"videoIds":["video-a","video-b"],"tags":["整理"],"mode":"add"}`))
	rr := httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleBulkVideoTags(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("add status = %d, body = %s", rr.Code, rr.Body.String())
	}
	for _, id := range []string{"video-a", "video-b"} {
		video, err := cat.GetVideo(ctx, id)
		if err != nil {
			t.Fatalf("get video %s: %v", id, err)
		}
		if !hasTag(video.Tags, "整理") {
			t.Fatalf("%s tags = %#v, want 整理", id, video.Tags)
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/videos/bulk-tags", strings.NewReader(`{"videoIds":["video-a"],"tags":["整理"],"mode":"remove"}`))
	rr = httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleBulkVideoTags(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("remove status = %d, body = %s", rr.Code, rr.Body.String())
	}
	videoA, err := cat.GetVideo(ctx, "video-a")
	if err != nil {
		t.Fatalf("get video-a: %v", err)
	}
	if hasTag(videoA.Tags, "整理") {
		t.Fatalf("video-a tags = %#v, want 整理 removed", videoA.Tags)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/api/videos/bulk-tags", strings.NewReader(`{"videoIds":["video-b"],"tags":["精选"],"mode":"replace"}`))
	rr = httptest.NewRecorder()
	(&AdminServer{Catalog: cat}).handleBulkVideoTags(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("replace status = %d, body = %s", rr.Code, rr.Body.String())
	}
	videoB, err := cat.GetVideo(ctx, "video-b")
	if err != nil {
		t.Fatalf("get video-b: %v", err)
	}
	if len(videoB.Tags) != 1 || videoB.Tags[0] != "精选" {
		t.Fatalf("video-b tags = %#v, want only 精选", videoB.Tags)
	}
}

func TestHandleRegenAllPreviewsInvokesHook(t *testing.T) {
	called := false
	server := &AdminServer{
		OnRegenAllPreviews: func() {
			called = true
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/videos/regen-preview", nil)
	rr := httptest.NewRecorder()
	server.handleRegenAllPreviews(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("regen all previews hook was not called")
	}
}

func TestHandleRegenFailedPreviewsInvokesHookWithDriveID(t *testing.T) {
	calledWith := ""
	server := &AdminServer{
		OnRegenFailedPreviews: func(driveID string) {
			calledWith = driveID
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/PikPak/previews/failed/regenerate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "PikPak")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	server.handleRegenFailedPreviews(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if calledWith != "PikPak" {
		t.Fatalf("hook called with %q, want PikPak", calledWith)
	}
}

func TestHandleRegenFailedFingerprintsInvokesHookWithDriveID(t *testing.T) {
	calledWith := ""
	server := &AdminServer{
		OnRegenFailedFingerprints: func(driveID string) {
			calledWith = driveID
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/drives/PikPak/fingerprints/failed/regenerate", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "PikPak")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rr := httptest.NewRecorder()
	server.handleRegenFailedFingerprints(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if calledWith != "PikPak" {
		t.Fatalf("hook called with %q, want PikPak", calledWith)
	}
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}
