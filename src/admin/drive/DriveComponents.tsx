import { PlayCircle, Power, PowerOff, RotateCcw } from "lucide-react";
import * as api from "../api";
import { formatBytes } from "../storageFormat";
import {
  generationStateLabel,
  generationStateClass,
  generationDetail,
  generationTitle,
} from "./constants";

export function StorageSummary({ storage }: { storage: api.AdminDriveStorage }) {
  return (
    <section className="admin-card admin-storage-summary" aria-label="本地媒体存储">
      <div className="admin-storage-summary__metric">
        <span>封面占用</span>
        <strong>{formatBytes(storage.thumbnailBytes)}</strong>
      </div>
      <div className="admin-storage-summary__metric">
        <span>预览视频占用</span>
        <strong>{formatBytes(storage.teaserBytes)}</strong>
      </div>
      <div className="admin-storage-summary__metric">
        <span>本地媒体合计</span>
        <strong>{formatBytes(storage.totalBytes)}</strong>
      </div>
      <div className="admin-storage-summary__metric">
        <span>磁盘可用</span>
        <strong>{formatBytes(storage.availableBytes)}</strong>
      </div>
    </section>
  );
}

export function GenerationCounts({
  ready,
  pending,
  failed,
  durationPending,
}: {
  ready?: number;
  pending?: number;
  failed?: number;
  durationPending?: number;
}) {
  return (
    <div className="admin-generation-counts">
      <span className="admin-drive-teaser__metric is-ready">
        就绪 {ready ?? 0}
      </span>
      <span className="admin-drive-teaser__metric is-pending">
        待生成 {pending ?? 0}
      </span>
      <span className="admin-drive-teaser__metric is-failed">
        失败 {failed ?? 0}
      </span>
      {(durationPending ?? 0) > 0 && (
        <span className="admin-drive-teaser__metric">
          待补时长 {durationPending}
        </span>
      )}
    </div>
  );
}

export function GenerationStatusLine({
  label,
  status,
}: {
  label: string;
  status?: api.DriveGenerationStatus;
}) {
  const state = status?.state || "idle";
  const queueLength = status?.queueLength ?? 0;
  const detail = generationDetail(status);
  const title = generationTitle(status, detail);
  const countText = queueLength > 0 ? `${label === "封面" ? "待处理" : "队列"} ${queueLength}` : "";

  return (
    <div className="admin-generation-row" title={title}>
      <span className="admin-generation-kind">{label}</span>
      <span className={`admin-status admin-generation-state is-${generationStateClass(state)}`}>
        {generationStateLabel(state)}
      </span>
      {(detail || queueLength > 0) && (
        <span className="admin-generation-detail">
          {[detail, countText].filter(Boolean).join(" / ")}
        </span>
      )}
    </div>
  );
}

export function StatusTag({
  kind,
  status,
  error,
  hasCred,
}: {
  kind: string;
  status: string;
  error?: string;
  hasCred: boolean;
}) {
  if (kind !== "spider91" && !hasCred) {
    return <span className="admin-status is-pending">未配置凭证</span>;
  }
  if (status === "ok") {
    if (kind === "spider91") {
      return <span className="admin-status is-ok">已就绪</span>;
    }
    return <span className="admin-status is-ok">已连接</span>;
  }
  if (status === "error")
    return (
      <span className="admin-status is-error" title={error}>
        错误
      </span>
    );
  return <span className="admin-status">{status || "未连接"}</span>;
}

export function DriveCardMetrics({ d }: { d: api.AdminDrive }) {
  return (
    <div className="admin-drive-card__info">
      <div className="admin-drive-card__metric">
        <span>封面数 (就绪/失败)</span>
        <strong>
          {d.thumbnailReadyCount ?? 0}
          <span style={{ fontSize: "11px", fontWeight: "normal", color: "var(--text-faint)" }}>
            {" "}/ {d.thumbnailFailedCount ?? 0}
          </span>
        </strong>
      </div>
      <div className="admin-drive-card__metric">
        <span>Teaser 数 (就绪/失败)</span>
        <strong>
          {d.teaserReadyCount ?? 0}
          <span style={{ fontSize: "11px", fontWeight: "normal", color: "var(--text-faint)" }}>
            {" "}/ {d.teaserFailedCount ?? 0}
          </span>
        </strong>
      </div>
      <div className="admin-drive-card__metric">
        <span>指纹数 (就绪/失败)</span>
        <strong>
          {d.fingerprintReadyCount ?? 0}
          <span style={{ fontSize: "11px", fontWeight: "normal", color: "var(--text-faint)" }}>
            {" "}/ {d.fingerprintFailedCount ?? 0}
          </span>
        </strong>
      </div>
    </div>
  );
}

