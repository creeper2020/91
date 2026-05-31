package googledrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/video-site/backend/internal/drives"
)

const uploadFileFields = "id,name,mimeType,size,md5Checksum,parents"

// UploadResult 是 UploadAndReportHash 的返回值。
//
// Hash 是 Google Drive 对普通二进制文件返回的 md5Checksum，可写入 catalog.content_hash
// 用于后续扫盘去重。Google Docs/快捷方式不会有 md5，这里只用于 spider91 上传的视频。
type UploadResult struct {
	FileID string
	Hash   string
	Size   int64
}

// Upload 实现 drives.Drive 接口；只返回 fileID。
// 完整上传元数据见 UploadAndReportHash。
func (d *Driver) Upload(ctx context.Context, parentID, name string, r io.Reader, size int64) (string, error) {
	res, err := d.UploadAndReportHash(ctx, parentID, name, r, size)
	if err != nil {
		return "", err
	}
	return res.FileID, nil
}

// UploadAndReportHash 通过 Google Drive resumable upload 上传文件并返回 file ID + MD5。
//
// 这里不把视频读进内存，也不先复制到临时文件；调用方传入的本地文件流会直接发给
// Google Drive。认证刷新失败或上传中断时返回错误，迁移器会保留本地源文件等待下次重试。
func (d *Driver) UploadAndReportHash(ctx context.Context, parentID, name string, r io.Reader, size int64) (UploadResult, error) {
	if r == nil {
		return UploadResult{}, errors.New("googledrive upload: nil reader")
	}
	if size < 0 {
		return UploadResult{}, fmt.Errorf("googledrive upload: invalid size %d", size)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return UploadResult{}, errors.New("googledrive upload: empty file name")
	}
	parentID = strings.TrimSpace(parentID)
	if parentID == "" {
		parentID = d.rootID
	}

	uploadURL, err := d.startResumableUpload(ctx, parentID, name, size)
	if err != nil {
		return UploadResult{}, err
	}
	return d.finishResumableUpload(ctx, uploadURL, name, r, size)
}

// Rename 修改 Google Drive 文件名。spider91 迁移后的文件名回填会用到。
func (d *Driver) Rename(ctx context.Context, fileID, newName string) error {
	fileID = strings.TrimSpace(fileID)
	newName = strings.TrimSpace(newName)
	if fileID == "" {
		return errors.New("googledrive rename: empty file id")
	}
	if newName == "" {
		return errors.New("googledrive rename: empty file name")
	}
	var item fileResp
	if err := d.request(ctx, d.itemURL(fileID), http.MethodPatch, func(req *resty.Request) {
		req.SetQueryParams(map[string]string{
			"supportsAllDrives": "true",
			"fields":            "id,name",
		})
		req.SetBody(map[string]any{"name": newName})
	}, &item); err != nil {
		return fmt.Errorf("googledrive rename: %w", err)
	}
	if item.ID == "" {
		return errors.New("googledrive rename: empty file id")
	}
	return nil
}

func (d *Driver) startResumableUpload(ctx context.Context, parentID, name string, size int64) (string, error) {
	return d.startResumableUploadOnce(ctx, parentID, name, size, true)
}

func (d *Driver) startResumableUploadOnce(ctx context.Context, parentID, name string, size int64, retry bool) (string, error) {
	if err := d.ensureAccessToken(ctx); err != nil {
		return "", fmt.Errorf("googledrive upload: %w", err)
	}
	body := map[string]any{
		"name":    name,
		"parents": []string{parentID},
	}
	req := d.client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+d.currentAccessToken()).
		SetHeader("Content-Type", "application/json; charset=UTF-8").
		SetHeader("X-Upload-Content-Type", guessMime(name)).
		SetQueryParams(map[string]string{
			"uploadType":        "resumable",
			"supportsAllDrives": "true",
			"fields":            uploadFileFields,
		}).
		SetBody(body)
	if size >= 0 {
		req.SetHeader("X-Upload-Content-Length", strconv.FormatInt(size, 10))
	}

	var googleErr googleErrorResp
	req.SetError(&googleErr)
	res, err := req.Post(d.uploadFilesURL())
	if err != nil {
		return "", fmt.Errorf("googledrive upload: start session: %w", err)
	}
	if isRateLimitResponse(res, googleErr) {
		return "", googleDriveRateLimitError(res, googleErrorMessage(googleErr))
	}
	if hasGoogleError(googleErr) {
		if isAuthError(res, googleErr) && retry {
			if err := d.refresh(ctx); err != nil {
				return "", fmt.Errorf("googledrive upload: refresh token: %w", err)
			}
			return d.startResumableUploadOnce(ctx, parentID, name, size, false)
		}
		if msg := googleErrorMessage(googleErr); msg != "" {
			return "", fmt.Errorf("googledrive upload: start session: %s", msg)
		}
		return "", fmt.Errorf("googledrive upload: start session status=%d", googleErr.Error.Code)
	}
	if res.IsError() {
		if res.StatusCode() == http.StatusUnauthorized && retry {
			if err := d.refresh(ctx); err != nil {
				return "", fmt.Errorf("googledrive upload: refresh token: %w", err)
			}
			return d.startResumableUploadOnce(ctx, parentID, name, size, false)
		}
		return "", fmt.Errorf("googledrive upload: start session status=%d body=%s", res.StatusCode(), strings.TrimSpace(res.String()))
	}
	loc := strings.TrimSpace(res.Header().Get("Location"))
	if loc == "" {
		return "", errors.New("googledrive upload: empty resumable upload URL")
	}
	return resolveUploadLocation(d.uploadFilesURL(), loc), nil
}

