import { useEffect, useState } from "react";
import {
  Check,
  FolderOutput,
  HardDrive,
  Loader2,
  Moon,
  Sparkles,
} from "lucide-react";
import * as api from "./api";
import type { Theme } from "./api";
import { useToast } from "./ToastContext";
import { applyTheme, getCurrentTheme } from "@/lib/theme";

function isTheme(value: unknown): value is Theme {
  return value === "dark" || value === "pink";
}

type Option = {
  id: Theme;
  title: string;
  subtitle: string;
  description: string;
  icon: typeof Moon;
};

const OPTIONS: Option[] = [
  {
    id: "dark",
    title: "暗黑 + 暖橙",
    subtitle: "Cinema Dark",
    description: "深邃灰阶 + 暖橙主色，适合夜间观影、长时间浏览。",
    icon: Moon,
  },
  {
    id: "pink",
    title: "奶油白 + 樱花粉",
    subtitle: "Sakura Cream",
    description: "柔和奶白底 + 樱花粉主色，清爽温柔，日间使用更舒适。",
    icon: Sparkles,
  },
];

const UPLOAD_KINDS = new Set(["pikpak", "p115", "p123", "onedrive", "googledrive"]);

function kindLabel(kind: string): string {
  const map: Record<string, string> = {
    quark: "夸克", p115: "115", p123: "123", pikpak: "PikPak",
    wopan: "W盘", onedrive: "OneDrive", googledrive: "Google Drive",
    localstorage: "本地存储", spider91: "91爬虫",
  };
  return map[kind] ?? kind;
}

export function ThemePage() {
  const [active, setActive] = useState<Theme>(getCurrentTheme());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState<Theme | null>(null);
  const { show } = useToast();

  const [drives, setDrives] = useState<api.AdminDrive[]>([]);
  const [defaultDriveId, setDefaultDriveId] = useState("");
  const [driveSaving, setDriveSaving] = useState(false);

  const uploadDrives = drives.filter((d) => UPLOAD_KINDS.has(d.kind));

  useEffect(() => {
    let alive = true;
    Promise.all([api.getSettings(), api.listDrives().catch(() => [])])
      .then(([s, dl]) => {
        if (!alive) return;
        if (isTheme(s.theme)) { setActive(s.theme); applyTheme(s.theme); }
        setDefaultDriveId(s.defaultUploadDriveId ?? "");
        setDrives(dl ?? []);
      })
      .catch(() => {})
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, []);

  async function handleSelect(next: Theme) {
    if (next === active || saving) return;
    const prev = active;
    setActive(next); applyTheme(next); setSaving(next);
    try {
      const resp = await api.updateSettings({ theme: next });
      if (isTheme(resp.theme)) { setActive(resp.theme); applyTheme(resp.theme); }
      show("主题已更新", "success");
    } catch (e) {
      setActive(prev); applyTheme(prev);
      show(e instanceof Error ? e.message : "保存失败", "error");
    } finally { setSaving(null); }
  }

  async function handleDriveChange(driveId: string) {
    if (driveId === defaultDriveId) return;
    setDriveSaving(true);
    try {
      const resp = await api.updateSettings({ defaultUploadDriveId: driveId });
      setDefaultDriveId(resp.defaultUploadDriveId ?? "");
      show(driveId ? "默认下载目录已更新" : "已设为本地保存", "success");
    } catch (e) {
      show(e instanceof Error ? e.message : "保存失败", "error");
    } finally { setDriveSaving(false); }
  }

  return (
    <div className="theme-page">
      <header className="theme-page__head">
        <h1 className="theme-page__title">主题外观</h1>
      </header>

      <div className="theme-grid">
        {OPTIONS.map((opt) => {
          const Icon = opt.icon;
          const isActive = active === opt.id;
          const isSaving = saving === opt.id;
          return (
            <button
              key={opt.id} type="button"
              className={`theme-card${isActive ? " is-active" : ""}`}
              data-preview={opt.id}
              onClick={() => handleSelect(opt.id)}
              disabled={loading || saving !== null}
              aria-pressed={isActive}
            >
              <div className="theme-card__preview" aria-hidden="true">
                <span className="theme-card__bar" />
                <div className="theme-card__player" />
                <div className="theme-card__lines">
                  <span className="theme-card__line theme-card__line--lg" />
                  <span className="theme-card__line theme-card__line--md" />
                </div>
                <div className="theme-card__chips">
                  <span className="theme-card__chip" />
                  <span className="theme-card__chip" />
                  <span className="theme-card__chip theme-card__chip--accent" />
                </div>
              </div>
              <div className="theme-card__body">
                <div className="theme-card__head">
                  <span className="theme-card__icon"><Icon size={16} /></span>
                  <div className="theme-card__title-wrap">
                    <span className="theme-card__title">{opt.title}</span>
                    <span className="theme-card__subtitle">{opt.subtitle}</span>
                  </div>
                  <span className="theme-card__state" aria-hidden="true">
                    {isSaving ? <Loader2 size={16} className="theme-card__spin" /> : isActive ? <Check size={16} /> : null}
                  </span>
                </div>
                <p className="theme-card__desc">{opt.description}</p>
              </div>
            </button>
          );
        })}
      </div>

      {/* ---- 默认下载目录 ---- */}
      <section className="settings-section">
        <div className="settings-section__header">
          <FolderOutput size={20} />
          <div>
            <h2 className="settings-section__title">默认下载目录</h2>
            <p className="settings-section__desc">
              用户上传或链接导入时，视频文件保存到哪里。
            </p>
          </div>
        </div>

        <div className="settings-drive-cards">
          {/* 本地保存 */}
          <button
            type="button"
            className={`settings-drive-card${defaultDriveId === "" ? " is-active" : ""}`}
            onClick={() => handleDriveChange("")}
            disabled={loading || driveSaving}
          >
            <div className="settings-drive-card__icon">
              <HardDrive size={20} />
            </div>
            <div className="settings-drive-card__body">
              <strong className="settings-drive-card__name">本地保存</strong>
              <span className="settings-drive-card__kind">不上传到网盘</span>
            </div>
            <span className="settings-drive-card__check" aria-hidden="true">
              {defaultDriveId === "" && !driveSaving && <Check size={16} />}
              {driveSaving && defaultDriveId === "" && <Loader2 size={14} className="admin-spin" />}
            </span>
          </button>

          {/* 已挂载的云盘 */}
          {uploadDrives.map((d) => (
            <button
              key={d.id}
              type="button"
              className={`settings-drive-card${defaultDriveId === d.id ? " is-active" : ""}`}
              onClick={() => handleDriveChange(d.id)}
              disabled={loading || driveSaving}
            >
              <div className="settings-drive-card__icon" data-kind={d.kind}>
                {d.kind.substring(0, 2).toUpperCase()}
              </div>
              <div className="settings-drive-card__body">
                <strong className="settings-drive-card__name">{d.name || d.id}</strong>
                <span className="settings-drive-card__kind">{kindLabel(d.kind)}</span>
              </div>
              <span className="settings-drive-card__check" aria-hidden="true">
                {defaultDriveId === d.id && !driveSaving && <Check size={16} />}
                {driveSaving && defaultDriveId === d.id && <Loader2 size={14} className="admin-spin" />}
              </span>
            </button>
          ))}
        </div>

        {uploadDrives.length === 0 && !loading && (
          <p className="settings-section__hint">
            当前没有支持上传的云盘，请先到「网盘管理」添加 PikPak / 115 / 123 / OneDrive。
          </p>
        )}
      </section>
    </div>
  );
}
