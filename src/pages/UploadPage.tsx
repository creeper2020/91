import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Check, Film, Link2, UploadCloud } from "lucide-react";
import { AppShell } from "@/components/AppShell";
import { SectionHeader } from "@/components/SectionHeader";
import { createImportJob, fetchImportJob, uploadVideo, type ImportJob } from "@/data/videos";
import { defaultUploadTitleFromFileName } from "@/lib/uploadTitle";
import type { VideoItem } from "@/types";

const UPLOAD_TAGS = ["奶子", "臀", "口角", "女大", "人妻", "AV"];

export default function UploadPage() {
  const [mode, setMode] = useState<"file" | "link">("file");
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [url, setUrl] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [uploaded, setUploaded] = useState<VideoItem | null>(null);
  const [importJob, setImportJob] = useState<ImportJob | null>(null);

  useEffect(() => {
    document.title = "上传视频 · 91";
  }, []);

  const fileMeta = useMemo(() => {
    if (!file) return "";
    const mb = file.size / 1024 / 1024;
    return `${file.name} · ${mb >= 1 ? mb.toFixed(1) : mb.toFixed(2)} MB`;
  }, [file]);

  useEffect(() => {
    if (!importJob || importJob.status === "done" || importJob.status === "failed") {
      return;
    }
    const timer = window.setInterval(async () => {
      try {
        const next = await fetchImportJob(importJob.id);
        setImportJob(next);
      } catch {
        // Keep the last visible state; the next tick or manual retry can recover.
      }
    }, 2500);
    return () => window.clearInterval(timer);
  }, [importJob?.id, importJob?.status]);

  function switchMode(next: "file" | "link") {
    setMode(next);
    setError("");
    setUploaded(null);
    setImportJob(null);
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const nextFile = event.target.files?.[0] ?? null;
    setFile(nextFile);
    setTitle(nextFile ? defaultUploadTitleFromFileName(nextFile.name) : "");
    setUploaded(null);
    setImportJob(null);
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

  async function handleImportSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!url.trim() || saving) return;
    setSaving(true);
    setError("");
    setUploaded(null);
    setImportJob(null);
    try {
      const job = await createImportJob({ url, title, tags });
      setImportJob(job);
      setUrl("");
      setTitle("");
      setTags([]);
    } catch {
      setError("导入任务创建失败，请检查链接或磁力地址");
    } finally {
      setSaving(false);
    }
  }

  return (
    <AppShell>
      <div className="container page-section">
        <SectionHeader title="上传视频" extra="本地视频会加入站内列表" />
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

            <UploadMetaFields title={title} setTitle={setTitle} tags={tags} toggleTag={toggleTag} />

            <UploadResult error={error} uploaded={uploaded} importJob={importJob} />

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
                onChange={(event) => setUrl(event.target.value)}
                placeholder="https://... 或 magnet:?"
                maxLength={2000}
              />
            </label>

            <UploadMetaFields title={title} setTitle={setTitle} tags={tags} toggleTag={toggleTag} />

            <UploadResult error={error} uploaded={uploaded} importJob={importJob} />

            <div className="upload-actions">
              <button className="upload-submit" type="submit" disabled={!url.trim() || saving}>
                <Link2 size={16} />
                {saving ? "创建中" : "导入"}
              </button>
            </div>
          </form>
        )}
      </div>
    </AppShell>
  );
}

function UploadMetaFields({
  title,
  setTitle,
  tags,
  toggleTag,
}: {
  title: string;
  setTitle: (title: string) => void;
  tags: string[];
  toggleTag: (tag: string) => void;
}) {
  return (
    <>
      <label className="upload-field">
        <span>视频名</span>
        <input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
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

function UploadResult({
  error,
  uploaded,
  importJob,
}: {
  error: string;
  uploaded: VideoItem | null;
  importJob: ImportJob | null;
}) {
  if (error) return <div className="upload-message is-error">{error}</div>;
  if (uploaded) {
    return (
      <div className="upload-message is-success">
        <Check size={16} />
        <Link to={uploaded.href}>查看 {uploaded.title}</Link>
      </div>
    );
  }
  if (!importJob) return null;
  if (importJob.status === "failed") {
    return (
      <div className="upload-message is-error">
        {importJob.error || "导入失败"}
      </div>
    );
  }
  if (importJob.status === "done") {
    const videos = importJob.videos ?? [];
    return (
      <div className="upload-message is-success upload-message--stack">
        <span>
          <Check size={16} /> 导入完成
        </span>
        {videos.map((video) => (
          <Link key={video.id} to={video.href}>
            查看 {video.title}
          </Link>
        ))}
      </div>
    );
  }
  const label =
    importJob.status === "queued"
      ? "排队中"
      : importJob.status === "running"
      ? "下载中"
      : importJob.status === "importing"
      ? "入库中"
      : importJob.message || "处理中";
  return <div className="upload-message">{label}</div>;
}
