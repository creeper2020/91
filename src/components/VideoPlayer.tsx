import {
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type MutableRefObject,
} from "react";
import Artplayer, { type Option } from "artplayer";
import type Hls from "hls.js";

type Props = {
  id?: string;
  src: string;
  poster: string;
  previewSrc?: string;
  title: string;
  /**
   * 用户首次按下播放时触发。同一个 VideoPlayer 实例只会触发一次；
   * 后续暂停-继续不会重复触发。换 src 时会重置（详情页切换视频用）。
   */
  onFirstPlay?: () => void;
};

type PlayerError = {
  title: string;
  message: string;
};

type GestureHud = {
  key: number;
  label: string;
};

type PreviewHover = {
  x: number;
  ratio: number;
  time: number;
};

type PlayerSettings = {
  volume: number;
  muted: boolean;
  playbackRate: number;
  brightness: number;
};

type VideoElementWithHls = HTMLVideoElement & {
  __hls?: Hls | null;
};

type MobileGestureMode = "seek" | "volume" | "brightness";
type MobileGestureSide = "left" | "right";
type PlayerGestureHudKind = "volume" | "brightness";
type MobileGestureState = {
  startX: number;
  startY: number;
  startTime: number;
  startVolume: number;
  startBrightness: number;
  side: MobileGestureSide;
  mode: MobileGestureMode | null;
  targetTime: number;
  moved: boolean;
  fastActive: boolean;
  previousRate: number;
  pressTimer: number | null;
};

type OrientationMode = "landscape" | "portrait";
type OrientationKind = "native" | "web";
type FullscreenElement = HTMLElement & {
  webkitRequestFullscreen?: () => Promise<void> | void;
  mozRequestFullScreen?: () => Promise<void> | void;
  msRequestFullscreen?: () => Promise<void> | void;
};
type FullscreenDocument = Document & {
  webkitFullscreenElement?: Element | null;
  mozFullScreenElement?: Element | null;
  msFullscreenElement?: Element | null;
  webkitExitFullscreen?: () => Promise<void> | void;
  mozCancelFullScreen?: () => Promise<void> | void;
  msExitFullscreen?: () => Promise<void> | void;
};
type LockableScreenOrientation = ScreenOrientation & {
  lock?: (orientation: OrientationMode) => Promise<void>;
  unlock?: () => void;
};

/** 长按多少毫秒后进入 2 倍速。短按属于普通点击，交给 ArtPlayer 处理。 */
const LONG_PRESS_MS = 400;
/** 长按时使用的播放倍速。 */
const FAST_RATE = 2;
/** 默认倍速。 */
const NORMAL_RATE = 1;

Artplayer.FAST_FORWARD_VALUE = FAST_RATE;

const SETTINGS_KEY = "video-site:player-settings";
const DEFAULT_SETTINGS: PlayerSettings = {
  volume: 0.7,
  muted: false,
  playbackRate: 1,
  brightness: 1,
};
const ORIENTATION_CONTROL_NAME = "orientationToggle";
const MANUAL_ORIENTATION_CLASS = "art-manual-orientation";
const FAST_RATE_CLASS = "art-fast-rate-active";
const FAST_RATE_HINT_CLASS = "video-player__art-rate-hint";
const PLAYER_GESTURE_HUD_CLASS = "video-player__art-gesture-hud";
const PLAYER_GESTURE_HUD_ICON_CLASS = "video-player__art-gesture-hud-icon";
const PLAYER_GESTURE_HUD_VALUE_CLASS = "video-player__art-gesture-hud-value";
const PREVIEW_WIDTH = 168;
const BRIGHTNESS_MIN = 0.45;
const BRIGHTNESS_MAX = 1.35;
const GESTURE_ACTIVATION_PX = 12;
const GESTURE_DIRECTION_LOCK_RATIO = 1.2;
const GESTURE_VERTICAL_SCALE = 1.15;
const GESTURE_SEEK_MIN_SECONDS = 30;
const GESTURE_SEEK_MAX_SECONDS = 120;
const GESTURE_SEEK_DURATION_RATIO = 0.12;
const playerGestureHudTimers = new WeakMap<HTMLElement, number>();

