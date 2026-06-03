package p123

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/skip2/go-qrcode"
)

const (
	defaultUserAPIBase = "https://user.123pan.cn/api"
	defaultQRLoginPage = "https://www.123pan.com/wx-app-login.html"
	defaultQRReferer   = "https://user.123pan.com/centerlogin"
	defaultQROrigin    = "https://user.123pan.com"
	defaultQRUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36"

	endpointQRCodeGenerate = "/user/qr-code/generate"
	endpointQRCodeResult   = "/user/qr-code/result"
	endpointQRCodeWXCode   = "/user/qr-code/wx_code"
)

type QRConfig struct {
	UserAPIBaseURL string
	HTTPClient     *http.Client
	Now            func() time.Time
}

type QRClient struct {
	userAPIBase string
	client      *resty.Client
	now         func() time.Time
}

type QRCodeSession struct {
	LoginUUID      string `json:"loginUuid"`
	UniID          string `json:"uniID"`
	QRCodeURL      string `json:"qrCodeUrl"`
	QRImageDataURL string `json:"qrImageDataUrl"`
	ExpiresAt      string `json:"expiresAt,omitempty"`
}

type QRCodeStatus struct {
	LoginStatus  int    `json:"loginStatus"`
	StatusText   string `json:"statusText"`
	ScanPlatform int    `json:"scanPlatform,omitempty"`
	PlatformText string `json:"platformText,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
}

func NewQRClient(c QRConfig) *QRClient {
	userAPIBase := strings.TrimRight(strings.TrimSpace(c.UserAPIBaseURL), "/")
	if userAPIBase == "" {
		userAPIBase = defaultUserAPIBase
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	return &QRClient{
		userAPIBase: userAPIBase,
		client: resty.NewWithClient(httpClient).
			SetTimeout(20*time.Second).
			SetHeader("Accept", "application/json, text/plain, */*"),
		now: now,
	}
}

func (c *QRClient) Generate(ctx context.Context) (QRCodeSession, error) {
	loginUUID, err := newLoginUUID()
	if err != nil {
		return QRCodeSession{}, err
	}
	var resp qrGenerateResp
	res, err := c.request(ctx, loginUUID).
		SetResult(&resp).
		Get(c.userAPIBase + endpointQRCodeGenerate)
	if err != nil {
		return QRCodeSession{}, err
	}
	if resp.Code != 0 {
		return QRCodeSession{}, qrAPIError(resp.Message, res.StatusCode(), resp.Code)
	}
	uniID := strings.TrimSpace(resp.Data.UniID)
	if uniID == "" {
		return QRCodeSession{}, errors.New("123pan qr: empty uniID")
	}
	qrURL := buildQRLoginURL(resp.Data.URL, uniID)
	png, err := qrcode.Encode(qrURL, qrcode.Medium, 220)
	if err != nil {
		return QRCodeSession{}, err
	}
	return QRCodeSession{
		LoginUUID:      loginUUID,
		UniID:          uniID,
		QRCodeURL:      qrURL,
		QRImageDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		ExpiresAt:      c.now().Add(5 * time.Minute).Format(time.RFC3339),
	}, nil
}

func (c *QRClient) Poll(ctx context.Context, loginUUID, uniID string) (QRCodeStatus, error) {
	loginUUID = strings.TrimSpace(loginUUID)
	uniID = strings.TrimSpace(uniID)
	if loginUUID == "" {
		return QRCodeStatus{}, errors.New("loginUuid is required")
	}
	if uniID == "" {
		return QRCodeStatus{}, errors.New("uniID is required")
	}
	var resp qrResultResp
	res, err := c.request(ctx, loginUUID).
		SetQueryParam("uniID", uniID).
		SetResult(&resp).
		Get(c.userAPIBase + endpointQRCodeResult)
	if err != nil {
		return QRCodeStatus{}, err
	}
	if resp.Code != 0 && resp.Code != 200 {
		return QRCodeStatus{}, qrAPIError(resp.Message, res.StatusCode(), resp.Code)
	}
	if resp.Code == 200 {
		resp.Data.LoginStatus = 3
		if resp.Data.ScanPlatform == 0 {
			resp.Data.ScanPlatform = resp.Data.LoginType
		}
	}
	status := QRCodeStatus{
		LoginStatus:  resp.Data.LoginStatus,
		StatusText:   qrLoginStatusText(resp.Data.LoginStatus),
		ScanPlatform: resp.Data.ScanPlatform,
		PlatformText: qrScanPlatformText(resp.Data.ScanPlatform),
	}
	if status.LoginStatus != 3 {
		return status, nil
	}
	if token := resp.TokenValue(); token != "" {
		status.AccessToken = normalizeAccessToken(token)
		return status, nil
	}
	if resp.Data.ScanPlatform == 4 {
		token, err := c.finishWechatLogin(ctx, loginUUID, uniID)
		if err != nil {
			return QRCodeStatus{}, err
		}
		status.AccessToken = normalizeAccessToken(token)
		return status, nil
	}
	return QRCodeStatus{}, errors.New("123pan qr: confirmed login returned empty token")
}

func (c *QRClient) finishWechatLogin(ctx context.Context, loginUUID, uniID string) (string, error) {
	var wxResp qrWXCodeResp
	res, err := c.request(ctx, loginUUID).
		SetBody(map[string]string{"uniID": uniID}).
		SetResult(&wxResp).
		Post(c.userAPIBase + endpointQRCodeWXCode)
	if err != nil {
		return "", err
	}
	if wxResp.Code != 0 {
		return "", qrAPIError(wxResp.Message, res.StatusCode(), wxResp.Code)
	}
	wxCode := strings.TrimSpace(wxResp.WXCode())
	if wxCode == "" {
		return "", errors.New("123pan qr: empty wechat code")
	}
	var signIn loginResp
	res, err = c.request(ctx, loginUUID).
		SetBody(map[string]any{
			"from":        "web",
			"wechat_code": wxCode,
			"type":        4,
		}).
		SetResult(&signIn).
		Post(c.userAPIBase + endpointSignIn)
	if err != nil {
		return "", err
	}
	if signIn.Code != 200 && signIn.Code != 0 {
		return "", qrAPIError(signIn.Message, res.StatusCode(), signIn.Code)
	}
	token := strings.TrimSpace(signIn.Data.Token)
	if token == "" {
		return "", errors.New("123pan qr: empty token")
	}
	return token, nil
}

func (c *QRClient) request(ctx context.Context, loginUUID string) *resty.Request {
	return c.client.R().
		SetContext(ctx).
		SetHeaders(map[string]string{
			"Content-Type": "application/json;charset=UTF-8",
			"platform":     defaultPlatform,
			"App-Version":  defaultAppVersion,
			"LoginUuid":    loginUUID,
			"Referer":      defaultQRReferer,
			"Origin":       defaultQROrigin,
			"User-Agent":   defaultQRUserAgent,
		})
}

func buildQRLoginURL(raw, uniID string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = defaultQRLoginPage
	}
	u, err := url.Parse(raw)
	if err != nil {
		return defaultQRLoginPage + "?env=production&uniID=" + url.QueryEscape(uniID) + "&source=123pan&type=login"
	}
	q := u.Query()
	q.Set("env", "production")
	q.Set("uniID", uniID)
	q.Set("source", "123pan")
	q.Set("type", "login")
	u.RawQuery = q.Encode()
	return u.String()
}

func newLoginUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	parts := []string{
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	}
	return strings.Join(parts, "-"), nil
}

func qrAPIError(message string, httpStatus, apiCode int) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = fmt.Sprintf("HTTP %d code=%d", httpStatus, apiCode)
	}
	return errors.New(message)
}

func qrLoginStatusText(status int) string {
	switch status {
	case 0:
		return "等待扫码"
	case 1:
		return "已扫码，等待确认"
	case 2:
		return "已拒绝"
	case 3:
		return "已确认"
	case 4:
		return "已过期"
	default:
		return "未知状态"
	}
}

func qrScanPlatformText(platform int) string {
	switch platform {
	case 4:
		return "微信"
	case 7:
		return "123 云盘 App"
	default:
		return ""
	}
}
