import {
  ChangeEvent,
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Link } from "react-router-dom";
import { Check, Film, Link2, UploadCloud, XCircle } from "lucide-react";
import { AppShell } from "@/components/AppShell";
import { SectionHeader } from "@/components/SectionHeader";
import {
  createImportJob,
  fetchImportJob,
  uploadVideo,
  type ImportJob,
} from "@/data/videos";
import { defaultUploadTitleFromFileName } from "@/lib/uploadTitle";
import type { VideoItem } from "@/types";

const UPLOAD_TAGS = ["奶子", "臀", "口交", "女大", "人妻", "AV"];

const STORAGE_KEY = "video-site-import-tasks";

/* ------------------------------------------------------------------ */
/*  Import task list helpers                                           */
/* ------------------------------------------------------------------ */

type TaskEntry = {
  job: ImportJob;
  /** 用户提交时填的名称，用于任务列表展示 */
  displayName: string;
  addedAt: number;
};

function isActive(status: string): boolean {
  return status === "queued" || status === "running" || status === "importing";
}

function progressPercent(status: string): number {
  switch (status) {
    case "queued":
      return 10;
    case "running":
      return 55;
    case "importing":
      return 80;
    case "done":
      return 100;
    case "failed":
      return 100;
    default:
      return 0;
  }
}

/** 从 URL 提取可读名称 */
function displayNameFromURL(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed.toLowerCase().startsWith("magnet:?")) {
    const xt = trimmed.match(/xt=urn:btih:([^&]+)/i);
    return xt ? `磁力 ${xt[1].substring(0, 12)}…` : "磁力链接";
  }
  try {
    const u = new URL(trimmed);
    const segs = u.pathname.split("/").filter(Boolean);
    const last = segs.length > 0 ? decodeURIComponent(segs[segs.length - 1]) : "";
    if (last && /\.[a-z0-9]{2,5}$/i.test(last)) return last;
    return u.hostname;
  } catch {
    return trimmed.substring(0, 40);
  }
}

/** 从任务推导显示名称：已完成用视频标题，否则用提交时的名称 */
function taskDisplayName(entry: TaskEntry): string {
  if (entry.job.status === "done" && entry.job.videos && entry.job.videos.length > 0) {
    return entry.job.videos.map((v) => v.title).join(", ");
  }
  return entry.displayName;
}

/** 从任务推导进度文字 */
function taskProgressText(entry: TaskEntry): string {
  const { job } = entry;
  if (job.status === "done") {
    const count = job.videos?.length ?? 0;
    return count > 0 ? `完成 · ${count} 个视频` : "已完成";
  }
  if (job.status === "failed") return job.error ? "失败" : "失败";
  return job.message || (job.status === "queued" ? "排队中" : job.status === "running" ? "下载中" : "入库中");
}

/* ---- localStorage persistence ---- */

function loadSavedTasks(): TaskEntry[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed: TaskEntry[] = JSON.parse(raw);
    // 只恢复未完成的任务，已完成/失败超过 1 小时的丢弃
    const cutoff = Date.now() - 3600_000;
    return parsed.filter(
      (t) => isActive(t.job.status) || t.addedAt > cutoff
    );
  } catch {
    return [];
  }
}

