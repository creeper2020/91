package api

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/video-site/backend/internal/auth"
	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives/localupload"
	"github.com/video-site/backend/internal/drives/spider91"
	"github.com/video-site/backend/internal/proxy"
)

const localUploadDriveID = localupload.DriveID

var allowedUploadExtensions = map[string]struct{}{
	".avi":  {},
	".flv":  {},
	".m4v":  {},
	".mkv":  {},
	".mov":  {},
	".mp4":  {},
	".mpeg": {},
	".mpg":  {},
	".ts":   {},
	".webm": {},
	".wmv":  {},
}

var allowedUploadTags = map[string]struct{}{
	"奶子": {},
	"臀":  {},
	"口角": {},
	"女大": {},
	"人妻": {},
	"AV": {},
}

type Server struct {
	Catalog         *catalog.Catalog
	Proxy           *proxy.Proxy
	LocalDir        string
	UploadDir       string
	OnVideoUploaded func(*catalog.Video)
	Importer        *ImportManager
	// ExternalImportToken enables token-protected import/upload endpoints for
	// automation such as the Telegram bot. Empty disables those endpoints.
	ExternalImportToken string

	// GetTheme 返回当前生效的主题（"dark" | "pink"）。前台 /api/settings/theme 用，
	// 不需要登录。无注入时返回 "dark"。
	GetTheme func() string
}

const (
	homePageSize = 12
)

// VideoDTO 是返回给前端的视频对象，字段名跟前端 VideoItem 对齐
type VideoDTO struct {
	ID              string   `json:"id"`
	Href            string   `json:"href"`
	Title           string   `json:"title"`
	Thumbnail       string   `json:"thumbnail"`
	PreviewSrc      string   `json:"previewSrc"`
	PreviewDuration int      `json:"previewDuration"`
	PreviewStrategy string   `json:"previewStrategy"`
	Duration        string   `json:"duration"`
	Badges          []string `json:"badges"`
	Quality         string   `json:"quality,omitempty"`
	SourceLabel     string   `json:"sourceLabel,omitempty"`
	Author          string   `json:"author"`
	Views           int      `json:"views"`
	Favorites       int      `json:"favorites"`
	Comments        int      `json:"comments"`
	Likes           int      `json:"likes"`
	Dislikes        int      `json:"dislikes"`
	PublishedAt     string   `json:"publishedAt"`
	Tags            []string `json:"tags,omitempty"`
	Category        string   `json:"category,omitempty"`
}

type VideoDetailDTO struct {
	VideoDTO
	VideoSrc      string        `json:"videoSrc"`
	HLSSrc        string        `json:"hlsSrc,omitempty"`
	Poster        string        `json:"poster"`
	Description   string        `json:"description"`
	EmbedURL      string        `json:"embedUrl"`
	Points        int           `json:"points,omitempty"`
	AuthorProfile AuthorProfile `json:"authorProfile"`
	RelatedVideos []VideoDTO    `json:"relatedVideos"`
	CommentsList  []Comment     `json:"commentsList"`
}

type AuthorProfile struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Href   string   `json:"href"`
	Badges []string `json:"badges"`
}

type Comment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	Likes     int    `json:"likes,omitempty"`
}

// RegisterRoutes 挂载前台 REST 路由。前台接口需要登录态。
func (s *Server) RegisterRoutes(r chi.Router, a *auth.Authenticator) {
	// 公开端点：拿当前生效的主题。登录页本身要在挂前就能读，所以单独挂在
	// 鉴权组之外。只暴露 theme 一个字段，避免泄露其他设置。
	r.Get("/api/settings/theme", s.handleGetTheme)
	r.Post("/api/imports/external", s.handleExternalCreateImport)
	r.Get("/api/imports/external/{id}", s.handleExternalGetImport)
	r.Post("/api/imports/external-upload", s.handleExternalUpload)

	r.Group(func(r chi.Router) {
		r.Use(a.Required)
		r.Get("/api/home", s.handleHome)
		r.Get("/api/list", s.handleList)
		r.Get("/api/video/{id}", s.handleVideoDetail)
		r.Put("/api/video/{id}/tags", s.handleUpdateVideoTags)
		r.Post("/api/video/{id}/like", s.handleLike)
		r.Delete("/api/video/{id}/like", s.handleUnlike)
		r.Post("/api/video/{id}/view", s.handleView)
		r.Post("/api/video/{id}/hide", s.handleHideVideo)
		r.Post("/api/upload", s.handleUploadVideo)
		r.Post("/api/imports", s.handleCreateImport)
		r.Get("/api/imports/{id}", s.handleGetImport)
		r.Get("/api/tags", s.handleTags)
		r.Post("/api/shorts/next", s.handleShortsNext)

		// 代理路由同样需要鉴权，防止绕过
		r.Get("/p/stream/{driveID}/{fileID}", s.handleStream)
		r.Get("/p/upload/{videoID}", s.handleUploadedVideo)
		r.Get("/p/spider91/{videoID}", s.handleSpider91Video)
		r.Get("/p/preview/{videoID}", s.handlePreview)
		r.Get("/p/thumb/{videoID}", s.handleThumb)
		r.Get("/p/hls/{videoID}/{file}", s.handleHLS)
	})
}

