import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { Check, Film, UploadCloud } from "lucide-react";
import { AppShell } from "@/components/AppShell";
import { SectionHeader } from "@/components/SectionHeader";
import { uploadVideo } from "@/data/videos";
import { defaultUploadTitleFromFileName } from "@/lib/uploadTitle";
import type { VideoItem } from "@/types";

const UPLOAD_TAGS = ["奶子", "臀", "口交", "女大", "人妻", "AV"];

export default function UploadPage() {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [uploaded, setUploaded] = useState<VideoItem | null>(null);

  useEffect(() => {
    document.title = "上传视频 · 91";
  }, []);

  const fileMeta = useMemo(() => {
    if (!file) return "";
    const mb = file.size / 1024 / 1024;
    return `${file.name} · ${mb >= 1 ? mb.toFixed(1) : mb.toFixed(2)} MB`;
  }, [file]);

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

  return (
    <AppShell>
      <div className="container page-section">
        <SectionHeader title="上传视频" extra="本地视频会加入站内列表" />
        <form className="upload-panel" onSubmit={handleSubmit}>
          <label className="upload-drop">
            <input
              type="file"
              accept="video/*,.avi,.mkv,.mov,.mp4,.webm"
              onChange={handleFileChange}
            />
            <span className="upload-drop__icon">
              <UploadCloud size={28} />
            </span>
            <span className="upload-drop__title">
              {file ? fileMeta : "选择视频文件"}
            </span>
          </label>

          <label className="upload-field">
            <span>视频名</span>
            <input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder="选择文件后自动填入"
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
      </div>
    </AppShell>
  );
}
