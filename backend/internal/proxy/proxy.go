package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/video-site/backend/internal/drives"
)

type streamURLWithHeader interface {
	StreamURLWithHeader(ctx context.Context, fileID string, header http.Header) (*drives.StreamLink, error)
}

// Registry 管理多个 Drive 实例
type Registry struct {
	mu     sync.RWMutex
	drives map[string]drives.Drive
}

func NewRegistry() *Registry {
	return &Registry{drives: make(map[string]drives.Drive)}
}

func (r *Registry) Set(id string, d drives.Drive) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drives[id] = d
}

func (r *Registry) Get(id string) (drives.Drive, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drives[id]
	return d, ok
}

func (r *Registry) All() []drives.Drive {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]drives.Drive, 0, len(r.drives))
	for _, d := range r.drives {
		out = append(out, d)
	}
	return out
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.drives, id)
}

// Proxy 根据 driveID + fileID 反向代理到真实网盘直链
type Proxy struct {
	Registry *Registry
	// linkCache key: driveID + "/" + fileID (+ User-Agent for UA-bound links)
	cacheMu sync.Mutex
	cache   map[string]cachedLink
	http    *http.Client
	Logf    func(format string, args ...any)
	diagSeq atomic.Uint64
}

type cachedLink struct {
	link    *drives.StreamLink
	fetched time.Time
}

func New(r *Registry) *Proxy {
	return &Proxy{
		Registry: r,
		cache:    make(map[string]cachedLink),
		http: &http.Client{
			Timeout: 0, // 流式不设超时
		},
		Logf: log.Printf,
	}
}

func (p *Proxy) getLink(ctx context.Context, d drives.Drive, driveID, fileID string, header http.Header) (*drives.StreamLink, error) {
	key := linkCacheKey(d, driveID, fileID, header)

	p.cacheMu.Lock()
	if c, ok := p.cache[key]; ok {
		// 缓存 30 秒，且不超过 link.Expires
		if time.Since(c.fetched) < 30*time.Second && time.Now().Before(c.link.Expires) {
			p.cacheMu.Unlock()
			return c.link, nil
		}
	}
	p.cacheMu.Unlock()

	var (
		link *drives.StreamLink
		err  error
	)
	if h, ok := d.(streamURLWithHeader); ok {
		link, err = h.StreamURLWithHeader(ctx, fileID, header)
	} else {
		link, err = d.StreamURL(ctx, fileID)
	}
	if err != nil {
		return nil, err
	}
	p.cacheMu.Lock()
	p.cache[key] = cachedLink{link: link, fetched: time.Now()}
	p.cacheMu.Unlock()
	return link, nil
}

func linkCacheKey(d drives.Drive, driveID, fileID string, header http.Header) string {
	key := driveID + "/" + fileID
	if _, ok := d.(streamURLWithHeader); ok {
		key += "|ua=" + header.Get("User-Agent")
	}
	return key
}

func (p *Proxy) ServeStream(w http.ResponseWriter, r *http.Request, driveID, fileID string) {
	diag := playbackDiagnostics{
		ID:      p.nextPlaybackDiagnosticID(),
		Started: time.Now(),
		Range:   r.Header.Get("Range"),
	}
	w.Header().Set("X-Playback-Diagnostic-Id", diag.ID)

	d, ok := p.Registry.Get(driveID)
	if !ok {
		http.Error(w, errDriveNotFound.Error(), errDriveNotFound.Code)
		return
	}
	diag.DriveKind = d.Kind()
	diag.DriveHash = shortDiagnosticHash(driveID)

	linkStart := time.Now()
	link, err := p.getLink(r.Context(), d, driveID, fileID, r.Header)
	diag.LinkDuration = time.Since(linkStart)
	if err != nil {
		p.logPlayback("[playback] id=%s mode=link_error drive_kind=%s drive=%s range=%q link_ms=%d total_ms=%d error=%q",
			diag.ID, diag.DriveKind, diag.DriveHash, diag.Range, durationMS(diag.LinkDuration), durationMS(time.Since(diag.Started)), safeErrorText(err))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if shouldRedirect(d) {
		w.Header().Set("X-Playback-Mode", "redirect")
		redirect(w, r, link)
		p.logPlayback("[playback] id=%s mode=redirect drive_kind=%s drive=%s status=%d range=%q link_ms=%d total_ms=%d",
			diag.ID, diag.DriveKind, diag.DriveHash, http.StatusFound, diag.Range, durationMS(diag.LinkDuration), durationMS(time.Since(diag.Started)))
		return
	}
	w.Header().Set("X-Playback-Mode", "proxy")
	p.serve(w, r, link, diag)
}

// shouldRedirect 返回 true 时，/p/stream 不再反代视频字节，
// 而是用 302 让浏览器直连网盘 CDN。
//
// 只把"自己签名 URL 即可下载、不需要持久 Header 鉴权"的网盘放进来：
//   - p115：CDN 签名链接，UA 通过 streamURLWithHeader 在取链时使用，
//     302 之后浏览器用自己的 UA 直连，CDN 仍然认签名
//   - pikpak：与 OpenList 一致，WebContentLink / media link 都是自签 URL，
//     CDN 不校验请求头，直连可获得最佳带宽并避免占用 backend 出站
//
// Google Drive 也走反代：Drive API media 下载需要后端持有的 Authorization
// Header，不能 302 给浏览器。OneDrive 也走反代：Graph downloadUrl 虽然
// 是短期免鉴权下载 URL，但浏览器直连会暴露临时地址，并绕过服务器侧访问控制。
//
// 其余网盘（如沃盘 / 夸克等）仍走反代，因为它们的下载
// 链接通常需要随请求带上后端持有的 Cookie / Authorization / Range
// 的特殊处理，浏览器拿不到这些上下文。
func shouldRedirect(d drives.Drive) bool {
	switch d.Kind() {
	case "p115", "pikpak":
		return true
	}
	return false
}

func redirect(w http.ResponseWriter, r *http.Request, link *drives.StreamLink) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
	http.Redirect(w, r, link.URL, http.StatusFound)
}

