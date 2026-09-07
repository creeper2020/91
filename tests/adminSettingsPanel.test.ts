import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import {
  SettingsRow,
  SettingsSection,
} from "../src/admin/settings/SettingsSection";

const appSource = readFileSync(new URL("../src/App.tsx", import.meta.url), "utf8");
const layoutSource = readFileSync(
  new URL("../src/admin/AdminLayout.tsx", import.meta.url),
  "utf8"
);
const pageSource = readFileSync(
  new URL("../src/admin/SettingsPage.tsx", import.meta.url),
  "utf8"
);
const pageTitleSource = readFileSync(
  new URL("../src/admin/adminPageTitle.ts", import.meta.url),
  "utf8"
);
const sectionSource = readFileSync(
  new URL("../src/admin/settings/SettingsSection.tsx", import.meta.url),
  "utf8"
);
const configYamlSource = readFileSync(
  new URL("../src/admin/settings/configYaml.ts", import.meta.url),
  "utf8"
);
const sourceEditorSource = readFileSync(
  new URL("../src/admin/settings/ConfigSourceEditor.tsx", import.meta.url),
  "utf8"
);
const sourceWorkspaceSource = readFileSync(
  new URL("../src/admin/settings/ConfigSourceWorkspace.tsx", import.meta.url),
  "utf8"
);
const diffModalSource = readFileSync(
  new URL("../src/admin/settings/ConfigDiffModal.tsx", import.meta.url),
  "utf8"
);
const apiSource = readFileSync(new URL("../src/admin/api.ts", import.meta.url), "utf8");
const adminCss = readFileSync(
  new URL("../src/styles/admin.css", import.meta.url),
  "utf8"
);

test("configuration panel is a dedicated protected admin route", () => {
  assert.match(appSource, /const SettingsPage = lazy/);
  assert.match(appSource, /path="settings"[\s\S]*?<SettingsPage \/>/);
  assert.match(layoutSource, /to="\/admin\/settings"/);
  assert.match(layoutSource, />配置面板</);
});

test("configuration panel groups typed fields from the real YAML document", () => {
  assert.match(pageSource, /title="定时任务"/);
  assert.match(pageSource, /description="控制每日扫盘和库内视频维护"/);
  assert.match(pageSource, /label="启动时间"/);
  assert.match(pageSource, /label="时区配置"/);
  assert.doesNotMatch(pageSource, /label="时区配置"\s+description=/);
  assert.match(pageSource, /label="时区配置"[\s\S]*?layout="inline"/);
  assert.match(pageSource, /<select[\s\S]*?id="nightly-timezone"/);
  assert.match(
    pageSource,
    /admin-config-picker-field__value[\s\S]*?draft\.nightlyTimezone \|\| "--"/
  );
  assert.doesNotMatch(pageSource, /北京时间/);
  assert.match(pageSource, /"Asia\/Shanghai"/);
  assert.match(pageSource, /America\/Los_Angeles/);
  assert.match(pageSource, /updateVisualField\("nightlyTimezone", event\.target\.value\)/);
  assert.doesNotMatch(pageSource, /24 小时制 · HH:mm/);
  assert.doesNotMatch(
    pageSource,
    /按服务器本地时区执行，每天最多自动触发一次；保存后无需重启。/
  );
  assert.match(pageSource, /<SettingsSection/);
  assert.match(pageSource, /<SettingsRow/);
  assert.match(pageSource, /type="time"/);
  assert.match(pageSource, /applyVisualFields/);
  assert.match(configYamlSource, /parseDocument/);
  assert.match(configYamlSource, /nightlyStartTimeEdits/);
  assert.match(configYamlSource, /nightlyTimezoneEdits/);
  assert.match(configYamlSource, /nightlyTimezone: string/);
  assert.match(configYamlSource, /\["nightly", "timezone"\]/);
  assert.doesNotMatch(configYamlSource, /document\.toString/);
  assert.match(pageSource, /api\.getConfigYAML\(\)/);
  assert.match(pageSource, /api\.updateConfigYAML\(pendingSave\.after, pendingSave\.version\)/);
  assert.match(pageSource, /有未保存更改/);
  assert.match(apiSource, /If-Match/);
  assert.match(apiSource, /ConfigConflictError/);
  assert.match(apiSource, /nightlyTimezone: string/);
});

