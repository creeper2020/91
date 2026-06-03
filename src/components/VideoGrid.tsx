import type { VideoItem } from "@/types";
import { VideoCard } from "./VideoCard";

type Props = {
  videos: VideoItem[];
  loading?: boolean;
  compact?: boolean;
  emptyText?: string;
  priorityCount?: number;
  skeletonCount?: number;
};

export function VideoGrid({
  videos,
  loading,
  compact,
  emptyText = "暂时没有视频",
  priorityCount = 0,
  skeletonCount = 8,
}: Props) {
  if (loading) {
    return (
      <div className="video-grid-loading" aria-busy="true">
        {Array.from({ length: skeletonCount }).map((_, i) => (
          <div key={i} className="skeleton-card" />
        ))}
      </div>
    );
  }

  if (!videos || videos.length === 0) {
    return <div className="video-grid-empty">{emptyText}</div>;
  }

  return (
    <div className={`video-grid ${compact ? "is-compact" : ""}`}>
      {(videos ?? []).map((v, index) => (
        <VideoCard key={v.id} video={v} priority={index < priorityCount} />
      ))}
    </div>
  );
}
