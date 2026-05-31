package preview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/video-site/backend/internal/catalog"
	"github.com/video-site/backend/internal/drives"
)

const hlsGenerationTimeout = 2 * time.Hour

type HLSGenerator interface {
	GenerateHLS(ctx context.Context, link *drives.StreamLink, videoID string) (string, error)
}

type HLSWorker struct {
	Gen     HLSGenerator
	Catalog *catalog.Catalog
	Drive   drives.Drive
	ch      chan *catalog.Video
	queue   videoQueue

	RateLimitCooldown time.Duration
	rateLimit         rateLimitState
	activity          taskActivity
}

func NewHLSWorker(gen HLSGenerator, cat *catalog.Catalog, drv drives.Drive) *HLSWorker {
	return &HLSWorker{
		Gen:     gen,
		Catalog: cat,
		Drive:   drv,
		ch:      make(chan *catalog.Video, 256),
	}
}

func (w *HLSWorker) Enqueue(v *catalog.Video) bool {
	if v == nil {
		return false
	}
	if !w.queue.reserve(v) {
		return true
	}
	select {
	case w.ch <- v:
		return true
	default:
		w.queue.release(v)
		return false
	}
}

func (w *HLSWorker) EnqueueBlocking(ctx context.Context, v *catalog.Video) bool {
	if v == nil {
		return false
	}
	if !w.queue.reserve(v) {
		return true
	}
	select {
	case w.ch <- v:
		return true
	case <-ctx.Done():
		w.queue.release(v)
		return false
	}
}

func (w *HLSWorker) Status() TaskStatus {
	if w == nil {
		return TaskStatus{State: "idle"}
	}
	currentID, _ := w.activity.current()
	return taskStatus(&w.activity, &w.rateLimit, w.queue.lengthExcluding(currentID))
}

func (w *HLSWorker) WaitIdle(ctx context.Context) error {
	if w == nil {
		return nil
	}
	return waitQueueIdle(ctx, &w.queue)
}

func (w *HLSWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-w.ch:
			w.processQueued(ctx, v)
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func (w *HLSWorker) processQueued(ctx context.Context, v *catalog.Video) {
	defer w.queue.release(v)
	w.activity.start(v)
	defer w.activity.done()
	if !waitForRateLimitCooldown(ctx, &w.rateLimit, "hls", w.Drive) {
		return
	}
	w.process(ctx, v)
}

func (w *HLSWorker) skipIfRateLimited() bool {
	until, ok, shouldLog := w.rateLimit.active(time.Now())
	if !ok {
		return false
	}
	if shouldLog {
		log.Printf("[hls] drive=%s rate-limited until=%s; keep queued videos pending", w.Drive.ID(), until.Format(time.RFC3339))
	}
	return true
}

func (w *HLSWorker) pauseForRecoverableError(err error, step string) bool {
	if _, ok := drives.RateLimitRetryAfter(err); ok {
		until := w.rateLimit.pause(time.Now(), defaultGenerationRateLimitCooldown)
		log.Printf("[hls] drive=%s rate-limited until=%s step=%s: %v", w.Drive.ID(), until.Format(time.RFC3339), step, err)
		return true
	}
	if !driveErrorShouldCooldown(w.Drive, err) {
		return false
	}
	until := w.rateLimit.pause(time.Now(), w.RateLimitCooldown)
	log.Printf("[hls] drive=%s transient media source error until=%s step=%s: %v", w.Drive.ID(), until.Format(time.RFC3339), step, err)
	return true
}

func (w *HLSWorker) process(ctx context.Context, v *catalog.Video) {
	if w.skipIfRateLimited() {
		return
	}
	_ = w.Catalog.UpdateHLS(ctx, v.ID, "", "generating", "")

	link, err := w.Drive.StreamURL(ctx, v.FileID)
	if err != nil {
		if w.pauseForRecoverableError(err, "streamURL") {
			_ = w.Catalog.UpdateHLS(ctx, v.ID, "", "pending", "")
			return
		}
		log.Printf("[hls] streamURL drive=%s: %v", w.Drive.ID(), err)
		_ = w.Catalog.UpdateHLS(ctx, v.ID, "", "failed", truncateHLError(err))
		return
	}

	dir, err := w.Gen.GenerateHLS(ctx, link, v.ID)
	if err != nil {
		if w.pauseForRecoverableError(err, "generate") {
			_ = w.Catalog.UpdateHLS(ctx, v.ID, "", "pending", "")
			return
		}
		log.Printf("[hls] generate drive=%s: %v", w.Drive.ID(), err)
		_ = w.Catalog.UpdateHLS(ctx, v.ID, "", "failed", truncateHLError(err))
		return
	}
	_ = w.Catalog.UpdateHLS(ctx, v.ID, dir, "ready", "")
	log.Printf("[hls] ready drive=%s", w.Drive.ID())
}

func truncateHLError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(redactURLs(err.Error()))
	if len(text) > 1000 {
		text = text[:1000]
	}
	return text
}

