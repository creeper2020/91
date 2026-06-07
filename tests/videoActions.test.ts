import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const actionsSource = readFileSync(
  new URL("../src/components/VideoActions.tsx", import.meta.url),
  "utf8"
);
const detailCss = readFileSync(
  new URL("../src/styles/video-detail.css", import.meta.url),
  "utf8"
);

test("detail dislike does not locally decrement persisted likes", () => {
  const match = /function handleDislike\(\) \{([\s\S]*?)\n  return \(/.exec(
    actionsSource
  );
  assert.ok(match, "handleDislike block should be present");
  assert.match(match[1], /setDisliked\(true\)/);
  assert.doesNotMatch(match[1], /setLikes/);
});

test("detail like and dislike buttons are visually separated", () => {
  assert.doesNotMatch(actionsSource, /vd-actions__divider/);
  assert.match(
    detailCss,
    /\.vd-actions__group\s*\{[^}]*gap:\s*var\(--space-2\)/s
  );
  assert.match(
    detailCss,
    /\.vd-actions__pill\s*\{[^}]*border:\s*1px solid var\(--border-subtle\)[^}]*border-radius:\s*var\(--radius-sm\)/s
  );
});
