package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives"
)

func TestImportManagerIngestToDriveUsesScannerVideoIDAndQueuesUpload(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	path := filepath.Join(t.TempDir(), "source clip.mp4")
	if err := os.WriteFile(path, []byte("video data"), 0o644); err != nil {
		t.Fatalf("write source video: %v", err)
	}

	drv := &apiImportFakeDrive{
		id:     "gdrive",
		kind:   "googledrive",
		rootID: "root",
		fileID: "google-file-1",
	}
	queuedID := ""
	server := &Server{
		Catalog: cat,
		OnVideoUploaded: func(v *catalog.Video) {
			queuedID = v.ID
		},
	}
	manager := NewImportManager(server, ImportManagerConfig{})

	video, err := manager.ingestToDrive(ctx, drv, path, "Uploaded Title", []string{"AV"})
	if err != nil {
		t.Fatalf("ingest to drive: %v", err)
	}

	wantID := "googledrive-gdrive-google-file-1"
	if video.ID != wantID {
		t.Fatalf("video id = %q, want scanner-compatible id %q", video.ID, wantID)
	}
	if queuedID != wantID {
		t.Fatalf("queued video id = %q, want %q", queuedID, wantID)
	}
	if drv.uploadParent != "root" {
		t.Fatalf("upload parent = %q, want root", drv.uploadParent)
	}
	if drv.uploadName != "source clip.mp4" {
		t.Fatalf("upload name = %q, want source clip.mp4", drv.uploadName)
	}
	got, err := cat.GetVideo(ctx, wantID)
	if err != nil {
		t.Fatalf("get catalog video: %v", err)
	}
	if got.DriveID != "gdrive" || got.FileID != "google-file-1" || got.ParentID != "root" {
		t.Fatalf("catalog video drive/file/parent = %q/%q/%q", got.DriveID, got.FileID, got.ParentID)
	}
	if got.PreviewStatus != "pending" {
		t.Fatalf("preview status = %q, want pending", got.PreviewStatus)
	}
	if got.FileName != "source clip.mp4" {
		t.Fatalf("catalog file name = %q, want source clip.mp4", got.FileName)
	}
}

func TestHandleExternalUploadUsesDefaultDriveAndTriggersScan(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	drv := &apiImportFakeDrive{
		id:     "gdrive",
		kind:   "googledrive",
		rootID: "root",
		fileID: "google-file-2",
	}
	queuedID := ""
	scannedDriveID := ""
	server := &Server{
		Catalog:             cat,
		ExternalImportToken: "secret",
		OnVideoUploaded: func(v *catalog.Video) {
			queuedID = v.ID
		},
	}
	server.Importer = NewImportManager(server, ImportManagerConfig{
		DefaultUploadDrive: func() drives.Drive {
			return drv
		},
		OnDriveUploadComplete: func(driveID string) {
			scannedDriveID = driveID
		},
	})

	req := multipartUploadRequest(t, map[string]string{
		"title": "Telegram Upload",
		"tags":  "AV",
	}, "telegram.mp4", "telegram video data")
	req = req.WithContext(ctx)
	req.Header.Set("X-Import-Token", "secret")
	rr := httptest.NewRecorder()

	server.handleExternalUpload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	wantID := "googledrive-gdrive-google-file-2"
	if queuedID != wantID {
		t.Fatalf("queued video id = %q, want %q", queuedID, wantID)
	}
	if scannedDriveID != "gdrive" {
		t.Fatalf("scan drive id = %q, want gdrive", scannedDriveID)
	}
	got, err := cat.GetVideo(ctx, wantID)
	if err != nil {
		t.Fatalf("get catalog video: %v", err)
	}
	if got.Author != "Telegram导入" {
		t.Fatalf("author = %q, want Telegram导入", got.Author)
	}
	if got.Title != "Telegram Upload" {
		t.Fatalf("title = %q, want form title", got.Title)
	}
	if got.FileName != "telegram.mp4" {
		t.Fatalf("catalog file name = %q, want telegram.mp4", got.FileName)
	}
	if drv.uploadName != "telegram.mp4" {
		t.Fatalf("upload name = %q, want telegram.mp4", drv.uploadName)
	}
}