// handleGetTheme 返回当前生效的主题。无需登录。响应永远是
// {"theme": "dark"} 或 {"theme": "pink"}，便于前端无脑解析。
func (s *Server) handleGetTheme(w http.ResponseWriter, r *http.Request) {
	theme := "dark"
	if s.GetTheme != nil {
		if v := s.GetTheme(); v == "pink" || v == "dark" {
			theme = v
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"theme": theme})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	items, err := s.pickHomeRecommendations(r.Context(), homePageSize)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, mapVideos(items))
}

func (s *Server) pickHomeRecommendations(ctx context.Context, total int) ([]*catalog.Video, error) {
	if total <= 0 {
		return nil, nil
	}

	// 首页候选优先来自已生成封面的视频，避免扫盘或预览任务排队时黑封面占满首屏。
	// 主页保留随机展示语义，避免每次都固定命中同一批头部视频。
	const candidatePool = 200
	readyItems, _, err := s.Catalog.ListVideos(ctx, catalog.ListParams{
		Sort: "latest", Page: 1, PageSize: candidatePool, ThumbnailReadyOnly: true,
	})
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(readyItems), func(i, j int) {
		readyItems[i], readyItems[j] = readyItems[j], readyItems[i]
	})

	items := appendUniqueVideos(nil, readyItems, total)
	if len(items) >= total {
		return items, nil
	}

	fallback, _, err := s.Catalog.ListVideos(ctx, catalog.ListParams{
		Sort: "latest", Page: 1, PageSize: candidatePool, PreferReadyThumbnails: true,
	})
	if err != nil {
		return nil, err
	}
	rand.Shuffle(len(fallback), func(i, j int) {
		fallback[i], fallback[j] = fallback[j], fallback[i]
	})
	return appendUniqueVideos(items, fallback, total), nil
}

func appendUniqueVideos(dst []*catalog.Video, candidates []*catalog.Video, limit int) []*catalog.Video {
	if len(dst) >= limit {
		return dst[:limit]
	}
	seen := make(map[string]struct{}, len(dst))
	for _, v := range dst {
		if v != nil {
			seen[v.ID] = struct{}{}
		}
	}
	for _, v := range candidates {
		if v == nil {
			continue
		}
		if _, ok := seen[v.ID]; ok {
			continue
		}
		dst = append(dst, v)
		seen[v.ID] = struct{}{}
		if len(dst) >= limit {
			return dst
		}
	}
	return dst
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("size"))
	if size <= 0 {
		size = 24
	}
	sort := q.Get("sort")
	params := catalog.ListParams{
		Keyword:  q.Get("q"),
		Tag:      q.Get("tag"),
		Category: q.Get("cat"),
		Sort:     sort,
		Page:     page,
		PageSize: size,
	}
	if sort == "" || sort == "latest" {
		params.PreferReadyThumbnails = true
	}
	items, total, err := s.Catalog.ListVideos(r.Context(), params)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": mapVideos(items),
		"total": total,
		"page":  params.Page,
		"size":  params.PageSize,
	})
}

