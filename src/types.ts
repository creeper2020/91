export type VideoItem = {
  id: string;
  href: string;
  title: string;
  thumbnail: string;
  previewSrc: string;
  previewDuration: number;
  previewStrategy: "teaser-file" | "sprite-frames";
  duration: string;
  badges: string[];
  quality?: "SD" | "HD";
  sourceLabel?: string;
  author: string;
  views: number;
  favorites?: number;
  comments?: number;
  likes?: number;
  dislikes?: number;
  publishedAt: string;
  rating?: number;
  tags?: string[];
  category?: string;
};

export type AuthorProfile = {
  id: string;
  name: string;
  href: string;
  badges: string[];
  signupAge?: string;
  level?: number;
  points?: number;
  videoCount?: number;
  followers?: number;
  following?: number;
  isFollowing?: boolean;
};

export type CommentItem = {
  id: string;
  author: string;
  body: string;
  createdAt: string;
  likes?: number;
};

export type VideoDetail = VideoItem & {
  videoSrc: string;
  hlsSrc?: string;
  poster: string;
  description: string;
  embedUrl: string;
  points?: number;
  authorProfile: AuthorProfile;
  relatedVideos: VideoItem[];
  commentsList: CommentItem[];
};

export type PreviewState = "idle" | "intent" | "loading" | "playing" | "error";

export type SortKey = "latest" | "hot" | "week" | "long" | "hd" | "featured";

export type TagItem = {
  id: string;
  label: string;
  count?: number;
};

export type CategoryItem = {
  id: string;
  label: string;
  href: string;
};

export type PromoItem = {
  id: string;
  kind: "channel" | "collection" | "event";
  label: string;
  title: string;
  meta?: string;
};
