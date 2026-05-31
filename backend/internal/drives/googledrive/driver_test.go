package googledrive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/video-site/backend/internal/drives"
)

func TestInitRefreshesTokenAndPersistsUpdate(t *testing.T) {
	var tokenRequestSeen bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/token" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		tokenRequestSeen = true
		want := map[string]string{
			"client_id":     "client-id",
			"client_secret": "client-secret",
			"refresh_token": "old-refresh",
			"grant_type":    "refresh_token",
		}
		for key, value := range want {
			if got := r.Form.Get(key); got != value {
				t.Fatalf("form %s = %q, want %q", key, got, value)
			}
		}
		writeJSON(t, w, map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	var persistedAccess, persistedRefresh string
	d := New(Config{
		ID:           "gd-main",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RefreshToken: "old-refresh",
		TokenURL:     srv.URL + "/token",
		APIBaseURL:   srv.URL,
		OnTokenUpdate: func(access, refresh string) {
			persistedAccess = access
			persistedRefresh = refresh
		},
	})

	if d.Kind() != Kind {
		t.Fatalf("kind = %q, want %s", d.Kind(), Kind)
	}
	if d.ID() != "gd-main" {
		t.Fatalf("id = %q, want gd-main", d.ID())
	}
	if d.RootID() != "root" {
		t.Fatalf("root id = %q, want root", d.RootID())
	}
	if err := d.Init(context.Background()); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !tokenRequestSeen {
		t.Fatal("token endpoint was not called")
	}
	if persistedAccess != "new-access" || persistedRefresh != "new-refresh" {
		t.Fatalf("persisted tokens = %q/%q, want new-access/new-refresh", persistedAccess, persistedRefresh)
	}
	if time.Until(d.accessTokenExpiresAt) <= 0 {
		t.Fatalf("access token expiry was not recorded: %v", d.accessTokenExpiresAt)
	}
}

func TestListFollowsPaginationAndMapsEntries(t *testing.T) {
	var listCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		if r.Method != http.MethodGet || r.URL.Path != "/files" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("q"); got != "'root' in parents and trashed=false" {
			t.Fatalf("q = %q, want root children query", got)
		}
		if got := r.URL.Query().Get("pageSize"); got != "1000" {
			t.Fatalf("pageSize = %q, want 1000", got)
		}
		if got := r.URL.Query().Get("supportsAllDrives"); got != "true" {
			t.Fatalf("supportsAllDrives = %q, want true", got)
		}
		if got := r.URL.Query().Get("includeItemsFromAllDrives"); got != "true" {
			t.Fatalf("includeItemsFromAllDrives = %q, want true", got)
		}
		if got := r.URL.Query().Get("fields"); !strings.Contains(got, "shortcutDetails(targetId,targetMimeType)") {
			t.Fatalf("fields = %q, want shortcutDetails", got)
		}

		listCalls++
		if r.URL.Query().Get("pageToken") == "" {
			writeJSON(t, w, map[string]any{
				"files": []map[string]any{
					{
						"id":           "folder-id",
						"name":         "Movies",
						"mimeType":     googleFolderMIME,
						"modifiedTime": "2026-05-10T12:30:00Z",
						"parents":      []string{"root"},
					},
					{
						"id":            "file-id",
						"name":          "demo.mp4",
						"mimeType":      "video/mp4",
						"size":          "12345",
						"md5Checksum":   "md5-demo",
						"modifiedTime":  "2026-05-10T13:30:00Z",
						"parents":       []string{"root"},
						"thumbnailLink": "https://thumb.example/demo.jpg",
					},
				},
				"nextPageToken": "next-page",
			})
			return
		}
		if got := r.URL.Query().Get("pageToken"); got != "next-page" {
			t.Fatalf("pageToken = %q, want next-page", got)
		}
		writeJSON(t, w, map[string]any{
			"files": []map[string]any{
				{
					"id":      "file-2",
					"name":    "second.mkv",
					"size":    "77",
					"parents": []string{"root"},
				},
			},
		})
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)

	got, err := d.List(context.Background(), "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2", listCalls)
	}
	if len(got) != 3 {
		t.Fatalf("entries len = %d, want 3", len(got))
	}
	if !got[0].IsDir || got[0].ID != "folder-id" || got[0].ParentID != "root" {
		t.Fatalf("folder entry = %#v", got[0])
	}
	if got[1].IsDir || got[1].MimeType != "video/mp4" || got[1].Size != 12345 || got[1].Hash != "md5-demo" || got[1].ThumbnailURL != "https://thumb.example/demo.jpg" {
		t.Fatalf("file entry = %#v", got[1])
	}
	if got[1].ModTime.IsZero() {
		t.Fatal("file mod time should be parsed")
	}
	if got[2].Name != "second.mkv" || got[2].Size != 77 || got[2].MimeType != "video/x-matroska" {
		t.Fatalf("paginated entry = %#v", got[2])
	}
}

