package catalog

import (
	"context"
	"testing"
	"time"
)

// TestRandomVideosExcluding 验证短视频"不重复随机"的核心数据层行为：
// 1) 已传入的 id 不会被返回
// 2) 同一调用返回的视频之间互不相同
// 3) limit 大于剩余可选时只返回剩余的全部
// 4) 隐藏的视频不会出现在结果中
func TestRandomVideosExcluding(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	// 6 个可见 + 1 个隐藏
	all := []string{"v1", "v2", "v3", "v4", "v5", "v6"}
	for i, id := range all {
		if err := cat.UpsertVideo(ctx, &Video{
			ID:          id,
			DriveID:     "drive",
			FileID:      "f-" + id,
			Title:       id,
			PublishedAt: now.Add(time.Duration(i) * time.Second),
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	if err := cat.UpsertVideo(ctx, &Video{
		ID: "v-hidden", DriveID: "drive", FileID: "f-hidden",
		Title: "hidden", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed hidden: %v", err)
	}
	if err := cat.HideVideo(ctx, "v-hidden"); err != nil {
		t.Fatalf("hide v-hidden: %v", err)
	}

	total, err := cat.CountVisibleVideos(ctx)
	if err != nil {
		t.Fatalf("count visible: %v", err)
	}
	if total != len(all) {
		t.Fatalf("visible count = %d, want %d", total, len(all))
	}

	// 1) 排除 v1, v2, v3，请求 2 个，应当从 {v4,v5,v6} 里随机取 2 个，互不相同
	got, err := cat.RandomVideosExcluding(ctx, []string{"v1", "v2", "v3"}, 2)
	if err != nil {
		t.Fatalf("random excluding: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d items, want 2", len(got))
	}
	seen := map[string]struct{}{}
	for _, v := range got {
		if v.ID == "v1" || v.ID == "v2" || v.ID == "v3" {
			t.Fatalf("excluded id %s was returned", v.ID)
		}
		if v.ID == "v-hidden" {
			t.Fatalf("hidden video was returned")
		}
		if _, dup := seen[v.ID]; dup {
			t.Fatalf("duplicate id in result: %s", v.ID)
		}
		seen[v.ID] = struct{}{}
	}

	// 2) limit 大于剩余可选时只返回全部剩余
	got2, err := cat.RandomVideosExcluding(ctx, []string{"v1", "v2", "v3", "v4"}, 10)
	if err != nil {
		t.Fatalf("random excluding (oversize limit): %v", err)
	}
	if len(got2) != 2 {
		t.Fatalf("oversize-limit result = %d items, want 2 (v5, v6)", len(got2))
	}

	// 3) 不传 exclude 时返回 limit 个不重复
	got3, err := cat.RandomVideosExcluding(ctx, nil, 4)
	if err != nil {
		t.Fatalf("random no exclude: %v", err)
	}
	if len(got3) != 4 {
		t.Fatalf("no-exclude result = %d, want 4", len(got3))
	}
	dedupe := map[string]struct{}{}
	for _, v := range got3 {
		if _, dup := dedupe[v.ID]; dup {
			t.Fatalf("no-exclude duplicate id: %s", v.ID)
		}
		dedupe[v.ID] = struct{}{}
	}

	// 4) limit <= 0 直接返回 nil
	got4, err := cat.RandomVideosExcluding(ctx, nil, 0)
	if err != nil {
		t.Fatalf("limit 0: %v", err)
	}
	if got4 != nil {
		t.Fatalf("limit 0 should return nil, got %v", got4)
	}
}

func TestRandomVideosForPreferredVideoChoosesLeastPopulatedTag(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	for _, v := range []*Video{
		{ID: "current", DriveID: "drive", FileID: "f-current", Title: "current", Tags: []string{"common", "rare"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "common-1", DriveID: "drive", FileID: "f-common-1", Title: "common 1", Tags: []string{"common"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "common-2", DriveID: "drive", FileID: "f-common-2", Title: "common 2", Tags: []string{"common"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "rare-1", DriveID: "drive", FileID: "f-rare-1", Title: "rare 1", Tags: []string{"rare"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed %s: %v", v.ID, err)
		}
	}

	tag, err := cat.LeastPopulatedVisibleUniqueTag(ctx, []string{"common", "rare"})
	if err != nil {
		t.Fatalf("least populated tag: %v", err)
	}
	if tag != "rare" {
		t.Fatalf("least populated tag = %q, want rare", tag)
	}

	got, err := cat.RandomVideosForPreferredVideoExcluding(ctx, "current", []string{"current"}, 1)
	if err != nil {
		t.Fatalf("random preferred: %v", err)
	}
	if len(got) != 1 || got[0].ID != "rare-1" {
		t.Fatalf("preferred result = %#v, want rare-1", videoIDs(got))
	}

	got, err = cat.RandomVideosForPreferredVideoExcluding(ctx, "current", nil, 1)
	if err != nil {
		t.Fatalf("random preferred without explicit exclude: %v", err)
	}
	if len(got) != 1 || got[0].ID == "current" {
		t.Fatalf("preferred result without explicit exclude = %#v, should not return current", videoIDs(got))
	}
}

func TestRandomVideosForPreferredVideoFallsBackToFillBatch(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	for _, v := range []*Video{
		{ID: "current", DriveID: "drive", FileID: "f-current", Title: "current", Tags: []string{"common", "rare"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "common-1", DriveID: "drive", FileID: "f-common-1", Title: "common 1", Tags: []string{"common"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "common-2", DriveID: "drive", FileID: "f-common-2", Title: "common 2", Tags: []string{"common"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "rare-1", DriveID: "drive", FileID: "f-rare-1", Title: "rare 1", Tags: []string{"rare"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "hidden-rare", DriveID: "drive", FileID: "f-hidden-rare", Title: "hidden rare", Tags: []string{"rare"}, PublishedAt: now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed %s: %v", v.ID, err)
		}
	}
	if err := cat.HideVideo(ctx, "hidden-rare"); err != nil {
		t.Fatalf("hide hidden-rare: %v", err)
	}

	got, err := cat.RandomVideosForPreferredVideoExcluding(ctx, "current", []string{"current"}, 3)
	if err != nil {
		t.Fatalf("random preferred: %v", err)
	}
	ids := videoIDs(got)
	if len(ids) != 3 {
		t.Fatalf("result ids = %#v, want 3 items", ids)
	}
	for _, excluded := range []string{"current", "hidden-rare"} {
		if hasVideoID(ids, excluded) {
			t.Fatalf("result ids = %#v, should not include %s", ids, excluded)
		}
	}
	if !hasVideoID(ids, "rare-1") {
		t.Fatalf("result ids = %#v, want rare-1 from least populated tag", ids)
	}
	if len(uniqueVideoIDs(ids)) != len(ids) {
		t.Fatalf("result ids = %#v, want no duplicates", ids)
	}
}

func TestRandomVideosForPreferredVideoFallbacksWhenPreferenceUnavailable(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/catalog.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	t.Cleanup(func() { _ = cat.Close() })

	now := time.Now()
	for _, v := range []*Video{
		{ID: "untagged", DriveID: "drive", FileID: "f-untagged", Title: "untagged", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "visible-1", DriveID: "drive", FileID: "f-visible-1", Title: "visible 1", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: "visible-2", DriveID: "drive", FileID: "f-visible-2", Title: "visible 2", PublishedAt: now, CreatedAt: now, UpdatedAt: now},
	} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("seed %s: %v", v.ID, err)
		}
	}

	got, err := cat.RandomVideosForPreferredVideoExcluding(ctx, "missing", []string{"untagged"}, 2)
	if err != nil {
		t.Fatalf("random missing preferred: %v", err)
	}
	if !sameVideoIDSet(videoIDs(got), []string{"visible-1", "visible-2"}) {
		t.Fatalf("missing preferred ids = %#v, want visible fallback videos", videoIDs(got))
	}

	got, err = cat.RandomVideosForPreferredVideoExcluding(ctx, "untagged", []string{"untagged"}, 2)
	if err != nil {
		t.Fatalf("random untagged preferred: %v", err)
	}
	if !sameVideoIDSet(videoIDs(got), []string{"visible-1", "visible-2"}) {
		t.Fatalf("untagged preferred ids = %#v, want visible fallback videos", videoIDs(got))
	}
}

func videoIDs(videos []*Video) []string {
	ids := make([]string, 0, len(videos))
	for _, v := range videos {
		ids = append(ids, v.ID)
	}
	return ids
}

func hasVideoID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func uniqueVideoIDs(ids []string) map[string]struct{} {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return seen
}

func sameVideoIDSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, value := range a {
		seen[value]++
	}
	for _, value := range b {
		if seen[value] == 0 {
			return false
		}
		seen[value]--
	}
	return true
}
