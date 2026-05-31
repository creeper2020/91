package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/video-site/backend/internal/auth"
	"github.com/video-site/backend/internal/catalog"
)

type AdminServer struct {
	Catalog *catalog.Catalog
	Auth    *auth.Authenticator
	// VersionFilePath points to the installer-written .version file.
	VersionFilePath string
	// GitHubRepo is the owner/name repo used for update checks.
	GitHubRepo string
	// ReleaseAPIURL and HTTPClient are injectable for tests. Production code leaves them empty.
	ReleaseAPIURL string
	HTTPClient    *http.Client
	// SetupRequired 表示当前是否仍处于首次部署初始化状态。
	SetupRequired func() bool
	// OnSetup 持久化首次部署时设置的管理员账号密码，并更新运行中认证器。
	OnSetup func(username, password string) error
	// LocalPreviewDir is the local directory that stores generated teasers and thumbs.
	LocalPreviewDir string
	// Hooks：外层注入实际执行者
	OnDriveSaved               func(driveID string) error
	OnDriveRemoved             func(driveID string)
	OnScanRequested            func(driveID string)
	OnRegenPreview             func(videoID string)
	OnGenerateHLS              func(videoID string)
	OnRegenAllPreviews         func()
	OnRegenFailedPreviews      func(driveID string)
	OnRegenFailedThumbnails    func(driveID string)
	OnRegenFailedFingerprints  func(driveID string)
	GetDriveGenerationStatuses func() map[string]DriveGenerationStatuses
	// OnTeaserEnabledChanged 在 per-drive teaser 开关被切换后调用。
	// enabled=true 时上层应该重新把 pending teaser 入队（类似旧的全局开关从关到开）；
	// enabled=false 时通常不用做事 —— worker 入队前会再次查 catalog，自然停止。
	OnTeaserEnabledChanged func(driveID string, enabled bool)
	// Theme 读写（"dark" | "pink"）
	GetTheme func() string
	SetTheme func(theme string) error
	// Spider91 上传目标 drive ID 读写
	GetSpider91UploadDriveID func() string
	SetSpider91UploadDriveID func(driveID string) error
	// 本地上传/链接导入默认上传目标 drive ID 读写。
	GetImportUploadDriveID func() string
	SetImportUploadDriveID func(driveID string) error
	// OnRunNightlyJob 触发一次完整的凌晨流水线（Phase1 扫盘 + Phase2 91 爬虫 +
	// Phase3 迁移）。立即返回 —— 实际任务在后台跑，admin 在日志或下次状态查询里
	// 看进度。返回 false 表示当前已有运行中或排队中的任务，没有叠加新任务。
	OnRunNightlyJob     func() bool
	GetNightlyJobStatus func() NightlyJobStatus
	// OnRunSpider91Migration 只触发 spider91 本地视频迁移，不重新爬取。
	OnRunSpider91Migration func() error
	// ListDriveDirChildren 列出某个 drive 在 parentID 目录下的直接子目录。
	// parentID 为空时使用 drive 的 RootID。返回 (子目录列表, error)。
	// 用于管理后台按需展开浏览网盘目录树；只返回目录条目，文件忽略。
	// 调用方应当处理 error 并以 5xx 返回前端。
	ListDriveDirChildren func(ctx context.Context, driveID, parentID string) ([]DriveDirEntry, error)
	// GetDriveCleanupPreview dry-run 扫描某个 drive，返回如果现在执行扫描清理会
	// 从媒体库移除的记录。实现方不能删除源文件或写 catalog。
	GetDriveCleanupPreview func(ctx context.Context, driveID string) (DriveCleanupPreview, error)
}

// DriveDirEntry 是 dirtree 接口的一条返回项：网盘上的一个目录节点。
type DriveDirEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type DriveCleanupPreviewItem struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	FileName       string `json:"fileName"`
	FileID         string `json:"fileId"`
	ParentID       string `json:"parentId,omitempty"`
	Category       string `json:"category,omitempty"`
	SizeBytes      int64  `json:"sizeBytes"`
	Reason         string `json:"reason"`
	MatchedKeyword string `json:"matchedKeyword,omitempty"`
}

type DriveCleanupPreview struct {
	DriveID       string                    `json:"driveId"`
	Scanned       int                       `json:"scanned"`
	Errors        int                       `json:"errors"`
	FullDriveScan bool                      `json:"fullDriveScan"`
	SafeToClean   bool                      `json:"safeToClean"`
	Reason        string                    `json:"reason,omitempty"`
	Total         int                       `json:"total"`
	Limited       bool                      `json:"limited"`
	Items         []DriveCleanupPreviewItem `json:"items"`
}

type GenerationStatus struct {
	State         string `json:"state"`
	CurrentTitle  string `json:"currentTitle,omitempty"`
	QueueLength   int    `json:"queueLength"`
	CooldownUntil string `json:"cooldownUntil,omitempty"`
}

type DriveGenerationStatuses struct {
	Thumbnail   GenerationStatus `json:"thumbnail"`
	Preview     GenerationStatus `json:"preview"`
	HLS         GenerationStatus `json:"hls"`
	Fingerprint GenerationStatus `json:"fingerprint"`
}

type NightlyJobStatus struct {
	State          string `json:"state"`
	Running        bool   `json:"running"`
	Queued         bool   `json:"queued"`
	StartedAt      string `json:"startedAt,omitempty"`
	LastFinishedAt string `json:"lastFinishedAt,omitempty"`
}

const spider91ListSourcesCredentialKey = "list_sources_json"