export function DriveGenerationPanel({
  d,
  regenFailedId,
  regenFailedThumbId,
  regenFailedFingerprintId,
  togglingTeaserId,
  onToggleTeaser,
  onRegenFailed,
  onRegenFailedThumbnails,
  onRegenFailedFingerprints,
}: {
  d: api.AdminDrive;
  regenFailedId: string;
  regenFailedThumbId: string;
  regenFailedFingerprintId: string;
  togglingTeaserId: string;
  onToggleTeaser: () => void;
  onRegenFailed: () => void;
  onRegenFailedThumbnails: () => void;
  onRegenFailedFingerprints: () => void;
}) {
  return (
    <div className="admin-detail-card">
      <header className="admin-detail-card__title">
        <div className="admin-detail-card__title-left">
          <PlayCircle size={16} />
          <span>生成状态</span>
        </div>
        <div className="admin-detail-actions-inline">
          <button
            className={`admin-btn ${d.teaserEnabled ? "is-success" : ""}`}
            onClick={onToggleTeaser}
            disabled={togglingTeaserId === d.id}
            style={{ padding: "4px 10px", fontSize: "11px" }}
          >
            {d.teaserEnabled ? <Power size={11} /> : <PowerOff size={11} />}
            <span>{d.teaserEnabled ? "预览视频生成：开" : "预览视频生成：关"}</span>
          </button>
        </div>
      </header>

      <div className="admin-detail-grid">
        <div className="admin-detail-row">
          <span className="admin-detail-label">封面生成状态</span>
          <div className="admin-detail-value">
            <GenerationStatusLine label="封面" status={d.thumbnailGenerationStatus} />
          </div>
        </div>
        <div className="admin-detail-row">
          <span className="admin-detail-label">封面数量</span>
          <div className="admin-detail-value">
            <GenerationCounts
              ready={d.thumbnailReadyCount}
              pending={d.thumbnailPendingCount}
              failed={d.thumbnailFailedCount}
              durationPending={d.thumbnailDurationPendingCount}
            />
          </div>
        </div>
        <div className="admin-detail-row">
          <span className="admin-detail-label">预览视频生成状态</span>
          <div className="admin-detail-value">
            <GenerationStatusLine label="预览" status={d.previewGenerationStatus} />
          </div>
        </div>
        <div className="admin-detail-row">
          <span className="admin-detail-label">预览视频数量</span>
          <div className="admin-detail-value">
            <GenerationCounts
              ready={d.teaserReadyCount}
              pending={d.teaserPendingCount}
              failed={d.teaserFailedCount}
            />
          </div>
        </div>
        <div className="admin-detail-row">
          <span className="admin-detail-label">视频指纹生成状态</span>
          <div className="admin-detail-value">
            <GenerationStatusLine label="指纹" status={d.fingerprintGenerationStatus} />
          </div>
        </div>
        <div className="admin-detail-row">
          <span className="admin-detail-label">视频指纹数量</span>
          <div className="admin-detail-value">
            <GenerationCounts
              ready={d.fingerprintReadyCount}
              pending={d.fingerprintPendingCount}
              failed={d.fingerprintFailedCount}
            />
          </div>
        </div>
      </div>

      <div className="admin-detail-actions">
        <button
          className="admin-btn"
          disabled={(d.thumbnailFailedCount ?? 0) <= 0 || regenFailedThumbId === d.id}
          onClick={onRegenFailedThumbnails}
        >
          <RotateCcw size={13} />
          <span>重试失败封面</span>
        </button>
        <button
          className="admin-btn"
          disabled={(d.teaserFailedCount ?? 0) <= 0 || regenFailedId === d.id}
          onClick={onRegenFailed}
        >
          <RotateCcw size={13} />
          <span>重试失败预览视频</span>
        </button>
        <button
          className="admin-btn"
          disabled={
            (d.fingerprintFailedCount ?? 0) <= 0 ||
            regenFailedFingerprintId === d.id
          }
          onClick={onRegenFailedFingerprints}
        >
          <RotateCcw size={13} />
          <span>重试失败指纹</span>
        </button>
      </div>
    </div>
  );
}