func (s *Server) handleVideoDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	v, err := s.Catalog.GetVideo(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if v.Hidden {
		writeErr(w, http.StatusNotFound, sql.ErrNoRows)
		return
	}
	related := s.pickRelatedVideos(r.Context(), v, 6)
	dto := mapVideo(v)
	if d, err := s.Catalog.GetDrive(r.Context(), v.DriveID); err == nil {
		dto.SourceLabel = driveKindLabel(d.Kind)
	}

	detail := VideoDetailDTO{
		VideoDTO:    dto,
		VideoSrc:    s.videoSource(v),
		HLSSrc:      s.hlsSource(v),
		Poster:      thumbnailURL(v),
		Description: v.Description,
		EmbedURL:    fmt.Sprintf(`<iframe src="/embed/%s" width="640" height="360" frameborder="0" allowfullscreen></iframe>`, v.ID),
		AuthorProfile: AuthorProfile{
			ID:     "author-" + v.Author,
			Name:   v.Author,
			Href:   "/author/" + v.Author,
			Badges: []string{},
		},
		RelatedVideos: mapVideos(related),
		CommentsList:  []Comment{},
	}
	// 推荐每次随机生成，禁止浏览器和中间层缓存详情响应
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, detail)
}

// pickRelatedVideos 选 total 个推荐视频。
// 推荐先按已生成封面的候选排序，不足时再回退到普通可见视频。排序优先考虑
// 标签重合，其次是分类、作者和来源目录；互动热度只在相似度接近时做轻微排序。
func (s *Server) pickRelatedVideos(ctx context.Context, current *catalog.Video, total int) []*catalog.Video {
	if total <= 0 || current == nil {
		return nil
	}

	picked := make([]*catalog.Video, 0, total)
	seen := map[string]struct{}{current.ID: {}}
	now := time.Now()

	readyPool := s.relatedCandidatePool(ctx, current, seen, true)
	picked = appendRankedRelated(picked, rankVideos(readyPool, total, func(v *catalog.Video) int {
		return relatedRecommendationScore(current, v, now)
	}), total, seen)
	if len(picked) >= total {
		return picked
	}

	fallbackPool := s.relatedCandidatePool(ctx, current, seen, false)
	return appendRankedRelated(picked, rankVideos(fallbackPool, total, func(v *catalog.Video) int {
		return relatedRecommendationScore(current, v, now)
	}), total, seen)
}

func (s *Server) relatedCandidatePool(ctx context.Context, current *catalog.Video, seen map[string]struct{}, readyOnly bool) []*catalog.Video {
	pool := make([]*catalog.Video, 0, 240)
	pool = append(pool, s.relatedTagPool(ctx, current.Tags, seen, readyOnly)...)
	pool = append(pool, s.relatedCategoryPool(ctx, current.Category, seen, readyOnly)...)
	pool = append(pool, s.relatedListPool(ctx, seen, readyOnly, 200)...)
	return dedupeCandidatePool(pool, seen)
}

func (s *Server) relatedTagPool(ctx context.Context, tags []string, seen map[string]struct{}, readyOnly bool) []*catalog.Video {
	var pool []*catalog.Video
	poolSeen := make(map[string]struct{})
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		items, _, err := s.Catalog.ListVideos(ctx, catalog.ListParams{
			Tag:                   tag,
			Sort:                  "latest",
			Page:                  1,
			PageSize:              30,
			ThumbnailReadyOnly:    readyOnly,
			PreferReadyThumbnails: !readyOnly,
		})
		if err != nil {
			continue
		}
		for _, v := range items {
			if v == nil {
				continue
			}
			if _, ok := seen[v.ID]; ok {
				continue
			}
			if _, ok := poolSeen[v.ID]; ok {
				continue
			}
			poolSeen[v.ID] = struct{}{}
			pool = append(pool, v)
		}
	}
	return pool
}

func (s *Server) relatedCategoryPool(ctx context.Context, category string, seen map[string]struct{}, readyOnly bool) []*catalog.Video {
	category = strings.TrimSpace(category)
	if category == "" {
		return nil
	}
	items, _, err := s.Catalog.ListVideos(ctx, catalog.ListParams{
		Category:              category,
		Sort:                  "latest",
		Page:                  1,
		PageSize:              60,
		ThumbnailReadyOnly:    readyOnly,
		PreferReadyThumbnails: !readyOnly,
	})
	if err != nil {
		return nil
	}
	pool := make([]*catalog.Video, 0, len(items))
	for _, v := range items {
		if v == nil {
			continue
		}
		if _, ok := seen[v.ID]; ok {
			continue
		}
		pool = append(pool, v)
	}
	return pool
}

func (s *Server) relatedListPool(ctx context.Context, seen map[string]struct{}, readyOnly bool, pageSize int) []*catalog.Video {
	items, _, err := s.Catalog.ListVideos(ctx, catalog.ListParams{
		Sort:                  "latest",
		Page:                  1,
		PageSize:              pageSize,
		ThumbnailReadyOnly:    readyOnly,
		PreferReadyThumbnails: !readyOnly,
	})
	if err != nil {
		return nil
	}
	pool := make([]*catalog.Video, 0, len(items))
	for _, v := range items {
		if v == nil {
			continue
		}
		if _, ok := seen[v.ID]; ok {
			continue
		}
		pool = append(pool, v)
	}
	return pool
}

