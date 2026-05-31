import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const shortsPageSource = readFileSync(
  new URL("../src/pages/ShortsPage.tsx", import.meta.url),
  "utf8"
);

test("shorts recommendation preference follows successful likes instead of watch time", () => {
  assert.doesNotMatch(shortsPageSource, /currentTime\s*>=\s*3/);
  assert.doesNotMatch(shortsPageSource, /onPreferenceReady/);

  const match = /const handleLikeToggle[\s\S]*?const hasLiked/.exec(
    shortsPageSource
  );
  assert.ok(match, "handleLikeToggle block should be present");

  assert.match(
    match[0],
    /if \(liked\) \{\s*preferredFromVideoIdRef\.current = videoId;\s*\} else if \(preferredFromVideoIdRef\.current === videoId\) \{\s*preferredFromVideoIdRef\.current = null;/
  );
});

test("shorts progress dragging uses immediate pointer state", () => {
  assert.match(shortsPageSource, /const scrubbingRef = useRef\(false\)/);
  assert.match(shortsPageSource, /scrubbingRef\.current = true;/);
  assert.match(shortsPageSource, /if \(!scrubbingRef\.current\) return;/);
  assert.doesNotMatch(shortsPageSource, /if \(!scrubbing\) return;/);
  assert.match(shortsPageSource, /function getSeekDuration/);
  assert.match(shortsPageSource, /onLostPointerCapture=\{handleProgressPointerEnd\}/);
});

test("shorts fullscreen changes preserve the active slide", () => {
  assert.match(shortsPageSource, /const activeIndexRef = useRef\(0\)/);
  assert.match(shortsPageSource, /const ignoreIntersectionUntilRef = useRef\(0\)/);
  assert.match(
    shortsPageSource,
    /if \(Date\.now\(\) < ignoreIntersectionUntilRef\.current\) return;/
  );
  assert.match(shortsPageSource, /function scheduleFullscreenActiveRestore\(\)/);
  assert.match(shortsPageSource, /scheduleFullscreenActiveRestore\(\);\s*setIsFullscreen/);
  assert.match(
    shortsPageSource,
    /function toggleFullscreen\(\) \{\s*scheduleFullscreenActiveRestore\(\);/
  );
  assert.match(shortsPageSource, /scrollIntoView\(\{ block: "start", inline: "nearest", behavior: "auto" \}\)/);
});