func TestListMapsFolderShortcutsAsDirectories(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if r.URL.Path == "/files/target-file-id" {
			writeJSON(t, w, map[string]any{
				"id":            "target-file-id",
				"name":          "target.mp4",
				"mimeType":      "video/mp4",
				"size":          "222",
				"sha1Checksum":  "sha1-target",
				"thumbnailLink": "https://thumb.example/target.jpg",
			})
			return
		}
		if r.URL.Path != "/files" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		writeJSON(t, w, map[string]any{
			"files": []map[string]any{
				{
					"id":       "shortcut-id",
					"name":     "Shortcut Movies",
					"mimeType": googleShortcutMIME,
					"parents":  []string{"root"},
					"shortcutDetails": map[string]any{
						"targetId":       "target-folder-id",
						"targetMimeType": googleFolderMIME,
					},
				},
				{
					"id":       "file-shortcut-id",
					"name":     "Movie shortcut",
					"mimeType": googleShortcutMIME,
					"parents":  []string{"root"},
					"shortcutDetails": map[string]any{
						"targetId":       "target-file-id",
						"targetMimeType": "video/mp4",
					},
				},
			},
		})
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)

	got, err := d.List(context.Background(), "root")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("entries len = %d, want 2", len(got))
	}
	if !got[0].IsDir || got[0].ID != "target-folder-id" || got[0].Name != "Shortcut Movies" || got[0].ParentID != "root" || got[0].MimeType != googleFolderMIME {
		t.Fatalf("folder shortcut entry = %#v", got[0])
	}
	if got[1].IsDir || got[1].ID != "target-file-id" || got[1].Size != 222 || got[1].Hash != "sha1-target" || got[1].ThumbnailURL != "https://thumb.example/target.jpg" {
		t.Fatalf("file shortcut entry = %#v", got[1])
	}
}

func TestStatAndStreamURLUseDriveAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		if r.Method != http.MethodGet || r.URL.Path != "/files/file-id" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("supportsAllDrives"); got != "true" {
			t.Fatalf("supportsAllDrives = %q, want true", got)
		}
		writeJSON(t, w, map[string]any{
			"id":          "file-id",
			"name":        "movie.mov",
			"mimeType":    "video/quicktime",
			"size":        "2048",
			"md5Checksum": "md5-movie",
			"parents":     []string{"parent-id"},
		})
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)

	entry, err := d.Stat(context.Background(), "file-id")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if entry.ID != "file-id" || entry.Name != "movie.mov" || entry.ParentID != "parent-id" || entry.Hash != "md5-movie" {
		t.Fatalf("entry = %#v", entry)
	}

	link, err := d.StreamURL(context.Background(), "file-id")
	if err != nil {
		t.Fatalf("stream url: %v", err)
	}
	wantURL := srv.URL + "/files/file-id?alt=media&supportsAllDrives=true"
	if link.URL != wantURL {
		t.Fatalf("stream url = %q, want %q", link.URL, wantURL)
	}
	if got := link.Headers.Get("Authorization"); got != "Bearer access-token" {
		t.Fatalf("stream authorization = %q, want bearer token", got)
	}
}

