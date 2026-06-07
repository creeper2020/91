package googledrive

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	Kind               = "googledrive"
	defaultAPIBase     = "https://www.googleapis.com/drive/v3"
	defaultTokenURL    = "https://oauth2.googleapis.com/token"
	googleFolderMIME   = "application/vnd.google-apps.folder"
	googleShortcutMIME = "application/vnd.google-apps.shortcut"

	googleListCooldown = 5 * time.Minute
	googleListInterval = 500 * time.Millisecond
)

const fileFields = "id,name,mimeType,size,modifiedTime,parents,md5Checksum,sha1Checksum,sha256Checksum,webContentLink,thumbnailLink,shortcutDetails(targetId,targetMimeType)"

type Driver struct {
	id            string
	rootID        string
	clientID      string
	clientSecret  string
	accessToken   string
	refreshToken  string
	tokenURL      string
	apiBaseURL    string
	client        *resty.Client
	onTokenUpdate func(access, refresh string)

	tokenMu              sync.Mutex
	accessTokenExpiresAt time.Time

	listMu       sync.Mutex
	lastListAt   time.Time
	listInterval time.Duration
	listCooldown time.Duration
}

type Config struct {
	ID            string
	RootID        string
	ClientID      string
	ClientSecret  string
	AccessToken   string
	RefreshToken  string
	TokenURL      string
	APIBaseURL    string
	OnTokenUpdate func(access, refresh string)
}

func New(c Config) *Driver {
	rootID := strings.TrimSpace(c.RootID)
	if rootID == "" {
		rootID = "root"
	}
	tokenURL := strings.TrimSpace(c.TokenURL)
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	apiBaseURL := strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBase
	}
	return &Driver{
		id:           c.ID,
		rootID:       rootID,
		clientID:     strings.TrimSpace(c.ClientID),
		clientSecret: strings.TrimSpace(c.ClientSecret),
		accessToken:  strings.TrimSpace(c.AccessToken),
		refreshToken: strings.TrimSpace(c.RefreshToken),
		tokenURL:     tokenURL,
		apiBaseURL:   apiBaseURL,
		client: resty.New().
			SetTimeout(30*time.Second).
			SetHeader("Accept", "application/json, text/plain, */*"),
		onTokenUpdate: c.OnTokenUpdate,
		listInterval:  googleListInterval,
		listCooldown:  googleListCooldown,
	}
}

func (d *Driver) Kind() string   { return Kind }
func (d *Driver) ID() string     { return d.id }
func (d *Driver) RootID() string { return d.rootID }

func (d *Driver) Init(ctx context.Context) error {
	if d.clientID == "" {
		return errors.New("googledrive init: client_id is required")
	}
	if d.clientSecret == "" {
		return errors.New("googledrive init: client_secret is required")
	}
	if d.refreshToken == "" {
		return errors.New("googledrive init: refresh_token is required")
	}
	return d.ensureAccessToken(ctx)
}