func (d *Driver) finishResumableUpload(ctx context.Context, uploadURL, name string, r io.Reader, size int64) (UploadResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, r)
	if err != nil {
		return UploadResult{}, fmt.Errorf("googledrive upload: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.currentAccessToken())
	req.Header.Set("Content-Type", guessMime(name))
	if size >= 0 {
		req.ContentLength = size
		req.Header.Set("Content-Length", strconv.FormatInt(size, 10))
		if size > 0 {
			req.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", size-1, size))
		}
	}

	httpClient := *d.client.GetClient()
	httpClient.Timeout = 0
	res, err := httpClient.Do(req)
	if err != nil {
		return UploadResult{}, fmt.Errorf("googledrive upload: put body: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return UploadResult{}, fmt.Errorf("googledrive upload: read response: %w", err)
	}
	var googleErr googleErrorResp
	_ = json.Unmarshal(data, &googleErr)
	if isHTTPRateLimitResponse(res.StatusCode, res.Header, googleErr) {
		return UploadResult{}, googleDriveHTTPRateLimitError(res.StatusCode, res.Header, string(data), googleErrorMessage(googleErr))
	}
	if hasGoogleError(googleErr) {
		if msg := googleErrorMessage(googleErr); msg != "" {
			return UploadResult{}, fmt.Errorf("googledrive upload: %s", msg)
		}
		return UploadResult{}, fmt.Errorf("googledrive upload: status=%d", googleErr.Error.Code)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return UploadResult{}, fmt.Errorf("googledrive upload: status=%d body=%s", res.StatusCode, strings.TrimSpace(string(data)))
	}

	var item fileResp
	if err := json.Unmarshal(data, &item); err != nil {
		return UploadResult{}, fmt.Errorf("googledrive upload: decode response: %w", err)
	}
	if item.ID == "" {
		return UploadResult{}, errors.New("googledrive upload: empty file id")
	}
	actualSize := size
	if parsed, err := strconv.ParseInt(item.Size, 10, 64); err == nil && parsed >= 0 {
		actualSize = parsed
	}
	return UploadResult{FileID: item.ID, Hash: item.MD5Checksum, Size: actualSize}, nil
}

func (d *Driver) uploadFilesURL() string {
	base := strings.TrimRight(d.apiBaseURL, "/")
	if strings.HasSuffix(base, "/drive/v3") {
		return strings.TrimSuffix(base, "/drive/v3") + "/upload/drive/v3/files"
	}
	return base + "/upload/files"
}

func isHTTPRateLimitResponse(status int, header http.Header, resp googleErrorResp) bool {
	if status == http.StatusTooManyRequests {
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
	if header.Get("Retry-After") == "" {
		return false
	}
	switch status {
	case http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func googleDriveHTTPRateLimitError(status int, header http.Header, body, message string) error {
	if strings.TrimSpace(message) == "" {
		message = "googledrive rate limited"
	}
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		message = fmt.Sprintf("%s: status=%d body=%s", message, status, trimmed)
	}
	return &drives.RateLimitError{
		Provider:   Kind,
		RetryAfter: parseRetryAfterHeader(header.Get("Retry-After")),
		Err:        errors.New(message),
	}
}

func resolveUploadLocation(base, loc string) string {
	u, err := url.Parse(loc)
	if err != nil || u.IsAbs() {
		return loc
	}
	b, err := url.Parse(base)
	if err != nil {
		return loc
	}
	return b.ResolveReference(u).String()
}
