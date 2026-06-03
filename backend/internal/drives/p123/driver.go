package p123

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/video-site/backend/internal/drives"
)

const (
	Kind = "p123"

	defaultMainAPIBase  = "https://www.123pan.com/b/api"
	defaultLoginAPIBase = "https://login.123pan.com/api"
	defaultReferer      = "https://www.123pan.com/"
	defaultPlatform     = "web"
	defaultAppVersion   = "3"
	defaultUserAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) video-site-123pan"

	endpointSignIn       = "/user/sign_in"
	endpointUserInfo     = "/user/info"
	endpointFileList     = "/file/list/new"
	endpointDownloadInfo = "/file/download_info"
	endpointMkdir        = "/file/upload_request"

	listInterval = 700 * time.Millisecond
	listCooldown = 10 * time.Minute
)

type Driver struct {
	id           string
	rootID       string
	username     string
	password     string
	accessToken  string
	platform     string
	mainAPIBase  string
	loginAPIBase string
	referer      string
	userAgent    string

	client     *resty.Client
	httpClient *http.Client

	onTokenUpdate func(access string)

	tokenMu sync.RWMutex

	listMu     sync.Mutex
	lastListAt time.Time

	fileMu sync.RWMutex
	files  map[string]cachedFile
}

type Config struct {
	ID          string
	RootID      string
	Username    string
	Password    string
	AccessToken string
	Platform    string

	MainAPIBaseURL  string
	LoginAPIBaseURL string

	OnTokenUpdate func(access string)
}