func TestRequestRefreshesAfterUnauthorized(t *testing.T) {
	var statCalls, tokenRefreshes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/files/file-id":
			statCalls++
			if statCalls == 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    401,
						"message": "Invalid Credentials",
						"status":  "UNAUTHENTICATED",
						"errors": []map[string]any{
							{"reason": "authError", "message": "Invalid Credentials"},
						},
					},
				}); err != nil {
					t.Fatalf("write json: %v", err)
				}
				return
			}
			if got := r.Header.Get("Authorization"); got != "Bearer new-access" {
				t.Fatalf("retry authorization = %q, want new access token", got)
			}
			writeJSON(t, w, map[string]any{
				"id":       "file-id",
				"name":     "movie.mp4",
				"mimeType": "video/mp4",
				"size":     "100",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			tokenRefreshes++
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			if got := r.Form.Get("refresh_token"); got != "old-refresh" {
				t.Fatalf("refresh_token = %q, want old-refresh", got)
			}
			writeJSON(t, w, map[string]any{
				"access_token": "new-access",
				"expires_in":   3600,
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	d := New(Config{
		ID:           "gd-main",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		AccessToken:  "expired-access",
		RefreshToken: "old-refresh",
		TokenURL:     srv.URL + "/token",
		APIBaseURL:   srv.URL,
	})
	d.accessTokenExpiresAt = time.Now().Add(time.Hour)

	entry, err := d.Stat(context.Background(), "file-id")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if entry.ID != "file-id" {
		t.Fatalf("entry = %#v, want file-id", entry)
	}
	if statCalls != 2 || tokenRefreshes != 1 {
		t.Fatalf("stat/refresh calls = %d/%d, want 2/1", statCalls, tokenRefreshes)
	}
}

func TestGoogleDrive429ReturnsRateLimitErrorWithRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    429,
				"message": "rate limit exceeded",
				"status":  "RESOURCE_EXHAUSTED",
				"errors": []map[string]any{
					{"reason": "userRateLimitExceeded", "message": "quota exceeded"},
				},
			},
		}); err != nil {
			t.Fatalf("write json: %v", err)
		}
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)

	_, err := d.Stat(context.Background(), "file-id")
	if err == nil {
		t.Fatal("stat succeeded, want rate limit error")
	}
	var rateLimit *drives.RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("error = %T %[1]v, want RateLimitError", err)
	}
	if rateLimit.RetryAfter != 2*time.Minute {
		t.Fatalf("retry after = %v, want 2m", rateLimit.RetryAfter)
	}
}

func TestListCoolsDownAndRetriesGoogleDriveRateLimit(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/files" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
		calls++
		if calls == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			if err := json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    429,
					"message": "rate limit exceeded",
				},
			}); err != nil {
				t.Fatalf("write json: %v", err)
			}
			return
		}
		writeJSON(t, w, map[string]any{
			"files": []map[string]any{
				{
					"id":       "file-id",
					"name":     "demo.mp4",
					"mimeType": "video/mp4",
					"size":     "100",
				},
			},
		})
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)
	d.listCooldown = time.Millisecond

	got, err := d.List(context.Background(), "root")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want retry after rate limit", calls)
	}
	if len(got) != 1 || got[0].ID != "file-id" {
		t.Fatalf("entries = %#v, want retried file", got)
	}
}

