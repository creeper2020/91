import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { previewController } from "../src/lib/previewController";
import { applyPreviewEnabled, syncPreviewSettings } from "../src/lib/previewSettings";
import { updateConfigYAML } from "../src/admin/api";

test("previews stay inactive until the global policy has loaded", () => {
  assert.equal(previewController.isEnabled(), false);
  previewController.setActiveId("already-generated");
  assert.equal(previewController.getActiveId(), null);
});

test("disabling previews clears the active card and notifies all subscribers", () => {
  applyPreviewEnabled(true);
  previewController.setActiveId("already-generated");
  const updates: Array<string | null> = [];
  const unsubscribe = previewController.subscribe(id => updates.push(id));
  applyPreviewEnabled(false);
  previewController.setActiveId("another-generated-video");
  assert.equal(previewController.getActiveId(), null);
  assert.deepEqual(updates, [null]);
  applyPreviewEnabled(true);
  assert.equal(previewController.getActiveId(), null);
  assert.deepEqual(updates, [null, null]);
  unsubscribe();
  applyPreviewEnabled(false);
});

test("public preview settings synchronize without trusting cached media URLs", async (t) => {
  t.mock.method(globalThis, "fetch", async (url: string, init: RequestInit) => {
    assert.equal(url, "/api/settings/preview");
    assert.equal(init.cache, "no-store");
    return Response.json({ previewEnabled: false });
  });
  applyPreviewEnabled(true);
  previewController.setActiveId("already-generated");
  await syncPreviewSettings();
  assert.equal(previewController.isEnabled(), false);
  assert.equal(previewController.getActiveId(), null);
});

test("a delayed settings response cannot undo a freshly saved global switch", async (t) => {
  let respond!: (response: Response) => void;
  t.mock.method(globalThis, "fetch", () => new Promise<Response>(resolve => { respond = resolve; }));
  const pending = syncPreviewSettings();
  applyPreviewEnabled(false);
  respond(Response.json({ previewEnabled: true }));
  await pending;
  assert.equal(previewController.isEnabled(), false);
});

test("failed or malformed settings responses do not enable previews", async (t) => {
  for (const response of [
    new Response(null, { status: 503 }),
    Response.json({}),
    Response.json({ previewEnabled: "true" }),
    new Response("invalid-json"),
  ]) {
    const mock = t.mock.method(globalThis, "fetch", async () => response);
    applyPreviewEnabled(true);
    await syncPreviewSettings();
    assert.equal(previewController.isEnabled(), false);
    mock.mock.restore();
  }
});

test("saving config immediately updates the shared frontend policy", async (t) => {
  const result = { settings: { previewEnabled: false }, restartRequired: false };
  t.mock.method(globalThis, "fetch", async () => Response.json(result));
  applyPreviewEnabled(true);
  previewController.setActiveId("already-generated");
  assert.deepEqual(await updateConfigYAML("preview: {enabled: false}", "version"), result);
  assert.equal(previewController.isEnabled(), false);
  assert.equal(previewController.getActiveId(), null);
});

test("every card surface gates media, delayed intent, overlays and touch interception", () => {
  for (const file of ["VideoCard", "RecommendedRail", "MobileVideoCollection"]) {
    const source = readFileSync(new URL(`../src/components/${file}.tsx`, import.meta.url), "utf8");
    assert.match(source, /const previewEnabled = usePreviewEnabled\(\)/);
    assert.match(source, /data-preview-enabled=\{previewEnabled\}/);
    assert.match(source, /if \(!previewEnabled\) cleanup(?:Preview)?\(\)/);
    assert.match(source, /!previewController\.isEnabled\(\) \|\| !video\.previewSrc/);
    assert.match(source, /previewEnabled: previewController\.isEnabled\(\) && Boolean\(video\.previewSrc\)/);
    assert.match(source, /previewEnabled && shouldRenderPreview &&/);
    assert.doesNotMatch(source, /\{previewState === "(?:loading|playing|error)"/);
  }
});

test("preview-only card animation selectors require the global switch", () => {
  const cards = readFileSync(new URL("../src/styles/video-card.css", import.meta.url), "utf8");
  const detail = readFileSync(new URL("../src/styles/video-detail.css", import.meta.url), "utf8");
  assert.doesNotMatch(cards, /\.video-card:(?:hover|active)/);
  assert.match(cards, /\.video-card\[data-preview-enabled="true"\]:hover \.thumb-image/);
  assert.match(cards, /\[data-preview-enabled="false"\][\s\S]*?transition: none/);
  assert.match(detail, /\.vd-rail__item\[data-preview-enabled="true"\] \.vd-rail__link:hover \.vd-rail__thumb img/);
  assert.match(detail, /\.vd-collection-item\[data-preview-enabled="true"\] \.vd-collection-item__link:active/);
});
