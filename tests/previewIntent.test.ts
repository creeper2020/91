import assert from "node:assert/strict";
import test from "node:test";

import {
  shouldInterceptPreviewTap,
  shouldStartInstantPreview,
  TOUCH_PREVIEW_DELAY_MS,
} from "../src/lib/previewIntent.ts";

test("touch preview media waits 200ms after the first tap intent", () => {
  assert.equal(TOUCH_PREVIEW_DELAY_MS, 200);
});

test("touch tap starts preview instead of navigating when preview is idle", () => {
  assert.equal(
    shouldInterceptPreviewTap({
      previewEnabled: true,
      canHover: false,
      pointerType: "touch",
      previewActive: false,
    }),
    true
  );
  assert.equal(shouldStartInstantPreview({ pointerType: "touch" }), true);
});

test("touch tap navigates when the same card preview is already active", () => {
  assert.equal(
    shouldInterceptPreviewTap({
      previewEnabled: true,
      canHover: false,
      pointerType: "touch",
      previewActive: true,
    }),
    false
  );
});

test("mouse click does not intercept normal navigation", () => {
  assert.equal(
    shouldInterceptPreviewTap({
      previewEnabled: true,
      canHover: true,
      pointerType: "mouse",
      previewActive: false,
    }),
    false
  );
  assert.equal(shouldStartInstantPreview({ pointerType: "mouse" }), false);
});
test("disabled previews never intercept touch navigation, even for an active preview", () => {
  for (const previewActive of [false, true]) {
    assert.equal(shouldInterceptPreviewTap({
      previewEnabled: false,
      previewActive,
      pointerType: "touch",
      canHover: false,
    }), false);
  }
});