func (p *Proxy) serve(w http.ResponseWriter, r *http.Request, link *drives.StreamLink, diag playbackDiagnostics) {
	// 构造上游请求
	u, err := url.Parse(link.URL)
	if err != nil {
		p.logPlayback("[playback] id=%s mode=proxy drive_kind=%s drive=%s status=0 range=%q link_ms=%d upstream_header_ms=0 first_body_ms=-1 bytes=0 total_ms=%d error=%q",
			diag.ID, diag.DriveKind, diag.DriveHash, diag.Range, durationMS(diag.LinkDuration), durationMS(time.Since(diag.Started)), "bad upstream url")
		http.Error(w, "bad upstream url", http.StatusBadGateway)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, u.String(), nil)
	if err != nil {
		p.logPlayback("[playback] id=%s mode=proxy drive_kind=%s drive=%s status=0 range=%q link_ms=%d upstream_header_ms=0 first_body_ms=-1 bytes=0 total_ms=%d error=%q",
			diag.ID, diag.DriveKind, diag.DriveHash, diag.Range, durationMS(diag.LinkDuration), durationMS(time.Since(diag.Started)), safeErrorText(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 复制上游请求头
	for k, vs := range link.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	// 透传 Range
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	upstreamStart := time.Now()
	resp, err := p.http.Do(req)
	upstreamHeaderDuration := time.Since(upstreamStart)
	if err != nil {
		p.logPlayback("[playback] id=%s mode=proxy drive_kind=%s drive=%s status=0 range=%q link_ms=%d upstream_header_ms=%d first_body_ms=-1 bytes=0 total_ms=%d error=%q",
			diag.ID, diag.DriveKind, diag.DriveHash, diag.Range, durationMS(diag.LinkDuration), durationMS(upstreamHeaderDuration), durationMS(time.Since(diag.Started)), safeErrorText(err))
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 透传响应头
	for _, k := range []string{
		"Content-Type", "Content-Length", "Content-Range",
		"Accept-Ranges", "Last-Modified", "Etag",
	} {
		if v := resp.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(resp.StatusCode)

	var (
		firstBodyDuration time.Duration
		sawFirstBodyByte  bool
	)
	body := &firstByteReader{
		r: resp.Body,
		onFirstByte: func() {
			firstBodyDuration = time.Since(upstreamStart)
			sawFirstBodyByte = true
		},
	}
	copied, copyErr := io.Copy(w, body)
	firstBodyMS := int64(-1)
	if sawFirstBodyByte {
		firstBodyMS = durationMS(firstBodyDuration)
	}
	p.logPlayback("[playback] id=%s mode=proxy drive_kind=%s drive=%s status=%d range=%q link_ms=%d upstream_header_ms=%d first_body_ms=%d bytes=%d total_ms=%d error=%q",
		diag.ID, diag.DriveKind, diag.DriveHash, resp.StatusCode, diag.Range, durationMS(diag.LinkDuration), durationMS(upstreamHeaderDuration), firstBodyMS, copied, durationMS(time.Since(diag.Started)), safeErrorText(copyErr))
}

// ServeLocal 服务本地 teaser 文件
func (p *Proxy) ServeLocal(w http.ResponseWriter, r *http.Request, path string) {
	http.ServeFile(w, r, path)
}

var errDriveNotFound = &httpError{Code: http.StatusNotFound, Msg: "drive not found"}

type httpError struct {
	Code int
	Msg  string
}

func (e *httpError) Error() string { return e.Msg }

type playbackDiagnostics struct {
	ID           string
	DriveKind    string
	DriveHash    string
	Range        string
	Started      time.Time
	LinkDuration time.Duration
}

type firstByteReader struct {
	r           io.Reader
	seen        bool
	onFirstByte func()
}

func (r *firstByteReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 && !r.seen {
		r.seen = true
		if r.onFirstByte != nil {
			r.onFirstByte()
		}
	}
	return n, err
}

func (p *Proxy) nextPlaybackDiagnosticID() string {
	seq := p.diagSeq.Add(1)
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), seq)
}

func (p *Proxy) logPlayback(format string, args ...any) {
	if p.Logf != nil {
		p.Logf(format, args...)
	}
}

func durationMS(d time.Duration) int64 {
	return d.Milliseconds()
}

func shortDiagnosticHash(s string) string {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

func safeErrorText(err error) string {
	if err == nil {
		return ""
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return truncateDiagnosticText(urlErr.Op + ": " + redactURLText(urlErr.Err.Error()))
	}
	return truncateDiagnosticText(redactURLText(err.Error()))
}

func redactURLText(s string) string {
	for _, scheme := range []string{"https://", "http://"} {
		for {
			idx := strings.Index(s, scheme)
			if idx < 0 {
				break
			}
			end := idx + len(scheme)
			for end < len(s) {
				switch s[end] {
				case ' ', '\t', '\n', '\r', '"', '\'':
					goto replace
				}
				end++
			}
		replace:
			s = s[:idx] + "[redacted-url]" + s[end:]
		}
	}
	return s
}

func truncateDiagnosticText(s string) string {
	if len(s) <= 160 {
		return s
	}
	return s[:157] + "..."
}
