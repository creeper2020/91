import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { resolveAdminScrollTarget } from "../src/admin/adminScrollRestoration";

const adminCss = readFileSync(new URL("../src/styles/admin.css", import.meta.url), "utf8");
const adminLayoutSource = readFileSync(
  new URL("../src/admin/AdminLayout.tsx", import.meta.url),
  "utf8"
);
const floatingActionHookSource = readFileSync(
  new URL("../src/admin/useAdminFloatingActionSpace.ts", import.meta.url),
  "utf8"
);
const actionPageSources = [
  "DrivesPage.tsx",
  "CrawlersPage.tsx",
  "CrawlersPageLoading.tsx",
  "VideosPage.tsx",
  "TagsPage.tsx",
  "SettingsPage.tsx",
  "UsersPage.tsx",
].map((file) => readFileSync(new URL(`../src/admin/${file}`, import.meta.url), "utf8"));
const tagsPageSource = actionPageSources[4];
const videosPageSource = actionPageSources[3];

test("admin shell follows CPA desktop-content and mobile-document scrolling", () => {
  assert.match(
    adminCss,
    /\.admin-shell\s*\{[^}]*height:\s*100vh;[^}]*min-height:\s*100vh;[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    adminCss,
    /\.admin-main\s*\{[^}]*height:\s*100vh;[^}]*min-height:\s*0;[^}]*overflow-y:\s*auto;[^}]*scrollbar-gutter:\s*stable;/s
  );
  assert.match(
    adminCss,
    /\.admin-sidebar\s*\{[^}]*height:\s*100vh;[^}]*min-height:\s*0;[^}]*overflow-y:\s*auto;/s
  );
  assert.match(
    adminCss,
    /\.admin-main--logs\s*\{[^}]*overflow:\s*hidden;[^}]*scrollbar-gutter:\s*auto;/s
  );
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-shell\s*\{[^}]*display:\s*block;[^}]*height:\s*auto;[^}]*min-height:\s*100dvh;[^}]*overflow:\s*visible;/s
  );
  assert.doesNotMatch(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-shell\s*\{[^}]*overflow-y:\s*(?:auto|scroll)/s
  );
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-shell\s*\{[^}]*--admin-mobile-scroll-range:\s*var\(--space-9\);/s
  );
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-main\s*\{[^}]*height:\s*auto;[^}]*min-height:\s*calc\(100dvh \+ var\(--admin-mobile-scroll-range\)\);[^}]*overflow:\s*visible;/s
  );
  assert.doesNotMatch(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-main\s*\{[^}]*overflow-y:\s*(?:auto|scroll)/s
  );
  assert.doesNotMatch(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-main--logs\s*\{[^}]*overflow-y:\s*(?:auto|scroll)/s
  );
  assert.doesNotMatch(
    adminCss,
    /\.admin-config-section\s*\{[^}]*(?:^|;)\s*(?:min-|max-)?height\s*:/m
  );
  assert.doesNotMatch(
    adminCss,
    /\.admin-config-section\s*\{[^}]*overflow(?:-y)?\s*:\s*(?:auto|scroll|hidden|clip)\b/s
  );
});

test("fixed admin actions reserve their rendered viewport footprint", () => {
  assert.match(floatingActionHookSource, /\[data-admin-floating-actions\]/);
  assert.match(floatingActionHookSource, /new ResizeObserver/);
  assert.match(floatingActionHookSource, /new MutationObserver/);
  assert.match(floatingActionHookSource, /getBoundingClientRect\(\)\.height/);
  assert.match(floatingActionHookSource, /env\(safe-area-inset-bottom, 0px\)/);
  assert.doesNotMatch(floatingActionHookSource, /visualViewport/);
  assert.match(
    floatingActionHookSource,
    /page\.style\.setProperty\("--admin-floating-actions-space", nextValue\)/
  );
  assert.match(
    adminCss,
    /\.admin-page--with-floating-actions\s*\{[^}]*padding-bottom:\s*var\(--admin-floating-actions-space, 0px\)/s
  );

  for (const source of actionPageSources) {
    assert.match(source, /useAdminFloatingActionSpace/);
    assert.match(source, /admin-page--with-floating-actions/);
    assert.match(source, /data-admin-floating-actions/);
  }

  assert.doesNotMatch(adminCss, /\.admin-(?:drives-page--list|crawlers-page)\s*\{[^}]*padding-bottom/s);
  assert.doesNotMatch(
    adminCss,
    /\.admin-(?:videos-current|videos-blacklist|tags-page)(?:\.has-bulk-actions)?\s*\{[^}]*padding-bottom/s
  );
});