func TestHandleUploadVideoUsesDefaultDriveAndTriggersScan(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	drv := &apiImportFakeDrive{
		id:     "gdrive",
		kind:   "googledrive",
		rootID: "root",
		fileID: "google-file-3",
	}
	queuedID := ""
	scannedDriveID := ""
	server := &Server{
		Catalog: cat,
		OnVideoUploaded: func(v *catalog.Video) {
			queuedID = v.ID
		},
	}
	server.Importer = NewImportManager(server, ImportManagerConfig{
		DefaultUploadDrive: func() drives.Drive {
			return drv
		},
		OnDriveUploadComplete: func(driveID string) {
			scannedDriveID = driveID
		},
	})

	req := multipartUploadRequest(t, map[string]string{
		"title": "Browser Upload",
		"tags":  "AV",
	}, "browser.mp4", "browser video data")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleUploadVideo(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	wantID := "googledrive-gdrive-google-file-3"
	if queuedID != wantID {
		t.Fatalf("queued video id = %q, want %q", queuedID, wantID)
	}
	if scannedDriveID != "gdrive" {
		t.Fatalf("scan drive id = %q, want gdrive", scannedDriveID)
	}
	got, err := cat.GetVideo(ctx, wantID)
	if err != nil {
		t.Fatalf("get catalog video: %v", err)
	}
	if got.Author != "用户上传" {
		t.Fatalf("author = %q, want 用户上传", got.Author)
	}
	if got.Title != "Browser Upload" {
		t.Fatalf("title = %q, want form title", got.Title)
	}
	if got.DriveID != "gdrive" || got.FileID != "google-file-3" {
		t.Fatalf("catalog video drive/file = %q/%q", got.DriveID, got.FileID)
	}
	if got.FileName != "browser.mp4" {
		t.Fatalf("catalog file name = %q, want browser.mp4", got.FileName)
	}
	if drv.uploadName != "browser.mp4" {
		t.Fatalf("upload name = %q, want browser.mp4", drv.uploadName)
	}
}

func TestHandleExternalUploadAddsNumberForDuplicateDriveName(t *testing.T) {
	ctx := context.Background()
	cat, err := catalog.Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() {
		if err := cat.Close(); err != nil {
			t.Fatalf("close catalog: %v", err)
		}
	})

	drv := &apiImportFakeDrive{
		id:     "gdrive",
		kind:   "googledrive",
		rootID: "root",
		fileID: "google-file-4",
		entries: []drives.Entry{
			{Name: "telegram.mp4"},
			{Name: "telegram-2.mp4"},
		},
	}
	server := &Server{
		Catalog:             cat,
		ExternalImportToken: "secret",
	}
	server.Importer = NewImportManager(server, ImportManagerConfig{
		DefaultUploadDrive: func() drives.Drive {
			return drv
		},
	})

	req := multipartUploadRequest(t, map[string]string{
		"title": "Telegram",
	}, "telegram.mp4", "telegram video data")
	req = req.WithContext(ctx)
	req.Header.Set("X-Import-Token", "secret")
	rr := httptest.NewRecorder()

	server.handleExternalUpload(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if drv.uploadName != "telegram-3.mp4" {
		t.Fatalf("upload name = %q, want telegram-3.mp4", drv.uploadName)
	}
	got, err := cat.GetVideo(ctx, "googledrive-gdrive-google-file-4")
	if err != nil {
		t.Fatalf("get catalog video: %v", err)
	}
	if got.FileName != "telegram-3.mp4" {
		t.Fatalf("catalog file name = %q, want telegram-3.mp4", got.FileName)
	}
}

type apiImportFakeDrive struct {
	id      string
	kind    string
	rootID  string
	fileID  string
	entries []drives.Entry

	uploadParent string
	uploadName   string
	uploadSize   int64
}

func (d *apiImportFakeDrive) Kind() string { return d.kind }
func (d *apiImportFakeDrive) ID() string   { return d.id }
func (d *apiImportFakeDrive) Init(context.Context) error {
	return nil
}
func (d *apiImportFakeDrive) List(context.Context, string) ([]drives.Entry, error) {
	return d.entries, nil
}
func (d *apiImportFakeDrive) Stat(context.Context, string) (*drives.Entry, error) {
	return nil, drives.ErrNotSupported
}
func (d *apiImportFakeDrive) StreamURL(context.Context, string) (*drives.StreamLink, error) {
	return nil, drives.ErrNotSupported
}
func (d *apiImportFakeDrive) Upload(_ context.Context, parentID, name string, r io.Reader, size int64) (string, error) {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return "", err
	}
	d.uploadParent = parentID
	d.uploadName = name
	d.uploadSize = size
	return d.fileID, nil
}
func (d *apiImportFakeDrive) EnsureDir(context.Context, string) (string, error) {
	return "", drives.ErrNotSupported
}
func (d *apiImportFakeDrive) RootID() string { return d.rootID }