test("media generation exposes three independent global concurrency controls", () => {
  assert.match(pageSource, /id: "config-preview"/);
  assert.match(pageSource, /title="媒体生成"/);
  assert.match(pageSource, /控制视频资源生成的并发数，请根据服务器性能和网盘API风控调整，如果性能允许推荐 1-3-1/);
  for (const label of ["封面并发", "预览并发", "视频指纹并发"]) {
    assert.ok(pageSource.includes(`label: "${label}"`));
  }
  for (const field of ["thumbnailConcurrency", "previewConcurrency", "fingerprintConcurrency"]) {
    assert.ok(configYamlSource.includes(`${field}: number`));
    assert.ok(apiSource.includes(`${field}: number`));
  }
  assert.doesNotMatch(pageSource, /单盘预览并发|封面与预览总并发|PREVIEW_CONCURRENCY_OPTIONS/);
  assert.match(pageSource, /GENERATION_CONCURRENCY_OPTIONS\.map/);
  assert.match(configYamlSource, /integerSettingEdits/);
  assert.match(adminCss, /\.admin-config-picker-field--concurrency\s*\{/);
});

test("nightly schedule stop switch uses the shared YAML save flow", () => {
  assert.match(pageSource, /\n\s+label="停止定时任务"/);
  assert.match(
    pageSource,
    /label="启动时间"[\s\S]*?label="时区配置"[\s\S]*?label="停止定时任务"/
  );
  assert.doesNotMatch(pageSource, /开启后不再自动执行每日任务/);
  assert.match(pageSource, /id="nightly-disabled-toggle"/);
  assert.match(pageSource, /aria-checked=\{draft\.nightlyDisabled\}/);
  assert.match(pageSource, /aria-labelledby="nightly-disabled-label"/);
  assert.match(
    pageSource,
    /const scheduleControlsDisabled = controlsDisabled \|\| draft\.nightlyDisabled/
  );
  assert.match(
    pageSource,
    /id="nightly-start-time"[\s\S]*?disabled=\{scheduleControlsDisabled\}/
  );
  assert.match(
    pageSource,
    /id="nightly-timezone"[\s\S]*?disabled=\{scheduleControlsDisabled\}/
  );
  assert.match(
    pageSource,
    /updateVisualField\("nightlyDisabled", !draft\.nightlyDisabled\)/
  );
  assert.doesNotMatch(pageSource, /visualDirtyFields\.has\("nightlyDisabled"\)/);
  assert.match(configYamlSource, /nightlyDisabled: boolean/);
  assert.match(configYamlSource, /\["nightly", "disabled"\]/);
  assert.match(configYamlSource, /nightlyDisabledEdits/);
  assert.match(apiSource, /settings:\s*\{[\s\S]*?nightlyDisabled: boolean/);
});

test("built-in tag changes use the configuration draft and shared save review", () => {
  assert.match(pageSource, /id: "config-tags"/);
  assert.match(pageSource, /title="内置标签"/);
  assert.match(pageSource, /description="管理系统内置标签"/);
  assert.match(pageSource, /label="内置标签"/);
  assert.doesNotMatch(pageSource, /内置标签开关/);
  assert.doesNotMatch(pageSource, /自定义标签不受影响/);
  assert.doesNotMatch(pageSource, /builtin-tags-description/);
  assert.doesNotMatch(pageSource, /api\.getSettings\(\)|api\.updateSettings\(/);
  assert.doesNotMatch(pageSource, /builtinTagsChange\b|builtinTagsDirty/);
  assert.match(pageSource, /role="switch"/);
  assert.match(pageSource, /aria-checked=\{draft\.builtinTagsEnabled\}/);
  assert.match(
    pageSource,
    /updateVisualField\("builtinTagsEnabled", !draft\.builtinTagsEnabled\)/
  );
  assert.match(
    pageSource,
    /api\.updateConfigYAML\(pendingSave\.after, pendingSave\.version\)[\s\S]*?builtinTagsChanged[\s\S]*?invalidateTagsCache\(\)/
  );
  assert.match(pageSource, /visualDirtyFields\.has\("builtinTagsEnabled"\)[\s\S]*?"待恢复"[\s\S]*?"待移除"/);
  assert.doesNotMatch(pageSource, /ConfirmModal|removeBuiltinTagsConfirmOpen/);
  assert.match(configYamlSource, /builtinTagsEnabled: boolean/);
  assert.match(configYamlSource, /\["tags", "builtin_pack_enabled"\]/);
  assert.match(configYamlSource, /builtinTagsEnabledEdits/);
  assert.match(configYamlSource, /key: "builtin_pack_enabled"/);
  assert.match(configYamlSource, /`\$\{field\.key\}: \$\{rendered\}`/);
  assert.match(diffModalSource, /const hasChanges = diff\.additions \+ diff\.deletions > 0/);
  assert.match(diffModalSource, /aria-label="config\.yaml 变更对比"/);
  assert.doesNotMatch(diffModalSource, /settingChanges|应用设置|Database|TriangleAlert/);
  assert.doesNotMatch(adminCss, /admin-config-diff-settings|admin-config-diff-setting__/);
  assert.match(apiSource, /settings:\s*\{[\s\S]*?builtinTagsEnabled: boolean/);
  assert.match(adminCss, /\.admin-config-control--switch\s*\{[^}]*display:\s*flex/s);
});

test("configuration panel keeps the CPA workspace mounted while loading", () => {
  assert.doesNotMatch(pageSource, /AdminLoading/);
  assert.doesNotMatch(pageSource, /if \(loading\) return\s*</);
  assert.match(pageSource, /if \(!loading && \(loadError \|\| !loaded\)\)/);
  assert.match(pageSource, /aria-busy=\{loading\}/);
  assert.match(pageSource, /const controlsDisabled = loading \|\| saving \|\| loaded === null/);
  assert.match(pageSource, /loading\s*\? "正在同步"/);
  assert.match(sourceWorkspaceSource, /<Suspense fallback=\{null\}>/);
  assert.doesNotMatch(
    `${pageSource}\n${sourceWorkspaceSource}`,
    /admin-config-source__loading|正在加载源码编辑器/
  );
  assert.doesNotMatch(adminCss, /\.admin-config-source__loading/);
});

test("configuration source workspace stays visible while CodeMirror loads", () => {
  assert.match(pageSource, /<ConfigSourceWorkspace[\s\S]*?value=\{workingYAML\}/);
  assert.match(
    sourceWorkspaceSource,
    /<div className="admin-config-source">[\s\S]*?<div className="admin-config-source__toolbar">[\s\S]*?<div className=\{`admin-config-source__editor[\s\S]*?<Suspense fallback=\{null\}>[\s\S]*?<LazyConfigSourceEditor/
  );
  assert.doesNotMatch(
    sourceEditorSource,
    /admin-config-source__toolbar|admin-config-source__editor/
  );
  assert.match(pageSource, /window\.requestIdleCallback\(preload, \{ timeout: 1_500 \}\)/);
  assert.match(pageSource, /preloadConfigSourceEditor\(\)\.catch/);
});

test("configuration diff code loads only while preparing a save", () => {
  assert.doesNotMatch(pageSource, /import \{ ConfigDiffModal \} from/);
  assert.match(pageSource, /import\("\.\/settings\/ConfigDiffModal"\)/);
  assert.match(pageSource, /const LazyConfigDiffModal = lazy\(loadConfigDiffModal\)/);
  assert.match(pageSource, /Promise\.all\(\[\s*api\.getConfigYAML\(\),\s*loadConfigDiffModal\(\)/s);
  assert.match(pageSource, /\{pendingSave && \([\s\S]*?<LazyConfigDiffModal/);
});

test("configuration panel follows the CLIProxy configuration workspace UI", () => {
  assert.match(pageTitleSource, /title: "配置面板"/);
  assert.match(pageSource, /const CONFIG_FIELD_COUNT = Object\.keys\(DEFAULT_DRAFT\)\.length/);
  assert.match(pageSource, /\{CONFIG_FIELD_COUNT\} 项常用配置/);
  assert.match(pageSource, /statusText="同步失败"/);
  assert.match(pageSource, /\? "正在同步"[\s\S]*?\? "配置有误"[\s\S]*?\? "正在保存"[\s\S]*?\? "有未保存更改"[\s\S]*?: "已同步"/);
  assert.match(pageSource, /可视化编辑/);
  assert.match(pageSource, /源码编辑/);
  assert.doesNotMatch(pageSource, /placeholder="搜索配置项\.\.\."/);
  assert.doesNotMatch(pageSource, /admin-config-search/);
  assert.match(pageSource, /admin-config-section-nav/);
  assert.match(
    sourceWorkspaceSource,
    /const loadConfigSourceEditor = \(\) => import\("\.\/ConfigSourceEditor"\)/
  );
  assert.match(sourceWorkspaceSource, /const LazyConfigSourceEditor = lazy\(loadConfigSourceEditor\)/);
  assert.match(sourceWorkspaceSource, /placeholder="搜索配置内容\.\.\."/);
  assert.match(sourceEditorSource, /<CodeMirror/);
  assert.match(sourceEditorSource, /yaml\(\)/);
  assert.match(pageSource, /ConfigDiffModal/);
  assert.match(pageSource, /差异已更新，请重新确认/);
  assert.match(diffModalSource, /buildConfigDiff/);
  assert.match(diffModalSource, /确认变更/);
  assert.match(diffModalSource, /@@ -\{hunk\.oldStart\}/);
  assert.match(diffModalSource, /is-addition/);
  assert.match(diffModalSource, /is-deletion/);
  assert.match(
    pageSource,
    /className="admin-config-mode-switch"\s+role="group"\s+aria-label="配置编辑模式"/
  );
  assert.match(
    pageSource,
    /<header className="admin-config-header">\s*<ConfigPageMeta[\s\S]*?className="admin-config-mode-switch"/
  );
  assert.match(
    adminCss,
    /\.admin-config-mode-switch\s*\{[^}]*display:\s*inline-flex;[^}]*border-radius:\s*var\(--radius-pill\);[^}]*background:\s*var\(--bg-surface\);/s
  );
  assert.match(
    adminCss,
    /\.admin-config-mode-switch button\.is-active\s*\{[^}]*background:\s*var\(--bg-page\);[^}]*box-shadow:\s*var\(--shadow-sm\);/s
  );
  assert.match(
    adminCss,
    /\.admin-config-header\s*\{[^}]*justify-content:\s*space-between;[^}]*gap:\s*8px 16px;/s
  );
  assert.doesNotMatch(pageSource, /className="admin-config-tabs"/);
  assert.match(
    pageSource,
    /aria-pressed=\{activeTab === "visual"\}[\s\S]*?disabled=\{loading \|\| saving\}/
  );
  assert.match(
    pageSource,
    /aria-pressed=\{activeTab === "source"\}[\s\S]*?disabled=\{loading \|\| saving\}/
  );
  assert.doesNotMatch(layoutSource, /isSettingsPage|admin-main--settings/);
  assert.doesNotMatch(adminCss, /admin-main--settings|admin-config-content-width/);
  assert.doesNotMatch(
    adminCss,
    /\.admin-config-page\s*\{[^}]*(?:max-)?width\s*:/s
  );
  assert.doesNotMatch(
    adminCss,
    /\.admin-config-header\s*\{[^}]*(?:max-)?width\s*:/s
  );
  assert.match(
    adminCss,
    /\.admin-config-meta\s*\{[^}]*font-family:\s*ui-monospace[^}]*font-variant-numeric:\s*tabular-nums;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-meta::before\s*\{[^}]*content:\s*"▍";[^}]*color:\s*var\(--success\);/s
  );
  assert.match(
    adminCss,
    /\.admin-config-section-nav\s*\{[^}]*position:\s*sticky[^}]*display:\s*flex;[^}]*overflow-x:\s*auto;[^}]*overflow-y:\s*hidden;[^}]*border-bottom:\s*1px solid var\(--border-default\)/s
  );
  assert.match(
    adminCss,
    /\.admin-config-section-nav button::after\s*\{[^}]*bottom:\s*0;[^}]*height:\s*2px;[^}]*background:\s*transparent;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-section-nav button\.is-active::after\s*\{[^}]*background:\s*var\(--text-strong\);/s
  );
  assert.doesNotMatch(adminCss, /\.admin-config-section-nav\s*\{[^}]*grid-template-columns:/s);
  assert.doesNotMatch(pageSource, /admin-config-section-nav__index/);
  assert.match(adminCss, /\.admin-config-sections\s*\{[^}]*display:\s*block/s);
  assert.match(
    adminCss,
    /\.admin-config-section\s*\{[^}]*border-radius:\s*8px;[^}]*background:\s*var\(--bg-surface\);/s
  );
  assert.match(
    adminCss,
    /\.admin-config-section__header\s*\{[^}]*background:\s*transparent;/s
  );
  assert.doesNotMatch(adminCss, /scroll-snap-type:\s*x mandatory/);
  assert.match(adminCss, /\.admin-config-diff-hunk__header/);
  assert.match(adminCss, /\.admin-config-diff-line\.is-addition/);
  assert.match(adminCss, /\.admin-config-diff-line\.is-deletion/);
  assert.match(
    adminCss,
    /\.admin-config-actions\s*\{[^}]*position:\s*fixed[^}]*width:\s*fit-content/s
  );
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-config-row\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\)/s
  );
  assert.match(
    adminCss,
    /\.admin-config-control--switch\s*\{[^}]*display:\s*flex;[^}]*flex-direction:\s*column;[^}]*align-items:\s*center;[^}]*gap:\s*4px;[^}]*min-width:\s*48px;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-row__copy label\s*\{[^}]*align-self:\s*flex-start;[^}]*width:\s*fit-content;[^}]*max-width:\s*100%;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-picker-field\s*\{[^}]*min-height:\s*38px;[^}]*border:\s*1px solid var\(--border-default\);/s
  );
  assert.match(
    adminCss,
    /\.admin-config-picker-field--time\s*\{[^}]*width:\s*72px;[^}]*padding:\s*0 9px;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-picker-field--timezone\s*\{[^}]*width:\s*160px;[^}]*padding:\s*0 8px;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-picker-field__value\s*\{[^}]*overflow:\s*hidden;[^}]*line-height:\s*1\.4;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-picker-field > :is\(input, select\)\s*\{[^}]*position:\s*absolute;[^}]*inset:\s*0;[^}]*color-scheme:\s*dark;[^}]*opacity:\s*0;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-picker-field select option\s*\{[^}]*background:\s*var\(--bg-elevated\);[^}]*color:\s*var\(--text-strong\);/s
  );
  assert.match(
    adminCss,
    /:root\[data-theme="pink"\][\s\S]*?\.admin-config-picker-field > :is\(input, select\)[\s\S]*?color-scheme:\s*light;/s
  );
  assert.match(
    pageSource,
    /admin-config-picker-field__value--time[\s\S]*?draft\.nightlyStartTime \|\| "--:--"[\s\S]*?event\.currentTarget\.showPicker\(\)/
  );
  assert.doesNotMatch(adminCss, /calendar-picker-indicator/);
  assert.match(pageSource, /data-admin-floating-actions/);
});

test("configuration source editor uses one scrollable CodeMirror viewport", () => {
  assert.doesNotMatch(
    `${pageSource}\n${sourceWorkspaceSource}`,
    /<textarea|sourceGutterRef|admin-config-source__gutter/
  );
  assert.match(sourceEditorSource, /height="100%"/);
  assert.match(sourceEditorSource, /lineNumbers:\s*true/);
  assert.match(sourceEditorSource, /foldGutter:\s*true/);
  assert.match(
    adminCss,
    /\.admin-config-source__editor \.cm-scroller\s*\{[^}]*overflow:\s*auto;[^}]*overscroll-behavior:\s*contain;[^}]*touch-action:\s*pan-x pan-y;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-source__editor\s*\{[^}]*height:\s*clamp\(500px, 70vh, 1040px\);[^}]*overflow:\s*hidden;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-source\s*\{[^}]*padding-bottom:\s*var\(--space-8\);/s
  );
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-config-source\s*\{[^}]*padding-bottom:\s*var\(--space-6\);/s
  );
});

test("configuration section navigation directly renders the selected panel", () => {
  const markup = renderToStaticMarkup(
    createElement(
      SettingsSection,
      {
        id: "config-automation",
        index: "01",
        icon: null,
        title: "定时任务",
        description: "维护任务设置",
      },
      createElement("span", null, "setting")
    )
  );
  assert.match(
    markup,
    /<section id="config-automation" class="admin-config-section" role="tabpanel" aria-labelledby="config-automation-tab"/
  );
  assert.match(sectionSource, /role="tabpanel"/);
  assert.match(pageSource, /role="tablist"/);
  assert.match(pageSource, /aria-selected=\{activeSection === section\.id\}/);
  assert.match(pageSource, /onClick=\{\(\) => setActiveSection\(section\.id\)\}/);
  assert.match(pageSource, /activeSection === "config-automation"/);
  assert.match(pageSource, /activeSection === "config-preview"/);
  assert.match(pageSource, /activeSection === "config-tags"/);
  assert.doesNotMatch(pageSource, /activeSection === "config-dedupe"/);
  assert.doesNotMatch(pageSource, /scrollTo\(/);
  assert.doesNotMatch(pageSource, /handleSectionsScroll/);
});

test("compact configuration rows stay inline on mobile", () => {
  const markup = renderToStaticMarkup(
    createElement(
      SettingsRow,
      {
        label: "启动时间",
        layout: "inline",
      },
      createElement("span", null, "03:00")
    )
  );

  assert.match(markup, /class="admin-config-row admin-config-row--inline"/);
  assert.match(pageSource, /label="启动时间"[\s\S]*?layout="inline"/);
  assert.match(pageSource, /label=\{label\}[\s\S]*?layout="inline"/);
  assert.match(pageSource, /label="内置标签"[\s\S]*?layout="inline"/);
  assert.match(
    adminCss,
    /@media \(max-width: 768px\)[\s\S]*?\.admin-config-row--inline\s*\{[^}]*grid-template-columns:\s*minmax\(0, 1fr\) auto;[^}]*align-items:\s*center;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-row--inline \.admin-config-control--picker\s*\{[^}]*justify-items:\s*end;/s
  );
  assert.match(
    adminCss,
    /\.admin-config-control--switch\s*\{[^}]*flex-direction:\s*column;[^}]*justify-content:\s*center;/s
  );
});