func (d *Driver) List(ctx context.Context, dirID string) ([]drives.Entry, error) {
	if dirID == "" {
		dirID = d.rootID
	}
	d.listMu.Lock()
	defer d.listMu.Unlock()

	var out []drives.Entry
	pageToken := ""
	for {
		if err := d.waitForListSlotLocked(ctx); err != nil {
			return nil, err
		}
		var resp listResp
		err := d.request(ctx, d.filesURL(), http.MethodGet, func(req *resty.Request) {
			req.SetQueryParams(map[string]string{
				"q":                         fmt.Sprintf("'%s' in parents and trashed=false", escapeDriveQueryString(dirID)),
				"pageSize":                  "1000",
				"fields":                    "nextPageToken,files(" + fileFields + ")",
				"supportsAllDrives":         "true",
				"includeItemsFromAllDrives": "true",
			})
			if pageToken != "" {
				req.SetQueryParam("pageToken", pageToken)
			}
		}, &resp)
		if err != nil {
			if wait, ok := drives.RateLimitRetryAfter(err); ok {
				if wait <= 0 {
					wait = d.listCooldown
					if wait <= 0 {
						wait = googleListCooldown
					}
				}
				log.Printf("[googledrive] list cooling down drive=%s dir=%s cooldown=%s err=%v", d.id, dirID, wait, err)
				if err := sleepContext(ctx, wait); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("googledrive list: %w", err)
		}
		if err := d.fillShortcutFileMetadata(ctx, resp.Files); err != nil {
			return nil, fmt.Errorf("googledrive shortcut metadata: %w", err)
		}
		for _, item := range resp.Files {
			out = append(out, fileToEntry(item, dirID))
		}
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return out, nil
}

func (d *Driver) waitForListSlotLocked(ctx context.Context) error {
	if d.listInterval <= 0 || d.lastListAt.IsZero() {
		d.lastListAt = time.Now()
		return ctx.Err()
	}
	next := d.lastListAt.Add(d.listInterval)
	now := time.Now()
	if now.Before(next) {
		if err := sleepContext(ctx, next.Sub(now)); err != nil {
			return err
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

func (d *Driver) Stat(ctx context.Context, fileID string) (*drives.Entry, error) {
	var item fileResp
	if err := d.request(ctx, d.itemURL(fileID), http.MethodGet, func(req *resty.Request) {
		req.SetQueryParams(commonFileQueryParams())
	}, &item); err != nil {
		return nil, fmt.Errorf("googledrive stat: %w", err)
	}
	e := fileToEntry(item, "")
	return &e, nil
}

func (d *Driver) StreamURL(ctx context.Context, fileID string) (*drives.StreamLink, error) {
	if err := d.ensureAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("googledrive download url: %w", err)
	}
	expires := d.currentAccessTokenExpiresAt()
	if expires.IsZero() {
		expires = time.Now().Add(10 * time.Minute)
	}
	u, err := url.Parse(d.itemURL(fileID))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("alt", "media")
	q.Set("supportsAllDrives", "true")
	u.RawQuery = q.Encode()
	return &drives.StreamLink{
		URL: u.String(),
		Headers: http.Header{
			"Authorization": {"Bearer " + d.currentAccessToken()},
		},
		Expires: expires,
	}, nil
}

func (d *Driver) EnsureDir(ctx context.Context, pathFromRoot string) (string, error) {
	currentID := d.rootID
	for _, name := range splitPath(pathFromRoot) {
		childID, err := d.findChildDir(ctx, currentID, name)
		if err != nil {
			return "", err
		}
		if childID == "" {
			childID, err = d.makeDir(ctx, currentID, name)
			if err != nil {
				return "", err
			}
		}
		currentID = childID
	}
	return currentID, nil
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

func (d *Driver) makeDir(ctx context.Context, parentID, name string) (string, error) {
	body := map[string]any{
		"name":     name,
		"mimeType": googleFolderMIME,
		"parents":  []string{parentID},
	}
	var item fileResp
	err := d.request(ctx, d.filesURL(), http.MethodPost, func(req *resty.Request) {
		req.SetQueryParams(map[string]string{
			"supportsAllDrives": "true",
			"fields":            "id,name,mimeType,parents",
		})
		req.SetBody(body)
	}, &item)
	if err != nil {
		return "", fmt.Errorf("googledrive mkdir %s: %w", name, err)
	}
	if item.ID == "" {
		return "", fmt.Errorf("googledrive mkdir %s: empty item id", name)
	}
	return item.ID, nil
}

func (d *Driver) request(ctx context.Context, rawURL, method string, configure func(*resty.Request), out any) error {
	return d.requestOnce(ctx, rawURL, method, configure, out, true)
}

func (d *Driver) requestOnce(ctx context.Context, rawURL, method string, configure func(*resty.Request), out any, retry bool) error {
	if err := d.ensureAccessToken(ctx); err != nil {
		return err
	}
	req := d.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+d.currentAccessToken())
	if configure != nil {
		configure(req)
	}
	if out != nil {
		req.SetResult(out)
	}
	var googleErr googleErrorResp
	req.SetError(&googleErr)
	res, err := req.Execute(method, rawURL)
	if err != nil {
		return err
	}
	if isRateLimitResponse(res, googleErr) {
		return googleDriveRateLimitError(res, googleErrorMessage(googleErr))
	}
	if hasGoogleError(googleErr) {
		if isAuthError(res, googleErr) && retry {
			if err := d.refresh(ctx); err != nil {
				return err
			}
			return d.requestOnce(ctx, rawURL, method, configure, out, false)
		}
		if msg := googleErrorMessage(googleErr); msg != "" {
			return errors.New(msg)
		}
		return fmt.Errorf("google drive api error: status=%d", googleErr.Error.Code)
	}
	if res.IsError() {
		if res.StatusCode() == http.StatusUnauthorized && retry {
			if err := d.refresh(ctx); err != nil {
				return err
			}
			return d.requestOnce(ctx, rawURL, method, configure, out, false)
		}
		return fmt.Errorf("google drive api error: status=%d body=%s", res.StatusCode(), strings.TrimSpace(res.String()))
	}
	return nil
}

func (d *Driver) ensureAccessToken(ctx context.Context) error {
	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()
	if d.accessToken != "" && time.Until(d.accessTokenExpiresAt) > time.Minute {
		return nil
	}
	return d.refreshLocked(ctx)
}

func (d *Driver) refresh(ctx context.Context) error {
	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()
	return d.refreshLocked(ctx)
}

func (d *Driver) refreshLocked(ctx context.Context) error {
	var out tokenResp
	res, err := d.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"client_id":     d.clientID,
			"client_secret": d.clientSecret,
			"refresh_token": d.refreshToken,
			"grant_type":    "refresh_token",
		}).
		SetResult(&out).
		SetError(&out).
		Post(d.tokenURL)
	if err != nil {
		return fmt.Errorf("googledrive refresh token: %w", err)
	}
	if res.StatusCode() == http.StatusTooManyRequests {
		return googleDriveRateLimitError(res, "token refresh throttled")
	}
	if out.Error != "" {
		if out.ErrorDescription != "" {
			return fmt.Errorf("googledrive refresh token: %s", out.ErrorDescription)
		}
		return fmt.Errorf("googledrive refresh token: %s", out.Error)
	}
	if res.IsError() {
		return fmt.Errorf("googledrive refresh token: status=%d body=%s", res.StatusCode(), strings.TrimSpace(res.String()))
	}
	if out.AccessToken == "" {
		return errors.New("googledrive refresh token: empty access_token")
	}
	d.accessToken = strings.TrimSpace(out.AccessToken)
	if strings.TrimSpace(out.RefreshToken) != "" {
		d.refreshToken = strings.TrimSpace(out.RefreshToken)
	}
	if out.ExpiresIn > 0 {
		d.accessTokenExpiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	} else {
		d.accessTokenExpiresAt = time.Now().Add(50 * time.Minute)
	}
	if d.onTokenUpdate != nil {
		d.onTokenUpdate(d.accessToken, d.refreshToken)
	}
	return nil
}

func (d *Driver) currentAccessToken() string {
	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()
	return d.accessToken
}

func (d *Driver) currentAccessTokenExpiresAt() time.Time {
	d.tokenMu.Lock()
	defer d.tokenMu.Unlock()
	return d.accessTokenExpiresAt
}

func commonFileQueryParams() map[string]string {
	return map[string]string{
		"fields":            fileFields,
		"supportsAllDrives": "true",
	}
}

func (d *Driver) fillShortcutFileMetadata(ctx context.Context, files []fileResp) error {
	for i := range files {
		item := &files[i]
		if !strings.EqualFold(item.MimeType, googleShortcutMIME) ||
			item.ShortcutDetails.TargetID == "" ||
			strings.EqualFold(item.ShortcutDetails.TargetMimeType, googleFolderMIME) {
			continue
		}
		var target fileResp
		if err := d.request(ctx, d.itemURL(item.ShortcutDetails.TargetID), http.MethodGet, func(req *resty.Request) {
			req.SetQueryParams(commonFileQueryParams())
		}, &target); err != nil {
			return err
		}
		if target.Size != "" {
			item.Size = target.Size
		}
		if target.MD5Checksum != "" {
			item.MD5Checksum = target.MD5Checksum
		}
		if target.SHA1Checksum != "" {
			item.SHA1Checksum = target.SHA1Checksum
		}
		if target.SHA256Checksum != "" {
			item.SHA256Checksum = target.SHA256Checksum
		}
		if target.ThumbnailLink != "" {
			item.ThumbnailLink = target.ThumbnailLink
		}
	}
	return nil
}

func hasGoogleError(resp googleErrorResp) bool {
	return resp.Error.Code != 0 || resp.Error.Message != "" || resp.Error.Status != "" || len(resp.Error.Errors) > 0
}

func googleErrorMessage(resp googleErrorResp) string {
	if strings.TrimSpace(resp.Error.Message) != "" {
		return strings.TrimSpace(resp.Error.Message)
	}
	if resp.Error.Status != "" {
		return resp.Error.Status
	}
	for _, detail := range resp.Error.Errors {
		if strings.TrimSpace(detail.Message) != "" {
			return strings.TrimSpace(detail.Message)
		}
		if strings.TrimSpace(detail.Reason) != "" {
			return strings.TrimSpace(detail.Reason)
		}
	}
	return ""
}

func isAuthError(res *resty.Response, resp googleErrorResp) bool {
	if res != nil && res.StatusCode() == http.StatusUnauthorized {
		return true
	}
	if strings.EqualFold(resp.Error.Status, "UNAUTHENTICATED") {
		return true
	}
	for _, reason := range googleErrorReasons(resp) {
		switch reason {
		case "autherror", "invalidauthentication", "invalidcredentials", "required":
			return true
		}
	}
	return false
}

func isRateLimitResponse(res *resty.Response, resp googleErrorResp) bool {
	if res != nil && res.StatusCode() == http.StatusTooManyRequests {
		return true
	}
	for _, reason := range googleErrorReasons(resp) {
		switch reason {
		case "ratelimitexceeded", "userratelimitexceeded", "dailylimitexceeded", "quotaexceeded", "resourceexhausted":
			return true
		}
	}
	if isRateLimitMessage(resp.Error.Message) || isRateLimitMessage(resp.Error.Status) {
		return true
	}
	if res == nil || res.Header().Get("Retry-After") == "" {
		return false
	}
	switch res.StatusCode() {
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func googleErrorReasons(resp googleErrorResp) []string {
	out := make([]string, 0, len(resp.Error.Errors))
	for _, detail := range resp.Error.Errors {
		out = append(out, normalizeReason(detail.Reason))
	}
	if resp.Error.Status != "" {
		out = append(out, normalizeReason(resp.Error.Status))
	}
	return out
}

func normalizeReason(reason string) string {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	return normalized
}

func isRateLimitMessage(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return strings.Contains(text, "too many requests") ||
		strings.Contains(text, "rate limit") ||
		strings.Contains(text, "quota") ||
		strings.Contains(text, "throttl")
}

func googleDriveRateLimitError(res *resty.Response, message string) error {
	if strings.TrimSpace(message) == "" {
		message = "googledrive rate limited"
	}
	if res != nil && strings.TrimSpace(res.String()) != "" {
		message = fmt.Sprintf("%s: status=%d body=%s", message, res.StatusCode(), strings.TrimSpace(res.String()))
	}
	return &drives.RateLimitError{
		Provider:   Kind,
		RetryAfter: parseRetryAfter(res),
		Err:        errors.New(message),
	}
}

func parseRetryAfter(res *resty.Response) time.Duration {
	if res == nil {
		return 0
	}
	return parseRetryAfterHeader(res.Header().Get("Retry-After"))
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
		d := time.Until(when)
		if d > 0 {
			return d
		}
	}
	return 0
}

func (d *Driver) filesURL() string {
	return d.apiBaseURL + "/files"
}

func (d *Driver) itemURL(itemID string) string {
	if itemID == "" {
		itemID = d.rootID
	}
	return d.filesURL() + "/" + url.PathEscape(itemID)
}

func fileToEntry(item fileResp, fallbackParentID string) drives.Entry {
	parentID := fallbackParentID
	if len(item.Parents) > 0 && item.Parents[0] != "" {
		parentID = item.Parents[0]
	}
	size, _ := strconv.ParseInt(item.Size, 10, 64)
	id := item.ID
	mimeType := item.MimeType
	isDir := item.MimeType == googleFolderMIME
	if strings.EqualFold(item.MimeType, googleShortcutMIME) && item.ShortcutDetails.TargetID != "" {
		id = item.ShortcutDetails.TargetID
		if item.ShortcutDetails.TargetMimeType != "" {
			mimeType = item.ShortcutDetails.TargetMimeType
		}
		isDir = strings.EqualFold(item.ShortcutDetails.TargetMimeType, googleFolderMIME)
	}
	if mimeType == "" && !isDir {
		mimeType = guessMime(item.Name)
	}
	hash := item.MD5Checksum
	if hash == "" {
		hash = item.SHA1Checksum
	}
	if hash == "" {
		hash = item.SHA256Checksum
	}
	return drives.Entry{
		ID:           id,
		Name:         item.Name,
		Size:         size,
		Hash:         hash,
		IsDir:        isDir,
		ParentID:     parentID,
		MimeType:     mimeType,
		ModTime:      item.ModifiedTime,
		ThumbnailURL: item.ThumbnailLink,
	}
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func escapeDriveQueryString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `\'`)
	return value
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
