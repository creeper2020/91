import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const detailCss = readFileSync(
  new URL("../src/styles/video-detail.css", import.meta.url),
  "utf8"
);
const playerSource = readFileSync(
  new URL("../src/components/VideoPlayer.tsx", import.meta.url),
  "utf8"
);
const detailPageSource = readFileSync(
  new URL("../src/pages/VideoDetailPage.tsx", import.meta.url),
  "utf8"
);

test("detail player poster uses full-frame contain scaling", () => {
  assert.match(
    detailCss,
    /\.video-player \.art-poster\s*\{[^}]*background-position:\s*center[^}]*background-repeat:\s*no-repeat[^}]*background-size:\s*contain/s
  );
});

test("detail player does not keep playback resume state", () => {
  assert.doesNotMatch(playerSource, /ResumePrompt/);
  assert.doesNotMatch(playerSource, /PlaybackRecord/);
  assert.doesNotMatch(playerSource, /PLAYBACK_KEY_PREFIX/);
  assert.doesNotMatch(playerSource, /maybeOfferResume/);
  assert.doesNotMatch(playerSource, /savePlaybackRecord/);
  assert.doesNotMatch(playerSource, /clearPlaybackRecord/);
  assert.doesNotMatch(playerSource, /video-player__resume/);
  assert.doesNotMatch(detailCss, /video-player__resume/);
});

test("detail loading skeleton matches current desktop video page layout", () => {
  assert.match(detailPageSource, /className="vd-layout vd-skeleton"/);
  assert.match(detailPageSource, /className="vd-skeleton__summary"/);
  assert.match(detailPageSource, /className="vd-skeleton__info"/);
  assert.match(detailPageSource, /className="vd-rail vd-skeleton__rail"/);
  assert.match(detailPageSource, /Array\.from\(\{ length: 6 \}\)/);
  assert.doesNotMatch(detailPageSource, /className="vd-skeleton__meta"/);
  assert.match(
    detailCss,
    /\.vd-skeleton__player\s*\{[^}]*aspect-ratio:\s*16 \/ 9[^}]*border-radius:\s*0/s
  );
  assert.match(
    detailCss,
    /\.vd-skeleton__summary,\s*\.vd-skeleton__info\s*\{[^}]*border:\s*1px solid var\(--border-default\)[^}]*border-radius:\s*var\(--radius-md\)/s
  );
  assert.match(
    detailCss,
    /\.vd-skeleton__rail-item\s*\{[^}]*grid-template-columns:\s*150px minmax\(0,\s*1fr\)/s
  );
  assert.doesNotMatch(
    detailCss,
    /\.vd-skeleton__player\s*\{[^}]*box-shadow:\s*var\(--shadow-lg\)/s
  );
});

test("detail video title uses a restrained size", () => {
  assert.match(
    detailCss,
    /\.vd-header__title\s*\{[^}]*font-size:\s*var\(--font-xl\)[^}]*line-height:\s*1\.34/s
  );
  assert.doesNotMatch(
    detailCss,
    /\.vd-header__title\s*\{[^}]*font-size:\s*var\(--font-2xl\)/s
  );
  assert.match(
    detailCss,
    /@media \(max-width:\s*480px\)\s*\{[\s\S]*\.vd-header__title\s*\{[^}]*font-size:\s*var\(--font-base\)/s
  );
});