type spider91SourceConfig struct {
	URL       string `json:"url"`
	TargetNew int    `json:"targetNew"`
}

func defaultSpider91Sources() []spider91SourceConfig {
	return []spider91SourceConfig{
		{URL: "https://www.91porn.com/v.php?category=top&viewtype=basic", TargetNew: 15},
		{URL: "https://91porn.com/v.php?category=mf&viewtype=basic", TargetNew: 50},
	}
}

func spider91SourcesFromCredentials(creds map[string]string) []spider91SourceConfig {
	raw := strings.TrimSpace(creds[spider91ListSourcesCredentialKey])
	if raw == "" {
		return defaultSpider91Sources()
	}
	var sources []spider91SourceConfig
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return defaultSpider91Sources()
	}
	cleaned, err := cleanSpider91Sources(sources)
	if err != nil || len(cleaned) == 0 {
		return defaultSpider91Sources()
	}
	return cleaned
}

func cleanSpider91Sources(sources []spider91SourceConfig) ([]spider91SourceConfig, error) {
	if len(sources) == 0 {
		return nil, errors.New("sources is required")
	}
	out := make([]spider91SourceConfig, 0, len(sources))
	seen := map[string]struct{}{}
	for _, source := range sources {
		rawURL := strings.TrimSpace(source.URL)
		if rawURL == "" {
			return nil, errors.New("source url is required")
		}
		if len(rawURL) > 600 {
			return nil, errors.New("source url is too long")
		}
		u, err := neturl.Parse(rawURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid source url: %s", rawURL)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, fmt.Errorf("unsupported source url scheme: %s", u.Scheme)
		}
		if source.TargetNew <= 0 {
			return nil, fmt.Errorf("targetNew must be > 0 for %s", rawURL)
		}
		if source.TargetNew > 500 {
			return nil, fmt.Errorf("targetNew must be <= 500 for %s", rawURL)
		}
		canonical := u.String()
		key := strings.ToLower(canonical)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, spider91SourceConfig{
			URL:       canonical,
			TargetNew: source.TargetNew,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("sources is required")
	}
	return out, nil
}

func spider91SourcesTargetNew(sources []spider91SourceConfig) int {
	total := 0
	for _, source := range sources {
		if source.TargetNew > 0 {
			total += source.TargetNew
		}
	}
	return total
}

func (a *AdminServer) Register(r chi.Router) {
	r.Route("/admin/api", func(r chi.Router) {
		// 登录、登出和首次部署初始化不需要鉴权
		r.Get("/setup", a.handleSetupStatus)
		r.Post("/setup", a.handleSetup)
		r.Post("/login", a.handleLogin)
		r.Post("/logout", a.handleLogout)
		r.Get("/me", a.handleMe)

		// 其余路由需鉴权
		r.Group(func(r chi.Router) {
			r.Use(a.Auth.Required)

			// 网盘
			r.Get("/drives", a.handleListDrives)
			r.Get("/drives/storage", a.handleDriveStorage)
			r.Post("/drives", a.handleUpsertDrive)
			r.Delete("/drives/{id}", a.handleDeleteDrive)
			r.Post("/drives/{id}/rescan", a.handleRescan)
			r.Post("/drives/{id}/teaser-enabled", a.handleSetDriveTeaserEnabled)
			r.Post("/drives/{id}/skip-dirs", a.handleSetDriveSkipDirs)
			r.Post("/drives/{id}/scan-dirs", a.handleSetDriveScanDirs)
			r.Post("/drives/{id}/scan-filter", a.handleSetDriveScanFilter)
			r.Post("/drives/{id}/spider91-sources", a.handleSetSpider91Sources)
			r.Get("/drives/{id}/cleanup-preview", a.handleDriveCleanupPreview)
			r.Get("/drives/{id}/dirtree", a.handleListDriveDirTree)
			r.Post("/drives/{id}/previews/failed/regenerate", a.handleRegenFailedPreviews)
			r.Post("/drives/{id}/thumbnails/failed/regenerate", a.handleRegenFailedThumbnails)
			r.Post("/drives/{id}/fingerprints/failed/regenerate", a.handleRegenFailedFingerprints)

			// 视频
			r.Get("/videos", a.handleAdminListVideos)
			r.Post("/videos/bulk-tags", a.handleBulkVideoTags)
			r.Put("/videos/{id}", a.handleUpdateVideo)
			r.Post("/videos/regen-preview", a.handleRegenAllPreviews)
			r.Post("/videos/{id}/regen-preview", a.handleRegenPreview)
			r.Post("/videos/{id}/hls", a.handleGenerateHLS)

			// 标签
			r.Get("/tags", a.handleListTags)
			r.Post("/tags", a.handleCreateTag)
			r.Delete("/tags/{id}", a.handleDeleteTag)

			// 运行时设置
			r.Get("/settings", a.handleGetSettings)
			r.Put("/settings", a.handlePutSettings)

			// 运维任务
			r.Get("/update/check", a.handleCheckUpdate)
			r.Get("/jobs/nightly/status", a.handleNightlyJobStatus)
			r.Post("/jobs/nightly/run", a.handleRunNightlyJob)
			r.Post("/jobs/spider91/migrate", a.handleRunSpider91Migration)
		})
	})
}

type updateCheckDTO struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseURL     string `json:"releaseUrl,omitempty"`
	CheckedAt      string `json:"checkedAt"`
}

type githubReleaseDTO struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type setupReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *AdminServer) setupRequired() bool {
	return a.SetupRequired != nil && a.SetupRequired()
}

