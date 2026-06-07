import { useMemo, useState } from "react";
import { Pencil, Tag, X } from "lucide-react";
import type { TagItem, VideoDetail } from "@/types";

type Props = {
  video: VideoDetail;
  availableTags?: TagItem[];
  tagSaving?: boolean;
  onTagsChange?: (tags: string[]) => Promise<void>;
};

/**
 * 简介 + 标签合并卡。
 * - 上半部分是简介：默认折叠 3 行，整块可点击展开/收起；简介为空时不渲染。
 * - 下半部分是标签：横向 chip 列表 + 一个"编辑"按钮调出标签编辑器。
 *
 * 视觉上和上一版的"两张分离卡"相比，整体感更强：
 * - 一张大卡内分两个小区块，区块之间用细分隔线
 * - 简介区块加 "简介" 标题前缀
 * - 标签区块加标签轮廓图标暗示
 */
export function VideoInfoPanel({
  video,
  availableTags = [],
  tagSaving = false,
  onTagsChange,
}: Props) {
  const [editingTags, setEditingTags] = useState(false);
  const [draftTags, setDraftTags] = useState<string[]>(video.tags ?? []);
  const [tagError, setTagError] = useState("");
  const [descExpanded, setDescExpanded] = useState(false);

  const tags = video.tags ?? [];
  const description = (video.description ?? "").trim();
  const showDescription = description.length > 0;
  const descriptionLong = description.length > 80 || description.includes("\n");

  const sortedAvailable = useMemo(() => {
    return [...availableTags].sort((a, b) => {
      const ac = a.count ?? 0;
      const bc = b.count ?? 0;
      if (bc !== ac) return bc - ac;
      return a.label.localeCompare(b.label, "zh-Hans-CN");
    });
  }, [availableTags]);

  function openTagEditor() {
    setDraftTags(tags);
    setTagError("");
    setEditingTags(true);
  }

  function closeTagEditor() {
    setEditingTags(false);
    setTagError("");
  }

  async function saveTags() {
    if (!onTagsChange) return;
    setTagError("");
    try {
      await onTagsChange(draftTags);
      setEditingTags(false);
    } catch (e) {
      setTagError(e instanceof Error ? e.message : "保存标签失败");
    }
  }

  return (
    <section className="vd-info" aria-label="视频信息">
      {showDescription && (
        <div
          className={`vd-info__desc${descExpanded ? " is-expanded" : ""}${
            descriptionLong ? " is-clickable" : ""
          }`}
          role={descriptionLong ? "button" : undefined}
          tabIndex={descriptionLong ? 0 : undefined}
          onClick={() => descriptionLong && setDescExpanded((v) => !v)}
          onKeyDown={(e) => {
            if (!descriptionLong) return;
            if (e.key === "Enter" || e.key === " ") {
              e.preventDefault();
              setDescExpanded((v) => !v);
            }
          }}
        >
          <div className="vd-info__section-head">
            <span className="vd-info__section-title">简介</span>
            {descriptionLong && (
              <span className="vd-info__desc-toggle">
                {descExpanded ? "收起" : "展开"}
              </span>
            )}
          </div>
          <p className="vd-info__desc-text">{description}</p>
        </div>
      )}

      <div className="vd-info__tags">
        <div className="vd-info__section-head">
          <span className="vd-info__section-title">
            <Tag size={15} strokeWidth={2} aria-hidden="true" />
            标签
          </span>
          {onTagsChange && (
            <button
              type="button"
              className="vd-info__tags-edit"
              onClick={openTagEditor}
              aria-label="编辑标签"
            >
              <Pencil size={13} />
              <span>编辑</span>
            </button>
          )}
        </div>
        <div className="vd-info__tags-list">
          {tags.length === 0 ? (
            <span className="vd-info__tags-empty">暂无标签</span>
          ) : (
            tags.map((t) => (
              <span key={t} className="vd-tag">
                #{t}
              </span>
            ))
          )}
        </div>
      </div>

      {editingTags && (
        <div className="vd-tag-editor" role="dialog" aria-label="编辑视频标签">
          <header className="vd-tag-editor__head">
            <span>选择适用的标签</span>
            <button
              type="button"
              className="vd-tag-editor__close"
              onClick={closeTagEditor}
              aria-label="关闭"
            >
              <X size={16} />
            </button>
          </header>

          <div className="vd-tag-editor__grid">
            {sortedAvailable.length === 0 ? (
              <div className="vd-tag-editor__empty">暂无可用标签</div>
            ) : (
              sortedAvailable.map((tag) => {
                const checked = draftTags.includes(tag.label);
                return (
                  <button
                    type="button"
                    key={tag.id}
                    className={`vd-tag-editor__chip${
                      checked ? " is-active" : ""
                    }`}
                    onClick={() =>
                      setDraftTags((prev) =>
                        prev.includes(tag.label)
                          ? prev.filter((t) => t !== tag.label)
                          : [...prev, tag.label]
                      )
                    }
                    aria-pressed={checked}
                  >
                    <span>{tag.label}</span>
                    {typeof tag.count === "number" && <em>{tag.count}</em>}
                  </button>
                );
              })
            )}
          </div>

          {tagError && <div className="vd-tag-editor__error">{tagError}</div>}

          <div className="vd-tag-editor__actions">
            <button
              type="button"
              className="vd-tag-editor__btn"
              onClick={closeTagEditor}
            >
              取消
            </button>
            <button
              type="button"
              className="vd-tag-editor__btn is-primary"
              onClick={saveTags}
              disabled={tagSaving}
            >
              {tagSaving ? "保存中..." : "保存"}
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