func New(c Config) *Driver {
	rootID := strings.TrimSpace(c.RootID)
	if rootID == "" {
		rootID = "0"
	}
	platform := strings.TrimSpace(c.Platform)
	if platform == "" {
		platform = defaultPlatform
	}
	mainAPIBase := strings.TrimRight(strings.TrimSpace(c.MainAPIBaseURL), "/")
	if mainAPIBase == "" {
		mainAPIBase = defaultMainAPIBase
	}
	loginAPIBase := strings.TrimRight(strings.TrimSpace(c.LoginAPIBaseURL), "/")
	if loginAPIBase == "" {
		loginAPIBase = defaultLoginAPIBase
	}
	return &Driver{
		id:            c.ID,
		rootID:        rootID,
		username:      strings.TrimSpace(c.Username),
		password:      strings.TrimSpace(c.Password),
		accessToken:   normalizeAccessToken(c.AccessToken),
		platform:      platform,
		mainAPIBase:   mainAPIBase,
		loginAPIBase:  loginAPIBase,
		referer:       defaultReferer,
		userAgent:     defaultUserAgent,
		onTokenUpdate: c.OnTokenUpdate,
		client: resty.New().
			SetTimeout(30*time.Second).
			SetHeader("Accept", "application/json, text/plain, */*"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		files: make(map[string]cachedFile),
	}
}

func (d *Driver) Kind() string   { return Kind }
func (d *Driver) ID() string     { return d.id }
func (d *Driver) RootID() string { return d.rootID }

func (d *Driver) Init(ctx context.Context) error {
	if d.currentToken() == "" {
		if err := d.login(ctx); err != nil {
			return err
		}
	}
	_, err := d.request(ctx, endpointUserInfo, http.MethodGet, nil, nil)
	return err
}

func (d *Driver) List(ctx context.Context, dirID string) ([]drives.Entry, error) {
	if strings.TrimSpace(dirID) == "" {
		dirID = d.rootID
	}
	d.listMu.Lock()
	defer d.listMu.Unlock()

	page := 1
	total := 0
	out := make([]drives.Entry, 0)
	for {
		var resp fileListResp
		query := map[string]string{
			"driveId":              "0",
			"limit":                "100",
			"next":                 "0",
			"orderBy":              "file_id",
			"orderDirection":       "desc",
			"parentFileId":         dirID,
			"trashed":              "false",
			"SearchData":           "",
			"Page":                 strconv.Itoa(page),
			"OnlyLookAbnormalFile": "0",
			"event":                "homeListFile",
			"operateType":          "4",
			"inDirectSpace":        "false",
		}
		for attempt := 0; ; attempt++ {
			if err := d.waitForListSlotLocked(ctx); err != nil {
				return nil, err
			}
			if _, err := d.request(ctx, endpointFileList, http.MethodGet, func(req *resty.Request) {
				req.SetQueryParams(query)
			}, &resp); err != nil {
				wait, ok := drives.RateLimitRetryAfter(err)
				if !ok {
					return nil, fmt.Errorf("123pan list: %w", err)
				}
				if wait <= 0 {
					wait = listCooldown
				}
				log.Printf("[p123] list cooling down drive=%s dir=%s page=%d cooldown=%s attempt=%d err=%v",
					d.id, dirID, page, wait, attempt+1, err)
				if err := sleepContext(ctx, wait); err != nil {
					return nil, err
				}
				continue
			}
			break
		}
		for _, f := range resp.Data.InfoList {
			d.cacheFile(f, dirID)
			out = append(out, fileToEntry(f, dirID))
		}
		total = resp.Data.Total
		page++
		if len(resp.Data.InfoList) == 0 || resp.Data.Next == "-1" || (total > 0 && len(out) >= total) {
			return out, nil
		}
	}
}

func (d *Driver) Stat(ctx context.Context, fileID string) (*drives.Entry, error) {
	f, parentID, err := d.findFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	e := fileToEntry(f, parentID)
	return &e, nil
}

func (d *Driver) StreamURL(ctx context.Context, fileID string) (*drives.StreamLink, error) {
	f, _, err := d.findFile(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("123pan stream metadata: %w", err)
	}
	body := map[string]any{
		"driveId":   0,
		"etag":      f.Etag,
		"fileId":    f.FileID,
		"fileName":  f.FileName,
		"s3keyFlag": f.S3KeyFlag,
		"size":      f.Size,
		"type":      f.Type,
	}
	var resp downloadInfoResp
	if _, err := d.request(ctx, endpointDownloadInfo, http.MethodPost, func(req *resty.Request) {
		req.SetBody(body)
	}, &resp); err != nil {
		return nil, fmt.Errorf("123pan download info: %w", err)
	}
	downloadURL := strings.TrimSpace(resp.URL())
	if downloadURL == "" {
		return nil, errors.New("123pan download info: empty url")
	}
	return d.resolveDownloadURL(ctx, downloadURL)
}

func (d *Driver) Upload(context.Context, string, string, io.Reader, int64) (string, error) {
	return "", drives.ErrNotSupported
}

func (d *Driver) EnsureDir(ctx context.Context, pathFromRoot string) (string, error) {
	parts := splitPath(pathFromRoot)
	currentID := d.rootID
	for _, name := range parts {
		childID, err := d.findChildDir(ctx, currentID, name)
		if err != nil {
			return "", err
		}
		if childID == "" {
			id, err := d.makeDir(ctx, currentID, name)
			if err != nil {
				return "", err
			}
			childID = id
		}
		currentID = childID
	}
	return currentID, nil
}

func (d *Driver) makeDir(ctx context.Context, parentID, name string) (string, error) {
	body := map[string]any{
		"driveId":      0,
		"etag":         "",
		"fileName":     name,
		"parentFileId": parentID,
		"size":         0,
		"type":         1,
	}
	var resp mkdirResp
	if _, err := d.request(ctx, endpointMkdir, http.MethodPost, func(req *resty.Request) {
		req.SetBody(body)
	}, &resp); err != nil {
		return "", fmt.Errorf("123pan mkdir %s: %w", name, err)
	}
	if resp.Data.FileID != 0 {
		return strconv.FormatInt(resp.Data.FileID, 10), nil
	}
	// 123 云盘创建目录的返回字段不稳定；创建成功但没回 fileId 时回读父目录确认。
	childID, err := d.findChildDir(ctx, parentID, name)
	if err != nil {
		return "", err
	}
	if childID == "" {
		return "", errors.New("123pan mkdir: empty file id")
	}
	return childID, nil
}

func (d *Driver) findChildDir(ctx context.Context, parentID, name string) (string, error) {
	entries, err := d.List(ctx, parentID)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir && e.Name == name {
			return e.ID, nil
		}
	}
	return "", nil
}