func (a *AdminServer) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"required": a.setupRequired()})
}

func (a *AdminServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	if !a.setupRequired() {
		http.Error(w, "setup already completed", http.StatusConflict)
		return
	}
	if a.OnSetup == nil || a.Auth == nil {
		http.Error(w, "setup is not available", http.StatusInternalServerError)
		return
	}
	var body setupReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	username := strings.TrimSpace(body.Username)
	password := body.Password
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
	if len(password) < 6 {
		http.Error(w, "password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if err := a.OnSetup(username, password); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	ok, err := a.Auth.Login(w, r, username, password)
	if err != nil {
		if errors.Is(err, auth.ErrLoginIPBanned) {
			http.Error(w, "ip banned", http.StatusForbidden)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		http.Error(w, "setup completed but login failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *AdminServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if a.setupRequired() {
		http.Error(w, "setup required", http.StatusPreconditionRequired)
		return
	}
	var body loginReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ok, err := a.Auth.Login(w, r, body.Username, body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrLoginIPBanned) {
			http.Error(w, "ip banned", http.StatusForbidden)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *AdminServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.Auth.Logout(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *AdminServer) handleMe(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("vs_admin")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	ok, _ := a.Catalog.ValidateSession(r.Context(), c.Value)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": ok})
}

func (a *AdminServer) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	info, err := a.checkUpdate(r.Context())
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, info)
}

func (a *AdminServer) checkUpdate(ctx context.Context) (updateCheckDTO, error) {
	current := a.installedVersion()
	if current == "" {
		current = "unknown"
	}
	release, err := a.latestRelease(ctx)
	if err != nil {
		return updateCheckDTO{
			CurrentVersion: current,
			CheckedAt:      time.Now().Format(time.RFC3339),
		}, err
	}
	latest := strings.TrimSpace(release.TagName)
	return updateCheckDTO{
		CurrentVersion: current,
		LatestVersion:  latest,
		HasUpdate:      current != "unknown" && latest != "" && current != latest,
		ReleaseURL:     release.HTMLURL,
		CheckedAt:      time.Now().Format(time.RFC3339),
	}, nil
}

func (a *AdminServer) installedVersion() string {
	path := strings.TrimSpace(a.VersionFilePath)
	if path == "" {
		path = ".version"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func (a *AdminServer) latestRelease(ctx context.Context) (githubReleaseDTO, error) {
	url := strings.TrimSpace(a.ReleaseAPIURL)
	if url == "" {
		repo := strings.TrimSpace(a.GitHubRepo)
		if repo == "" {
			repo = "nianzhibai/91"
		}
		url = "https://api.github.com/repos/" + repo + "/releases/latest"
	}
	client := a.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return githubReleaseDTO{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "video-site-91")
	res, err := client.Do(req)
	if err != nil {
		return githubReleaseDTO{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return githubReleaseDTO{}, fmt.Errorf("github release check failed: HTTP %d", res.StatusCode)
	}
	var release githubReleaseDTO
	if err := json.NewDecoder(res.Body).Decode(&release); err != nil {
		return githubReleaseDTO{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubReleaseDTO{}, errors.New("github release check returned empty tag")
	}
	return release, nil
}

func (a *AdminServer) handleListDrives(w http.ResponseWriter, r *http.Request) {
	drives, err := a.Catalog.ListDrives(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	teaserCounts, err := a.Catalog.CountTeasersByDrive(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	thumbnailCounts, err := a.Catalog.CountThumbnailsByDrive(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	fingerprintCounts, err := a.Catalog.CountFingerprintsByDrive(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	generationStatuses := map[string]DriveGenerationStatuses{}
	if a.GetDriveGenerationStatuses != nil {
		generationStatuses = a.GetDriveGenerationStatuses()
	}
	// 出参不返回凭证明文，只告诉前端是否已配置
	type out struct {
		ID            string `json:"id"`
		Kind          string `json:"kind"`
		Name          string `json:"name"`
		RootID        string `json:"rootId"`
		ScanRootID    string `json:"scanRootId"`
		Status        string `json:"status"`
		LastError     string `json:"lastError,omitempty"`
		HasCredential bool   `json:"hasCredential"`
		// TeaserEnabled 控制是否给本盘生成 teaser/封面。前端用它在网盘列表/编辑表单展示开关状态。
		TeaserEnabled bool `json:"teaserEnabled"`
		// SkipDirIDs 是旧版"扫描跳过目录"集合，保留给旧客户端兼容。
		SkipDirIDs []string `json:"skipDirIds"`
		// ScanDirIDs 是用户在 admin 配置的"需要扫描目录"集合（drive 侧目录 fileID）。
		// 非空时 scanner 只扫描这些目录及其子目录。
		ScanDirIDs []string `json:"scanDirIds"`
		// MinScanFileSizeBytes 是扫描入库的最小文件大小阈值；0 表示关闭大小过滤。
		MinScanFileSizeBytes int64 `json:"minScanFileSizeBytes"`
		// SkipFileNameKeywords 是扫描时按文件名跳过视频的关键词列表。
		SkipFileNameKeywords []string `json:"skipFileNameKeywords"`
		// LastCrawlAt 是 spider91 上次成功爬取的 unix 秒（来自 credentials.last_crawl_at）。
		// 其它 kind 留 0；前端用它显示"上次抓取: N 小时前"。
		LastCrawlAt                   int64                  `json:"lastCrawlAt,omitempty"`
		ThumbnailGenerationStatus     GenerationStatus       `json:"thumbnailGenerationStatus"`
		PreviewGenerationStatus       GenerationStatus       `json:"previewGenerationStatus"`
		HLSGenerationStatus           GenerationStatus       `json:"hlsGenerationStatus"`
		FingerprintGenerationStatus   GenerationStatus       `json:"fingerprintGenerationStatus"`
		ThumbnailReadyCount           int                    `json:"thumbnailReadyCount"`
		ThumbnailPendingCount         int                    `json:"thumbnailPendingCount"`
		ThumbnailFailedCount          int                    `json:"thumbnailFailedCount"`
		ThumbnailDurationPendingCount int                    `json:"thumbnailDurationPendingCount"`
		TeaserReadyCount              int                    `json:"teaserReadyCount"`
		TeaserPendingCount            int                    `json:"teaserPendingCount"`
		TeaserFailedCount             int                    `json:"teaserFailedCount"`
		TeaserSkippedCount            int                    `json:"teaserSkippedCount"`
		FingerprintReadyCount         int                    `json:"fingerprintReadyCount"`
		FingerprintPendingCount       int                    `json:"fingerprintPendingCount"`
		FingerprintFailedCount        int                    `json:"fingerprintFailedCount"`
		Spider91Sources               []spider91SourceConfig `json:"spider91Sources,omitempty"`
		Spider91TargetNew             int                    `json:"spider91TargetNew,omitempty"`
	}
	list := make([]out, 0, len(drives))
	for _, d := range drives {
		counts := teaserCounts[d.ID]
		thumbCounts := thumbnailCounts[d.ID]
		fingerprintCount := fingerprintCounts[d.ID]
		generation := generationStatuses[d.ID]
		if generation.Thumbnail.State == "" {
			generation.Thumbnail.State = "idle"
		}
		if generation.Preview.State == "" {
			generation.Preview.State = "idle"
		}
		if generation.HLS.State == "" {
			generation.HLS.State = "idle"
		}
		if generation.Fingerprint.State == "" {
			generation.Fingerprint.State = "idle"
		}
		// spider91 没有用户凭证概念；只要存在 drive 行就视为"已配置"。
		// last_crawl_at 是后端自动写入的运行状态字段，不计入 hasCredential 判定。
		hasCred := false
		userCredKeys := 0
		for k := range d.Credentials {
			if k == "last_crawl_at" {
				continue
			}
			userCredKeys++
		}
		hasCred = userCredKeys > 0 || d.Kind == "spider91"

		var lastCrawlAt int64
		var spider91Sources []spider91SourceConfig
		var spider91TargetNew int
		if d.Credentials != nil {
			if raw, ok := d.Credentials["last_crawl_at"]; ok && raw != "" {
				if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
					lastCrawlAt = v
				}
			}
		}
		if d.Kind == "spider91" {
			spider91Sources = spider91SourcesFromCredentials(d.Credentials)
			spider91TargetNew = spider91SourcesTargetNew(spider91Sources)
		}

		list = append(list, out{
			ID: d.ID, Kind: d.Kind, Name: d.Name,
			RootID: d.RootID, ScanRootID: d.ScanRootID,
			Status: d.Status, LastError: d.LastError,
			HasCredential:                 hasCred,
			TeaserEnabled:                 d.TeaserEnabled,
			SkipDirIDs:                    append([]string{}, d.SkipDirIDs...),
			ScanDirIDs:                    append([]string{}, d.ScanDirIDs...),
			MinScanFileSizeBytes:          d.MinScanFileSizeBytes,
			SkipFileNameKeywords:          append([]string{}, d.SkipFileNameKeywords...),
			LastCrawlAt:                   lastCrawlAt,
			ThumbnailGenerationStatus:     generation.Thumbnail,
			PreviewGenerationStatus:       generation.Preview,
			HLSGenerationStatus:           generation.HLS,
			FingerprintGenerationStatus:   generation.Fingerprint,
			ThumbnailReadyCount:           thumbCounts.Ready,
			ThumbnailPendingCount:         thumbCounts.Pending,
			ThumbnailFailedCount:          thumbCounts.Failed,
			ThumbnailDurationPendingCount: thumbCounts.DurationPending,
			TeaserReadyCount:              counts.Ready,
			TeaserPendingCount:            counts.Pending,
			TeaserFailedCount:             counts.Failed,
			TeaserSkippedCount:            counts.Skipped,
			FingerprintReadyCount:         fingerprintCount.Ready,
			FingerprintPendingCount:       fingerprintCount.Pending,
			FingerprintFailedCount:        fingerprintCount.Failed,
			Spider91Sources:               spider91Sources,
			Spider91TargetNew:             spider91TargetNew,
		})
	}
	writeJSON(w, http.StatusOK, list)
}

type upsertDriveReq struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	RootID      string            `json:"rootId"`
	ScanRootID  string            `json:"scanRootId"`
	Credentials map[string]string `json:"credentials"`
	// TeaserEnabled 是 per-drive teaser/封面生成开关。
	// 用 *bool 区分 "未传" / "传了 false"：未传时表示客户端不打算改这个字段，
	// 沿用 catalog 现有值；新建时未传一律默认开启（true）。
	TeaserEnabled *bool `json:"teaserEnabled,omitempty"`
	// SkipDirIDs 同样用指针区分 "未传"（沿用旧值）/ "传了空数组"（清空）。
	// 推荐前端"设置跳过目录"走专用 POST /drives/{id}/skip-dirs；
	// 这里支持是为了允许批量编辑场景一次性提交。
	SkipDirIDs *[]string `json:"skipDirIds,omitempty"`
	// ScanDirIDs 是新版"需要扫描目录"白名单；未传沿用旧值，空数组表示关闭白名单。
	ScanDirIDs *[]string `json:"scanDirIds,omitempty"`
	// MinScanFileSizeBytes 同样支持"未传 = 沿用旧值"；新建时默认 0。
	MinScanFileSizeBytes *int64 `json:"minScanFileSizeBytes,omitempty"`
	// SkipFileNameKeywords 同样支持"未传 = 沿用旧值"；新建时默认空数组。
	SkipFileNameKeywords *[]string `json:"skipFileNameKeywords,omitempty"`
}

func (a *AdminServer) handleUpsertDrive(w http.ResponseWriter, r *http.Request) {
	var body upsertDriveReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.ID == "" || body.Kind == "" {
		http.Error(w, "id and kind are required", http.StatusBadRequest)
		return
	}
	// 凭证 / TeaserEnabled 都支持 "未传 = 沿用旧值"：先把现存 drive 拉出来一次。
	var existing *catalog.Drive
	if existingDrive, err := a.Catalog.GetDrive(r.Context(), body.ID); err == nil {
		existing = existingDrive
	}
	if len(body.Credentials) == 0 && existing != nil && len(existing.Credentials) > 0 {
		body.Credentials = existing.Credentials
	}

	// teaserEnabled 解析顺序：
	//   1. 请求显式带了 → 用请求值
	//   2. 请求没带 + 编辑现有 drive → 沿用旧值
	//   3. 请求没带 + 新建 drive → 默认 true（用户没特别说就生成）
	teaserEnabled := true
	switch {
	case body.TeaserEnabled != nil:
		teaserEnabled = *body.TeaserEnabled
	case existing != nil:
		teaserEnabled = existing.TeaserEnabled
	}

	// skipDirIds 解析顺序：
	//   1. 请求显式带了（包括空数组）→ 用请求值（空数组 = 清空）
	//   2. 请求没带 + 编辑现有 drive → 沿用旧值
	//   3. 请求没带 + 新建 drive → nil（不跳过任何目录）
	var skipDirIDs []string
	switch {
	case body.SkipDirIDs != nil:
		skipDirIDs = *body.SkipDirIDs
	case body.ScanDirIDs != nil:
		skipDirIDs = nil
	case existing != nil:
		skipDirIDs = existing.SkipDirIDs
	}

	// scanDirIds 解析顺序同 skipDirIds；非空时作为新版扫描白名单生效。
	var scanDirIDs []string
	switch {
	case body.ScanDirIDs != nil:
		scanDirIDs = *body.ScanDirIDs
	case existing != nil:
		scanDirIDs = existing.ScanDirIDs
	}

	minScanFileSizeBytes := int64(0)
	switch {
	case body.MinScanFileSizeBytes != nil:
		minScanFileSizeBytes = *body.MinScanFileSizeBytes
	case existing != nil:
		minScanFileSizeBytes = existing.MinScanFileSizeBytes
	}
	if minScanFileSizeBytes < 0 {
		http.Error(w, "minScanFileSizeBytes must be >= 0", http.StatusBadRequest)
		return
	}

	var skipFileNameKeywords []string
	switch {
	case body.SkipFileNameKeywords != nil:
		skipFileNameKeywords = cleanStringList(*body.SkipFileNameKeywords)
	case existing != nil:
		skipFileNameKeywords = existing.SkipFileNameKeywords
	}

	d := &catalog.Drive{
		ID: body.ID, Kind: body.Kind, Name: body.Name,
		RootID: body.RootID, ScanRootID: body.ScanRootID,
		Credentials:          body.Credentials,
		Status:               "disconnected",
		TeaserEnabled:        teaserEnabled,
		SkipDirIDs:           skipDirIDs,
		ScanDirIDs:           scanDirIDs,
		MinScanFileSizeBytes: minScanFileSizeBytes,
		SkipFileNameKeywords: skipFileNameKeywords,
	}
	if err := a.Catalog.UpsertDrive(r.Context(), d); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if a.OnDriveSaved != nil {
		if err := a.OnDriveSaved(body.ID); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "warning": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *AdminServer) handleDeleteDrive(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := a.Catalog.DeleteDrive(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if a.OnDriveRemoved != nil {
		a.OnDriveRemoved(id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *AdminServer) handleRescan(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.OnScanRequested != nil {
		a.OnScanRequested(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

type setSpider91SourcesReq struct {
	Sources []spider91SourceConfig `json:"sources"`
}

func (a *AdminServer) handleSetSpider91Sources(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	var body setSpider91SourcesReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	sources, err := cleanSpider91Sources(body.Sources)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	d, err := a.Catalog.GetDrive(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "drive not found", http.StatusNotFound)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if d.Kind != "spider91" {
		http.Error(w, "drive is not spider91", http.StatusBadRequest)
		return
	}
	if d.Credentials == nil {
		d.Credentials = map[string]string{}
	}
	raw, err := json.Marshal(sources)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	targetNew := spider91SourcesTargetNew(sources)
	d.Credentials[spider91ListSourcesCredentialKey] = string(raw)
	d.Credentials["target_new"] = strconv.Itoa(targetNew)
	if err := a.Catalog.UpsertDrive(r.Context(), d); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"sources":   sources,
		"targetNew": targetNew,
	})
}

// handleRunNightlyJob 触发一次完整的凌晨流水线（不论当前时间，不论今日是否已跑）。
// 立即返回 202；进度通过 backend 日志和下次 GET /admin/api/drives 的状态变化观察。
// 流水线已在跑时 Runner 最多排队一个后续触发；如果已有待触发请求，新的点击会被忽略。
func (a *AdminServer) handleRunNightlyJob(w http.ResponseWriter, r *http.Request) {
	accepted := false
	if a.OnRunNightlyJob != nil {
		accepted = a.OnRunNightlyJob()
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":       true,
		"accepted": accepted,
		"status":   a.nightlyJobStatus(),
	})
}

func (a *AdminServer) handleNightlyJobStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.nightlyJobStatus())
}

func (a *AdminServer) nightlyJobStatus() NightlyJobStatus {
	if a.GetNightlyJobStatus == nil {
		return NightlyJobStatus{State: "idle"}
	}
	status := a.GetNightlyJobStatus()
	if strings.TrimSpace(status.State) == "" {
		status.State = "idle"
	}
	return status
}

// handleRunSpider91Migration 只把已下载到本地的 spider91 视频迁移到上传目标。
// 不重新爬取；用于 admin 手动确认迁移是否启动。
func (a *AdminServer) handleRunSpider91Migration(w http.ResponseWriter, r *http.Request) {
	if a.OnRunSpider91Migration == nil {
		http.Error(w, "spider91 migration is not available", http.StatusNotFound)
		return
	}
	if err := a.OnRunSpider91Migration(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// teaserEnabledReq 是 POST /admin/api/drives/{id}/teaser-enabled 的入参。
type teaserEnabledReq struct {
	Enabled bool `json:"enabled"`
}

// handleSetDriveTeaserEnabled 切换某盘的 teaser 生成开关。
//
// 行为：
//   - 写 catalog.drives.teaser_enabled
//   - 调 OnTeaserEnabledChanged（main 注入；从关到开时会重新入队 pending teaser）
//   - 返回切换后的新值，方便前端乐观更新但又能以服务端为准
//
// 与 upsertDrive 的区别：那条接口要重传 kind / name / rootId 等，开关切换不该
// 牵连这些字段（顺手覆盖凭证或 rootID 容易出 bug）。所以单独走一条。
func (a *AdminServer) handleSetDriveTeaserEnabled(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	var body teaserEnabledReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := a.Catalog.SetDriveTeaserEnabled(r.Context(), id, body.Enabled); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "drive not found", http.StatusNotFound)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if a.OnTeaserEnabledChanged != nil {
		a.OnTeaserEnabledChanged(id, body.Enabled)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "teaserEnabled": body.Enabled})
}

// skipDirsReq 是 POST /admin/api/drives/{id}/skip-dirs 的入参。
//
// 整体覆盖语义：传啥就保存啥（不是增量合并）。dirIds 可以是 nil/空数组 表示
// 清空跳过列表。
type skipDirsReq struct {
	DirIDs []string `json:"dirIds"`
}

// scanDirsReq 是 POST /admin/api/drives/{id}/scan-dirs 的入参。
//
// 整体覆盖语义：传啥就保存啥（不是增量合并）。dirIds 可以是 nil/空数组 表示
// 关闭目录白名单，回到按扫描起点完整扫描。
type scanDirsReq struct {
	DirIDs []string `json:"dirIds"`
}

// handleSetDriveSkipDirs 更新某盘的"扫描跳过目录"集合。
//
// 与 upsertDrive 的区别：那条接口要重传 kind / name / rootId / credentials 等字段，
// 用户保存跳过目录时不该牵连这些。所以单独走一条 PUT 风格接口。
//
// 行为：
//   - 写 catalog.drives.skip_dir_ids（整体覆盖）
//   - 不重新触发扫描；下次 nightly Phase 1 或 admin 手动重扫时生效
//   - 返回保存后的列表，方便前端乐观更新但又能以服务端为准
func (a *AdminServer) handleSetDriveSkipDirs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	var body skipDirsReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	cleaned := cleanStringList(body.DirIDs)
	if err := a.Catalog.SetDriveSkipDirIDs(r.Context(), id, cleaned); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "drive not found", http.StatusNotFound)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "skipDirIds": cleaned})
}

// handleSetDriveScanDirs 更新某盘的"需要扫描目录"白名单。
//
// 行为：
//   - 写 catalog.drives.scan_dir_ids（整体覆盖）
//   - 非空时扫描只从这些目录开始递归；空数组表示按 scanRoot/root 完整扫描
//   - 不会删除源文件；后续扫描清理只删除 catalog 里的旧记录
func (a *AdminServer) handleSetDriveScanDirs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	var body scanDirsReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	cleaned := cleanStringList(body.DirIDs)
	if err := a.Catalog.SetDriveScanDirIDs(r.Context(), id, cleaned); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "drive not found", http.StatusNotFound)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "scanDirIds": cleaned})
}

// scanFilterReq 是 POST /admin/api/drives/{id}/scan-filter 的入参。
type scanFilterReq struct {
	MinFileSizeBytes     int64     `json:"minFileSizeBytes"`
	SkipFileNameKeywords *[]string `json:"skipFileNameKeywords,omitempty"`
}

// handleSetDriveScanFilter 更新某盘的扫描过滤阈值。
//
// minFileSizeBytes=0 表示关闭大小过滤；skipFileNameKeywords 为空表示关闭文件名
// 关键词过滤。设置后不会立即触发扫描；下次扫描时，被过滤的视频文件不会入库，也
// 不会进入 SeenFileIDs，完整扫描会顺带清理既有记录。
func (a *AdminServer) handleSetDriveScanFilter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	var body scanFilterReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if body.MinFileSizeBytes < 0 {
		http.Error(w, "minFileSizeBytes must be >= 0", http.StatusBadRequest)
		return
	}

	keywords := []string{}
	if body.SkipFileNameKeywords != nil {
		keywords = cleanStringList(*body.SkipFileNameKeywords)
	} else if existing, err := a.Catalog.GetDrive(r.Context(), id); err == nil {
		keywords = existing.SkipFileNameKeywords
	} else if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "drive not found", http.StatusNotFound)
		return
	} else {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := a.Catalog.SetDriveScanFilter(r.Context(), id, body.MinFileSizeBytes, keywords); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "drive not found", http.StatusNotFound)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                   true,
		"minFileSizeBytes":     body.MinFileSizeBytes,
		"skipFileNameKeywords": keywords,
	})
}

