package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives"
)

type ImportManagerConfig struct {
	PythonPath string
	ScriptPath string
	TempDir    string
	WorkDir    string
	// DefaultUploadDrive returns the drive to upload imported videos to.
	// nil means save locally.
	DefaultUploadDrive func() drives.Drive
	// OnDriveUploadComplete is called after an imported/external upload is
	// successfully written to a cloud drive. The app layer uses it to kick off
	// a normal drive scan with a long-lived server context.
	OnDriveUploadComplete func(driveID string)
}

type ImportManager struct {
	server *Server
	cfg    ImportManagerConfig

	mu    sync.Mutex
	jobs  map[string]*ImportJob
	order []string
	queue chan string
}

type ImportJob struct {
	ID         string     `json:"id"`
	SourceURL  string     `json:"sourceUrl"`
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	Error      string     `json:"error,omitempty"`
	VideoIDs   []string   `json:"videoIds,omitempty"`
	Videos     []VideoDTO `json:"videos,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`

	title string
	tags  []string
}

type createImportRequest struct {
	URL   string   `json:"url"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

type downloaderOutput struct {
	Files []downloadedFile `json:"files"`
}

type downloadedFile struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

func NewImportManager(server *Server, cfg ImportManagerConfig) *ImportManager {
	pythonPath := strings.TrimSpace(cfg.PythonPath)
	if pythonPath == "" {
		pythonPath = "python3"
	}
	cfg.PythonPath = pythonPath
	return &ImportManager{
		server: server,
		cfg:    cfg,
		jobs:   make(map[string]*ImportJob),
		queue:  make(chan string, 16),
	}
}

func (m *ImportManager) Start(ctx context.Context) {
	if m == nil {
		return
	}
	go m.run(ctx)
}

func (m *ImportManager) EnqueueURL(ctx context.Context, req createImportRequest) (*ImportJob, error) {
	if m == nil {
		return nil, errors.New("import manager is not configured")
	}
	sourceURL := strings.TrimSpace(req.URL)
	if err := validateImportURL(sourceURL); err != nil {
		return nil, uploadBadRequest(err)
	}
	tags, err := parseUploadTags(req.Tags)
	if err != nil {
		return nil, uploadBadRequest(err)
	}
	id, err := newImportID(time.Now())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job := &ImportJob{
		ID:        id,
		SourceURL: sourceURL,
		Status:    "queued",
		Message:   "waiting for downloader",
		CreatedAt: now,
		UpdatedAt: now,
		title:     strings.TrimSpace(req.Title),
		tags:      tags,
	}

	m.mu.Lock()
	m.jobs[id] = job
	m.order = append(m.order, id)
	m.trimLocked()
	m.mu.Unlock()

	select {
	case m.queue <- id:
		return job.clone(), nil
	case <-ctx.Done():
		m.setFailed(id, ctx.Err())
		return nil, ctx.Err()
	}
}

func (m *ImportManager) Get(id string) (*ImportJob, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, false
	}
	return job.clone(), true
}

func (m *ImportManager) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-m.queue:
			m.runJob(ctx, id)
		}
	}
}

func (m *ImportManager) runJob(parent context.Context, id string) {
	job, ok := m.Get(id)
	if !ok {
		return
	}
	m.update(id, func(j *ImportJob) {
		j.Status = "running"
		j.Message = "downloading source"
	})

	jobDir, err := m.jobTempDir(id)
	if err != nil {
		m.setFailed(id, err)
		return
	}
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		m.setFailed(id, err)
		return
	}
	defer os.RemoveAll(jobDir)

	files, err := m.download(parent, job.SourceURL, jobDir)
	if err != nil {
		m.setFailed(id, err)
		return
	}
	if len(files) == 0 {
		m.setFailed(id, errors.New("downloader returned no video files"))
		return
	}

	m.update(id, func(j *ImportJob) {
		j.Status = "importing"
		j.Message = "adding videos to local catalog"
	})

	uploadedDriveIDs := map[string]struct{}{}
	defer func() {
		for driveID := range uploadedDriveIDs {
			m.notifyDriveUploadComplete(driveID)
		}
	}()

	for idx, file := range files {
		path, ok := cleanPathWithin(jobDir, file.Path)
		if !ok {
			m.setFailed(id, fmt.Errorf("downloaded file is outside job dir: %s", file.Path))
			return
		}
		title := importVideoTitle(job.title, file.Title, filepath.Base(path), idx, len(files))
		var video *catalog.Video
		var ingestErr error
		if drv := m.defaultUploadDrive(); drv != nil {
			video, ingestErr = m.ingestFileToDrive(parent, drv, path, title, job.tags, "链接导入")
			if ingestErr == nil {
				uploadedDriveIDs[drv.ID()] = struct{}{}
			}
		} else {
			f, err := os.Open(path)
			if err != nil {
				m.setFailed(id, err)
				return
			}
			video, ingestErr = m.server.ingestLocalVideo(parent, localVideoIngestInput{
				Reader:       f,
				OriginalName: filepath.Base(path),
				Title:        title,
				Tags:         job.tags,
				Author:       "链接导入",
			})
			closeErr := f.Close()
			if closeErr != nil {
				m.setFailed(id, closeErr)
				return
			}
		}
		if ingestErr != nil {
			m.setFailed(id, ingestErr)
			return
		}
		if m.server.Catalog != nil {
			if current, err := m.server.Catalog.GetVideo(parent, video.ID); err == nil {
				video = current
			}
		}
		m.update(id, func(j *ImportJob) {
			j.VideoIDs = append(j.VideoIDs, video.ID)
			j.Videos = append(j.Videos, mapVideo(video))
		})
	}

	m.update(id, func(j *ImportJob) {
		now := time.Now()
		j.Status = "done"
		j.Message = "done"
		j.FinishedAt = &now
	})
}

func (m *ImportManager) download(ctx context.Context, sourceURL, jobDir string) ([]downloadedFile, error) {
	scriptPath := strings.TrimSpace(m.cfg.ScriptPath)
	if scriptPath == "" {
		return nil, errors.New("import downloader script is not configured")
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return nil, fmt.Errorf("import downloader script not available: %w", err)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 12*time.Hour)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, m.cfg.PythonPath, scriptPath, "--url", sourceURL, "--output-dir", jobDir, "--json")
	if strings.TrimSpace(m.cfg.WorkDir) != "" {
		cmd.Dir = m.cfg.WorkDir
	} else {
		cmd.Dir = filepath.Dir(scriptPath)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		if cmdCtx.Err() != nil {
			return nil, cmdCtx.Err()
		}
		return nil, fmt.Errorf("download failed: %s", errText)
	}

	var out downloaderOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parse downloader output: %w", err)
	}
	cleaned := make([]downloadedFile, 0, len(out.Files))
	for _, file := range out.Files {
		if strings.TrimSpace(file.Path) == "" {
			continue
		}
		cleaned = append(cleaned, file)
	}
	return cleaned, nil
}

func (m *ImportManager) setFailed(id string, err error) {
	m.update(id, func(j *ImportJob) {
		now := time.Now()
		j.Status = "failed"
		j.Message = "failed"
		if err != nil {
			j.Error = err.Error()
		}
		j.FinishedAt = &now
	})
}

func (m *ImportManager) update(id string, fn func(*ImportJob)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return
	}
	fn(job)
	job.UpdatedAt = time.Now()
}

func (m *ImportManager) trimLocked() {
	const keep = 100
	if len(m.order) <= keep {
		return
	}
	remove := len(m.order) - keep
	for _, id := range m.order[:remove] {
		delete(m.jobs, id)
	}
	m.order = append([]string(nil), m.order[remove:]...)
}

func (m *ImportManager) importTempDir() string {
	if strings.TrimSpace(m.cfg.TempDir) != "" {
		return m.cfg.TempDir
	}
	if m.server != nil && m.server.localUploadDir() != "" {
		return filepath.Join(filepath.Dir(m.server.localUploadDir()), "imports")
	}
	return filepath.Join(os.TempDir(), "video-site-imports")
}

func (m *ImportManager) jobTempDir(id string) (string, error) {
	return filepath.Abs(filepath.Join(m.importTempDir(), id))
}

func (j *ImportJob) clone() *ImportJob {
	if j == nil {
		return nil
	}
	out := *j
	out.VideoIDs = append([]string(nil), j.VideoIDs...)
	out.Videos = append([]VideoDTO(nil), j.Videos...)
	out.tags = append([]string(nil), j.tags...)
	return &out
}

func validateImportURL(raw string) error {
	if raw == "" {
		return errors.New("url is required")
	}
	if strings.HasPrefix(strings.ToLower(raw), "magnet:?") {
		return nil
	}
	u, err := neturl.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", u.Scheme)
	}
	return nil
}

func importVideoTitle(explicit, downloaderTitle, fileName string, index, total int) string {
	base := strings.TrimSpace(explicit)
	if base == "" {
		base = strings.TrimSpace(downloaderTitle)
	}
	if base == "" {
		base = uploadTitleFromFileName(fileName)
	}
	if total > 1 && strings.TrimSpace(explicit) != "" {
		return fmt.Sprintf("%s %d", base, index+1)
	}
	return base
}

func cleanPathWithin(root, path string) (string, bool) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(path) == "" {
		return "", false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return pathAbs, true
}

func newImportID(now time.Time) (string, error) {
	id, err := newUploadID(now)
	if err != nil {
		return "", err
	}
	return strings.Replace(id, "upload-", "import-", 1), nil
}

func (s *Server) handleCreateImport(w http.ResponseWriter, r *http.Request) {
	var req createImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if s.Importer == nil {
		writeErr(w, http.StatusInternalServerError, errors.New("import manager is not configured"))
		return
	}
	job, err := s.Importer.EnqueueURL(r.Context(), req)
	if err != nil {
		writeErr(w, uploadStatusCode(err), err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleGetImport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s.Importer == nil {
		http.NotFound(w, r)
		return
	}
	job, ok := s.Importer.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleExternalCreateImport(w http.ResponseWriter, r *http.Request) {
	if !s.validExternalImportToken(r) {
		http.NotFound(w, r)
		return
	}
	s.handleCreateImport(w, r)
}

func (s *Server) handleExternalGetImport(w http.ResponseWriter, r *http.Request) {
	if !s.validExternalImportToken(r) {
		http.NotFound(w, r)
		return
	}
	s.handleGetImport(w, r)
}

func (s *Server) handleExternalUpload(w http.ResponseWriter, r *http.Request) {
	if !s.validExternalImportToken(r) {
		http.NotFound(w, r)
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

	tags, err := parseUploadTags(uploadTagValues(r))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	if s.Importer != nil {
		if drv := s.Importer.defaultUploadDrive(); drv != nil {
			video, err := s.Importer.ingestReaderToDrive(r.Context(), drv, file, filepath.Base(strings.TrimSpace(header.Filename)), r.FormValue("title"), tags, "Telegram导入")
			if err != nil {
				writeErr(w, uploadStatusCode(err), err)
				return
			}
			s.Importer.notifyDriveUploadComplete(drv.ID())
			writeJSON(w, http.StatusCreated, mapVideo(video))
			return
		}
	}

	if s.localUploadDir() == "" {
		writeErr(w, http.StatusInternalServerError, errors.New("local storage is not configured"))
		return
	}
	video, err := s.ingestLocalVideo(r.Context(), localVideoIngestInput{
		Reader:       file,
		OriginalName: filepath.Base(strings.TrimSpace(header.Filename)),
		Title:        r.FormValue("title"),
		Tags:         tags,
		Author:       "Telegram导入",
	})
	if err != nil {
		writeErr(w, uploadStatusCode(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, mapVideo(video))
}

func (s *Server) validExternalImportToken(r *http.Request) bool {
	token := strings.TrimSpace(s.ExternalImportToken)
	if token == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("X-Import-Token"))
	if got == "" {
		if bearer := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(bearer), "bearer ") {
			got = strings.TrimSpace(bearer[len("bearer "):])
		}
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func (m *ImportManager) defaultUploadDrive() drives.Drive {
	if m.cfg.DefaultUploadDrive == nil {
		return nil
	}
	return m.cfg.DefaultUploadDrive()
}

// ingestToDrive 上传视频到指定网盘并在 catalog 创建条目。
// 失败时不创建 catalog 行，调用方负责清理。
func (m *ImportManager) ingestFileToDrive(ctx context.Context, drv drives.Drive, localPath, title string, tags []string, author string) (*catalog.Video, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open downloaded file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat downloaded file: %w", err)
	}
	return m.ingestReaderToDrive(ctx, drv, f, filepath.Base(localPath), title, tags, author, info.Size())
}

func (m *ImportManager) ingestToDrive(ctx context.Context, drv drives.Drive, localPath, title string, tags []string) (*catalog.Video, error) {
	return m.ingestFileToDrive(ctx, drv, localPath, title, tags, "链接导入")
}

func (m *ImportManager) ingestReaderToDrive(ctx context.Context, drv drives.Drive, r io.Reader, originalName, title string, tags []string, author string, sizeOverride ...int64) (*catalog.Video, error) {
	if m.server.Catalog == nil {
		return nil, errors.New("catalog is not configured")
	}
	if drv == nil {
		return nil, errors.New("upload drive is not configured")
	}
	if r == nil {
		return nil, uploadBadRequest(errors.New("video file is required"))
	}
	name := filepath.Base(strings.TrimSpace(originalName))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "video"
	}
	ext := strings.ToLower(filepath.Ext(name))
	if _, ok := allowedUploadExtensions[ext]; !ok {
		return nil, uploadBadRequest(fmt.Errorf("unsupported video extension: %s", ext))
	}
	size := int64(-1)
	if len(sizeOverride) > 0 {
		size = sizeOverride[0]
	} else if seeker, ok := r.(interface {
		Seek(int64, int) (int64, error)
	}); ok {
		current, seekErr := seeker.Seek(0, io.SeekCurrent)
		if seekErr == nil {
			end, endErr := seeker.Seek(0, io.SeekEnd)
			if endErr == nil {
				size = end - current
				_, _ = seeker.Seek(current, io.SeekStart)
			}
		}
	}
	if size <= 0 {
		return nil, uploadBadRequest(errors.New("uploaded video is empty"))
	}

	now := time.Now()
	uploadID, err := newUploadID(now)
	if err != nil {
		return nil, err
	}

	m.update("", func(_ *ImportJob) {}) // noop, just to keep pattern

	rootID := drv.RootID()
	if rootID == "" {
		rootID = "root"
	}

	uploadStart := time.Now()
	fileID, uploadErr := drv.Upload(ctx, rootID, uploadID+ext, r, size)
	if uploadErr != nil {
		return nil, fmt.Errorf("upload to %s: %w", drv.Kind(), uploadErr)
	}
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("upload to %s returned empty file id", drv.Kind())
	}
	log.Printf("[import] uploaded %s to %s drive=%s fileID=%s took %s",
		name, drv.Kind(), drv.ID(), fileID, time.Since(uploadStart).Round(time.Millisecond))

	if title == "" {
		title = uploadTitleFromFileName(name)
	}
	author = strings.TrimSpace(author)
	if author == "" {
		author = "链接导入"
	}

	video := &catalog.Video{
		ID:            driveVideoID(drv, fileID),
		DriveID:       drv.ID(),
		FileID:        fileID,
		FileName:      name,
		ParentID:      rootID,
		Title:         title,
		Author:        author,
		Tags:          append([]string(nil), tags...),
		Size:          size,
		Ext:           strings.TrimPrefix(ext, "."),
		PreviewStatus: "pending",
		PublishedAt:   now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := m.server.Catalog.UpsertVideo(ctx, video); err != nil {
		return nil, fmt.Errorf("catalog upsert: %w", err)
	}
	if m.server.OnVideoUploaded != nil {
		m.server.OnVideoUploaded(video)
	}
	return video, nil
}

func (m *ImportManager) notifyDriveUploadComplete(driveID string) {
	if m == nil || m.cfg.OnDriveUploadComplete == nil {
		return
	}
	driveID = strings.TrimSpace(driveID)
	if driveID == "" {
		return
	}
	m.cfg.OnDriveUploadComplete(driveID)
}

func driveVideoID(drv drives.Drive, fileID string) string {
	return drv.Kind() + "-" + drv.ID() + "-" + fileID
}