function saveTasks(tasks: TaskEntry[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(tasks));
  } catch {
    /* quota exceeded — ignore */
  }
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function UploadPage() {
  const [mode, setMode] = useState<"file" | "link">("file");

  // file mode state
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [uploaded, setUploaded] = useState<VideoItem | null>(null);

  // link mode state
  const [url, setUrl] = useState("");
  const [linkTitle, setLinkTitle] = useState("");
  const [tasks, setTasks] = useState<TaskEntry[]>(() => loadSavedTasks());

  // 持久化到 localStorage
  useEffect(() => {
    saveTasks(tasks);
  }, [tasks]);

  // 恢复后立即对未完成任务补一轮查询
  useEffect(() => {
    const hasStale = tasks.some((t) => isActive(t.job.status));
    if (!hasStale) return;
    (async () => {
      const updated: TaskEntry[] = [];
      for (const entry of tasks) {
        if (!isActive(entry.job.status)) {
          updated.push(entry);
          continue;
        }
        try {
          const next = await fetchImportJob(entry.job.id);
          updated.push({ ...entry, job: next });
        } catch {
          updated.push(entry);
        }
      }
      setTasks(updated);
    })();
    // only on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // poll active tasks
  const tasksRef = useRef(tasks);
  tasksRef.current = tasks;

  useEffect(() => {
    const hasActive = tasks.some((t) => isActive(t.job.status));
    if (!hasActive) return;
    const timer = window.setInterval(async () => {
      const current = tasksRef.current;
      const updated: TaskEntry[] = [];
      for (const entry of current) {
        if (!isActive(entry.job.status)) {
          updated.push(entry);
          continue;
        }
        try {
          const next = await fetchImportJob(entry.job.id);
          updated.push({ ...entry, job: next });
        } catch {
          updated.push(entry);
        }
      }
      setTasks(updated);
    }, 3000);
    return () => window.clearInterval(timer);
  }, [tasks.some((t) => isActive(t.job.status))]);

  const fileMeta = useMemo(() => {
    if (!file) return "";
    const mb = file.size / 1024 / 1024;
    return `${file.name} · ${mb >= 1 ? mb.toFixed(1) : mb.toFixed(2)} MB`;
  }, [file]);

  function switchMode(next: "file" | "link") {
    setMode(next);
    setError("");
    setUploaded(null);
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const nextFile = event.target.files?.[0] ?? null;
    setFile(nextFile);
    setTitle(nextFile ? defaultUploadTitleFromFileName(nextFile.name) : "");
    setUploaded(null);
    setError("");
  }

  function toggleTag(tag: string) {
    setTags((current) =>
      current.includes(tag)
        ? current.filter((item) => item !== tag)
        : [...current, tag]
    );
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!file || saving) return;
    setSaving(true);
    setError("");
    setUploaded(null);
    try {
      const video = await uploadVideo({ file, title, tags });
      setUploaded(video);
      setFile(null);
      setTitle("");
      setTags([]);
    } catch {
      setError("上传失败，请检查文件格式后重试");
    } finally {
      setSaving(false);
    }
  }

  const handleImportSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (!url.trim() || saving) return;
      setSaving(true);
      setError("");
      try {
        const job = await createImportJob({ url, title: linkTitle, tags });
        const name = linkTitle.trim() || displayNameFromURL(url);
        setTasks((prev) => [{ job, displayName: name, addedAt: Date.now() }, ...prev]);
        setUrl("");
        setLinkTitle("");
        setTags([]);
      } catch {
        setError("导入任务创建失败，请检查链接或磁力地址");
      } finally {
        setSaving(false);
      }
    },
    [url, linkTitle, tags, saving]
  );

  function removeTask(id: string) {
    setTasks((prev) => prev.filter((t) => t.job.id !== id));
  }

  return (
    <AppShell>
      <div className="container page-section">
        <SectionHeader title="上传视频" extra="视频会加入站内列表" />

        <div className="upload-mode-tabs" role="tablist" aria-label="上传方式">
          <button
            type="button"
            className={mode === "file" ? "is-active" : ""}
            onClick={() => switchMode("file")}
          >
            <UploadCloud size={15} /> 文件
          </button>
          <button
            type="button"
            className={mode === "link" ? "is-active" : ""}
            onClick={() => switchMode("link")}
          >
            <Link2 size={15} /> 链接
          </button>
        </div>

        {mode === "file" ? (
          <form className="upload-panel" onSubmit={handleSubmit}>
            <label className="upload-drop">
              <input
                type="file"
                accept="video/*,.avi,.flv,.m4v,.mkv,.mov,.mp4,.mpeg,.mpg,.ts,.webm,.wmv"
                onChange={handleFileChange}
              />
              <span className="upload-drop__icon">
                <UploadCloud size={28} />
              </span>
              <span className="upload-drop__title">
                {file ? fileMeta : "选择视频文件"}
              </span>
            </label>

            <UploadMetaFields
              title={title}
              setTitle={setTitle}
              tags={tags}
              toggleTag={toggleTag}
            />

            {error ? <div className="upload-message is-error">{error}</div> : null}
            {uploaded ? (
              <div className="upload-message is-success">
                <Check size={16} />
                <Link to={uploaded.href}>查看 {uploaded.title}</Link>
              </div>
            ) : null}

            <div className="upload-actions">
              <button className="upload-submit" type="submit" disabled={!file || saving}>
                <Film size={16} />
                {saving ? "上传中" : "上传"}
              </button>
            </div>
          </form>
        ) : (
          <form className="upload-panel" onSubmit={handleImportSubmit}>
            <label className="upload-field">
              <span>链接</span>
              <input
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://... 或 magnet:?"
                maxLength={2000}
              />
            </label>

            <label className="upload-field">
              <span>视频名</span>
              <input
                value={linkTitle}
                onChange={(e) => setLinkTitle(e.target.value)}
                placeholder="留空则自动提取"
                maxLength={120}
              />
            </label>

            <div className="upload-field">
              <span>标签</span>
              <div className="upload-tags">
                {UPLOAD_TAGS.map((tag) => {
                  const active = tags.includes(tag);
                  return (
                    <button
                      key={tag}
                      type="button"
                      className={`upload-tag ${active ? "is-active" : ""}`}
                      onClick={() => toggleTag(tag)}
                      aria-pressed={active}
                    >
                      {active ? <Check size={14} /> : null}
                      {tag}
                    </button>
                  );
                })}
              </div>
            </div>

            {error ? <div className="upload-message is-error">{error}</div> : null}

            <div className="upload-actions">
              <button className="upload-submit" type="submit" disabled={!url.trim() || saving}>
                <Link2 size={16} />
                {saving ? "创建中" : "导入"}
              </button>
            </div>
          </form>
        )}

        {/* Task list */}
        {tasks.length > 0 && (
          <div className="import-task-list">
            <div className="import-task-list__header">下载列表</div>
            {tasks.map((entry) => (
              <ImportTaskRow
                key={entry.job.id}
                entry={entry}
                onRemove={() => removeTask(entry.job.id)}
              />
            ))}
          </div>
        )}
      </div>
    </AppShell>
  );
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function UploadMetaFields({
  title,
  setTitle,
  tags,
  toggleTag,
}: {
  title: string;
  setTitle: (t: string) => void;
  tags: string[];
  toggleTag: (tag: string) => void;
}) {
  return (
    <>
      <label className="upload-field">
        <span>视频名</span>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="留空则使用文件名"
          maxLength={120}
        />
      </label>
      <div className="upload-field">
        <span>标签</span>
        <div className="upload-tags">
          {UPLOAD_TAGS.map((tag) => {
            const active = tags.includes(tag);
            return (
              <button
                key={tag}
                type="button"
                className={`upload-tag ${active ? "is-active" : ""}`}
                onClick={() => toggleTag(tag)}
                aria-pressed={active}
              >
                {active ? <Check size={14} /> : null}
                {tag}
              </button>
            );
          })}
        </div>
      </div>
    </>
  );
}

function ImportTaskRow({
  entry,
  onRemove,
}: {
  entry: TaskEntry;
  onRemove: () => void;
}) {
  const { job } = entry;
  const active = isActive(job.status);
  const done = job.status === "done";
  const failed = job.status === "failed";

  const barClass = done
    ? "import-task__bar is-done"
    : failed
    ? "import-task__bar is-failed"
    : "import-task__bar is-active";

  const name = taskDisplayName(entry);
  const progress = taskProgressText(entry);
  const pct = progressPercent(job.status);

  return (
    <div className="import-task">
      <div className="import-task__info">
        <span className="import-task__name" title={name}>
          {done ? <Link to={job.videos![0].href}>{name}</Link> : name}
        </span>
        <span
          className={`import-task__status ${done ? "is-done" : failed ? "is-failed" : ""}`}
        >
          {progress}
        </span>
      </div>
      <div className={barClass}>
        <div className="import-task__bar-fill" style={{ width: `${pct}%` }} />
      </div>
      {failed && job.error && (
        <div className="import-task__error">{job.error}</div>
      )}
      {!active && (
        <button type="button" className="import-task__remove" onClick={onRemove} title="移除">
          <XCircle size={14} />
        </button>
      )}
    </div>
  );
}
