package p123

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/video-site/backend/internal/drives"
)

func TestStreamURLResolvesDownloadInfoRedirect(t *testing.T) {
	ctx := context.Background()
	var downloadReferer string
	var download *httptest.Server
	download = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/resolve":
			downloadReferer = r.Header.Get("Referer")
			http.Redirect(w, r, download.URL+"/cdn/video.mp4", http.StatusFound)
		case "/cdn/video.mp4":
			t.Fatalf("driver followed redirect unexpectedly")
		default:
			http.NotFound(w, r)
		}
	}))
	defer download.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/sign_in":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]string{"token": "token-1"},
			})
		case "/b/api/user/info":
			if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
				t.Fatalf("Authorization = %q, want bearer token", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		case "/b/api/file/list/new":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"Next":  "-1",
					"Total": 1,
					"InfoList": []map[string]any{
						{
							"FileName":  "video.mp4",
							"Size":      1234,
							"UpdateAt":  "2026-01-02 03:04:05",
							"FileId":    100,
							"Type":      0,
							"Etag":      "ABCDEF",
							"S3KeyFlag": "flag-1",
						},
					},
				},
			})
		case "/b/api/file/download_info":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode download_info body: %v", err)
			}
			if got := body["fileName"]; got != "video.mp4" {
				t.Fatalf("fileName = %#v, want cached file metadata", got)
			}
			if got := body["etag"]; got != "ABCDEF" {
				t.Fatalf("etag = %#v, want cached etag", got)
			}
			entryURL := download.URL + "/entry?params=" + base64.StdEncoding.EncodeToString([]byte(download.URL+"/resolve"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]string{"DownloadUrl": entryURL},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	var savedToken string
	d := New(Config{
		ID:              "123-main",
		Username:        "user@example.com",
		Password:        "secret",
		MainAPIBaseURL:  api.URL + "/b/api",
		LoginAPIBaseURL: api.URL + "/api",
		OnTokenUpdate: func(access string) {
			savedToken = access
		},
	})
	if err := d.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if savedToken != "token-1" {
		t.Fatalf("saved token = %q, want token-1", savedToken)
	}
	if _, err := d.List(ctx, d.RootID()); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	link, err := d.StreamURL(ctx, "100")
	if err != nil {
		t.Fatalf("StreamURL() error = %v", err)
	}
	if got := link.URL; got != download.URL+"/cdn/video.mp4" {
		t.Fatalf("URL = %q, want final CDN URL", got)
	}
	if got := link.Headers.Get("Referer"); !strings.HasPrefix(got, download.URL) {
		t.Fatalf("Referer = %q, want original download host", got)
	}
	if downloadReferer != defaultReferer {
		t.Fatalf("resolve Referer = %q, want %q", downloadReferer, defaultReferer)
	}
}

func TestInitUsesAccessTokenWithoutLogin(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/sign_in":
			t.Fatalf("driver should not password-login when access_token is configured")
		case "/b/api/user/info":
			if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
				t.Fatalf("Authorization = %q, want bearer token", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer api.Close()

	d := New(Config{
		ID:              "123-main",
		AccessToken:     "Bearer token-1",
		MainAPIBaseURL:  api.URL + "/b/api",
		LoginAPIBaseURL: api.URL + "/api",
	})
	if err := d.Init(context.Background()); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
}

func TestLoginRiskErrorSuggestsAccessToken(t *testing.T) {
	err := loginError("当前账号存在境外登录风险，请使用短信验证码或者微信进行登录。")
	if err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("loginError() = %v, want access_token guidance", err)
	}
}

func TestRequestCode429ReturnsRateLimitError(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "2")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    429,
			"message": "请求太频繁",
		})
	}))
	defer api.Close()

	d := New(Config{
		ID:             "123-main",
		AccessToken:    "token-1",
		MainAPIBaseURL: api.URL,
	})
	_, err := d.request(context.Background(), endpointFileList, http.MethodGet, nil, nil)
	var rateLimit *drives.RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("error = %T %[1]v, want RateLimitError", err)
	}
	if rateLimit.RetryAfter != 2*time.Second {
		t.Fatalf("RetryAfter = %s, want 2s", rateLimit.RetryAfter)
	}
}

func TestListCoolsDownAndRetriesRateLimit(t *testing.T) {
	var listCalls int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/file/list/new" {
			http.NotFound(w, r)
			return
		}
		listCalls++
		if listCalls == 1 {
			w.Header().Set("Retry-After", "1")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    429,
				"message": "请求太频繁",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"Next":  "-1",
				"Total": 1,
				"InfoList": []map[string]any{
					{
						"FileName":  "video.mp4",
						"Size":      1234,
						"UpdateAt":  "2026-01-02 03:04:05",
						"FileId":    100,
						"Type":      0,
						"Etag":      "ABCDEF",
						"S3KeyFlag": "flag-1",
					},
				},
			},
		})
	}))
	defer api.Close()

	d := New(Config{
		ID:             "123-main",
		AccessToken:    "token-1",
		MainAPIBaseURL: api.URL,
	})
	entries, err := d.List(context.Background(), d.RootID())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listCalls != 2 {
		t.Fatalf("list calls = %d, want 2", listCalls)
	}
	if len(entries) != 1 || entries[0].ID != "100" {
		t.Fatalf("entries = %#v, want one file", entries)
	}
}

func TestResolveDownloadURL429ReturnsRateLimitError(t *testing.T) {
	download := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}))
	defer download.Close()

	d := New(Config{ID: "123-main"})
	_, err := d.resolveDownloadURL(context.Background(), download.URL)
	var rateLimit *drives.RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("error = %T %[1]v, want RateLimitError", err)
	}
	if rateLimit.RetryAfter != 3*time.Second {
		t.Fatalf("RetryAfter = %s, want 3s", rateLimit.RetryAfter)
	}
}
