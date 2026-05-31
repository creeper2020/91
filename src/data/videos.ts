import type { VideoDetail, VideoItem } from "@/types";

// 真实后端接口调用。未配置网盘时，各接口返回空数据。
export function fetchHomeVideos(): Promise<VideoItem[]> {
  return apiGet<VideoItem[]>("/api/home").catch(() => []);
}

export function fetchListing(
  page: number,
  pageSize: number,
  params?: { q?: string; tag?: string; cat?: string; sort?: string }
): Promise<{ items: VideoItem[]; total: number }> {
  const qs = new URLSearchParams({
    page: String(page),
    size: String(pageSize),
  });
  if (params?.q) qs.set("q", params.q);
  if (params?.tag) qs.set("tag", params.tag);
  if (params?.cat) qs.set("cat", params.cat);
  if (params?.sort) qs.set("sort", params.sort);
  return apiGet<{ items: VideoItem[]; total: number }>(
    `/api/list?${qs.toString()}`
  ).catch(() => ({ items: [], total: 0 }));
}

export function fetchVideoDetail(id: string): Promise<VideoDetail | null> {
  return apiGet<VideoDetail>(`/api/video/${encodeURIComponent(id)}`).catch(
    () => null
  );
}

export function updateVideoTags(
  id: string,
  tags: string[]
): Promise<VideoItem> {
  return apiJSON<VideoItem>(`/api/video/${encodeURIComponent(id)}/tags`, {
    method: "PUT",
    body: JSON.stringify({ tags }),
  });
}

export function hideVideo(id: string): Promise<{ ok: boolean }> {
  return apiJSON<{ ok: boolean }>(
    `/api/video/${encodeURIComponent(id)}/hide`,
    { method: "POST" }
  );
}

export function recordView(id: string): Promise<{ views: number }> {
  return apiJSON<{ views: number }>(
    `/api/video/${encodeURIComponent(id)}/view`,
    { method: "POST" }
  );
}

export type UploadVideoInput = {
  file: File;
  title: string;
  tags: string[];
};

export function uploadVideo(input: UploadVideoInput): Promise<VideoItem> {
  const body = new FormData();
  body.append("file", input.file);
  if (input.title.trim()) {
    body.append("title", input.title.trim());
  }
  for (const tag of input.tags) {
    body.append("tags", tag);
  }
  return apiForm<VideoItem>("/api/upload", body);
}

export type ImportJob = {
  id: string;
  sourceUrl: string;
  status: "queued" | "running" | "importing" | "done" | "failed" | string;
  message?: string;
  error?: string;
  videoIds?: string[];
  videos?: VideoItem[];
  createdAt: string;
  updatedAt: string;
  finishedAt?: string;
};

export type CreateImportInput = {
  url: string;
  title: string;
  tags: string[];
};

export function createImportJob(input: CreateImportInput): Promise<ImportJob> {
  return apiJSON<ImportJob>("/api/imports", {
    method: "POST",
    body: JSON.stringify({
      url: input.url.trim(),
      title: input.title.trim(),
      tags: input.tags,
    }),
  });
}

export function fetchImportJob(id: string): Promise<ImportJob> {
  return apiGet<ImportJob>(`/api/imports/${encodeURIComponent(id)}`);
}

export type TagItem = { id: string; label: string; count?: number };

export function fetchTags(): Promise<TagItem[]> {
  return apiGet<TagItem[]>("/api/tags").catch(() => []);
}

/** 短视频模式单条记录。比 VideoItem 多 videoSrc / poster。 */
export type ShortsItem = VideoItem & {
  videoSrc: string;
  hlsSrc?: string;
  poster: string;
};

/** 短视频"取下一批"接口的响应。 */
export type ShortsNextResponse = {
  items: ShortsItem[];
  total: number;
  /** true 表示这批返回少于 count，前端播放完毕后应清空 seenIds 开新一轮 */
  roundComplete: boolean;
};

/**
 * 拉取短视频流的下一批候选。把当前轮已看过的 video id 列表传给后端，
 * 服务器从未在列表中的视频里随机抽 count 条返回。preferredFromVideoId
 * 来自用户最近一次点赞成功的视频，用于按相似标签优先推荐。
 *
 * 失败时返回空批 + roundComplete=false，由调用方决定是否重试。
 */
export function fetchShortsNext(
  seenIds: string[],
  count: number,
  preferredFromVideoId?: string
): Promise<ShortsNextResponse> {
  return apiJSON<ShortsNextResponse>("/api/shorts/next", {
    method: "POST",
    body: JSON.stringify({ seenIds, count, preferredFromVideoId }),
  }).catch(() => ({ items: [], total: 0, roundComplete: false }));
}

async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(path, { credentials: "include" });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

async function apiJSON<T>(path: string, init: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

async function apiForm<T>(path: string, body: FormData): Promise<T> {
  const res = await fetch(path, {
    method: "POST",
    credentials: "include",
    body,
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
