import {
  forwardRef,
  memo,
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { Link, useLocation } from "react-router";
import { Eye } from "lucide-react";
import type {
  PreviewState,
  VideoCollectionItem,
  VideoCollectionSummary,
  VideoItem,
} from "@/types";
import { formatCount } from "@/lib/format";
import { previewController } from "@/lib/previewController";
import {
  shouldInterceptPreviewTap,
  shouldStartInstantPreview,
  TOUCH_PREVIEW_DELAY_MS,
} from "@/lib/previewIntent";
import { useInViewport } from "@/lib/useInViewport";
import { useIsActivePreview, usePreviewEnabled } from "@/lib/useIsActivePreview";
import { useLazyVideoCollection } from "@/lib/useLazyVideoCollection";
import { resolveVideoReturnPath, routeToPath } from "@/lib/videoReturnPath";
import {
  continueVideoDetailNavigationState,
  type VideoDetailNavigationState,
} from "@/lib/videoListingBackground";
import { PreviewVideo } from "./PreviewVideo";
import { VideoRailMobileHeading } from "./VideoRailMobileHeading";
import { VideoRailRowsSkeleton } from "./VideoRailSkeleton";
import { VideoThumbnail } from "./VideoThumbnail";

type Props = {
  videos: VideoItem[];
  videoId: string;
  collection?: VideoCollectionSummary;
  recommendationsLoading?: boolean;
  recommendationsError?: string;
  onRetryRecommendations?: () => void;
};

const HOVER_DELAY_MS = 300;
const HOVER_POINTER_QUERY = "(hover: hover) and (pointer: fine)";
const COLLECTION_POSITION_MAX_FRAMES = 8;
const COLLECTION_POSITION_STABLE_FRAMES = 2;
const COLLECTION_POSITION_TOLERANCE_PX = 1;

/**
 * Moves one item toward the vertical center of its own scroll container.
 * Returns true only when no further movement is possible or necessary and the
 * item is actually visible. Repeating this across a few frames lets skipped
 * content-visibility rows settle without relying on a fixed timeout.
 */
function alignCollectionItem(
  list: HTMLUListElement,
  current: HTMLLIElement
): boolean {
  if (list.clientHeight <= 0) return false;

  const listRect = list.getBoundingClientRect();
  const currentRect = current.getBoundingClientRect();
  if (listRect.height <= 0 || currentRect.height <= 0) return false;

  const centerDelta =
    currentRect.top + currentRect.height / 2 -
    (listRect.top + listRect.height / 2);
  const previousScrollTop = list.scrollTop;
  const maxScrollTop = Math.max(0, list.scrollHeight - list.clientHeight);
  const nextScrollTop = Math.min(
    maxScrollTop,
    Math.max(0, previousScrollTop + centerDelta)
  );

  if (
    Math.abs(nextScrollTop - previousScrollTop) >
    COLLECTION_POSITION_TOLERANCE_PX
  ) {
    list.scrollTop = nextScrollTop;
    return false;
  }

  return (
    currentRect.bottom >
      listRect.top + COLLECTION_POSITION_TOLERANCE_PX &&
    currentRect.top <
      listRect.bottom - COLLECTION_POSITION_TOLERANCE_PX
  );
}

type RailView = "recommended" | "collection";

/**
 * 详情页右侧 / 移动端下方的视频列表。桌面端始终使用推荐 / 合集标签栏，
 * 合集可用时允许切换；移动端保持单标题推荐列表，合集由专用底部浮窗负责。
 *
 * 不直接复用 VideoCard：那个组件结构是上下两段（缩略图 + 标题/meta），而这里需要
 * 左右横排的紧凑布局，覆盖样式会很乱。推荐项继续复用同一套预览基础设施。
 */
export function RecommendedRail({
  videos,
  videoId,
  collection,
  recommendationsLoading = false,
  recommendationsError = "",
  onRetryRecommendations,
}: Props) {
  const hasRecommendations = Array.isArray(videos) && videos.length > 0;
  const recommendationPanelAvailable =
    recommendationsLoading || hasRecommendations || !!recommendationsError;
  const hasCollection = !!collection && collection.total > 1;
  const [activeView, setActiveView] = useState<RailView>(() =>
    recommendationPanelAvailable ? "recommended" : "collection"
  );
  const [desktop, setDesktop] = useState(() =>
    typeof window !== "undefined"
      ? window.matchMedia("(min-width: 769px)").matches
      : false
  );
  const [collectionLoadStartedFor, setCollectionLoadStartedFor] = useState<
    string | null
  >(() => (!recommendationPanelAvailable && hasCollection ? videoId : null));
  const collectionViewActive =
    desktop && hasCollection && activeView === "collection";
  const collectionLoadEnabled =
    collectionViewActive ||
    (desktop && hasCollection && collectionLoadStartedFor === videoId);
  const { data, error, retry } = useLazyVideoCollection(
    videoId,
    collectionLoadEnabled,
    { includePreview: true }
  );
  const tabGroupId = useId();
  const recommendedTabRef = useRef<HTMLButtonElement | null>(null);
  const collectionTabRef = useRef<HTMLButtonElement | null>(null);
  const collectionListRef = useRef<HTMLUListElement | null>(null);
  const currentCollectionItemRef = useRef<HTMLLIElement | null>(null);
  const location = useLocation();
  const locationState = location.state as { from?: unknown } | null;
  const returnPath =
    typeof locationState?.from === "string"
      ? resolveVideoReturnPath(locationState.from)
      : resolveVideoReturnPath(routeToPath(location));
  const detailNavigationState = useMemo(
    () => continueVideoDetailNavigationState(returnPath, location.state),
    [location.state, returnPath]
  );
  const recommendedTabId = `${tabGroupId}-recommended-tab`;
  const collectionTabId = `${tabGroupId}-collection-tab`;
  const recommendedPanelId = `${tabGroupId}-recommended-panel`;
  const collectionPanelId = `${tabGroupId}-collection-panel`;

  useEffect(() => {
    if (!hasCollection && activeView === "collection") {
      setActiveView("recommended");
    } else if (
      activeView === "recommended" &&
      !recommendationPanelAvailable &&
      hasCollection
    ) {
      setActiveView("collection");
    }
  }, [activeView, hasCollection, recommendationPanelAvailable]);

  useEffect(() => {
    const media = window.matchMedia("(min-width: 769px)");
    const handleChange = () => {
      setDesktop(media.matches);
      if (!media.matches && recommendationPanelAvailable) {
        setActiveView("recommended");
      }
    };
    handleChange();
    media.addEventListener("change", handleChange);
    return () => media.removeEventListener("change", handleChange);
  }, [recommendationPanelAvailable]);

  useEffect(() => {
    if (
      !collectionViewActive ||
      !data ||
      data.items.length === 0
    ) {
      return;
    }
    let frame = 0;
    let attempts = 0;
    let stableFrames = 0;

    const positionCurrentItem = () => {
      const list = collectionListRef.current;
      const current = currentCollectionItemRef.current;
      attempts += 1;
      stableFrames =
        list && current && alignCollectionItem(list, current)
          ? stableFrames + 1
          : 0;

      if (
        attempts < COLLECTION_POSITION_MAX_FRAMES &&
        stableFrames < COLLECTION_POSITION_STABLE_FRAMES
      ) {
        frame = window.requestAnimationFrame(positionCurrentItem);
      }
    };

    frame = window.requestAnimationFrame(positionCurrentItem);
    return () => window.cancelAnimationFrame(frame);
  }, [collectionViewActive, data, videoId]);

  function selectView(nextView: RailView) {
    if (nextView === "recommended" && !recommendationPanelAvailable) return;
    if (nextView === "collection" && !hasCollection) return;
    if (nextView === activeView) return;
    if (nextView === "collection") {
      setCollectionLoadStartedFor(videoId);
    }
    previewController.setActiveId(null);
    setActiveView(nextView);
  }

  function handleTabKeyDown(event: React.KeyboardEvent<HTMLButtonElement>) {
    if (!recommendationPanelAvailable) return;
    let nextView: RailView | null = null;
    if (event.key === "ArrowLeft" || event.key === "Home") {
      nextView = "recommended";
    } else if (event.key === "ArrowRight" || event.key === "End") {
      nextView = "collection";
    }
    if (!nextView) return;
    if (nextView === "collection" && !hasCollection) return;
    event.preventDefault();
    selectView(nextView);
    const nextRef =
      nextView === "recommended" ? recommendedTabRef : collectionTabRef;
    nextRef.current?.focus();
  }

  const showCollection = hasCollection && activeView === "collection";
  const recommendedItems = useMemo(
    () =>
      videos.map((video) => (
        <RecommendedItem
          key={video.id}
          video={video}
          navigationState={detailNavigationState}
        />
      )),
    [detailNavigationState, videos]
  );
  const collectionItems = useMemo(
    () =>
      data?.items.map((video) => {
        const current = video.id === videoId;
        return (
          <RecommendedItem
            key={video.id}
            ref={current ? currentCollectionItemRef : undefined}
            video={video}
            current={current}
            navigationState={detailNavigationState}
            variant="collection"
          />
        );
      }),
    [data, detailNavigationState, videoId]
  );

  if (!recommendationPanelAvailable && !hasCollection) return null;

  return (
    <aside
      className={`vd-rail${
        recommendationPanelAvailable ? "" : " vd-rail--collection-only"
      }`}
      aria-label="视频推荐与相关合集"
    >
      <div
        className="content-tabs vd-rail__tabs"
        role="tablist"
        aria-label="视频列表"
      >
        <button
          ref={recommendedTabRef}
          id={recommendedTabId}
          type="button"
          className="content-tabs__tab vd-rail__tab"
          role="tab"
          aria-selected={activeView === "recommended"}
          aria-controls={recommendedPanelId}
          tabIndex={activeView === "recommended" ? 0 : -1}
          disabled={!recommendationPanelAvailable}
          onClick={() => selectView("recommended")}
          onKeyDown={handleTabKeyDown}
        >
          推荐视频
        </button>
        <button
          ref={collectionTabRef}
          id={collectionTabId}
          type="button"
          className="content-tabs__tab vd-rail__tab"
          role="tab"
          aria-selected={showCollection}
          aria-controls={collectionPanelId}
          tabIndex={showCollection ? 0 : -1}
          disabled={!hasCollection}
          onClick={() => selectView("collection")}
          onKeyDown={handleTabKeyDown}
        >
          相关合集
        </button>
      </div>

      <VideoRailMobileHeading />

      <div
        id={recommendedPanelId}
        className="vd-rail__tabpanel vd-rail__tabpanel--recommended"
        role="tabpanel"
        aria-labelledby={recommendedTabId}
        hidden={showCollection}
      >
        {recommendationsLoading && !hasRecommendations ? (
          <VideoRailRowsSkeleton label="正在加载推荐视频" />
        ) : recommendationsError && !hasRecommendations ? (
          <div className="vd-rail__state" role="alert">
            <span>{recommendationsError}</span>
            {onRetryRecommendations && (
              <button type="button" onClick={onRetryRecommendations}>
                重新加载
              </button>
            )}
          </div>
        ) : (
          <ul className="vd-rail__list">{recommendedItems}</ul>
        )}
      </div>

      {hasCollection && (
        <div
          id={collectionPanelId}
          className="vd-rail__tabpanel vd-rail__tabpanel--collection"
          role="tabpanel"
          aria-labelledby={collectionTabId}
          hidden={!showCollection}
        >
          {!data && !error ? (
            <VideoRailRowsSkeleton />
          ) : error && !data ? (
            <div className="vd-rail__state" role="alert">
              <span>{error}</span>
              <button type="button" onClick={retry}>
                重新加载
              </button>
            </div>
          ) : !data || data.items.length === 0 ? (
            <div className="vd-rail__state" role="status">
              当前合集暂无视频
            </div>
          ) : (
            <ul
              ref={collectionListRef}
              className="vd-rail__list vd-rail__collection-list"
              aria-label={`${data.name}，共 ${data.total} 个视频`}
            >
              {collectionItems}
            </ul>
          )}
        </div>
      )}
    </aside>
  );
}

type RailItemProps = {
  video: VideoItem | VideoCollectionItem;
  navigationState: VideoDetailNavigationState;
  current?: boolean;
  variant?: "recommended" | "collection";
};

const RecommendedItem = memo(
  forwardRef<HTMLLIElement, RailItemProps>(RecommendedItemContent)
);
RecommendedItem.displayName = "RecommendedItem";

function RecommendedItemContent(
  {
    video,
    navigationState,
    current = false,
    variant = "recommended",
  }: RailItemProps,
  forwardedRef: React.ForwardedRef<HTMLLIElement>
) {
  const [previewState, setPreviewState] = useState<PreviewState>("idle");
  const [shouldRenderPreview, setShouldRenderPreview] = useState(false);
  const [progress, setProgress] = useState(0);
  const [thumbnailActivated, setThumbnailActivated] = useState(
    variant !== "collection"
  );

  const rootRef = useRef<HTMLLIElement | null>(null);
  const previewIntentTimerRef = useRef<number | null>(null);
  const touchPreviewArmedRef = useRef(false);
  const lastPointerTypeRef = useRef<string>("");
  const videoRef = useRef<HTMLVideoElement | null>(null);

  const previewIsActive = useIsActivePreview(video.id);
  const previewEnabled = usePreviewEnabled();

  useEffect(() => {
    if (!previewEnabled) cleanup();
  }, [previewEnabled, video.id]);
  const inView = useInViewport(rootRef);
  const setRootRef = useCallback(
    (node: HTMLLIElement | null) => {
      rootRef.current = node;
      if (typeof forwardedRef === "function") {
        forwardedRef(node);
      } else if (forwardedRef) {
        forwardedRef.current = node;
      }
    },
    [forwardedRef]
  );

  // 全局预览换卡时立即清理
  useEffect(() => {
    if (
      !previewIsActive &&
      (shouldRenderPreview || touchPreviewArmedRef.current)
    ) {
      cleanup();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [previewIsActive, video.id]);

  // 合集可能包含大量视频。浏览器原生 lazy 阈值较大，仍可能一次请求几十张图；
  // 先等卡片进入共享视口观察范围，再创建图片资源，并在首次激活后保留它。
  useEffect(() => {
    if (inView) setThumbnailActivated(true);
  }, [inView]);

  // 离开视口立即停
  useEffect(() => {
    if (!inView && (shouldRenderPreview || touchPreviewArmedRef.current)) {
      cleanup();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inView]);

  // 卸载清理
  useEffect(() => {
    return () => {
      cleanup();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function cleanup() {
    clearPreviewIntentTimer();
    touchPreviewArmedRef.current = false;

    const el = videoRef.current;
    if (el) {
      try {
        el.pause();
        el.removeAttribute("src");
        el.load();
      } catch {
        // noop
      }
    }

    setShouldRenderPreview(false);
    setPreviewState("idle");
    setProgress(0);

    if (previewController.getActiveId() === video.id) {
      previewController.setActiveId(null);
    }
  }

  function startPreviewIntent() {
    if (!previewController.isEnabled() || !video.previewSrc) return;
    if (!inView) return;
    if (previewIntentTimerRef.current) return;
    setPreviewState("intent");
    previewIntentTimerRef.current = window.setTimeout(() => {
      previewIntentTimerRef.current = null;
      startPreviewNow({ requireInView: true });
    }, HOVER_DELAY_MS);
  }

  function startTouchPreviewIntent() {
    if (!previewController.isEnabled() || !video.previewSrc) return;
    clearPreviewIntentTimer();
    touchPreviewArmedRef.current = true;
    previewController.setActiveId(video.id);
    setPreviewState("intent");
    previewIntentTimerRef.current = window.setTimeout(() => {
      previewIntentTimerRef.current = null;
      if (
        !previewController.isEnabled() ||
        !touchPreviewArmedRef.current ||
        previewController.getActiveId() !== video.id
      ) {
        return;
      }
      startPreviewNow({ requireInView: false });
    }, TOUCH_PREVIEW_DELAY_MS);
  }

  function clearPreviewIntentTimer() {
    if (previewIntentTimerRef.current === null) return;
    window.clearTimeout(previewIntentTimerRef.current);
    previewIntentTimerRef.current = null;
  }

  function startPreviewNow(options: { requireInView: boolean }) {
    if (!previewController.isEnabled() || !video.previewSrc) return;
    if (options.requireInView && !inView) return;
    clearPreviewIntentTimer();
    previewController.setActiveId(video.id);
    setShouldRenderPreview(true);
    setPreviewState("loading");
  }

  function stopPreview() {
    cleanup();
  }

  function handlePointerEnter(event: React.PointerEvent<HTMLLIElement>) {
    lastPointerTypeRef.current = event.pointerType;
    if (shouldStartInstantPreview({ pointerType: event.pointerType })) return;
    startPreviewIntent();
  }

  function handlePointerLeave(event: React.PointerEvent<HTMLLIElement>) {
    if (shouldStartInstantPreview({ pointerType: event.pointerType })) return;
    stopPreview();
  }

  function handlePointerDown(event: React.PointerEvent<HTMLLIElement>) {
    lastPointerTypeRef.current = event.pointerType;
  }

  function handleClickCapture(event: React.MouseEvent<HTMLAnchorElement>) {
    const previewActive =
      previewController.getActiveId() === video.id &&
      (touchPreviewArmedRef.current || shouldRenderPreview);
    if (
      !shouldInterceptPreviewTap({
        previewEnabled: previewController.isEnabled() && Boolean(video.previewSrc),
        pointerType: lastPointerTypeRef.current,
        canHover: window.matchMedia(HOVER_POINTER_QUERY).matches,
        previewActive,
      })
    ) {
      if (touchPreviewArmedRef.current && !shouldRenderPreview) cleanup();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    startTouchPreviewIntent();
  }

  const author = "author" in video ? video.author : "";

  return (
    <li
      ref={setRootRef}
      data-preview-enabled={previewEnabled}
      className={`vd-rail__item${
        variant === "collection" ? " vd-rail__collection-item" : ""
      }`}
      onPointerEnter={handlePointerEnter}
      onPointerLeave={handlePointerLeave}
      onPointerDown={handlePointerDown}
      onFocus={startPreviewIntent}
      onBlur={stopPreview}
    >
      <Link
        to={video.href}
        state={navigationState}
        className="vd-rail__link"
        aria-current={current ? "page" : undefined}
        onClickCapture={handleClickCapture}
        onClick={current ? (event) => event.preventDefault() : undefined}
      >
        <div className="vd-rail__thumb">
          <VideoThumbnail
            src={video.thumbnail}
            enabled={thumbnailActivated}
          />
          {previewEnabled && shouldRenderPreview && video.previewSrc && (
            <PreviewVideo
              ref={videoRef}
              src={video.previewSrc}
              state={previewState}
              onCanPlay={() => setPreviewState("playing")}
              onError={() => setPreviewState("error")}
              onTimeUpdate={(p) => setProgress(p)}
            />
          )}
          {previewEnabled && previewState === "loading" && (
            <span className="preview-loader" />
          )}
          {previewEnabled && previewState === "error" && (
            <span className="preview-error">预览加载失败</span>
          )}
          {previewEnabled && previewState === "playing" && (
            <div className="preview-progress" aria-hidden="true">
              <div
                className="preview-progress__bar"
                style={{ width: `${Math.min(100, progress * 100)}%` }}
              />
            </div>
          )}
          {video.duration && (!previewEnabled || previewState !== "playing") && (
            <span className="vd-rail__duration">{video.duration}</span>
          )}
          {current && (!previewEnabled || previewState !== "playing") && (
            <span className="vd-rail__current">当前视频</span>
          )}
        </div>
        <div className="vd-rail__body">
          <h3 className="vd-rail__title" title={video.title}>
            {video.title}
          </h3>
          <div className="vd-rail__meta">
            {author && <span className="vd-rail__author">{author}</span>}
            {variant === "collection" ? (
              <span className="vd-rail__views">
                <Eye size={12} aria-hidden="true" />
                {formatCount(video.views)} 观看
              </span>
            ) : (
              <span>{formatCount(video.views)} 观看</span>
            )}
            {video.publishedAt && <span>{video.publishedAt}</span>}
          </div>
        </div>
      </Link>
    </li>
  );
}
