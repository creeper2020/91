import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { AppShell } from "@/components/AppShell";
import { VideoPlayer } from "@/components/VideoPlayer";
import { VideoActions } from "@/components/VideoActions";
import { VideoMetaHeader } from "@/components/VideoMetaHeader";
import { VideoInfoPanel } from "@/components/VideoInfoPanel";
import { RecommendedRail } from "@/components/RecommendedRail";
import {
  fetchTags,
  fetchVideoDetail,
  hideVideo,
  recordView,
  updateVideoTags,
} from "@/data/videos";
import type { TagItem, VideoDetail } from "@/types";

export default function VideoDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [detail, setDetail] = useState<VideoDetail | null>(null);
  const [tags, setTags] = useState<TagItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [tagSaving, setTagSaving] = useState(false);
  const [hideSaving, setHideSaving] = useState(false);
  const detailTopRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!id) return;
    let active = true;
    window.scrollTo({ top: 0, behavior: "auto" });
    setLoading(true);
    Promise.all([fetchVideoDetail(id), fetchTags()]).then(([d, tagList]) => {
      if (!active) return;
      setDetail(d);
      setTags(tagList);
      setLoading(false);
      document.title = d ? `${d.title} · 91` : "视频不存在";
    });
    return () => {
      active = false;
    };
  }, [id]);

  useLayoutEffect(() => {
    if (loading || !detail) return;
    window.requestAnimationFrame(() => {
      detailTopRef.current?.scrollIntoView({
        block: "start",
        behavior: "auto",
      });
    });
  }, [loading, detail?.id]);

  async function handleTagsChange(nextTags: string[]) {
    if (!detail) return;
    setTagSaving(true);
    try {
      const updated = await updateVideoTags(detail.id, nextTags);
      setDetail({ ...detail, tags: updated.tags ?? [] });
    } finally {
      setTagSaving(false);
    }
  }

  async function handleHideVideo() {
    if (!detail || hideSaving) return;
    if (!window.confirm("确定以后不再展示这个视频吗？")) return;
    setHideSaving(true);
    try {
      await hideVideo(detail.id);
      navigate("/list", { replace: true });
    } catch {
      setHideSaving(false);
      window.alert("隐藏失败，请稍后重试");
    }
  }

  function handleFirstPlay() {
    if (!detail) return;
    // 失败静默忽略，不打扰用户播放体验
    recordView(detail.id).catch(() => undefined);
  }

  if (loading) {
    return (
      <AppShell mobileAutoHideNav>
        <div className="vd-page">
          <div className="vd-ambient" aria-hidden="true" />
          <div className="container vd-page__inner">
            <div
              className="vd-layout vd-skeleton"
              aria-busy="true"
              aria-label="视频详情加载中"
            >
              <div className="vd-main">
                <div className="vd-skeleton__player" />

                <div className="vd-skeleton__summary">
                  <div className="vd-skeleton__chips">
                    <span className="vd-skeleton__chip vd-skeleton__chip--source" />
                    <span className="vd-skeleton__chip" />
                    <span className="vd-skeleton__chip vd-skeleton__chip--plain" />
                    <span className="vd-skeleton__chip vd-skeleton__chip--plain" />
                  </div>
                  <div className="vd-skeleton__title" />
                  <div className="vd-skeleton__actions">
                    <span />
                    <span />
                    <span />
                  </div>
                </div>

                <div className="vd-skeleton__info">
                  <span className="vd-skeleton__section-head" />
                  <span className="vd-skeleton__line" />
                  <span className="vd-skeleton__line vd-skeleton__line--short" />
                  <div className="vd-skeleton__tag-row">
                    <span />
                    <span />
                    <span />
                  </div>
                </div>
              </div>

              <aside className="vd-rail vd-skeleton__rail">
                <div className="vd-rail__head">
                  <span className="vd-rail__head-icon" aria-hidden="true">
                    <span />
                    <span />
                  </span>
                  <span className="vd-skeleton__rail-head" />
                </div>
                <ul className="vd-rail__list vd-skeleton__rail-list">
                  {Array.from({ length: 6 }).map((_, index) => (
                    <li key={index} className="vd-skeleton__rail-item">
                      <span className="vd-skeleton__rail-thumb" />
                      <span className="vd-skeleton__rail-body">
                        <span className="vd-skeleton__rail-title" />
                        <span className="vd-skeleton__rail-title vd-skeleton__rail-title--short" />
                        <span className="vd-skeleton__rail-meta" />
                      </span>
                    </li>
                  ))}
                </ul>
              </aside>
            </div>
          </div>
        </div>
      </AppShell>
    );
  }

  if (!detail) {
    return (
      <AppShell mobileAutoHideNav>
        <div className="vd-page">
          <div className="container vd-page__inner">
            <div className="vd-empty">视频不存在或已被移除</div>
          </div>
        </div>
      </AppShell>
    );
  }

  return (
    <AppShell mobileAutoHideNav>
      <div className="vd-page">
        {/* Ambient 背景层：用海报作模糊底色，叠加渐变过渡到页面背景 */}
        <div
          className="vd-ambient"
          aria-hidden="true"
          style={{
            backgroundImage: detail.poster
              ? `url(${detail.poster})`
              : undefined,
          }}
        />

        <div className="container vd-page__inner">
          <div className="vd-layout">
            <div className="vd-main" ref={detailTopRef}>
              <div className="vd-player-wrap">
                <div className="vd-player">
                  <VideoPlayer
                    id={detail.id}
                    src={detail.videoSrc}
                    poster={detail.poster}
                    previewSrc={detail.previewSrc}
                    title={detail.title}
                    onFirstPlay={handleFirstPlay}
                  />
                </div>
              </div>

              <section className="vd-summary" aria-label="当前视频">
                <VideoMetaHeader video={detail} />

                <VideoActions
                  video={detail}
                  onHideVideo={handleHideVideo}
                  hideSaving={hideSaving}
                />
              </section>

              <VideoInfoPanel
                video={detail}
                availableTags={tags}
                tagSaving={tagSaving}
                onTagsChange={handleTagsChange}
              />
            </div>

            <RecommendedRail videos={detail.relatedVideos} />
          </div>
        </div>
      </div>
    </AppShell>
  );
}
