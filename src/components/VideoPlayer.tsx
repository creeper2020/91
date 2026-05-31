import { useEffect, useRef, useState } from "react";
import Hls from "hls.js";

type Props = {
  src: string;
  hlsSrc?: string;
  poster: string;
  title: string;
  status?: string;
  /**
   * 用户首次按下播放时触发。同一个 VideoPlayer 实例只会触发一次；
   * 后续暂停-继续不会重复触发。换 src 时会重置（详情页切换视频用）。
   */
  onFirstPlay?: () => void;
};

/** 长按多少毫秒后进入 2 倍速。短按属于普通点击，交给原生 controls 处理。 */
const LONG_PRESS_MS = 400;
/** 长按时使用的播放倍速。 */
const FAST_RATE = 2;
/** 默认倍速。 */
const NORMAL_RATE = 1;

export function VideoPlayer({
  src,
  hlsSrc,
  poster,
  title,
  status,
  onFirstPlay,
}: Props) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const playedRef = useRef(false);
  // 长按计时器：按下后启动，到时未松开则进入 2 倍速
  const pressTimerRef = useRef<number | null>(null);
  // 当前是否处在"长按 2 倍速"状态
  const [fastActive, setFastActive] = useState(false);
  const fastActiveRef = useRef(false);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const preferredSrc = hlsSrc || src;
    let hls: Hls | null = null;

    if (!preferredSrc) {
      video.removeAttribute("src");
      video.load();
      return;
    }

    let didFallback = false;
    const fallbackToSource = () => {
      if (didFallback || !src) return;
      didFallback = true;
      if (hls) {
        hls.destroy();
        hls = null;
      }
      video.src = src;
      video.load();
    };

    if (hlsSrc && Hls.isSupported()) {
      hls = new Hls({
        enableWorker: true,
        lowLatencyMode: false,
        maxBufferLength: 60,
        backBufferLength: 30,
      });
      hls.loadSource(hlsSrc);
      hls.attachMedia(video);
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          fallbackToSource();
        }
      });
    } else {
      video.src = preferredSrc;
      if (hlsSrc) {
        video.addEventListener("error", fallbackToSource);
      }
    }

    return () => {
      video.removeEventListener("error", fallbackToSource);
      if (hls) {
        hls.destroy();
      }
      video.removeAttribute("src");
      video.load();
    };
  }, [src, hlsSrc]);

  // 长按 2 倍速：直接在 video DOM 元素上监听事件。
  // 这样在 iOS / Android 进入原生全屏播放器后，事件依然能触达，
  // 而 React 合成事件 (onPointerDown 等) 在原生全屏下会失效。
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    function clearPressTimer() {
      if (pressTimerRef.current !== null) {
        window.clearTimeout(pressTimerRef.current);
        pressTimerRef.current = null;
      }
    }

    function activateFast() {
      if (!video) return;
      // 暂停 / 结束状态下长按 2 倍速没有意义
      if (video.paused || video.ended) return;
      video.playbackRate = FAST_RATE;
      fastActiveRef.current = true;
      setFastActive(true);
    }

    function deactivateFast() {
      if (!video) return;
      clearPressTimer();
      if (!fastActiveRef.current) return;
      fastActiveRef.current = false;
      setFastActive(false);
      video.playbackRate = NORMAL_RATE;
    }

    function startPress() {
      if (!video) return;
      if (video.paused || video.ended) return;
      clearPressTimer();
      pressTimerRef.current = window.setTimeout(() => {
        pressTimerRef.current = null;
        activateFast();
      }, LONG_PRESS_MS);
    }

    function endPress() {
      clearPressTimer();
      if (fastActiveRef.current) {
        fastActiveRef.current = false;
        setFastActive(false);
        if (video) video.playbackRate = NORMAL_RATE;
      }
    }

    // ---- 触屏：iOS / Android 全屏下也会派发 ----
    function handleTouchStart() {
      startPress();
    }
    function handleTouchEnd() {
      endPress();
    }
    function handleTouchCancel() {
      endPress();
    }

    // ---- 鼠标：桌面右键已通过 contextmenu 阻止默认，这里只处理左键长按 ----
    function handleMouseDown(e: MouseEvent) {
      if (e.button !== 0) return;
      startPress();
    }
    function handleMouseUp() {
      endPress();
    }
    function handleMouseLeave() {
      endPress();
    }

    // ---- 视频自身状态：暂停 / 切走时立刻恢复正常倍速 ----
    function handlePauseOrEnd() {
      deactivateFast();
    }

    video.addEventListener("touchstart", handleTouchStart, { passive: true });
    video.addEventListener("touchend", handleTouchEnd);
    video.addEventListener("touchcancel", handleTouchCancel);
    video.addEventListener("mousedown", handleMouseDown);
    video.addEventListener("mouseup", handleMouseUp);
    video.addEventListener("mouseleave", handleMouseLeave);
    video.addEventListener("pause", handlePauseOrEnd);
    video.addEventListener("ended", handlePauseOrEnd);

    return () => {
      clearPressTimer();
      video.removeEventListener("touchstart", handleTouchStart);
      video.removeEventListener("touchend", handleTouchEnd);
      video.removeEventListener("touchcancel", handleTouchCancel);
      video.removeEventListener("mousedown", handleMouseDown);
      video.removeEventListener("mouseup", handleMouseUp);
      video.removeEventListener("mouseleave", handleMouseLeave);
      video.removeEventListener("pause", handlePauseOrEnd);
      video.removeEventListener("ended", handlePauseOrEnd);
    };
  }, []);

  // 切换视频时重置首次播放标记和倍速
  useEffect(() => {
    playedRef.current = false;
    fastActiveRef.current = false;
    setFastActive(false);
    if (pressTimerRef.current !== null) {
      window.clearTimeout(pressTimerRef.current);
      pressTimerRef.current = null;
    }
    if (videoRef.current) {
      videoRef.current.playbackRate = NORMAL_RATE;
    }
  }, [src, hlsSrc]);

  function handlePlay() {
    if (!playedRef.current) {
      playedRef.current = true;
      onFirstPlay?.();
    }
  }

  return (
    <div className="video-player">
      <video
        ref={videoRef}
        poster={poster}
        controls
        controlsList="nodownload"
        disablePictureInPicture
        preload="metadata"
        playsInline
        aria-label={title}
        onPlay={handlePlay}
        onContextMenu={(e) => e.preventDefault()}
      />
      {status && (
        <div className="video-player__status" aria-live="polite">
          {status}
        </div>
      )}
      {fastActive && (
        <div className="video-player__rate-hint" aria-hidden="true">
          2x
        </div>
      )}
    </div>
  );
}
