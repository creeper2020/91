package p123

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQRCodeGenerateBuildsImage(t *testing.T) {
	var seenLoginUUID string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/user/qr-code/generate" {
			http.NotFound(w, r)
			return
		}
		seenLoginUUID = r.Header.Get("LoginUuid")
		if seenLoginUUID == "" {
			t.Fatalf("missing LoginUuid header")
		}
		if r.Header.Get("platform") != defaultPlatform {
			t.Fatalf("platform header = %q, want %q", r.Header.Get("platform"), defaultPlatform)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"data": map[string]string{
				"uniID": "uni-1",
				"url":   "https://www.123pan.com/wx-app-login.html",
			},
		})
	}))
	t.Cleanup(api.Close)

	got, err := NewQRClient(QRConfig{UserAPIBaseURL: api.URL + "/api"}).Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got.LoginUUID != seenLoginUUID {
		t.Fatalf("loginUuid = %q, want header %q", got.LoginUUID, seenLoginUUID)
	}
	if got.UniID != "uni-1" {
		t.Fatalf("uniID = %q, want uni-1", got.UniID)
	}
	if !strings.Contains(got.QRCodeURL, "uniID=uni-1") || !strings.Contains(got.QRCodeURL, "type=login") {
		t.Fatalf("qrCodeUrl = %q, want login params", got.QRCodeURL)
	}
	if !strings.HasPrefix(got.QRImageDataURL, "data:image/png;base64,") {
		t.Fatalf("qrImageDataUrl missing png data url prefix")
	}
	if got.ExpiresAt == "" {
		t.Fatalf("expiresAt is empty")
	}
}

func TestQRCodePollCompletesWechatLogin(t *testing.T) {
	var wxCodeRequested bool
	var signInRequested bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("LoginUuid") != "login-1" {
			t.Fatalf("LoginUuid = %q, want login-1", r.Header.Get("LoginUuid"))
		}
		switch r.URL.Path {
		case "/api/user/qr-code/result":
			if r.URL.Query().Get("uniID") != "uni-1" {
				t.Fatalf("uniID = %q, want uni-1", r.URL.Query().Get("uniID"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"loginStatus":  3,
					"scanPlatform": 4,
				},
			})
		case "/api/user/qr-code/wx_code":
			wxCodeRequested = true
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode wx_code body: %v", err)
			}
			if body["uniID"] != "uni-1" {
				t.Fatalf("wx_code uniID = %q, want uni-1", body["uniID"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]string{"wxCode": "wx-code-1"},
			})
		case "/api/user/sign_in":
			signInRequested = true
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode sign_in body: %v", err)
			}
			if body["wechat_code"] != "wx-code-1" {
				t.Fatalf("wechat_code = %#v, want wx-code-1", body["wechat_code"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]string{"token": "Bearer token-1"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	got, err := NewQRClient(QRConfig{UserAPIBaseURL: api.URL + "/api"}).Poll(context.Background(), "login-1", "uni-1")
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if !wxCodeRequested || !signInRequested {
		t.Fatalf("wechat completion calls wx=%v signIn=%v, want both", wxCodeRequested, signInRequested)
	}
	if got.LoginStatus != 3 || got.AccessToken != "token-1" || got.PlatformText != "微信" {
		t.Fatalf("status = %#v, want confirmed wechat token", got)
	}
}

func TestQRCodePollUsesAppToken(t *testing.T) {
	var wxCodeRequested bool
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/qr-code/result":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"loginStatus":  3,
					"scanPlatform": 7,
					"token":        "app-token",
				},
			})
		case "/api/user/qr-code/wx_code":
			wxCodeRequested = true
			http.Error(w, "unexpected wx_code", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(api.Close)

	got, err := NewQRClient(QRConfig{UserAPIBaseURL: api.URL + "/api"}).Poll(context.Background(), "login-1", "uni-1")
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if wxCodeRequested {
		t.Fatalf("wx_code should not be called when app token is already returned")
	}
	if got.AccessToken != "app-token" || got.PlatformText != "123 云盘 App" {
		t.Fatalf("status = %#v, want app token", got)
	}
}

func TestQRCodePollUsesOfficialAppSuccessCode(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/user/qr-code/result" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"login_type": 7,
				"token":      "app-token",
			},
		})
	}))
	t.Cleanup(api.Close)

	got, err := NewQRClient(QRConfig{UserAPIBaseURL: api.URL + "/api"}).Poll(context.Background(), "login-1", "uni-1")
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if got.LoginStatus != 3 || got.ScanPlatform != 7 || got.AccessToken != "app-token" {
		t.Fatalf("status = %#v, want official app success token", got)
	}
}