func TestEnsureDirCreatesMissingFolders(t *testing.T) {
	var created []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("authorization = %q, want bearer token", got)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/files" && r.URL.Query().Get("q") == "'root' in parents and trashed=false":
			writeJSON(t, w, map[string]any{
				"files": []map[string]any{
					{
						"id":       "existing-id",
						"name":     "existing",
						"mimeType": googleFolderMIME,
						"parents":  []string{"root"},
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/files" && r.URL.Query().Get("q") == "'existing-id' in parents and trashed=false":
			writeJSON(t, w, map[string]any{"files": []map[string]any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/files":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode mkdir body: %v", err)
			}
			created = append(created, body["name"].(string))
			if body["mimeType"] != googleFolderMIME {
				t.Fatalf("mimeType = %#v, want folder mime", body["mimeType"])
			}
			writeJSON(t, w, map[string]any{
				"id":       "created-id",
				"name":     body["name"],
				"mimeType": googleFolderMIME,
				"parents":  []string{"existing-id"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)

	got, err := d.EnsureDir(context.Background(), "/existing/previews")
	if err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	if got != "created-id" {
		t.Fatalf("dir id = %q, want created-id", got)
	}
	if len(created) != 1 || created[0] != "previews" {
		t.Fatalf("created folders = %#v, want previews", created)
	}
}

func TestUploadAndReportHashUsesResumableUpload(t *testing.T) {
	var sawStart bool
	var sawPut bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/upload/files":
			sawStart = true
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
			}
			if got := r.URL.Query().Get("uploadType"); got != "resumable" {
				t.Fatalf("uploadType = %q, want resumable", got)
			}
			if got := r.URL.Query().Get("supportsAllDrives"); got != "true" {
				t.Fatalf("supportsAllDrives = %q, want true", got)
			}
			if got := r.Header.Get("X-Upload-Content-Type"); got != "video/mp4" {
				t.Fatalf("upload content type = %q, want video/mp4", got)
			}
			if got := r.Header.Get("X-Upload-Content-Length"); got != "4" {
				t.Fatalf("upload content length = %q, want 4", got)
			}
			var body struct {
				Name    string   `json:"name"`
				Parents []string `json:"parents"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode upload metadata: %v", err)
			}
			if body.Name != "file.mp4" {
				t.Fatalf("metadata name = %q, want file.mp4", body.Name)
			}
			if len(body.Parents) != 1 || body.Parents[0] != "parent-id" {
				t.Fatalf("metadata parents = %#v, want parent-id", body.Parents)
			}
			w.Header().Set("Location", "/upload/session/1")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/upload/session/1":
			sawPut = true
			if got := r.Header.Get("Content-Type"); got != "video/mp4" {
				t.Fatalf("put content type = %q, want video/mp4", got)
			}
			if got := r.Header.Get("Content-Range"); got != "bytes 0-3/4" {
				t.Fatalf("content range = %q, want bytes 0-3/4", got)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read upload body: %v", err)
			}
			if string(body) != "data" {
				t.Fatalf("upload body = %q, want data", string(body))
			}
			writeJSON(t, w, map[string]any{
				"id":          "uploaded-id",
				"name":        "file.mp4",
				"size":        "4",
				"md5Checksum": "8d777f385d3dfec8815d20f7496026dc",
				"parents":     []string{"parent-id"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	d := newAuthorizedTestDriver(srv.URL)
	res, err := d.UploadAndReportHash(context.Background(), "parent-id", "file.mp4", strings.NewReader("data"), 4)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if res.FileID != "uploaded-id" || res.Hash != "8d777f385d3dfec8815d20f7496026dc" || res.Size != 4 {
		t.Fatalf("upload result = %#v", res)
	}
	if !sawStart || !sawPut {
		t.Fatalf("sawStart=%v sawPut=%v, want both true", sawStart, sawPut)
	}
}

func TestDriverImplementsInterface(t *testing.T) {
	var _ drives.Drive = (*Driver)(nil)
}

func newAuthorizedTestDriver(apiBaseURL string) *Driver {
	d := New(Config{
		ID:           "gd-main",
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		APIBaseURL:   apiBaseURL,
	})
	d.accessTokenExpiresAt = time.Now().Add(time.Hour)
	d.listInterval = 0
	return d
}

func writeJSON(t *testing.T, w http.ResponseWriter, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
