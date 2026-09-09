package preview

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGlobalSwitchRejectsNewAndQueuedPreviews(t *testing.T) {
	ctx := context.Background()
	cat, video := seedPreviewTestVideo(t, "global-preview")
	gen := &fakeTeaserGenerator{}
	drv := &previewFakeDrive{}
	worker := NewWorker(gen, cat, drv)
	enabled := false
	worker.Enabled = func() bool { return enabled }
	if worker.Enqueue(video) || worker.EnqueueBlocking(ctx, video) || worker.Status().QueueLength != 0 {
		t.Fatal("disabled worker admitted a preview")
	}
	enabled = true
	if !worker.Enqueue(video) {
		t.Fatal("enabled worker rejected a preview")
	}
	enabled = false
	worker.processQueued(ctx, <-worker.ch)
	got, err := cat.GetVideo(ctx, video.ID)
	if err != nil || got.PreviewStatus != "pending" || worker.Status().QueueLength != 0 || gen.generateCalls != 0 {
		t.Fatalf("queued preview was not left pending: %+v, %v", got, err)
	}
	enabled = true
	if !worker.Enqueue(video) {
		t.Fatal("re-enabled worker rejected pending preview")
	}
	run := worker.prepareQueued(ctx, <-worker.ch)
	if run == nil {
		t.Fatal("could not prepare preview")
	}
	enabled = false
	run()
	if gen.generateCalls != 0 || worker.Status().QueueLength != 0 {
		t.Fatal("preview started after global switch was disabled")
	}
	enabled = true
	worker.Enqueue(video)
	worker.processQueued(ctx, <-worker.ch)
	got, err = cat.GetVideo(ctx, video.ID)
	if err != nil || got.PreviewStatus != "ready" || gen.generateCalls != 1 {
		t.Fatalf("re-enabled preview did not complete: %+v, %v", got, err)
	}
}

func TestDisablingPreviewsLetsRunningGenerationFinish(t *testing.T) {
	cat, video := seedPreviewTestVideo(t, "running-global-preview")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	release := make(chan struct{})
	gen := &blockingTeaserGenerator{started: make(chan struct{}, 1), release: release}
	worker := NewWorker(gen, cat, &previewFakeDrive{})
	var enabled atomic.Bool
	enabled.Store(true)
	worker.Enabled = enabled.Load
	if !worker.Enqueue(video) {
		t.Fatal("could not enqueue preview")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		worker.processQueued(ctx, <-worker.ch)
	}()
	defer func() { cancel(); <-done }()
	select {
	case <-gen.started:
	case <-ctx.Done():
		t.Fatal("generation did not start")
	}
	enabled.Store(false)
	close(release)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("running generation did not finish")
	}
	got, err := cat.GetVideo(ctx, video.ID)
	if err != nil || got.PreviewStatus != "ready" || got.PreviewLocal == "" {
		t.Fatalf("running generation was interrupted: %+v, %v", got, err)
	}
}
