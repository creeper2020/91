import { previewController } from "./previewController";

let revision = 0;

export function applyPreviewEnabled(enabled: boolean): void {
  revision += 1;
  previewController.setEnabled(enabled);
}

export async function syncPreviewSettings(): Promise<void> {
  const requestRevision = ++revision;
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10_000);
  try {
    const response = await fetch("/api/settings/preview", {
      credentials: "include",
      cache: "no-store",
      signal: controller.signal,
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const data: { previewEnabled?: unknown } = await response.json();
    if (requestRevision === revision) {
      previewController.setEnabled(data.previewEnabled === true);
    }
  } catch {
    if (requestRevision === revision) previewController.setEnabled(false);
  } finally {
    clearTimeout(timeout);
  }
}

// Refresh other tabs and long-lived listing pages without coupling cards to HTTP.
export function watchPreviewSettings(): () => void {
  const refresh = () => {
    if (document.visibilityState !== "hidden") void syncPreviewSettings();
  };
  refresh();
  const timer = window.setInterval(refresh, 15_000);
  window.addEventListener("focus", refresh);
  document.addEventListener("visibilitychange", refresh);
  return () => {
    window.clearInterval(timer);
    window.removeEventListener("focus", refresh);
    document.removeEventListener("visibilitychange", refresh);
    revision += 1;
  };
}