func (g *Generator) GenerateHLS(ctx context.Context, link *drives.StreamLink, videoID string) (string, error) {
	if strings.TrimSpace(g.cfg.LocalDir) == "" {
		return "", errors.New("local preview directory is not configured")
	}
	root := filepath.Join(g.cfg.LocalDir, "hls")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	name := hlsAssetDirName(videoID)
	finalDir := filepath.Join(root, name)
	tmpDir := filepath.Join(root, ".tmp-"+name+"-"+fmt.Sprint(time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	ctx2, cancel := context.WithTimeout(ctx, hlsGenerationTimeout)
	defer cancel()
	ffmpegLink, cleanup, err := prepareFFmpegLink(ctx2, link)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := g.runHLSFFmpeg(ctx2, tmpDir, ffmpegLink, "fmp4"); err != nil {
		if ctx2.Err() != nil {
			return "", err
		}
		_ = os.RemoveAll(tmpDir)
		if mkErr := os.MkdirAll(tmpDir, 0o755); mkErr != nil {
			return "", mkErr
		}
		if fallbackErr := g.runHLSFFmpeg(ctx2, tmpDir, ffmpegLink, "mpegts"); fallbackErr != nil {
			return "", fmt.Errorf("%v; mpegts fallback: %w", err, fallbackErr)
		}
	}

	playlist := filepath.Join(tmpDir, "index.m3u8")
	info, err := os.Stat(playlist)
	if err != nil {
		return "", err
	}
	if info.Size() == 0 {
		return "", errors.New("hls playlist is empty")
	}

	if err := os.RemoveAll(finalDir); err != nil {
		return "", err
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return "", err
	}
	return finalDir, nil
}

func (g *Generator) runHLSFFmpeg(ctx context.Context, dir string, link *drives.StreamLink, mode string) error {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
	}
	args = append(args, ffmpegHTTPInputOptions(link)...)
	args = append(args,
		"-i", link.URL,
		"-map", "0:v:0?",
		"-map", "0:a:0?",
		"-c", "copy",
		"-sn",
		"-dn",
		"-avoid_negative_ts", "make_zero",
		"-f", "hls",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_flags", "independent_segments",
	)
	switch mode {
	case "fmp4":
		args = append(args,
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", "init.mp4",
			"-hls_segment_filename", "seg-%05d.m4s",
		)
	case "mpegts":
		args = append(args,
			"-hls_segment_filename", "seg-%05d.ts",
		)
	default:
		return fmt.Errorf("unknown hls mode: %s", mode)
	}
	args = append(args, "-y", "index.m3u8")

	cmd := exec.CommandContext(ctx, g.cfg.FFmpegPath, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ffmpegCommandError("ffmpeg hls "+mode, err, out)
	}
	return nil
}

func hlsAssetDirName(videoID string) string {
	sum := sha256.Sum256([]byte(videoID))
	return hex.EncodeToString(sum[:16])
}