func dedupeCandidatePool(candidates []*catalog.Video, seen map[string]struct{}) []*catalog.Video {
	pool := make([]*catalog.Video, 0, len(candidates))
	poolSeen := make(map[string]struct{}, len(candidates))
	for _, v := range candidates {
		if v == nil {
			continue
		}
		if _, ok := seen[v.ID]; ok {
			continue
		}
		if _, ok := poolSeen[v.ID]; ok {
			continue
		}
		poolSeen[v.ID] = struct{}{}
		pool = append(pool, v)
	}
	return pool
}

func appendRankedRelated(picked []*catalog.Video, ranked []*catalog.Video, targetLen int, seen map[string]struct{}) []*catalog.Video {
	if len(picked) >= targetLen || len(ranked) == 0 {
		return picked
	}
	for _, v := range ranked {
		if len(picked) >= targetLen {
			break
		}
		if v == nil {
			continue
		}
		if _, ok := seen[v.ID]; ok {
			continue
		}
		seen[v.ID] = struct{}{}
		picked = append(picked, v)
	}
	return picked
}

func rankVideos(candidates []*catalog.Video, limit int, score func(*catalog.Video) int) []*catalog.Video {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	ranked := dedupeCandidatePool(candidates, nil)
	rand.Shuffle(len(ranked), func(i, j int) {
		ranked[i], ranked[j] = ranked[j], ranked[i]
	})
	sort.SliceStable(ranked, func(i, j int) bool {
		left := ranked[i]
		right := ranked[j]
		leftScore := score(left)
		rightScore := score(right)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if !left.PublishedAt.Equal(right.PublishedAt) {
			return left.PublishedAt.After(right.PublishedAt)
		}
		return left.ID < right.ID
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func relatedRecommendationScore(current *catalog.Video, v *catalog.Video, now time.Time) int {
	if current == nil || v == nil {
		return 0
	}
	score := 0
	score += sharedTagCount(current.Tags, v.Tags) * 100000
	if sameNonEmpty(current.Category, v.Category) {
		score += 40000
	}
	if sameNonEmpty(current.Author, v.Author) {
		score += 25000
	}
	if current.ParentID != "" && current.ParentID == v.ParentID {
		score += 15000
	}
	if current.DriveID != "" && current.DriveID == v.DriveID {
		score += 3000
	}
	if isPreviewReady(v) {
		score += 1200
	}
	score += relatedEngagementScore(v)
	score += relatedFreshnessScore(v.PublishedAt, now)
	return score
}

func relatedEngagementScore(v *catalog.Video) int {
	if v == nil {
		return 0
	}
	score := 0
	score += minInt(v.Likes*10, 600)
	score += minInt(v.Favorites*8, 400)
	score += minInt(v.Comments*5, 200)
	score += minInt(v.Views/200, 300)
	return score
}

func relatedFreshnessScore(publishedAt time.Time, now time.Time) int {
	if publishedAt.IsZero() {
		return 0
	}
	days := int(now.Sub(publishedAt).Hours() / 24)
	if days < 0 {
		days = 0
	}
	switch {
	case days <= 1:
		return 1200
	case days <= 7:
		return 900
	case days <= 30:
		return 600
	case days <= 90:
		return 300
	default:
		return 0
	}
}

func sharedTagCount(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	seen := make(map[string]struct{}, len(a))
	for _, tag := range a {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag != "" {
			seen[tag] = struct{}{}
		}
	}
	count := 0
	for _, tag := range b {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			count++
		}
	}
	return count
}

func isPreviewReady(v *catalog.Video) bool {
	return v != nil && strings.EqualFold(strings.TrimSpace(v.PreviewStatus), "ready")
}

func sameNonEmpty(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	stats, err := s.Catalog.ListTags(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	type tag struct {
		ID    string `json:"id"`
		Label string `json:"label"`
		Count int    `json:"count"`
	}
	out := make([]tag, 0, len(stats))
	for _, stat := range stats {
		out = append(out, tag{ID: stat.Label, Label: stat.Label, Count: stat.Count})
	}
	writeJSON(w, http.StatusOK, out)
}

// shortsNextReq 客户端把当前轮已看过的 video id 列表传上来。
// PreferredFromVideoID 来自短视频页最近一次点赞成功的视频，用于优先推荐相似标签。
type shortsNextReq struct {
	SeenIDs              []string `json:"seenIds"`
	Count                int      `json:"count"`
	PreferredFromVideoID string   `json:"preferredFromVideoId"`
}

// ShortsItemDTO 是短视频流单条的精简结构。比 VideoDTO 多 videoSrc / poster，
// 方便前端直接喂给 <video>，不必再为每条请求 /api/video/:id。
type ShortsItemDTO struct {
	VideoDTO
	VideoSrc string `json:"videoSrc"`
	HLSSrc   string `json:"hlsSrc,omitempty"`
	Poster   string `json:"poster"`
}

// handleShortsNext 为短视频模式提供"不重复随机视频"接口。
//
// 行为：
//   - 入参 seenIds 为客户端当前轮已看过的视频 id（来自 localStorage）
//   - 服务器从未在 seenIds 中的可见视频里随机抽至多 count 条返回
//   - 当返回数量 < count 且小于全库可见总数时，说明本轮即将结束，
//     返回 roundComplete=true，前端应在用户看完返回的这些后清空本地已看记录开新一轮
//   - 当 seenIds 已经覆盖全库时，本接口直接返回新一轮的随机一批
//     （传 seenIds=[] 即可让客户端在轮次完成后重新开始）
func (s *Server) handleShortsNext(w http.ResponseWriter, r *http.Request) {
	var body shortsNextReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	count := body.Count
	if count <= 0 {
		count = 5
	}
	if count > 20 {
		count = 20
	}

	total, err := s.Catalog.CountVisibleVideos(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// 如果客户端已看记录已经 ≥ 全库，则视为新一轮，直接忽略 seenIds
	exclude := body.SeenIDs
	if total > 0 && len(exclude) >= total {
		exclude = nil
	}

	var items []*catalog.Video
	if strings.TrimSpace(body.PreferredFromVideoID) != "" {
		items, err = s.Catalog.RandomVideosForPreferredVideoExcluding(r.Context(), body.PreferredFromVideoID, exclude, count)
	} else {
		items, err = s.Catalog.RandomVideosExcluding(r.Context(), exclude, count)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	// 注入 sourceLabel 以便前端展示来源网盘
	driveLabels := make(map[string]string)
	out := make([]ShortsItemDTO, 0, len(items))
	for _, v := range items {
		dto := mapVideo(v)
		if label, ok := driveLabels[v.DriveID]; ok {
			dto.SourceLabel = label
		} else if d, err := s.Catalog.GetDrive(r.Context(), v.DriveID); err == nil {
			label := driveKindLabel(d.Kind)
			driveLabels[v.DriveID] = label
			dto.SourceLabel = label
		}
		out = append(out, ShortsItemDTO{
			VideoDTO: dto,
			VideoSrc: s.videoSource(v),
			HLSSrc:   s.hlsSource(v),
			Poster:   thumbnailURL(v),
		})
	}

	// roundComplete: 服务端能给出的视频数小于 count，说明剩余可选已耗尽，
	// 前端把这批播完后应该清空本地 seenIds 开新一轮。
	roundComplete := len(out) < count

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"items":         out,
		"total":         total,
		"roundComplete": roundComplete,
	})
}

type updateVideoTagsReq struct {
	Tags []string `json:"tags"`
}

func (s *Server) handleUpdateVideoTags(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body updateVideoTagsReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Catalog.SetManualVideoTags(r.Context(), id, body.Tags); err != nil {
		if errors.Is(err, catalog.ErrUnknownTag) {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	v, err := s.Catalog.GetVideo(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, mapVideo(v))
}

func (s *Server) handleLike(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	likes, err := s.Catalog.IncrementLike(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"likes": likes})
}

// handleUnlike 取消点赞：likes - 1（保底 0）。
// 短视频模式中爱心按钮点击切换状态时使用。
func (s *Server) handleUnlike(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	likes, err := s.Catalog.DecrementLike(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"likes": likes})
}

func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	views, err := s.Catalog.IncrementView(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"views": views})
}

func (s *Server) handleHideVideo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.Catalog.HideVideo(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUploadVideo(w http.ResponseWriter, r *http.Request) {
	if s.localUploadDir() == "" {
		writeErr(w, http.StatusInternalServerError, errors.New("local storage is not configured"))
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, errors.New("video file is required"))
		return
	}
	defer file.Close()

	originalName := filepath.Base(strings.TrimSpace(header.Filename))
	tags, err := parseUploadTags(uploadTagValues(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	video, err := s.ingestLocalVideo(r.Context(), localVideoIngestInput{
		Reader:       file,
		OriginalName: originalName,
		Title:        r.FormValue("title"),
		Tags:         tags,
		Author:       "用户上传",
	})
	if err != nil {
		writeErr(w, uploadStatusCode(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, mapVideo(video))
}

type localVideoIngestInput struct {
	Reader       io.Reader
	OriginalName string
	Title        string
	Tags         []string
	Author       string
}

type statusError struct {
	status int
	err    error
}

func (e statusError) Error() string {
	if e.err == nil {
		return http.StatusText(e.status)
	}
	return e.err.Error()
}

func (e statusError) Unwrap() error {
	return e.err
}

func uploadStatusCode(err error) int {
	var se statusError
	if errors.As(err, &se) && se.status > 0 {
		return se.status
	}
	return http.StatusInternalServerError
}

func uploadBadRequest(err error) error {
	return statusError{status: http.StatusBadRequest, err: err}
}

func (s *Server) ingestLocalVideo(ctx context.Context, in localVideoIngestInput) (*catalog.Video, error) {
	if s.localUploadDir() == "" {
		return nil, errors.New("local storage is not configured")
	}
	if s.Catalog == nil {
		return nil, errors.New("catalog is not configured")
	}
	if in.Reader == nil {
		return nil, uploadBadRequest(errors.New("video file is required"))
	}

	originalName := filepath.Base(strings.TrimSpace(in.OriginalName))
	if originalName == "." || originalName == string(filepath.Separator) || originalName == "" {
		originalName = "video"
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if _, ok := allowedUploadExtensions[ext]; !ok {
		return nil, uploadBadRequest(fmt.Errorf("unsupported video extension: %s", ext))
	}

	now := time.Now()
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = uploadTitleFromFileName(originalName)
	}
	author := strings.TrimSpace(in.Author)
	if author == "" {
		author = "用户上传"
	}
	tags := append([]string(nil), in.Tags...)

	uploadID, err := newUploadID(now)
	if err != nil {
		return nil, err
	}
	storedName := uploadID + ext
	dst, err := s.localUploadFilePath(storedName)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return nil, err
	}
	size, copyErr := io.Copy(out, in.Reader)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return nil, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return nil, closeErr
	}
	if size <= 0 {
		_ = os.Remove(dst)
		return nil, uploadBadRequest(errors.New("uploaded video is empty"))
	}

	video := &catalog.Video{
		ID:            localUploadDriveID + "-" + uploadID,
		DriveID:       localUploadDriveID,
		FileID:        storedName,
		FileName:      originalName,
		Title:         title,
		Author:        author,
		Tags:          tags,
		Size:          size,
		Ext:           strings.TrimPrefix(ext, "."),
		PreviewStatus: "pending",
		PublishedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.Catalog.UpsertVideo(ctx, video); err != nil {
		_ = os.Remove(dst)
		return nil, err
	}
	if s.OnVideoUploaded != nil {
		s.OnVideoUploaded(video)
	}
	return video, nil
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	driveID := chi.URLParam(r, "driveID")
	fileID := chi.URLParam(r, "fileID")
	s.Proxy.ServeStream(w, r, driveID, fileID)
}
func (s *Server) handleUploadedVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	v, err := s.Catalog.GetVideo(r.Context(), videoID)
	if err != nil || v.Hidden || v.DriveID != localUploadDriveID {
		http.NotFound(w, r)
		return
	}
	path, err := s.localUploadFilePath(v.FileID)
	if err != nil {
		http.Error(w, "invalid upload file", http.StatusForbidden)
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeFile(w, r, path)
}

// handleSpider91Video 服务 spider91 drive 下载到本地的视频文件。
// 路径形如 /p/spider91/<videoID>，videoID = "spider91-<driveID>-<sourceID>"。
// 通过 catalog 拿到 file_id（"<sourceID>.mp4"），再让 driver 解析到绝对路径并 ServeFile。
func (s *Server) handleSpider91Video(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	v, err := s.Catalog.GetVideo(r.Context(), videoID)
	if err != nil || v.Hidden {
		http.NotFound(w, r)
		return
	}
	if s.Proxy == nil || s.Proxy.Registry == nil {
		http.NotFound(w, r)
		return
	}
	d, ok := s.Proxy.Registry.Get(v.DriveID)
	if !ok || d.Kind() != spider91.Kind {
		http.NotFound(w, r)
		return
	}
	sd, ok := d.(*spider91.Driver)
	if !ok {
		http.NotFound(w, r)
		return
	}
	path, err := sd.VideoPath(v.FileID)
	if err != nil {
		http.Error(w, "invalid video id", http.StatusForbidden)
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	http.ServeFile(w, r, path)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	v, err := s.Catalog.GetVideo(r.Context(), videoID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if v.PreviewStatus != "ready" {
		http.Error(w, "preview not ready", http.StatusNotFound)
		return
	}
	if v.PreviewLocal != "" {
		if !strings.HasPrefix(filepath.Clean(v.PreviewLocal), filepath.Clean(s.LocalDir)) {
			http.Error(w, "invalid local path", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		s.Proxy.ServeLocal(w, r, v.PreviewLocal)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	// 直接读本地 thumbs 目录中 <videoID>.jpg
	path := filepath.Join(s.LocalDir, "thumbs", videoID+".jpg")
	clean := filepath.Clean(path)
	if !strings.HasPrefix(clean, filepath.Clean(s.LocalDir)) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	if _, err := os.Stat(clean); err != nil {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	s.Proxy.ServeLocal(w, r, clean)
}

func (s *Server) handleHLS(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	name := chi.URLParam(r, "file")
	if !validHLSFileName(name) {
		http.Error(w, "invalid hls file", http.StatusForbidden)
		return
	}
	v, err := s.Catalog.GetVideo(r.Context(), videoID)
	if err != nil || v.Hidden || v.HLSStatus != "ready" || v.HLSDir == "" {
		http.NotFound(w, r)
		return
	}
	root := filepath.Clean(s.LocalDir)
	dir := filepath.Clean(v.HLSDir)
	if !pathWithin(root, dir) {
		http.Error(w, "invalid hls directory", http.StatusForbidden)
		return
	}
	path := filepath.Join(dir, name)
	clean := filepath.Clean(path)
	if !pathWithin(dir, clean) {
		http.Error(w, "invalid hls path", http.StatusForbidden)
		return
	}
	if _, err := os.Stat(clean); err != nil {
		http.NotFound(w, r)
		return
	}
	setHLSContentType(w, name)
	if strings.EqualFold(filepath.Ext(name), ".m3u8") {
		w.Header().Set("Cache-Control", "private, max-age=60")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=86400")
	}
	http.ServeFile(w, r, clean)
}

// ---------- helpers ----------

func mapVideo(v *catalog.Video) VideoDTO {
	badges := v.Badges
	if badges == nil {
		badges = []string{}
	}
	tags := v.Tags
	if tags == nil {
		tags = []string{}
	}
	return VideoDTO{
		ID:              v.ID,
		Href:            "/video/" + v.ID,
		Title:           v.Title,
		Thumbnail:       thumbnailURL(v),
		PreviewSrc:      previewURL(v),
		PreviewDuration: 12,
		PreviewStrategy: "teaser-file",
		Duration:        formatDuration(v.DurationSeconds),
		Badges:          badges,
		Quality:         v.Quality,
		Author:          v.Author,
		Views:           v.Views,
		Favorites:       v.Favorites,
		Comments:        v.Comments,
		Likes:           v.Likes,
		Dislikes:        v.Dislikes,
		PublishedAt:     v.PublishedAt.Format("2006-01-02"),
		Tags:            tags,
		Category:        v.Category,
	}
}

func previewURL(v *catalog.Video) string {
	base := "/p/preview/" + v.ID
	if v.UpdatedAt.IsZero() {
		return base
	}
	return base + "?v=" + strconv.FormatInt(v.UpdatedAt.UnixMilli(), 10)
}

func (s *Server) hlsSource(v *catalog.Video) string {
	if v == nil || v.HLSStatus != "ready" || v.HLSDir == "" {
		return ""
	}
	base := "/p/hls/" + v.ID + "/index.m3u8"
	if !v.HLSUpdatedAt.IsZero() {
		return base + "?v=" + strconv.FormatInt(v.HLSUpdatedAt.UnixMilli(), 10)
	}
	if !v.UpdatedAt.IsZero() {
		return base + "?v=" + strconv.FormatInt(v.UpdatedAt.UnixMilli(), 10)
	}
	return base
}

func thumbnailURL(v *catalog.Video) string {
	if v.ThumbnailURL != "" {
		return v.ThumbnailURL
	}
	return "/p/thumb/" + v.ID
}

func (s *Server) videoSource(v *catalog.Video) string {
	if v.DriveID == localUploadDriveID {
		return "/p/upload/" + v.ID
	}
	if s.Proxy != nil && s.Proxy.Registry != nil {
		if d, ok := s.Proxy.Registry.Get(v.DriveID); ok && d.Kind() == spider91.Kind {
			return "/p/spider91/" + v.ID
		}
	}
	return fmt.Sprintf("/p/stream/%s/%s", v.DriveID, v.FileID)
}

// videoSource 兼容旧调用点，没有 server context 时按之前逻辑回退到 /p/stream。
// 内部新增的代码请使用 (*Server).videoSource。
func videoSource(v *catalog.Video) string {
	if v.DriveID == localUploadDriveID {
		return "/p/upload/" + v.ID
	}
	return fmt.Sprintf("/p/stream/%s/%s", v.DriveID, v.FileID)
}

func driveKindLabel(kind string) string {
	switch kind {
	case "quark":
		return "夸克网盘"
	case "p115":
		return "115 网盘"
	case "pikpak":
		return "PikPak"
	case "wopan":
		return "联通沃盘"
	case "onedrive":
		return "OneDrive"
	case "googledrive":
		return "Google Drive"
	case spider91.Kind:
		return "91 爬虫"
	default:
		return kind
	}
}

func validHLSFileName(name string) bool {
	if strings.TrimSpace(name) == "" || filepath.Base(name) != name {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m3u8", ".m4s", ".mp4", ".ts":
		return true
	default:
		return false
	}
}

func setHLSContentType(w http.ResponseWriter, name string) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m3u8":
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	case ".m4s":
		w.Header().Set("Content-Type", "video/iso.segment")
	case ".mp4":
		w.Header().Set("Content-Type", "video/mp4")
	case ".ts":
		w.Header().Set("Content-Type", "video/mp2t")
	}
}

func pathWithin(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func (s *Server) localUploadFilePath(fileID string) (string, error) {
	if strings.TrimSpace(fileID) == "" || filepath.Base(fileID) != fileID {
		return "", errors.New("invalid upload file id")
	}
	root := s.localUploadDir()
	if root == "" {
		return "", errors.New("local upload storage is not configured")
	}
	path := filepath.Join(root, fileID)
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(os.PathSeparator)) {
		return "", errors.New("invalid upload file id")
	}
	return cleanPath, nil
}

func (s *Server) localUploadDir() string {
	if s.UploadDir != "" {
		return s.UploadDir
	}
	if s.LocalDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.LocalDir), "uploads")
}

func uploadTagValues(r *http.Request) []string {
	if r.MultipartForm == nil {
		return nil
	}
	values := append([]string{}, r.MultipartForm.Value["tags"]...)
	values = append(values, r.MultipartForm.Value["tag"]...)
	return values
}

func uploadTitleFromFileName(fileName string) string {
	name := strings.TrimSpace(filepath.Base(fileName))
	ext := filepath.Ext(name)
	if ext != "" {
		if trimmed := strings.TrimSuffix(name, ext); strings.TrimSpace(trimmed) != "" {
			return trimmed
		}
	}
	if name != "" {
		return name
	}
	return "upload-" + time.Now().Format("20060102150405")
}

func parseUploadTags(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, label := range splitUploadTags(value) {
			if _, ok := allowedUploadTags[label]; !ok {
				return nil, fmt.Errorf("unsupported upload tag: %s", label)
			}
			if _, ok := seen[label]; ok {
				continue
			}
			seen[label] = struct{}{}
			out = append(out, label)
		}
	}
	return out, nil
}

func splitUploadTags(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if label := strings.TrimSpace(field); label != "" {
			out = append(out, label)
		}
	}
	return out
}

func newUploadID(now time.Time) (string, error) {
	var suffix [6]byte
	if _, err := crand.Read(suffix[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("upload-%d-%s", now.UnixNano(), hex.EncodeToString(suffix[:])), nil
}

func mapVideos(vs []*catalog.Video) []VideoDTO {
	out := make([]VideoDTO, 0, len(vs))
	for _, v := range vs {
		out = append(out, mapVideo(v))
	}
	return out
}

func formatDuration(sec int) string {
	if sec <= 0 {
		return "00:00"
	}
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