func (a *AdminServer) handleDriveCleanupPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if a.GetDriveCleanupPreview == nil {
		http.Error(w, "cleanup preview is not available", http.StatusNotImplemented)
		return
	}
	preview, err := a.GetDriveCleanupPreview(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "drive not found", http.StatusNotFound)
			return
		}
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func mergeLabels(existing []string, added []string) []string {
	out := cleanStringList(existing)
	seen := make(map[string]struct{}, len(out)+len(added))
	for _, label := range out {
		seen[strings.ToLower(label)] = struct{}{}
	}
	for _, label := range cleanStringList(added) {
		key := strings.ToLower(label)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, label)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func removeLabels(existing []string, removed []string) []string {
	blocked := map[string]struct{}{}
	for _, label := range cleanStringList(removed) {
		blocked[strings.ToLower(label)] = struct{}{}
	}
	out := make([]string, 0, len(existing))
	for _, label := range cleanStringList(existing) {
		if _, ok := blocked[strings.ToLower(label)]; ok {
			continue
		}
		out = append(out, label)
	}
	if out == nil {
		return []string{}
	}
	return out
}

// handleListDriveDirTree 列出某 drive 在指定父目录下的直接子目录。
//
// 查询参数 ?parent=<dirID>：留空 = drive 的 RootID。前端按需展开调用 ——
// 每展开一层调一次，避免一次性递归整个网盘（115 限频会很难受）。
//
// 错误：drive 未挂载 / List 失败 → 500，body 是错误文案；前端展示给用户。
func (a *AdminServer) handleListDriveDirTree(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if a.ListDriveDirChildren == nil {
		writeErr(w, http.StatusInternalServerError, errors.New("dirtree not configured"))
		return
	}
	parent := r.URL.Query().Get("parent")
	entries, err := a.ListDriveDirChildren(r.Context(), id, parent)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if entries == nil {
		entries = []DriveDirEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (a *AdminServer) handleAdminListVideos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("size"))
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 100
	}
	items, total, err := a.Catalog.ListVideos(r.Context(), catalog.ListParams{
		Keyword:  strings.TrimSpace(q.Get("keyword")),
		DriveID:  q.Get("driveId"),
		Tag:      strings.TrimSpace(q.Get("tag")),
		Category: strings.TrimSpace(q.Get("category")),
		Page:     page,
		PageSize: size,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func (a *AdminServer) handleListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.Catalog.ListTags(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

type createTagReq struct {
	Label   string   `json:"label"`
	Aliases []string `json:"aliases"`
}

func (a *AdminServer) handleCreateTag(w http.ResponseWriter, r *http.Request) {
	var body createTagReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	classified, err := a.Catalog.CreateTagAndClassify(r.Context(), body.Label, body.Aliases, "user")
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"label":      body.Label,
		"classified": classified,
	})
}

func (a *AdminServer) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeErr(w, http.StatusBadRequest, errors.New("invalid tag id"))
		return
	}
	removedVideos, err := a.Catalog.DeleteTag(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			writeErr(w, http.StatusNotFound, err)
		case errors.Is(err, catalog.ErrSystemTag):
			writeErr(w, http.StatusBadRequest, err)
		default:
			writeErr(w, http.StatusInternalServerError, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removedVideos": removedVideos})
}

type updateVideoReq struct {
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
	Category    string   `json:"category"`
	Badges      []string `json:"badges"`
	Description string   `json:"description"`
	Thumbnail   string   `json:"thumbnail"`
	Quality     string   `json:"quality"`
	DurationSec int      `json:"durationSeconds"`
}

type bulkVideoTagsReq struct {
	VideoIDs []string `json:"videoIds"`
	Tags     []string `json:"tags"`
	Mode     string   `json:"mode"`
}

func (a *AdminServer) handleBulkVideoTags(w http.ResponseWriter, r *http.Request) {
	var body bulkVideoTagsReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	videoIDs := cleanStringList(body.VideoIDs)
	if len(videoIDs) == 0 {
		http.Error(w, "videoIds is required", http.StatusBadRequest)
		return
	}
	if len(videoIDs) > 500 {
		http.Error(w, "too many videos selected", http.StatusBadRequest)
		return
	}
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "" {
		mode = "add"
	}
	if mode != "add" && mode != "remove" && mode != "replace" {
		http.Error(w, "mode must be add, remove, or replace", http.StatusBadRequest)
		return
	}
	tags := cleanStringList(body.Tags)
	if mode != "replace" && len(tags) == 0 {
		http.Error(w, "tags is required", http.StatusBadRequest)
		return
	}

	videos := make([]*catalog.Video, 0, len(videoIDs))
	for _, id := range videoIDs {
		v, err := a.Catalog.GetVideo(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		videos = append(videos, v)
	}

	updated := 0
	for _, v := range videos {
		nextTags := tags
		switch mode {
		case "add":
			nextTags = mergeLabels(v.Tags, tags)
		case "remove":
			nextTags = removeLabels(v.Tags, tags)
		}
		if err := a.Catalog.SetManualVideoTags(r.Context(), v.ID, nextTags); err != nil {
			if errors.Is(err, catalog.ErrUnknownTag) {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		updated++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"updated": updated,
	})
}

func (a *AdminServer) handleUpdateVideo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body updateVideoReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	v, err := a.Catalog.GetVideo(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if body.Title != "" {
		v.Title = body.Title
	}
	if body.Author != "" {
		v.Author = body.Author
	}
	if body.Category != "" {
		v.Category = body.Category
	}
	if body.Badges != nil {
		v.Badges = body.Badges
	}
	if body.Description != "" {
		v.Description = body.Description
	}
	if body.Thumbnail != "" {
		v.ThumbnailURL = body.Thumbnail
	}
	if body.Quality != "" {
		v.Quality = body.Quality
	}
	if body.DurationSec > 0 {
		v.DurationSeconds = body.DurationSec
	}
	if err := a.Catalog.UpsertVideo(r.Context(), v); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if body.Tags != nil {
		if err := a.Catalog.SetManualVideoTags(r.Context(), id, body.Tags); err != nil {
			if errors.Is(err, catalog.ErrUnknownTag) {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		v, err = a.Catalog.GetVideo(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *AdminServer) handleRegenPreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.OnRegenPreview != nil {
		a.OnRegenPreview(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (a *AdminServer) handleGenerateHLS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.OnGenerateHLS == nil {
		http.Error(w, "hls generation is not available", http.StatusNotImplemented)
		return
	}
	a.OnGenerateHLS(id)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (a *AdminServer) handleRegenAllPreviews(w http.ResponseWriter, r *http.Request) {
	if a.OnRegenAllPreviews != nil {
		a.OnRegenAllPreviews()
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (a *AdminServer) handleRegenFailedPreviews(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.OnRegenFailedPreviews != nil {
		a.OnRegenFailedPreviews(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// handleRegenFailedThumbnails 触发某 drive 下所有 thumbnail_status=failed 的封面
// 重新入队生成。和 handleRegenFailedPreviews 行为对称（一个管 teaser，一个管封面）。
//
// 立即返回 202；实际执行在后台 goroutine 跑，状态可在下次 GET /admin/api/drives
// 的 thumbnailFailedCount / thumbnailGenerationStatus 看变化。
func (a *AdminServer) handleRegenFailedThumbnails(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.OnRegenFailedThumbnails != nil {
		a.OnRegenFailedThumbnails(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

func (a *AdminServer) handleRegenFailedFingerprints(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if a.OnRegenFailedFingerprints != nil {
		a.OnRegenFailedFingerprints(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// ---------- Settings ----------

// settingsDTO 是 GET/PUT /admin/api/settings 的入参/出参。
//
// 注意：早期的全局 previewEnabled 字段已经下沉为每盘 teaser_enabled，
// 不再出现在这里；前端要切换某个盘的 teaser 生成请用 POST /admin/api/drives 上传
// teaserEnabled 字段。保留 settings 用作主题、spider91 上传目标这类全局配置。
type settingsDTO struct {
	Theme                 string `json:"theme"`
	Spider91UploadDriveID string `json:"spider91UploadDriveId"`
	ImportUploadDriveID   string `json:"importUploadDriveId"`
}

func (a *AdminServer) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	theme := "dark"
	if a.GetTheme != nil {
		if v := a.GetTheme(); v != "" {
			theme = v
		}
	}
	spider91UploadID := ""
	if a.GetSpider91UploadDriveID != nil {
		spider91UploadID = a.GetSpider91UploadDriveID()
	}
	importUploadID := ""
	if a.GetImportUploadDriveID != nil {
		importUploadID = a.GetImportUploadDriveID()
	}
	writeJSON(w, http.StatusOK, settingsDTO{
		Theme:                 theme,
		Spider91UploadDriveID: spider91UploadID,
		ImportUploadDriveID:   importUploadID,
	})
}

func (a *AdminServer) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	// 用 map 区分"没传"和"传了空字符串"两种语义；空 spider91 上传 ID 表示
	// 本地保存不上传。
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if v, ok := raw["theme"]; ok && a.SetTheme != nil {
		var theme string
		if err := json.Unmarshal(v, &theme); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if theme != "" {
			if err := a.SetTheme(theme); err != nil {
				writeErr(w, http.StatusBadRequest, err)
				return
			}
		}
	}

	if v, ok := raw["spider91UploadDriveId"]; ok && a.SetSpider91UploadDriveID != nil {
		var driveID string
		if err := json.Unmarshal(v, &driveID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := a.SetSpider91UploadDriveID(driveID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}

	if v, ok := raw["importUploadDriveId"]; ok && a.SetImportUploadDriveID != nil {
		var driveID string
		if err := json.Unmarshal(v, &driveID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := a.SetImportUploadDriveID(driveID); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
	}

	// 回显当前值
	resp := settingsDTO{}
	if a.GetTheme != nil {
		resp.Theme = a.GetTheme()
	}
	if a.GetSpider91UploadDriveID != nil {
		resp.Spider91UploadDriveID = a.GetSpider91UploadDriveID()
	}
	if a.GetImportUploadDriveID != nil {
		resp.ImportUploadDriveID = a.GetImportUploadDriveID()
	}
	writeJSON(w, http.StatusOK, resp)
}