test("detail player uses custom mobile gestures instead of ArtPlayer native gestures", () => {
  assert.match(playerSource, /gesture:\s*false/);
  assert.match(playerSource, /fastForward:\s*false/);
  assert.match(playerSource, /function bindMobilePlayerGestures/);
  assert.match(playerSource, /let suppressNextClick = false/);
  assert.match(playerSource, /endPress\(true\)/);
  assert.match(playerSource, /event\.stopImmediatePropagation\(\)/);
  assert.match(playerSource, /addEventListener\("click", handleClick, true\)/);
  assert.match(playerSource, /state\.mode = "seek"/);
  assert.match(playerSource, /state\.side === "right" \? "volume" : "brightness"/);
  assert.doesNotMatch(playerSource, /function isPlayerLandscapeExpanded/);
  assert.doesNotMatch(playerSource, /getEffectivePlayerOrientation\(art\) === "landscape"/);
  assert.match(playerSource, /if \(!isPlayerExpanded\(art\)\) \{\s*resetGesture\(\);/);
  assert.match(playerSource, /if \(!isPlayerExpanded\(art\)\) return;\s*onGestureHud\(seekGestureLabel/);
  assert.match(playerSource, /const FAST_RATE_CLASS = "art-fast-rate-active"/);
  assert.match(playerSource, /const FAST_RATE_HINT_CLASS = "video-player__art-rate-hint"/);
  assert.match(playerSource, /const PLAYER_GESTURE_HUD_CLASS = "video-player__art-gesture-hud"/);
  assert.match(playerSource, /setPlayerFastRateHint\(art, active\)/);
  assert.match(playerSource, /player\.appendChild\(hint\)/);
  assert.match(playerSource, /showPlayerGestureHud\(art, "volume", formatPercent\(normalized\)\)/);
  assert.match(playerSource, /showPlayerGestureHud\(art, "brightness", formatBrightnessPercent\(nextBrightness\)\)/);
  assert.match(playerSource, /stroke-width="1\.7"/);
  assert.match(playerSource, /M15\.4 9\.2a4\.2 4\.2 0 0 1 0 5\.6/);
  assert.match(playerSource, /M4\.8 9\.7h3l4\.3-3\.6v11\.8l-4\.3-3\.6h-3/);
  assert.doesNotMatch(playerSource, /stroke-width="2\.2"/);
  assert.doesNotMatch(playerSource, /onGestureHud\(`音量 /);
  assert.doesNotMatch(playerSource, /onGestureHud\(`亮度 /);
  assert.match(playerSource, /fullscreen:\s*true/);
  assert.match(playerSource, /fullscreenWeb:\s*!enableOrientationControl/);
  assert.doesNotMatch(playerSource, /addTextTrack\("captions", "Playback rate"/);
  assert.doesNotMatch(playerSource, /new VTTCue\(/);
  assert.doesNotMatch(playerSource, /onGestureHud\(`\$\{FAST_RATE\}x`/);
  assert.match(playerSource, /addEventListener\("touchmove", handleTouchMove, \{ passive: false \}\)/);
});

test("detail player fullscreen long-press rate hint lives inside ArtPlayer", () => {
  assert.match(
    detailCss,
    /\.video-player__rate-hint,\s*\.video-player__art-rate-hint\s*\{[\s\S]*position:\s*absolute[\s\S]*top:\s*12px/s
  );
  assert.match(
    detailCss,
    /\.video-player__art-rate-hint\s*\{[^}]*z-index:\s*130/s
  );
  assert.match(
    detailCss,
    /\.art-video-player\.art-fullscreen \.video-player__art-rate-hint,[\s\S]*\.art-video-player\.art-fullscreen-web \.video-player__art-rate-hint,[\s\S]*position:\s*fixed/s
  );
});

test("detail player mobile brightness gesture only filters the video surface", () => {
  assert.match(
    detailCss,
    /\.video-player \.art-video,\s*\.video-player \.art-poster\s*\{[^}]*filter:\s*brightness\(var\(--video-player-brightness, 1\)\)/s
  );
  assert.match(
    detailCss,
    /@media \(hover: none\) and \(pointer: coarse\)\s*\{[\s\S]*\.video-player \.art-video-player,[\s\S]*touch-action:\s*pan-y/s
  );
  assert.match(
    detailCss,
    /\.video-player \.art-video-player\.art-fullscreen,[\s\S]*\.video-player \.art-video-player\.art-fullscreen-web,[\s\S]*touch-action:\s*none/s
  );
  assert.match(
    detailCss,
    /\.video-player__art-gesture-hud\s*\{[^}]*top:\s*16%[^}]*background:\s*rgba\(18,\s*18,\s*20,\s*0\.8\)[^}]*font-size:\s*18px/s
  );
  assert.match(
    detailCss,
    /\.video-player__art-gesture-hud-icon\s*\{[^}]*width:\s*18px[^}]*height:\s*18px[^}]*transform:\s*translateY\(-1px\)/s
  );
  assert.match(
    detailCss,
    /\.video-player__art-gesture-hud-icon svg\s*\{[^}]*width:\s*18px[^}]*height:\s*18px/s
  );
  assert.match(
    detailCss,
    /\.art-video-player\.art-fullscreen \.video-player__art-gesture-hud,[\s\S]*\.art-video-player\.art-manual-orientation \.video-player__art-gesture-hud\s*\{[^}]*position:\s*fixed/s
  );
});