func (d *Driver) resolveDownloadURL(ctx context.Context, downloadURL string) (*drives.StreamLink, error) {
	original, err := url.Parse(downloadURL)
	if err != nil {
		return nil, err
	}
	target := original.String()
	if params := original.Query().Get("params"); params != "" {
		if decoded, err := base64.StdEncoding.DecodeString(params); err == nil && len(decoded) > 0 {
			if u, err := url.Parse(string(decoded)); err == nil {
				target = u.String()
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", defaultReferer)
	req.Header.Set("User-Agent", d.userAgent)
	res, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	finalURL := ""
	if res.StatusCode >= 300 && res.StatusCode < 400 {
		finalURL = strings.TrimSpace(res.Header.Get("Location"))
	} else if res.StatusCode < 300 {
		var redirect redirectResp
		if err := json.NewDecoder(res.Body).Decode(&redirect); err == nil {
			finalURL = redirect.URL()
		}
		if finalURL == "" {
			finalURL = target
		}
	} else {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		if isP123RateLimitHTTPResponse(res.StatusCode, res.Header.Get("Retry-After"), string(body)) {
			return nil, p123RateLimitErrorFromHTTP("download redirect", res.StatusCode, res.Header.Get("Retry-After"), string(body))
		}
		return nil, fmt.Errorf("123pan download redirect: status %d", res.StatusCode)
	}
	if finalURL == "" {
		return nil, errors.New("123pan download redirect: empty url")
	}

	headers := http.Header{}
	if original.Scheme != "" && original.Host != "" {
		headers.Set("Referer", fmt.Sprintf("%s://%s/", original.Scheme, original.Host))
	} else {
		headers.Set("Referer", defaultReferer)
	}
	headers.Set("User-Agent", d.userAgent)
	return &drives.StreamLink{
		URL:     finalURL,
		Headers: headers,
		Expires: time.Now().Add(10 * time.Minute),
	}, nil
}

func (d *Driver) request(ctx context.Context, endpoint, method string, configure func(*resty.Request), out any) ([]byte, error) {
	if d.currentToken() == "" {
		if err := d.login(ctx); err != nil {
			return nil, err
		}
	}

	rawURL := d.mainAPIBase + endpoint
	for attempt := 0; attempt < 2; attempt++ {
		req := d.client.R().
			SetContext(ctx).
			SetHeaders(map[string]string{
				"origin":        "https://www.123pan.com",
				"referer":       d.referer,
				"authorization": "Bearer " + d.currentToken(),
				"user-agent":    d.userAgent,
				"platform":      d.platform,
				"app-version":   defaultAppVersion,
			})
		if configure != nil {
			configure(req)
		}
		if out != nil {
			req.SetResult(out)
		}
		res, err := req.Execute(method, signAPIURL(rawURL))
		if err != nil {
			return nil, err
		}
		body := res.Body()
		var env apiEnvelope
		decodeErr := json.Unmarshal(body, &env)
		if isP123RateLimitResponse(res, env.Code, env.Message) {
			return nil, p123RateLimitError(res, env.Code, env.Message)
		}
		if decodeErr != nil {
			if res.IsError() {
				return nil, fmt.Errorf("123pan request: status=%d body=%s", res.StatusCode(), strings.TrimSpace(res.String()))
			}
			return nil, fmt.Errorf("parse 123pan response: %w", decodeErr)
		}
		if env.Code == 0 {
			return body, nil
		}
		if env.Code == 401 && attempt == 0 {
			if err := d.login(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if env.Message == "" {
			env.Message = fmt.Sprintf("code=%d", env.Code)
		}
		return nil, errors.New(env.Message)
	}
	return nil, errors.New("123pan request: unauthorized")
}

func isP123RateLimitResponse(res *resty.Response, code int, message string) bool {
	if code == http.StatusTooManyRequests || isP123RateLimitMessage(message) {
		return true
	}
	if res == nil {
		return false
	}
	return isP123RateLimitHTTPResponse(res.StatusCode(), res.Header().Get("Retry-After"), res.String())
}

func isP123RateLimitHTTPResponse(status int, retryAfter, body string) bool {
	if status == http.StatusTooManyRequests {
		return true
	}
	if retryAfter != "" {
		switch status {
		case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
	}
	if isP123RateLimitMessage(body) {
		return true
	}
	return false
}

func isP123RateLimitMessage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return strings.Contains(text, "请求太频繁") ||
		strings.Contains(text, "请求过于频繁") ||
		strings.Contains(text, "请求频繁") ||
		strings.Contains(text, "操作频繁") ||
		strings.Contains(text, "频率限制") ||
		strings.Contains(text, "请求次数过多") ||
		strings.Contains(text, "too many request") ||
		strings.Contains(text, "too many requests") ||
		strings.Contains(text, "rate limit") ||
		strings.Contains(text, "rate-limit") ||
		strings.Contains(text, "ratelimit") ||
		strings.Contains(text, "throttl") ||
		strings.Contains(text, "temporarily blocked") ||
		strings.Contains(text, "request has been blocked") ||
		strings.Contains(text, "blocked") ||
		strings.Contains(text, "访问被阻断")
}

func p123RateLimitError(res *resty.Response, code int, message string) error {
	if strings.TrimSpace(message) == "" {
		message = "123pan rate limited"
	}
	if code != 0 {
		message = fmt.Sprintf("code=%d %s", code, message)
	}
	if res != nil && strings.TrimSpace(res.String()) != "" {
		message = fmt.Sprintf("%s: status=%d body=%s", message, res.StatusCode(), strings.TrimSpace(res.String()))
	}
	return &drives.RateLimitError{
		Provider:   Kind,
		RetryAfter: parseRetryAfterHeader(responseRetryAfter(res)),
		Err:        errors.New(message),
	}
}

func p123RateLimitErrorFromHTTP(step string, status int, retryAfter, body string) error {
	message := fmt.Sprintf("123pan %s rate limited: status=%d", step, status)
	if strings.TrimSpace(body) != "" {
		message += " body=" + strings.TrimSpace(body)
	}
	return &drives.RateLimitError{
		Provider:   Kind,
		RetryAfter: parseRetryAfterHeader(retryAfter),
		Err:        errors.New(message),
	}
}

func responseRetryAfter(res *resty.Response) string {
	if res == nil {
		return ""
	}
	return res.Header().Get("Retry-After")
}

func parseRetryAfterHeader(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return 0
}

func (d *Driver) login(ctx context.Context) error {
	if d.username == "" || d.password == "" {
		return errors.New("123pan login: username and password are required")
	}
	body := map[string]any{
		"passport": d.username,
		"password": d.password,
		"remember": true,
	}
	if strings.Contains(d.username, "@") {
		body = map[string]any{
			"mail":     d.username,
			"password": d.password,
			"type":     2,
		}
	}
	var resp loginResp
	res, err := d.client.R().
		SetContext(ctx).
		SetHeaders(map[string]string{
			"origin":      "https://www.123pan.com",
			"referer":     d.referer,
			"user-agent":  "Dart/2.19(dart:io)-video-site",
			"platform":    d.platform,
			"app-version": defaultAppVersion,
		}).
		SetBody(body).
		SetResult(&resp).
		Post(d.loginAPIBase + endpointSignIn)
	if err != nil {
		return err
	}
	if resp.Code != 200 {
		if resp.Message == "" {
			resp.Message = fmt.Sprintf("status=%d code=%d", res.StatusCode(), resp.Code)
		}
		return loginError(resp.Message)
	}
	if strings.TrimSpace(resp.Data.Token) == "" {
		return errors.New("123pan login: empty token")
	}
	d.setToken(resp.Data.Token)
	return nil
}

func (d *Driver) currentToken() string {
	d.tokenMu.RLock()
	defer d.tokenMu.RUnlock()
	return d.accessToken
}

func (d *Driver) setToken(token string) {
	token = normalizeAccessToken(token)
	d.tokenMu.Lock()
	d.accessToken = token
	d.tokenMu.Unlock()
	if d.onTokenUpdate != nil {
		d.onTokenUpdate(token)
	}
}

func (d *Driver) waitForListSlotLocked(ctx context.Context) error {
	if d.lastListAt.IsZero() {
		d.lastListAt = time.Now()
		return ctx.Err()
	}
	next := d.lastListAt.Add(listInterval)
	now := time.Now()
	if now.Before(next) {
		timer := time.NewTimer(next.Sub(now))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	d.lastListAt = time.Now()
	return ctx.Err()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (d *Driver) cacheFile(f panFile, parentID string) {
	id := strconv.FormatInt(f.FileID, 10)
	if id == "0" {
		return
	}
	d.fileMu.Lock()
	d.files[id] = cachedFile{file: f, parentID: parentID}
	d.fileMu.Unlock()
}

func (d *Driver) cachedFile(fileID string) (panFile, string, bool) {
	d.fileMu.RLock()
	defer d.fileMu.RUnlock()
	c, ok := d.files[fileID]
	return c.file, c.parentID, ok
}

func (d *Driver) findFile(ctx context.Context, fileID string) (panFile, string, error) {
	fileID = strings.TrimSpace(fileID)
	if fileID == "" {
		return panFile{}, "", errors.New("empty file id")
	}
	if f, parentID, ok := d.cachedFile(fileID); ok {
		return f, parentID, nil
	}
	f, parentID, ok, err := d.findFileInDir(ctx, fileID, d.rootID, make(map[string]struct{}))
	if err != nil {
		return panFile{}, "", err
	}
	if !ok {
		return panFile{}, "", fmt.Errorf("file %s not found", fileID)
	}
	return f, parentID, nil
}

func (d *Driver) findFileInDir(ctx context.Context, targetID, dirID string, visited map[string]struct{}) (panFile, string, bool, error) {
	if _, ok := visited[dirID]; ok {
		return panFile{}, "", false, nil
	}
	visited[dirID] = struct{}{}
	entries, err := d.List(ctx, dirID)
	if err != nil {
		return panFile{}, "", false, err
	}
	for _, e := range entries {
		if e.ID == targetID {
			f, parentID, ok := d.cachedFile(e.ID)
			if !ok {
				return panFile{}, "", false, nil
			}
			return f, parentID, true, nil
		}
	}
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		if f, parentID, ok, err := d.findFileInDir(ctx, targetID, e.ID, visited); err != nil || ok {
			return f, parentID, ok, err
		}
	}
	return panFile{}, "", false, nil
}

func normalizeAccessToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) >= len("Bearer ") && strings.EqualFold(token[:len("Bearer ")], "Bearer ") {
		token = strings.TrimSpace(token[len("Bearer "):])
	}
	return token
}

func loginError(message string) error {
	message = strings.TrimSpace(message)
	if strings.Contains(message, "境外登录风险") ||
		(strings.Contains(message, "短信验证码") && strings.Contains(message, "微信")) {
		return errors.New("123pan login: 账号密码登录被 123 云盘风控拦截，请在浏览器完成短信/微信验证后复制 access_token，并在后台编辑该 123 云盘时只填写 access_token")
	}
	if message == "" {
		message = "login failed"
	}
	return errors.New(message)
}

func signPath(apiPath, platform, version string) (string, string) {
	table := []byte{'a', 'd', 'e', 'f', 'g', 'h', 'l', 'm', 'y', 'i', 'j', 'n', 'o', 'p', 'k', 'q', 'r', 's', 't', 'u', 'b', 'c', 'v', 'w', 's', 'z'}
	random := fmt.Sprintf("%.f", math.Round(1e7*rand.Float64()))
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	timestamp := fmt.Sprint(now.Unix())
	nowStr := []byte(now.Format("200601021504"))
	for i := 0; i < len(nowStr); i++ {
		nowStr[i] = table[nowStr[i]-48]
	}
	timeSign := fmt.Sprint(crc32.ChecksumIEEE(nowStr))
	data := strings.Join([]string{timestamp, random, apiPath, platform, version, timeSign}, "|")
	dataSign := fmt.Sprint(crc32.ChecksumIEEE([]byte(data)))
	return timeSign, strings.Join([]string{timestamp, random, dataSign}, "-")
}

func signAPIURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := u.Query()
	k, v := signPath(u.Path, defaultPlatform, defaultAppVersion)
	query.Add(k, v)
	u.RawQuery = query.Encode()
	return u.String()
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func fileToEntry(f panFile, parentID string) drives.Entry {
	return drives.Entry{
		ID:       strconv.FormatInt(f.FileID, 10),
		Name:     f.FileName,
		Size:     f.Size,
		Hash:     strings.ToLower(f.Etag),
		IsDir:    f.Type == 1,
		ParentID: parentID,
		MimeType: guessMime(f.FileName),
		ModTime:  f.UpdateAt.Time(),
	}
}

func guessMime(name string) string {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".mp4":
		return "video/mp4"
	case ".mkv":
		return "video/x-matroska"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	}
	return "application/octet-stream"
}

var _ drives.Drive = (*Driver)(nil)