test("mobile list pagination contains only real records", () => {
  assert.doesNotMatch(tagsPageSource, /placeholderTags|admin-tag-card--placeholder/);
  assert.doesNotMatch(videosPageSource, /placeholderRows|admin-video-placeholder-row/);
  assert.doesNotMatch(adminCss, /\.admin-(?:tag-card|video)-placeholder/);
  assert.match(videosPageSource, /const MOBILE_BLACKLIST_PAGE_SIZE = 20;/);
});

test("mobile logs keep one stable, intentional inner viewport", () => {
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-log-panel\s*\{[^}]*min-height:\s*360px;[^}]*height:\s*420px;[^}]*max-height:\s*480px;/s
  );
  assert.match(
    adminCss,
    /\.admin-log-panel__viewport\s*\{[^}]*overflow:\s*auto;[^}]*overscroll-behavior:\s*contain;/s
  );
  assert.doesNotMatch(adminCss, /\.admin-log-panel\s*\{[^}]*\bdvh\b/s);
});

test("admin route navigation resets new pages and restores history positions", () => {
  assert.match(adminLayoutSource, /window\.history\.scrollRestoration = "manual"/);
  assert.match(adminLayoutSource, /const mainScrollRef = useRef<HTMLElement>\(null\)/);
  assert.match(adminLayoutSource, /const pageContentRef = useRef<HTMLDivElement>\(null\)/);
  assert.match(
    adminLayoutSource,
    /const activeScrollRouteRef = useRef<AdminScrollRouteIdentity \| null>\(null\)/
  );
  assert.match(adminLayoutSource, /const routeKey = location\.key/);
  assert.match(adminLayoutSource, /const ADMIN_MOBILE_MEDIA_QUERY = "\(max-width: 768px\)"/);
  assert.match(
    adminLayoutSource,
    /storedScrollTop:\s*scrollPositionsRef\.current\.get\(routeKey\)/
  );
  assert.match(
    adminLayoutSource,
    /window\.scrollTo\(\{ top: scrollTop, left: 0, behavior: "auto" \}\)/
  );
  assert.match(
    adminLayoutSource,
    /main\?\.scrollTo\(\{ top: scrollTop, left: 0, behavior: "auto" \}\)/
  );
  assert.match(adminLayoutSource, /window\.addEventListener\("scroll", saveDocumentScrollPosition/);
  assert.match(adminLayoutSource, /main\?\.addEventListener\("scroll", saveMainScrollPosition/);
  assert.match(adminLayoutSource, /scrollPositionsRef\.current\.set\(activeRoute\.key, scrollTop\)/);
  assert.match(adminLayoutSource, /activeScrollRouteRef\.current = \{/);
  assert.match(adminLayoutSource, /mediaQuery\.addEventListener\("change", handleScrollOwnerChange\)/);
  assert.match(adminLayoutSource, /\}, \[location\.key, location\.pathname\]\);/);
  assert.match(adminLayoutSource, /new ResizeObserver\(scheduleRestore\)/);
  assert.match(adminLayoutSource, /new MutationObserver\(scheduleRestore\)/);
  assert.match(adminLayoutSource, /ref=\{pageContentRef\}/);
  assert.match(adminLayoutSource, /admin-main--logs/);
  assert.match(adminLayoutSource, /admin-page-content--logs/);
});

test("admin scroll target distinguishes history, same-route state, and new routes", () => {
  const previousRoute = { key: "list", pathname: "/admin/videos" };

  assert.equal(
    resolveAdminScrollTarget({
      previousRoute,
      nextPathname: "/admin/videos",
      storedScrollTop: 180,
      currentScrollTop: 640,
    }),
    180
  );
  assert.equal(
    resolveAdminScrollTarget({
      previousRoute,
      nextPathname: "/admin/videos",
      storedScrollTop: 0,
      currentScrollTop: 640,
    }),
    0
  );
  assert.equal(
    resolveAdminScrollTarget({
      previousRoute,
      nextPathname: "/admin/videos",
      storedScrollTop: undefined,
      currentScrollTop: 640,
    }),
    640
  );
  assert.equal(
    resolveAdminScrollTarget({
      previousRoute,
      nextPathname: "/admin/users",
      storedScrollTop: undefined,
      currentScrollTop: 640,
    }),
    0
  );
});
