import { useEffect, useMemo, useState } from "react";
import { Film, Plus, RefreshCw, Search, Tags, Trash2 } from "lucide-react";
import * as api from "./api";
import { useToast } from "./ToastContext";

export function TagsPage() {
  const [tags, setTags] = useState<api.AdminTag[]>([]);
  const [label, setLabel] = useState("");
  const [aliases, setAliases] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [filterSource, setFilterSource] = useState<string>("all");
  const { show } = useToast();

  async function refresh() {
    setLoading(true);
    try {
      setTags(await api.listTags());
    } catch (e) {
      show(e instanceof Error ? e.message : "加载标签失败", "error");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  async function handleCreate() {
    const cleanLabel = label.trim();
    if (!cleanLabel) return;
    setSaving(true);
    try {
      const r = await api.createTag(cleanLabel, splitList(aliases));
      show(`已添加标签，自动归类 ${r.classified} 个视频`, "success");
      setLabel("");
      setAliases("");
      await refresh();
    } catch (e) {
      show(e instanceof Error ? e.message : "添加标签失败", "error");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(tag: api.AdminTag) {
    if (tag.source === "system" || deletingId !== null) return;
    const confirmed = window.confirm(`删除标签「${tag.label}」？已关联视频会同步移除这个标签。`);
    if (!confirmed) return;
    setDeletingId(tag.id);
    try {
      const r = await api.deleteTag(tag.id);
      show(`已删除标签，影响 ${r.removedVideos} 个视频`, "success");
      await refresh();
    } catch (e) {
      show(e instanceof Error ? e.message : "删除标签失败", "error");
    } finally {
      setDeletingId(null);
    }
  }

  const stats = useMemo(() => {
    let totalVideos = 0;
    let systemCount = 0;
    let userCount = 0;
    let collectionCount = 0;
    let legacyCount = 0;

    tags.forEach((t) => {
      totalVideos += t.count ?? 0;
      if (t.source === "system") systemCount++;
      else if (t.source === "user") userCount++;
      else if (t.source === "collection") collectionCount++;
      else if (t.source === "legacy") legacyCount++;
    });

    return {
      totalTags: tags.length,
      totalVideos,
      systemCount,
      userCount,
      collectionCount,
      legacyCount,
    };
  }, [tags]);

  const filteredTags = useMemo(() => {
    return tags.filter((t) => {
      const query = searchQuery.trim().toLowerCase();
      const matchesSearch =
        !query ||
        t.label.toLowerCase().includes(query) ||
        (t.aliases ?? []).some((a) => a.toLowerCase().includes(query));
      const matchesSource = filterSource === "all" || t.source === filterSource;
      return matchesSearch && matchesSource;
    });
  }, [tags, searchQuery, filterSource]);

  return (
    <section>
      <header className="admin-page__header">
        <h1 className="admin-page__title">标签管理</h1>
        <button className="admin-btn" onClick={refresh}>
          <RefreshCw size={13} /> 刷新
        </button>
      </header>

      <div className="admin-tags-layout">
        {/* 左栏：创建与统计 */}
        <div>
          <div className="admin-card">
            <div className="admin-card__title">
              <Plus size={15} /> 新增分类标签
            </div>
            <div className="admin-form">
              <div className="admin-form__row">
                <label>标签名</label>
                <input
                  value={label}
                  onChange={(e) => setLabel(e.target.value)}
                  placeholder="例如：清纯"
                />
              </div>
              <div className="admin-form__row">
                <label>别名</label>
                <input
                  value={aliases}
                  onChange={(e) => setAliases(e.target.value)}
                  placeholder="逗号分隔，例如：纯欲, 清新"
                />
                <div className="admin-form__help">
                  新增后会按别名和标签名匹配已有视频的标题、作者和目录并自动归类。
                </div>
              </div>
              <button
                className="admin-btn is-primary"
                onClick={handleCreate}
                disabled={saving || !label.trim()}
              >
                <Plus size={13} /> {saving ? "添加中..." : "添加并自动归类"}
              </button>
            </div>
          </div>

          <div className="admin-card">
            <div className="admin-card__title">
              <Tags size={15} /> 系统标签统计
            </div>
            <div className="admin-tag-stats-list">
              <div className="admin-tag-stat-item">
                <span>总标签数</span>
                <strong>{stats.totalTags}</strong>
              </div>
              <div className="admin-tag-stat-item">
                <span>关联视频次</span>
                <strong>{stats.totalVideos}</strong>
              </div>
              <div className="admin-tag-stat-item">
                <span>系统提取</span>
                <strong>{stats.systemCount}</strong>
              </div>
              <div className="admin-tag-stat-item">
                <span>用户创建</span>
                <strong>{stats.userCount}</strong>
              </div>
            </div>
          </div>
        </div>

        {/* 右栏：看板网格与搜索栏 */}
        <div>
          <div className="admin-tags-toolbar">
            <div className="admin-tags-search">
              <Search className="admin-tags-search__icon" size={14} />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="搜索标签名或别名..."
              />
            </div>

            <div className="admin-tags-filter-tabs">
              <button
                type="button"
                className={`admin-tags-filter-tab ${filterSource === "all" ? "is-active" : ""}`}
                onClick={() => setFilterSource("all")}
              >
                全部 ({tags.length})
              </button>
              <button
                type="button"
                className={`admin-tags-filter-tab ${filterSource === "system" ? "is-active" : ""}`}
                onClick={() => setFilterSource("system")}
              >
                系统 ({stats.systemCount})
              </button>
              <button
                type="button"
                className={`admin-tags-filter-tab ${filterSource === "user" ? "is-active" : ""}`}
                onClick={() => setFilterSource("user")}
              >
                用户 ({stats.userCount})
              </button>
              <button
                type="button"
                className={`admin-tags-filter-tab ${filterSource === "collection" ? "is-active" : ""}`}
                onClick={() => setFilterSource("collection")}
              >
                合集 ({stats.collectionCount})
              </button>
            </div>
          </div>

          {loading ? (
            <div className="admin-empty">加载中...</div>
          ) : filteredTags.length === 0 ? (
            <div className="admin-card admin-empty">
              没有找到匹配的标签。
            </div>
          ) : (
            <div className="admin-tags-grid">
              {filteredTags.map((tag) => (
                <div key={tag.id} className="admin-tag-card">
                  <div className="admin-tag-card__head">
                    <span className="admin-tag-card__title">{tag.label}</span>
                    <span className="admin-tag-card__source-badge" data-source={tag.source}>
                      {sourceLabel(tag.source)}
                    </span>
                  </div>

                  {tag.aliases && tag.aliases.length > 0 && (
                    <div className="admin-tag-card__aliases">
                      {tag.aliases.map((alias) => (
                        <span key={alias} className="admin-tag-card__alias-pill">
                          {alias}
                        </span>
                      ))}
                    </div>
                  )}

                  <div className="admin-tag-card__footer">
                    <span>ID: {tag.id}</span>
                    <span className="admin-tag-card__footer-actions">
                      <span className="admin-tag-card__count">
                        <Film size={11} />
                        <strong>{tag.count} 视频</strong>
                      </span>
                      {tag.source !== "system" && (
                        <button
                          type="button"
                          className="admin-tag-card__delete"
                          onClick={() => handleDelete(tag)}
                          disabled={deletingId !== null}
                          title="删除标签"
                        >
                          <Trash2 size={10} />
                          {deletingId === tag.id ? "删除中" : "删除"}
                        </button>
                      )}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

function splitList(s: string): string[] {
  return s
    .split(/[,，、\s]+/)
    .map((x) => x.trim())
    .filter(Boolean);
}

function sourceLabel(source: string): string {
  if (source === "system") return "系统";
  if (source === "collection") return "合集";
  if (source === "legacy") return "旧数据";
  return "用户";
}