export function VideoPlayer({
  src,
  poster,
  previewSrc,
  title,
  onFirstPlay,
}: Props) {
  const mountRef = useRef<HTMLDivElement | null>(null);
  const artRef = useRef<Artplayer | null>(null);
  const previewVideoRef = useRef<HTMLVideoElement | null>(null);
  const onFirstPlayRef = useRef<Props["onFirstPlay"]>(onFirstPlay);
  const playedRef = useRef(false);
  const [retryNonce, setRetryNonce] = useState(0);
  const [playerError, setPlayerError] = useState<PlayerError | null>(null);
  const [gestureHud, setGestureHud] = useState<GestureHud | null>(null);
  const [previewHover, setPreviewHover] = useState<PreviewHover | null>(null);
  const gestureHudTimerRef = useRef<number | null>(null);

  useEffect(() => {
    onFirstPlayRef.current = onFirstPlay;
  }, [onFirstPlay]);

  useEffect(() => {
    const mount = mountRef.current;
    if (!mount) return;

    playedRef.current = false;
    setPlayerError(null);
    setPreviewHover(null);

    const cleanupPlayer = mountArtPlayer({
      mount,
      src,
      poster,
      title,
      artRef,
      playedRef,
      onFirstPlayRef,
      onFastChange: noop,
      onError: setPlayerError,
      onPreviewHover: setPreviewHover,
      onGestureHud: showGestureHud,
    });

    return cleanupPlayer;
  }, [poster, retryNonce, src, title]);

  useEffect(() => {
    return () => {
      if (gestureHudTimerRef.current !== null) {
        window.clearTimeout(gestureHudTimerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    if (!previewSrc || !previewHover) return;
    syncPreviewVideo(previewVideoRef.current, previewHover.ratio);
  }, [previewHover, previewSrc]);

  function showGestureHud(label: string, duration = 700) {
    if (gestureHudTimerRef.current !== null) {
      window.clearTimeout(gestureHudTimerRef.current);
    }
    setGestureHud({ key: Date.now(), label });
    gestureHudTimerRef.current = window.setTimeout(() => {
      setGestureHud(null);
      gestureHudTimerRef.current = null;
    }, duration);
  }

  function retryPlayback() {
    setPlayerError(null);
    setRetryNonce((n) => n + 1);
  }

  async function copySource() {
    const absolute = new URL(src, window.location.href).href;
    try {
      await navigator.clipboard.writeText(absolute);
      showGestureHud("播放地址已复制", 900);
    } catch {
      fallbackCopyText(absolute);
      showGestureHud("播放地址已复制", 900);
    }
  }

  const previewStyle = previewHover
    ? ({ left: `${previewHover.x}px` } as CSSProperties)
    : undefined;

  return (
    <div className="video-player">
      <div ref={mountRef} className="video-player__mount" />

      {playerError && (
        <div className="video-player__error" role="alert">
          <div className="video-player__error-title">{playerError.title}</div>
          <div className="video-player__error-message">{playerError.message}</div>
          <div className="video-player__error-actions">
            <button type="button" onClick={retryPlayback}>
              重试
            </button>
            <button type="button" onClick={copySource}>
              复制地址
            </button>
          </div>
        </div>
      )}

      {previewSrc && previewHover && (
        <div
          className="video-player__seek-preview"
          style={previewStyle}
          aria-hidden="true"
        >
          <video
            ref={previewVideoRef}
            src={previewSrc}
            poster={poster}
            muted
            playsInline
            preload="metadata"
            onLoadedMetadata={() =>
              syncPreviewVideo(previewVideoRef.current, previewHover.ratio)
            }
          />
          <span>{formatClock(previewHover.time)}</span>
        </div>
      )}

      {gestureHud && (
        <div
          key={gestureHud.key}
          className="video-player__gesture-hud"
          aria-hidden="true"
        >
          {gestureHud.label}
        </div>
      )}

    </div>
  );
}

function inferSourceType(src: string) {
  const lower = src.toLowerCase();
  const cleanPath = lower.split("#")[0].split("?")[0];
  if (cleanPath.endsWith(".m3u8") || lower.includes(".m3u8")) return "m3u8";
  return undefined;
}

function mountArtPlayer({
  mount,
  src,
  poster,
  title,
  artRef,
  playedRef,
  onFirstPlayRef,
  onFastChange,
  onError,
  onPreviewHover,
  onGestureHud,
}: {
  mount: HTMLDivElement;
  src: string;
  poster: string;
  title: string;
  artRef: MutableRefObject<Artplayer | null>;
  playedRef: MutableRefObject<boolean>;
  onFirstPlayRef: MutableRefObject<Props["onFirstPlay"]>;
  onFastChange: (active: boolean) => void;
  onError: (error: PlayerError | null) => void;
  onPreviewHover: (hover: PreviewHover | null) => void;
  onGestureHud: (label: string, duration?: number) => void;
}) {
  const sourceType = inferSourceType(src);
  const settings = readPlayerSettings();
  const fastActiveRef = { current: false };
  const loadHlsSource = createHlsSourceLoader(onError);
  const enableOrientationControl = shouldEnableMobileOrientationControl();
  const option: Option = {
    id: "91-detail-player",
    container: mount,
    url: src,
    poster,
    theme: "var(--video-player-progress)",
    lang: "zh-cn",
    volume: settings.volume,
    muted: settings.muted,
    autoplay: false,
    autoSize: false,
    playbackRate: true,
    aspectRatio: true,
    setting: true,
    hotkey: true,
    pip: true,
    mutex: true,
    fullscreen: true,
    fullscreenWeb: !enableOrientationControl,
    miniProgressBar: true,
    backdrop: false,
    playsInline: true,
    lock: true,
    gesture: false,
    fastForward: false,
    airplay: true,
    customType: {
      hls: loadHlsSource,
      m3u8: loadHlsSource,
    },
    moreVideoAttr: {
      preload: "metadata",
      playsInline: true,
    },
    controls: enableOrientationControl ? [createOrientationControl()] : [],
    contextmenu: [],
    cssVar: {
      "--art-theme": "var(--video-player-progress)",
    },
  };
  if (sourceType) {
    option.type = sourceType;
  }

  const art = new Artplayer(option);
  artRef.current = art;

  const video = art.video as VideoElementWithHls;
  video.setAttribute("aria-label", title);
  video.setAttribute("controlsList", "nodownload");
  video.setAttribute("webkit-playsinline", "true");
  video.disablePictureInPicture = false;
  video.playbackRate = settings.playbackRate;
  applyPlayerBrightness(art, settings.brightness);

  function preventContextMenu(event: Event) {
    event.preventDefault();
  }

  function handlePlay() {
    if (!playedRef.current) {
      playedRef.current = true;
      onFirstPlayRef.current?.();
    }
    onError(null);
  }

  function handleLoadStart() {
    onError(null);
  }

  function handleReady() {
    onError(null);
  }

  function handleVideoError() {
    onError({
      title: "视频源加载失败",
      message: mediaErrorMessage(video.error),
    });
  }

  function resetFastRate() {
    fastActiveRef.current = false;
    setPlayerFastRateHint(art, false);
    onFastChange(false);
  }

  function handleVolumeChange() {
    writePlayerSettings({
      volume: clamp(video.volume, 0, 1),
      muted: video.muted,
    });
  }

  function handleRateChange() {
    if (fastActiveRef.current) return;
    if (!Number.isFinite(video.playbackRate)) return;
    writePlayerSettings({
      playbackRate: clamp(video.playbackRate, 0.5, 3),
    });
  }

  const handleFastChange = (active: boolean) => {
    fastActiveRef.current = active;
    setPlayerFastRateHint(art, active);
    onFastChange(active);
  };

  const unbindFastRate = bindLongPressFast(video, handleFastChange);
  const unbindMobileGestures = bindMobilePlayerGestures(
    art,
    video,
    handleFastChange,
    onGestureHud
  );
  const unbindProgressPreview = bindProgressPreview(
    art,
    video,
    mount,
    onPreviewHover
  );
  const unbindOrientationToggle = enableOrientationControl
    ? bindOrientationToggle(art)
    : noop;

  mount.addEventListener("contextmenu", preventContextMenu);
  video.addEventListener("volumechange", handleVolumeChange);
  video.addEventListener("ratechange", handleRateChange);

  art.on("video:loadstart", handleLoadStart);
  art.on("video:loadeddata", handleReady);
  art.on("video:canplay", handleReady);
  art.on("video:playing", handleReady);
  art.on("video:error", handleVideoError);
  art.on("error", handleVideoError);
  art.on("video:play", handlePlay);
  art.on("video:pause", resetFastRate);
  art.on("video:ended", resetFastRate);

  return () => {
    unbindFastRate();
    unbindMobileGestures();
    unbindProgressPreview();
    unbindOrientationToggle();
    setPlayerFastRateHint(art, false);
    mount.removeEventListener("contextmenu", preventContextMenu);
    video.removeEventListener("volumechange", handleVolumeChange);
    video.removeEventListener("ratechange", handleRateChange);
    destroyHls(video);
    art.off("video:loadstart", handleLoadStart);
    art.off("video:loadeddata", handleReady);
    art.off("video:canplay", handleReady);
    art.off("video:playing", handleReady);
    art.off("video:error", handleVideoError);
    art.off("error", handleVideoError);
    art.off("video:play", handlePlay);
    art.off("video:pause", resetFastRate);
    art.off("video:ended", resetFastRate);
    art.destroy(true);
    if (artRef.current === art) {
      artRef.current = null;
    }
    onPreviewHover(null);
  };
}

function shouldEnableMobileOrientationControl() {
  const coarsePointer = window.matchMedia?.(
    "(hover: none) and (pointer: coarse)"
  ).matches;
  if (coarsePointer) return true;

  return /Android|iPhone|iPad|iPod|Mobile/i.test(navigator.userAgent);
}

function shouldEnableMobileGestures() {
  return shouldEnableMobileOrientationControl();
}

function isPlayerExpanded(art: Artplayer) {
  return Boolean(
    art.fullscreen || art.fullscreenWeb || getNativeFullscreenElement()
  );
}

function setPlayerFastRateHint(art: Artplayer, active: boolean) {
  const player = art.template.$player;
  player.classList.toggle(FAST_RATE_CLASS, active);

  let hint = player.querySelector<HTMLElement>(`.${FAST_RATE_HINT_CLASS}`);
  if (!active) {
    hint?.remove();
    return;
  }

  if (!hint) {
    hint = document.createElement("div");
    hint.className = FAST_RATE_HINT_CLASS;
    hint.setAttribute("aria-hidden", "true");
    hint.textContent = `${FAST_RATE}x`;
    player.appendChild(hint);
  }
}

function showPlayerGestureHud(
  art: Artplayer,
  kind: PlayerGestureHudKind,
  value: string,
  duration = 680
) {
  const player = art.template.$player;
  const currentTimer = playerGestureHudTimers.get(player);
  if (currentTimer !== undefined) {
    window.clearTimeout(currentTimer);
  }

  let hud = player.querySelector<HTMLElement>(`.${PLAYER_GESTURE_HUD_CLASS}`);
  if (!hud) {
    hud = document.createElement("div");
    hud.setAttribute("aria-hidden", "true");
    player.appendChild(hud);
  }

  hud.className = [
    PLAYER_GESTURE_HUD_CLASS,
    `${PLAYER_GESTURE_HUD_CLASS}--${kind}`,
    kind === "volume" && value === "0%" ? `${PLAYER_GESTURE_HUD_CLASS}--muted` : "",
  ]
    .filter(Boolean)
    .join(" ");
  hud.replaceChildren();

  const icon = document.createElement("span");
  icon.className = PLAYER_GESTURE_HUD_ICON_CLASS;
  icon.innerHTML = playerGestureHudIcon(kind, value);

  const valueElement = document.createElement("span");
  valueElement.className = PLAYER_GESTURE_HUD_VALUE_CLASS;
  valueElement.textContent = value;

  hud.append(icon, valueElement);

  const timer = window.setTimeout(() => {
    hud?.remove();
    playerGestureHudTimers.delete(player);
  }, duration);
  playerGestureHudTimers.set(player, timer);
}

function clearPlayerGestureHud(art: Artplayer) {
  const player = art.template.$player;
  const currentTimer = playerGestureHudTimers.get(player);
  if (currentTimer !== undefined) {
    window.clearTimeout(currentTimer);
    playerGestureHudTimers.delete(player);
  }
  player.querySelector<HTMLElement>(`.${PLAYER_GESTURE_HUD_CLASS}`)?.remove();
}

function playerGestureHudIcon(kind: PlayerGestureHudKind, value: string) {
  if (kind === "brightness") {
    return `
      <svg viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="4.2" stroke="currentColor" stroke-width="1.7"/>
        <path d="M12 2.8v2.1M12 19.1v2.1M4.9 4.9l1.5 1.5M17.6 17.6l1.5 1.5M2.8 12h2.1M19.1 12h2.1M4.9 19.1l1.5-1.5M17.6 6.4l1.5-1.5" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
      </svg>
    `;
  }

  if (value === "0%") {
    return `
      <svg viewBox="0 0 24 24" fill="none">
        <path d="M4.8 9.7h3l4.3-3.6v11.8l-4.3-3.6h-3V9.7Z" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
        <path d="m16.1 9.9 4.1 4.1M20.2 9.9 16.1 14" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
      </svg>
    `;
  }

  return `
    <svg viewBox="0 0 24 24" fill="none">
      <path d="M4.8 9.7h3l4.3-3.6v11.8l-4.3-3.6h-3V9.7Z" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
      <path d="M15.4 9.2a4.2 4.2 0 0 1 0 5.6M18 6.7a7.7 7.7 0 0 1 0 10.6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/>
    </svg>
  `;
}

function noop() {
  // noop
}

function createOrientationControl(): NonNullable<Option["controls"]>[number] {
  return {
    name: ORIENTATION_CONTROL_NAME,
    position: "right",
    index: 55,
    tooltip: "横竖屏切换",
    html: `
      <span class="video-player__orientation-control-icon video-player__orientation-control-icon--to-landscape" aria-hidden="true">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
          <path d="M14.4 11.2h2.7c1.7 0 3 1.3 3 3v4.1c0 1.7-1.3 3-3 3h-3.8" fill="none" stroke="currentColor" stroke-opacity=".42" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/>
          <rect x="3.1" y="6.7" width="9.7" height="14.1" rx="2.4" fill="none" stroke="currentColor" stroke-width="2.3"/>
          <path d="M11.8 2.8h2.9c2.6 0 4.7 1.8 5 4.2" fill="none" stroke="currentColor" stroke-width="2.3" stroke-linecap="round"/>
          <path d="M17.4 4.6 19.8 7 22 4.5" fill="none" stroke="currentColor" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </span>
      <span class="video-player__orientation-control-icon video-player__orientation-control-icon--to-portrait" aria-hidden="true">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
          <g transform="rotate(180 12 12)">
            <path d="M12.8 14.4v2.7c0 1.7-1.3 3-3 3H5.7c-1.7 0-3-1.3-3-3v-3.8" fill="none" stroke="currentColor" stroke-opacity=".42" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/>
            <rect x="3.2" y="3.1" width="14.1" height="9.7" rx="2.4" fill="none" stroke="currentColor" stroke-width="2.3"/>
            <path d="M21.2 11.8v2.9c0 2.6-1.8 4.7-4.2 5" fill="none" stroke="currentColor" stroke-width="2.3" stroke-linecap="round"/>
            <path d="M19.4 17.4 17 19.8 19.5 22" fill="none" stroke="currentColor" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/>
          </g>
        </svg>
      </span>
    `,
    mounted(element) {
      element.setAttribute("role", "button");
      element.setAttribute("tabindex", "0");
      updateOrientationControl(this, element);
      this.events.proxy(element, "keydown", (event) => {
        const keyEvent = event as KeyboardEvent;
        if (keyEvent.key !== "Enter" && keyEvent.key !== " ") return;
        keyEvent.preventDefault();
        void togglePlayerOrientation(this);
      });
    },
    click() {
      void togglePlayerOrientation(this);
    },
  };
}

function bindOrientationToggle(art: Artplayer) {
  function handleResize() {
    updateManualWebOrientation(art);
    updateOrientationControl(art);
  }

  function handleFullscreenWeb(state: boolean) {
    if (!state && getManualOrientationKind(art) === "web") {
      clearManualOrientation(art);
      return;
    }
    handleResize();
  }

  function handleFullscreen(state: boolean) {
    if (!state && getManualOrientationKind(art) === "native") {
      clearManualOrientation(art);
      return;
    }
    updateOrientationControl(art);
  }

  window.addEventListener("resize", handleResize);
  window.addEventListener("orientationchange", handleResize);
  getScreenOrientation()?.addEventListener?.("change", handleResize);
  art.on("fullscreenWeb", handleFullscreenWeb);
  art.on("fullscreen", handleFullscreen);
  updateOrientationControl(art);

  return () => {
    clearManualOrientation(art);
    window.removeEventListener("resize", handleResize);
    window.removeEventListener("orientationchange", handleResize);
    getScreenOrientation()?.removeEventListener?.("change", handleResize);
    art.off("fullscreenWeb", handleFullscreenWeb);
    art.off("fullscreen", handleFullscreen);
  };
}

async function togglePlayerOrientation(art: Artplayer) {
  const target = nextOrientationTarget(art);
  const locked = await lockNativeOrientation(art, target);
  if (locked) {
    clearManualWebRotation(art);
    setManualOrientation(art, target, "native");
    art.notice.show = `已切换${orientationLabel(target)}`;
    updateOrientationControl(art);
    return;
  }

  await exitNativeFullscreen();
  if (!art.fullscreenWeb) {
    art.fullscreenWeb = true;
  }
  setManualOrientation(art, target, "web");
  updateManualWebOrientation(art);
  art.notice.show = `已切换${orientationLabel(target)}`;
  updateOrientationControl(art);
}

async function lockNativeOrientation(
  art: Artplayer,
  target: OrientationMode
) {
  const orientation = getScreenOrientation();
  if (!orientation?.lock) return false;

  try {
    const fullscreen = await requestNativeFullscreen(art.template.$player);
    if (!fullscreen) return false;
    await orientation.lock(target);
    return true;
  } catch {
    return false;
  }
}

async function requestNativeFullscreen(element: HTMLElement) {
  if (getNativeFullscreenElement()) return true;
  const target = element as FullscreenElement;
  try {
    if (target.requestFullscreen) {
      await target.requestFullscreen({ navigationUI: "hide" });
      return true;
    }
    const request =
      target.webkitRequestFullscreen ||
      target.mozRequestFullScreen ||
      target.msRequestFullscreen;
    if (!request) return false;
    await maybePromise(request.call(target));
    return true;
  } catch {
    return false;
  }
}

async function exitNativeFullscreen() {
  if (!getNativeFullscreenElement()) return;
  const doc = document as FullscreenDocument;
  const exit =
    doc.exitFullscreen ||
    doc.webkitExitFullscreen ||
    doc.mozCancelFullScreen ||
    doc.msExitFullscreen;
  if (!exit) return;
  try {
    await maybePromise(exit.call(document));
  } catch {
    // ignore
  }
}

function getNativeFullscreenElement() {
  const doc = document as FullscreenDocument;
  return (
    document.fullscreenElement ||
    doc.webkitFullscreenElement ||
    doc.mozFullScreenElement ||
    doc.msFullscreenElement ||
    null
  );
}

function getScreenOrientation() {
  return window.screen?.orientation as LockableScreenOrientation | undefined;
}

async function maybePromise(value: Promise<void> | void) {
  if (value && typeof value.then === "function") {
    await value;
  }
}

function nextOrientationTarget(art: Artplayer): OrientationMode {
  const active = getManualOrientationTarget(art) ?? getViewportOrientation();
  return active === "landscape" ? "portrait" : "landscape";
}

function getViewportOrientation(): OrientationMode {
  const type = getScreenOrientation()?.type;
  if (type?.startsWith("landscape")) return "landscape";
  if (type?.startsWith("portrait")) return "portrait";
  return window.innerWidth > window.innerHeight ? "landscape" : "portrait";
}

function setManualOrientation(
  art: Artplayer,
  target: OrientationMode,
  kind: OrientationKind
) {
  const { dataset } = art.template.$player;
  dataset.videoPlayerOrientationTarget = target;
  dataset.videoPlayerOrientationKind = kind;
}

function getManualOrientationTarget(art: Artplayer) {
  const value = art.template.$player.dataset.videoPlayerOrientationTarget;
  return value === "landscape" || value === "portrait" ? value : null;
}

function getManualOrientationKind(art: Artplayer) {
  const value = art.template.$player.dataset.videoPlayerOrientationKind;
  return value === "native" || value === "web" ? value : null;
}

function clearManualOrientation(art: Artplayer) {
  const kind = getManualOrientationKind(art);
  delete art.template.$player.dataset.videoPlayerOrientationTarget;
  delete art.template.$player.dataset.videoPlayerOrientationKind;
  clearManualWebRotation(art);
  if (kind === "native") {
    try {
      getScreenOrientation()?.unlock?.();
    } catch {
      // ignore
    }
  }
  updateOrientationControl(art);
}

function updateManualWebOrientation(art: Artplayer) {
  if (getManualOrientationKind(art) !== "web") return;
  const target = getManualOrientationTarget(art);
  if (!target) return;
  if (!art.fullscreenWeb) {
    clearManualOrientation(art);
    return;
  }
  if (target !== getViewportOrientation()) {
    applyManualWebRotation(art);
  } else {
    clearManualWebRotation(art);
  }
}

function applyManualWebRotation(art: Artplayer) {
  const player = art.template.$player;
  const viewWidth = document.documentElement.clientWidth;
  const viewHeight = document.documentElement.clientHeight;
  player.style.width = `${viewHeight}px`;
  player.style.height = `${viewWidth}px`;
  player.style.transformOrigin = "0 0";
  player.style.transform = `rotate(90deg) translate(0, -${viewWidth}px)`;
  player.classList.add(MANUAL_ORIENTATION_CLASS);
  art.emit("resize");
}

function clearManualWebRotation(art: Artplayer) {
  const player = art.template.$player;
  player.classList.remove(MANUAL_ORIENTATION_CLASS);
  player.style.transform = "";
  player.style.transformOrigin = "";
  if (art.fullscreenWeb) {
    player.style.width = "100%";
    player.style.height = "100%";
  } else {
    player.style.width = "";
    player.style.height = "";
  }
  art.emit("resize");
}

function updateOrientationControl(art: Artplayer, mountedElement?: HTMLElement) {
  const controls = (art as Artplayer & {
    controls?: Record<string, HTMLElement | undefined>;
  }).controls;
  const element = mountedElement ?? controls?.[ORIENTATION_CONTROL_NAME];
  if (!element) return;
  const next = nextOrientationTarget(art);
  const label = `切换${orientationLabel(next)}`;
  element.dataset.nextOrientation = next;
  element.setAttribute("aria-label", label);
  element.setAttribute("title", label);
}

function orientationLabel(mode: OrientationMode) {
  return mode === "landscape" ? "横屏" : "竖屏";
}

function applyPlayerBrightness(art: Artplayer, brightness: number) {
  art.template.$player.style.setProperty(
    "--video-player-brightness",
    clamp(brightness, BRIGHTNESS_MIN, BRIGHTNESS_MAX).toFixed(2)
  );
}

function getPlayerBrightness(art: Artplayer) {
  const raw = art.template.$player.style.getPropertyValue(
    "--video-player-brightness"
  );
  if (!raw.trim()) return DEFAULT_SETTINGS.brightness;
  return clampNumber(
    Number(raw),
    DEFAULT_SETTINGS.brightness,
    BRIGHTNESS_MIN,
    BRIGHTNESS_MAX
  );
}

function mobileGestureSeekSpan(duration: number) {
  return Math.min(
    duration,
    clamp(
      duration * GESTURE_SEEK_DURATION_RATIO,
      GESTURE_SEEK_MIN_SECONDS,
      GESTURE_SEEK_MAX_SECONDS
    )
  );
}

function seekGestureLabel(
  startTime: number,
  targetTime: number,
  duration: number
) {
  const action = targetTime >= startTime ? "快进" : "快退";
  return `${action} ${formatClock(targetTime)} / ${formatClock(duration)}`;
}

function formatBrightnessPercent(brightness: number) {
  const normalized =
    (clamp(brightness, BRIGHTNESS_MIN, BRIGHTNESS_MAX) - BRIGHTNESS_MIN) /
    (BRIGHTNESS_MAX - BRIGHTNESS_MIN);
  return formatPercent(normalized);
}

function createHlsSourceLoader(
  onError: (error: PlayerError | null) => void
) {
  return function loadHlsSource(
    video: HTMLVideoElement,
    url: string,
    art: Artplayer
  ) {
    const target = video as VideoElementWithHls;
    destroyHls(target);
    onError(null);

    void import("hls.js")
      .then((hlsModule) => {
        if (art.isDestroy || !video.isConnected) return;
        loadHlsSourceWith(video, url, art, hlsModule.default, onError);
      })
      .catch(() => {
        if (art.isDestroy) return;
        onError({
          title: "HLS 内核加载失败",
          message: "播放器组件加载失败，请刷新页面后重试。",
        });
      });
  };
}

function loadHlsSourceWith(
  video: HTMLVideoElement,
  url: string,
  art: Artplayer,
  HlsCtor: typeof Hls,
  onError: (error: PlayerError | null) => void
) {
  const target = video as VideoElementWithHls;
  destroyHls(target);

  if (HlsCtor.isSupported()) {
    const hls = new HlsCtor({
      enableWorker: true,
      lowLatencyMode: true,
      backBufferLength: 90,
    });

    target.__hls = hls;
    art.hls = hls;
    hls.loadSource(url);
    hls.attachMedia(video);
    hls.on(HlsCtor.Events.ERROR, (_event, data) => {
      if (!data.fatal) return;

      if (data.type === HlsCtor.ErrorTypes.NETWORK_ERROR) {
        art.notice.show = "网络错误，正在重试";
        hls.startLoad();
        return;
      }

      if (data.type === HlsCtor.ErrorTypes.MEDIA_ERROR) {
        art.notice.show = "媒体错误，正在恢复";
        hls.recoverMediaError();
        return;
      }

      destroyHls(target);
      onError({
        title: "HLS 播放失败",
        message: "当前视频流无法解析，请稍后重试或复制播放地址排查。",
      });
    });
    return;
  }

  if (
    video.canPlayType("application/vnd.apple.mpegurl") ||
    video.canPlayType("application/x-mpegURL")
  ) {
    video.src = url;
    return;
  }

  onError({
    title: "当前浏览器不支持 HLS",
    message: "请换用新版 Chrome、Edge 或 Safari 后再试。",
  });
}

function destroyHls(video: VideoElementWithHls) {
  if (!video.__hls) return;
  video.__hls.destroy();
  video.__hls = null;
}

function bindLongPressFast(
  video: HTMLVideoElement,
  onFastChange: (active: boolean) => void
) {
  let pressTimer: number | null = null;
  let fastActive = false;
  let previousRate = NORMAL_RATE;
  let suppressNextClick = false;

  function clearPressTimer() {
    if (pressTimer !== null) {
      window.clearTimeout(pressTimer);
      pressTimer = null;
    }
  }

  function setFast(next: boolean) {
    if (fastActive === next) return;
    if (next) {
      previousRate =
        Number.isFinite(video.playbackRate) && video.playbackRate > 0
          ? video.playbackRate
          : NORMAL_RATE;
    }
    fastActive = next;
    video.playbackRate = next ? FAST_RATE : previousRate;
    onFastChange(next);
  }

  function activateFast() {
    if (video.paused || video.ended) return;
    setFast(true);
  }

  function startPress() {
    if (video.paused || video.ended) return;
    clearPressTimer();
    pressTimer = window.setTimeout(() => {
      pressTimer = null;
      activateFast();
    }, LONG_PRESS_MS);
  }

  function endPress(suppressClick = false) {
    clearPressTimer();
    const wasFastActive = fastActive;
    setFast(false);
    if (wasFastActive && suppressClick) {
      suppressNextClick = true;
    }
  }

  function handleMouseDown(event: MouseEvent) {
    if (event.button !== 0) return;
    startPress();
  }

  function handleMouseUp(event: MouseEvent) {
    if (event.button !== 0) return;
    endPress(true);
  }

  function handlePressEnd() {
    endPress();
  }

  function handleClick(event: MouseEvent) {
    if (!suppressNextClick) return;
    suppressNextClick = false;
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
  }

  video.addEventListener("mousedown", handleMouseDown);
  video.addEventListener("mouseup", handleMouseUp);
  video.addEventListener("click", handleClick, true);
  video.addEventListener("mouseleave", handlePressEnd);
  video.addEventListener("pause", handlePressEnd);
  video.addEventListener("ended", handlePressEnd);

  return () => {
    clearPressTimer();
    setFast(false);
    video.removeEventListener("mousedown", handleMouseDown);
    video.removeEventListener("mouseup", handleMouseUp);
    video.removeEventListener("click", handleClick, true);
    video.removeEventListener("mouseleave", handlePressEnd);
    video.removeEventListener("pause", handlePressEnd);
    video.removeEventListener("ended", handlePressEnd);
  };
}

function bindMobilePlayerGestures(
  art: Artplayer,
  video: HTMLVideoElement,
  onFastChange: (active: boolean) => void,
  onGestureHud: (label: string, duration?: number) => void
) {
  if (!shouldEnableMobileGestures()) return noop;

  const player = art.template.$player;
  let state: MobileGestureState | null = null;

  function clearPressTimer() {
    if (!state || state.pressTimer === null) return;
    window.clearTimeout(state.pressTimer);
    state.pressTimer = null;
  }

  function setTouchFast(next: boolean) {
    if (!state || state.fastActive === next) return;
    if (next) {
      state.previousRate =
        Number.isFinite(video.playbackRate) && video.playbackRate > 0
          ? video.playbackRate
          : NORMAL_RATE;
      state.fastActive = true;
      onFastChange(true);
      video.playbackRate = FAST_RATE;
      return;
    }

    const previousRate = state.previousRate;
    state.fastActive = false;
    onFastChange(false);
    video.playbackRate = previousRate;
  }

  function resetGesture() {
    clearPressTimer();
    if (state?.fastActive) {
      setTouchFast(false);
    }
    state = null;
  }

  function handleTouchStart(event: TouchEvent) {
    if (event.touches.length !== 1 || art.isLock) return;

    const touch = event.touches[0];
    const rect = player.getBoundingClientRect();
    const localX = touch.clientX - rect.left;
    state = {
      startX: touch.clientX,
      startY: touch.clientY,
      startTime: video.currentTime || 0,
      startVolume: video.muted ? 0 : clamp(video.volume, 0, 1),
      startBrightness: getPlayerBrightness(art),
      side: localX < rect.width / 2 ? "left" : "right",
      mode: null,
      targetTime: video.currentTime || 0,
      moved: false,
      fastActive: false,
      previousRate: video.playbackRate || NORMAL_RATE,
      pressTimer: null,
    };

    state.pressTimer = window.setTimeout(() => {
      if (!state || state.mode || state.moved || video.paused || video.ended) {
        return;
      }
      setTouchFast(true);
    }, LONG_PRESS_MS);
  }

  function lockGestureMode(dx: number, dy: number) {
    if (!state) return;
    const absX = Math.abs(dx);
    const absY = Math.abs(dy);
    if (absX < GESTURE_ACTIVATION_PX && absY < GESTURE_ACTIVATION_PX) return;

    state.moved = true;
    clearPressTimer();

    if (absX >= absY * GESTURE_DIRECTION_LOCK_RATIO) {
      state.mode = "seek";
      return;
    }

    if (absY >= absX * GESTURE_DIRECTION_LOCK_RATIO) {
      if (!isPlayerExpanded(art)) {
        resetGesture();
        return;
      }
      state.mode = state.side === "right" ? "volume" : "brightness";
    }
  }

  function handleTouchMove(event: TouchEvent) {
    if (!state) return;
    if (event.touches.length !== 1) {
      resetGesture();
      return;
    }

    const touch = event.touches[0];
    const dx = touch.clientX - state.startX;
    const dy = touch.clientY - state.startY;

    if (state.fastActive) {
      event.preventDefault();
      return;
    }

    if (!state.mode) {
      lockGestureMode(dx, dy);
      if (!state || !state.mode) return;
    }

    event.preventDefault();

    if (state.mode === "seek") {
      handleSeekGesture(event, dx);
      return;
    }

    if (state.mode === "volume") {
      handleVolumeGesture(touch.clientY);
      return;
    }

    handleBrightnessGesture(touch.clientY);
  }

  function handleSeekGesture(event: TouchEvent, dx: number) {
    if (!state) return;
    const duration = video.duration;
    if (!Number.isFinite(duration) || duration <= 0) return;
    const rect = player.getBoundingClientRect();
    const span = mobileGestureSeekSpan(duration);
    const targetTime = clamp(
      state.startTime + (dx / Math.max(1, rect.width)) * span,
      0,
      duration
    );
    state.targetTime = targetTime;
    art.emit("setBar", "played", targetTime / duration, event);
    if (!isPlayerExpanded(art)) return;
    onGestureHud(seekGestureLabel(state.startTime, targetTime, duration), 560);
  }

  function handleVolumeGesture(currentY: number) {
    if (!state) return;
    const rect = player.getBoundingClientRect();
    const delta = (state.startY - currentY) / Math.max(1, rect.height);
    const nextVolume = clamp(state.startVolume + delta, 0, 1);
    const normalized = Math.round(nextVolume * 100) / 100;
    video.volume = normalized;
    video.muted = normalized <= 0;
    showPlayerGestureHud(art, "volume", formatPercent(normalized));
  }

  function handleBrightnessGesture(currentY: number) {
    if (!state) return;
    const rect = player.getBoundingClientRect();
    const delta =
      ((state.startY - currentY) / Math.max(1, rect.height)) *
      GESTURE_VERTICAL_SCALE;
    const nextBrightness = clamp(
      state.startBrightness + delta,
      BRIGHTNESS_MIN,
      BRIGHTNESS_MAX
    );
    applyPlayerBrightness(art, nextBrightness);
    showPlayerGestureHud(art, "brightness", formatBrightnessPercent(nextBrightness));
  }

  function handleTouchEnd() {
    if (!state) return;

    if (state.mode === "seek") {
      const duration = video.duration;
      if (Number.isFinite(duration) && duration > 0) {
        art.seek = clamp(state.targetTime, 0, duration);
        if (isPlayerExpanded(art)) {
          onGestureHud(
            seekGestureLabel(state.startTime, state.targetTime, duration),
            720
          );
        }
      }
    } else if (state.mode === "brightness") {
      writePlayerSettings({
        brightness: getPlayerBrightness(art),
      });
    } else if (state.mode === "volume") {
      writePlayerSettings({
        volume: clamp(video.volume, 0, 1),
        muted: video.muted,
      });
    }

    resetGesture();
  }

  video.addEventListener("touchstart", handleTouchStart, { passive: true });
  video.addEventListener("touchmove", handleTouchMove, { passive: false });
  video.addEventListener("touchend", handleTouchEnd);
  video.addEventListener("touchcancel", resetGesture);
  video.addEventListener("pause", resetGesture);
  video.addEventListener("ended", resetGesture);
  window.addEventListener("blur", resetGesture);

  return () => {
    clearPlayerGestureHud(art);
    resetGesture();
    video.removeEventListener("touchstart", handleTouchStart);
    video.removeEventListener("touchmove", handleTouchMove);
    video.removeEventListener("touchend", handleTouchEnd);
    video.removeEventListener("touchcancel", resetGesture);
    video.removeEventListener("pause", resetGesture);
    video.removeEventListener("ended", resetGesture);
    window.removeEventListener("blur", resetGesture);
  };
}

function bindProgressPreview(
  art: Artplayer,
  video: HTMLVideoElement,
  mount: HTMLDivElement,
  onPreviewHover: (hover: PreviewHover | null) => void
) {
  const progress = art.query<HTMLElement>(".art-progress");
  if (!progress) return () => undefined;
  const progressEl = progress;

  function update(event: PointerEvent | MouseEvent) {
    if ("pointerType" in event && event.pointerType === "touch") return;
    const duration = video.duration;
    if (!Number.isFinite(duration) || duration <= 0) return;
    const rect = progressEl.getBoundingClientRect();
    const hostRect = mount.getBoundingClientRect();
    const ratio = clamp((event.clientX - rect.left) / Math.max(1, rect.width), 0, 1);
    const edge = Math.min(PREVIEW_WIDTH / 2 + 8, hostRect.width / 2);
    const maxX = Math.max(edge, hostRect.width - edge);
    onPreviewHover({
      x: clamp(event.clientX - hostRect.left, edge, maxX),
      ratio,
      time: ratio * duration,
    });
  }

  function hide() {
    onPreviewHover(null);
  }

  progressEl.addEventListener("pointermove", update);
  progressEl.addEventListener("pointerdown", update);
  progressEl.addEventListener("pointerleave", hide);
  window.addEventListener("pointerup", hide);
  window.addEventListener("blur", hide);

  return () => {
    progressEl.removeEventListener("pointermove", update);
    progressEl.removeEventListener("pointerdown", update);
    progressEl.removeEventListener("pointerleave", hide);
    window.removeEventListener("pointerup", hide);
    window.removeEventListener("blur", hide);
  };
}

function readPlayerSettings(): PlayerSettings {
  const saved = safeGetJSON<Partial<PlayerSettings>>(SETTINGS_KEY) ?? {};
  return {
    volume: clampNumber(saved.volume, DEFAULT_SETTINGS.volume, 0, 1),
    muted: typeof saved.muted === "boolean" ? saved.muted : DEFAULT_SETTINGS.muted,
    playbackRate: clampNumber(saved.playbackRate, DEFAULT_SETTINGS.playbackRate, 0.5, 3),
    brightness: clampNumber(
      saved.brightness,
      DEFAULT_SETTINGS.brightness,
      BRIGHTNESS_MIN,
      BRIGHTNESS_MAX
    ),
  };
}

function writePlayerSettings(patch: Partial<PlayerSettings>) {
  safeSetJSON(SETTINGS_KEY, { ...readPlayerSettings(), ...patch });
}

function mediaErrorMessage(error: MediaError | null) {
  switch (error?.code) {
    case MediaError.MEDIA_ERR_ABORTED:
      return "视频加载已取消，请重试。";
    case MediaError.MEDIA_ERR_NETWORK:
      return "视频源网络连接失败，请稍后重试。";
    case MediaError.MEDIA_ERR_DECODE:
      return "视频编码无法解码，可能需要转码或换用浏览器。";
    case MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED:
      return "视频源暂不可用或格式不受当前浏览器支持。";
    default:
      return "视频源暂时无法播放，请重试或复制地址排查。";
  }
}

function syncPreviewVideo(video: HTMLVideoElement | null, ratio: number) {
  if (!video || !Number.isFinite(video.duration) || video.duration <= 0) return;
  const target = clamp(ratio * video.duration, 0, Math.max(0, video.duration - 0.05));
  if (Math.abs(video.currentTime - target) > 0.25) {
    try {
      video.currentTime = target;
    } catch {
      // ignore
    }
  }
}

function fallbackCopyText(text: string) {
  const textarea = document.createElement("textarea");
  textarea.value = text;
  textarea.setAttribute("readonly", "true");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    document.execCommand("copy");
  } catch {
    // ignore
  } finally {
    textarea.remove();
  }
}

function safeGetJSON<T>(key: string): T | null {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

function safeSetJSON(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // ignore
  }
}

function clampNumber(
  value: unknown,
  fallback: number,
  min: number,
  max: number
) {
  return typeof value === "number" && Number.isFinite(value)
    ? clamp(value, min, max)
    : fallback;
}

function clamp(n: number, min: number, max: number) {
  return n < min ? min : n > max ? max : n;
}

function formatClock(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return "00:00";
  const total = Math.floor(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  if (h > 0) {
    return `${String(h).padStart(2, "0")}:${String(m).padStart(
      2,
      "0"
    )}:${String(s).padStart(2, "0")}`;
  }
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

function formatPercent(value: number) {
  return `${Math.round(clamp(value, 0, 1) * 100)}%`;
}
