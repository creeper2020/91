# 视频聚合站首页实现方案

## 1. 项目目标

本项目目标是实现一个类似高密度视频聚合站的首页体验，重点借鉴其页面结构、视频浏览效率、搜索入口和鼠标悬停自动预览交互。

- 顶部工具条
- 主导航
- 二级用户导航
- 搜索框
- 热门标签
- 视频网格
- 鼠标悬停自动预览
- 分页和页脚

第一版优先完成首页，不做完整登录、上传、支付、会员、评论和后台管理。

## 2. 设计方向

整体风格建议保留原参考站的高密度媒体目录感，但升级成更现代、干净、合规的视频浏览平台。

视觉关键词：

- 高密度
- 直接
- 媒体站
- 暗色导航
- 橙黄色强调色
- 紧凑视频卡片
- 快速预览

推荐色彩：

```css
:root {
  --color-page: #f4f4f2;
  --color-topbar: #232323;
  --color-nav: #111111;
  --color-card: #151515;
  --color-card-border: #030303;
  --color-accent: #ff8800;
  --color-text: #202020;
  --color-text-invert: #ffffff;
  --color-muted: #8a8a8a;
  --color-line: #dddddd;
}
```

### 2.1 参考站细节取样

本轮取样覆盖了首页、列表页和详情页三类模板。只记录结构、样式和交互，不复用原站内容、素材和品牌。

关键观察：

- 首页是“导航 + 横幅 + 搜索 + 热门标签 + 今日内容”的入口页，当前页约 `24` 个视频卡片。
- 列表页是更纯粹的视频目录页，当前页约 `42` 个视频卡片，有分页、视图切换和更强的浏览密度。
- 详情页是“播放器主列 + 右侧推荐列”的结构，右侧推荐卡片也复用同一套 hover 预览能力。
- 视频卡片不是大卡片，而是紧凑信息块：封面、徽标、时长、单行标题、作者、观看量、收藏数、评论数、点赞/点踩。
- 视觉系统主要由深色导航、黑色卡片、橙色强调色、白色文字和灰色辅助信息组成。
- 自动预览是站内所有视频卡片的基础能力，不只是首页特效。
- 原站同时存在两种预览机制：独立 `mp4` teaser 预览，以及旧式多张缩略图轮播。
- 页面使用懒加载、返回顶部按钮、分页组件、Video.js 播放器和右侧推荐列。

## 3. 页面结构

页面从上到下分为以下区域。

### 3.1 顶部工具条

高度约 `30px`。

内容：

- 左侧语言切换
- 右侧注册
- 右侧登录

样式：

- 背景 `#232323`
- 字体 `12px`
- 链接默认灰色
- hover 变白
- 保持固定高度，不做复杂装饰

### 3.2 主导航栏

高度约 `56-64px`。

内容：

- 左侧 Logo
- 右侧主栏目
- 移动端汉堡菜单

推荐栏目：

- 上传
- 视频
- 频道
- 排行榜
- 会员
- 创作者

样式：

- 背景黑色或深灰
- 当前栏目使用浅色背景或橙色下划线
- 字体 `14-16px`
- 移动端折叠菜单

### 3.3 二级菜单

二级菜单放用户相关入口。

推荐入口：

- 我的视频
- 我的收藏
- 我关注的用户
- 我关注的视频
- 我的留言
- 历史记录

样式：

- 背景沿用深色
- 链接使用橙色
- hover 变白
- 横向排列
- 小屏横向滚动

### 3.4 横幅推荐区

参考站这里是广告位。实际实现时建议改成合规内容：

- 精选频道
- 推荐合集
- 今日专题
- 活动入口

布局：

- 桌面端 3-6 个横幅
- 移动端横向滑动
- 横幅高度控制在 `72-120px`

不要让横幅压过主体内容。它应该提供商业站/门户站的密度感，但不能成为页面主体。

### 3.5 搜索区

搜索区是用户进入内容的核心入口。

结构：

- 关键词输入框
- 搜索类型选择
- 搜索按钮
- 热门标签列表

搜索类型：

- 搜索视频
- 搜索用户
- 视频 ID
- 日期

交互：

- 输入关键词后点击搜索按钮
- 回车提交
- 热门标签点击后填入关键词并筛选

### 3.6 内容标题条

用于区分当前内容模块。

示例标题：

- 今日排行
- 最新视频
- 热门推荐
- 本周精选

样式：

- 橙色背景
- 白色文字
- 高度约 `40px`
- 字体 `16px`
- 可居中，也可左对齐

### 3.7 视频网格

这是首页主体。

推荐栅格：

- 宽屏：5 列
- 桌面：4 列
- 平板：3 列
- 手机：2 列
- 极窄屏：1 列

每页数量：

- 第一版建议 24 个视频卡片

参考站差异：

- 首页适合 `24` 个卡片，强调入口和热词。
- 列表页适合 `36-48` 个卡片，强调连续浏览。
- 详情页侧栏适合 `6-10` 个推荐卡片，强调回流。

### 3.8 列表页结构

列表页用于承载分类、排行、搜索结果和标签结果。

页面结构：

- 顶部工具条
- 主导航
- 二级分类菜单
- 横幅推荐区
- 搜索区
- 当前分类标题
- 视图/排序工具条
- 视频网格
- 分页
- 页脚

推荐工具条：

- 最新
- 最热
- 本周
- 最长
- 高清
- 精选
- 基础视图
- 详细视图

列表页和首页共用同一个 `VideoCard`，但可以允许更高密度布局。

### 3.9 详情页结构

详情页用于播放单个视频，并把用户导向推荐内容。

页面结构：

- 顶部工具条
- 主导航
- 二级分类菜单
- 搜索区
- 主内容双列布局
- 左侧播放器
- 左侧操作区
- 左侧视频信息区
- 左侧评论区
- 右侧推荐视频
- 页脚

左侧主列建议宽度：

- 桌面端占 `8/12`
- 移动端占 `100%`

右侧推荐列建议宽度：

- 桌面端占 `4/12`
- 移动端下沉到播放器下面

详情页组件：

- 播放器区域：封面、视频源、播放控件、广告/推荐插槽可选。
- 操作区：观看量、评论数、收藏数、点赞、点踩、收藏按钮。
- 信息区：发布时间、作者、描述、标签、分享/嵌入链接。
- 评论区：评论列表、分页、登录提示。
- 推荐列：复用 `VideoCard`，保留 hover 预览。

参考站详情页细节：

- 标题不是大号 hero，而是橙色横条标题，放在播放器上方。
- 播放器使用 `16:9` 容器，外层有轻微内边距，桌面端宽度占主列 `100%`。
- 播放器下方第一排是统计信息：时长、观看量、评论数、收藏数、积分/热度。
- 播放器下方第二排是操作按钮：点赞、点踩、收藏、写评论、下载提示、移除/举报入口。
- 统计值使用橙色强调，标签使用灰色或白色，信息排布很紧凑。
- 信息面板用橙色横条分区，包含发布时间、作者、作者状态、关注按钮、作者数据、描述。
- 描述区支持长文本折叠/展开，避免详情页被描述撑得过长。
- 嵌入链接使用只读 textarea，点击后可全选。
- 评论区是独立分区，有标题条、列表容器和分页加载。
- 右侧栏标题为推荐/热门视频，下面先是可选广告/推荐位，再是紧凑视频卡片列表。
- 右侧推荐卡片比首页更窄，元信息会换行显示，仍保留 hover 预览。

详情页布局建议：

```txt
VideoDetailPage
  AppShell
    SearchPanel
    DetailLayout
      MainColumn
        DetailTitleBar
        VideoPlayer
        VideoStats
        VideoActions
        VideoInfoPanel
        EmbedLinkBox
        CommentPanel
      SideColumn
        RecommendedRail
```

详情页桌面布局：

- 页面容器最大宽度 `1140-1200px`。
- 主列宽度 `66%` 左右。
- 侧栏宽度 `30-34%`。
- 主列和侧栏间距 `20-24px`。
- 播放器、信息面板、评论面板垂直堆叠。
- 侧栏卡片保持紧凑，不要做成大图瀑布流。

详情页移动布局：

- 主列和侧栏改为单列。
- 播放器固定 `16:9`，宽度 `100%`。
- 标题条允许两行，但不遮挡播放器。
- 统计信息改为两行或横向滚动。
- 操作按钮改为图标按钮网格。
- 推荐列下沉到评论区前或评论区后，第一版建议放在评论区前，提高回流。

详情页交互：

- 播放器支持播放、暂停、进度、音量、全屏。
- 点赞/点踩有选中态，第一版可以只做本地状态。
- 收藏按钮有未收藏、已收藏、需要登录三种状态。
- 写评论按钮滚动到评论区。
- 嵌入链接点击后自动选中。
- 推荐卡片 hover 后播放预览，离开后停止。

## 4. 视频卡片设计

### 4.1 数据结构

```ts
export type VideoItem = {
  id: string;
  href: string;
  title: string;
  thumbnail: string;
  previewSrc: string;
  previewDuration: number;
  previewStrategy: "teaser-file" | "sprite-frames";
  duration: string;
  badges: string[];
  quality?: "SD" | "HD";
  sourceLabel?: string;
  author: string;
  views: number;
  favorites?: number;
  comments?: number;
  likes?: number;
  dislikes?: number;
  publishedAt: string;
  rating?: number;
};

export type VideoDetail = VideoItem & {
  videoSrc: string;
  poster: string;
  description: string;
  embedUrl: string;
  points?: number;
  authorProfile: {
    id: string;
    name: string;
    href: string;
    badges: string[];
    signupAge?: string;
    level?: number;
    points?: number;
    videoCount?: number;
    followers?: number;
    following?: number;
    isFollowing?: boolean;
  };
  relatedVideos: VideoItem[];
  commentsList: CommentItem[];
};

export type CommentItem = {
  id: string;
  author: string;
  body: string;
  createdAt: string;
  likes?: number;
};
```

### 4.2 卡片组成

每张视频卡片包含：

- 封面图
- 自动预览视频层
- 左上角徽标，例如 `HD`、`原创`、`精选`
- 右下角时长
- 标题
- 作者
- 播放量
- 收藏数
- 评论数
- 发布时间
- 点赞率或收藏按钮

### 4.3 视觉规格

```css
.video-card {
  background: var(--color-card);
  border: 1px solid var(--color-card-border);
  border-radius: 4px;
  padding: 8px;
  color: var(--color-text-invert);
}

.thumb-frame {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  background: #000;
}

.video-title {
  display: block;
  margin-top: 6px;
  font-size: 15px;
  line-height: 1.35;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.video-meta {
  margin-top: 4px;
  font-size: 12px;
  color: var(--color-muted);
}
```

## 5. 自动预览实现

### 5.1 原理

参考站的自动预览逻辑是：

1. 视频封面容器上有 `playvthumb_视频ID` 形式的 id。
2. 鼠标移入时从 id 中解析视频 ID。
3. 动态创建 `<video>`。
4. 将预览视频覆盖到封面图上。
5. 视频可以播放后淡入。
6. 鼠标移出后删除 `<video>` 和加载条。

补充说明：

- 参考站前端不是根据完整视频时长实时裁剪预览片段。
- 它在 hover 时直接请求一条独立的预览资源：`https://vthumb.killcovid2021.com/thumb/{videoId}.mp4`。
- 已抽查多个预览文件，时长均为固定 `10 秒`。
- 前端代码里没有“从第几秒开始截取”的参数，所以更像是后端预先生成好的 teaser clip。
- 仅从前端代码无法百分百确认这 `10 秒` 对应完整视频的开头、中段还是后台挑选片段。

我们实现时不建议照搬原脚本，而是用 React 状态和更稳的资源管理来做。

### 5.2 预览资源生成策略

推荐采用“独立 teaser 文件”的方式，而不是 hover 时裁剪完整视频。

资源规则：

- 每个视频生成一个独立预览文件。
- 预览文件命名为 `{videoId}.mp4` 或 `{videoId}.webm`。
- 默认预览时长 `10 秒`。
- 预览文件体积目标 `300KB-1.5MB`。
- 卡片数据只存 `previewSrc`，前端不关心它来自视频哪一段。

生成策略：

- 短视频：可以取开头后 `1-2 秒` 开始的 `10 秒`，避免黑屏和片头。
- 中长视频：可以取 `20%-35%` 位置附近的 `10 秒`。
- 很长视频：可以从多个候选片段中选画面变化较大的 `10 秒`。
- 如果后端还没有智能选段能力，第一版统一取 `min(2s, duration * 0.1)` 作为起点即可。

后端产物：

```txt
/media/videos/{videoId}.mp4
/media/thumbs/{videoId}.jpg
/media/previews/{videoId}.mp4
```

前端播放：

- hover 时只加载 `/media/previews/{videoId}.mp4`。
- 不直接加载完整视频。
- 不依赖完整视频的 `currentTime`。

### 5.3 状态设计

```ts
type PreviewState = "idle" | "intent" | "loading" | "playing" | "error";
```

状态含义：

- `idle`：默认状态，只显示封面
- `intent`：鼠标刚进入，等待 hover 延迟
- `loading`：开始加载预览视频
- `playing`：预览视频已经播放
- `error`：预览加载失败

### 5.4 交互规则

鼠标端：

- `pointerenter` 后等待 `300ms`
- 如果鼠标仍在卡片上，创建视频
- 视频 `canplay` 后淡入
- `pointerleave` 后立即停止并清理

键盘端：

- 卡片获得焦点时可以触发预览
- 卡片失焦时停止预览

移动端：

- 不使用 hover
- 点击封面后开始预览
- 再次点击或滑出视口时停止

### 5.5 React 组件伪代码

```tsx
import { useEffect, useRef, useState } from "react";

type VideoItem = {
  id: string;
  href: string;
  title: string;
  thumbnail: string;
  previewSrc: string;
  previewDuration: number;
  previewStrategy: "teaser-file" | "sprite-frames";
  duration: string;
  badges: string[];
  quality?: "SD" | "HD";
  sourceLabel?: string;
  author: string;
  views: number;
  favorites?: number;
  comments?: number;
  likes?: number;
  dislikes?: number;
  publishedAt: string;
  rating?: number;
};

type PreviewState = "idle" | "intent" | "loading" | "playing" | "error";

export function VideoCard({ video }: { video: VideoItem }) {
  const [previewState, setPreviewState] = useState<PreviewState>("idle");
  const [shouldRenderPreview, setShouldRenderPreview] = useState(false);
  const hoverTimerRef = useRef<number | null>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  function startPreviewIntent() {
    setPreviewState("intent");

    hoverTimerRef.current = window.setTimeout(() => {
      setShouldRenderPreview(true);
      setPreviewState("loading");
    }, 300);
  }

  function stopPreview() {
    if (hoverTimerRef.current) {
      window.clearTimeout(hoverTimerRef.current);
      hoverTimerRef.current = null;
    }

    const videoEl = videoRef.current;

    if (videoEl) {
      videoEl.pause();
      videoEl.removeAttribute("src");
      videoEl.load();
    }

    setShouldRenderPreview(false);
    setPreviewState("idle");
  }

  useEffect(() => {
    return () => {
      stopPreview();
    };
  }, []);

  return (
    <article
      className="video-card"
      tabIndex={0}
      onPointerEnter={startPreviewIntent}
      onPointerLeave={stopPreview}
      onFocus={startPreviewIntent}
      onBlur={stopPreview}
    >
      <div className="thumb-frame">
        <img
          className="thumb-image"
          src={video.thumbnail}
          alt=""
          loading="lazy"
        />

        {shouldRenderPreview && (
          <video
            ref={videoRef}
            className={`preview-video ${
              previewState === "playing" ? "is-visible" : ""
            }`}
            src={video.previewSrc}
            muted
            autoPlay
            loop
            playsInline
            preload="metadata"
            onCanPlay={() => setPreviewState("playing")}
            onError={() => setPreviewState("error")}
          />
        )}

        {previewState === "loading" && <span className="preview-loader" />}

        <span className="duration">{video.duration}</span>

        <div className="badge-row">
          {video.badges.map((badge) => (
            <span className="video-badge" key={badge}>
              {badge}
            </span>
          ))}
        </div>
      </div>

      <h3 className="video-title">{video.title}</h3>

      <div className="video-meta">
        <span>{video.author}</span>
        <span>{video.views.toLocaleString()} 次观看</span>
        {typeof video.favorites === "number" && (
          <span>{video.favorites.toLocaleString()} 收藏</span>
        )}
        {typeof video.comments === "number" && (
          <span>{video.comments.toLocaleString()} 评论</span>
        )}
        <span>{video.publishedAt}</span>
      </div>
    </article>
  );
}
```

### 5.6 自动预览 CSS

```css
.thumb-image,
.preview-video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.thumb-image {
  z-index: 1;
}

.preview-video {
  z-index: 2;
  opacity: 0;
  transition: opacity 180ms ease;
}

.preview-video.is-visible {
  opacity: 1;
}

.duration {
  position: absolute;
  right: 6px;
  bottom: 6px;
  z-index: 3;
  padding: 2px 5px;
  border-radius: 3px;
  background: rgba(0, 0, 0, 0.72);
  color: #fff;
  font-size: 12px;
  line-height: 1.2;
}

.badge-row {
  position: absolute;
  top: 6px;
  left: 6px;
  z-index: 3;
  display: flex;
  gap: 4px;
}

.video-badge {
  padding: 2px 5px;
  border-radius: 3px;
  background: var(--color-accent);
  color: #000;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.2;
}

.preview-loader {
  position: absolute;
  left: 0;
  bottom: 0;
  z-index: 4;
  height: 3px;
  background: var(--color-accent);
  animation: preview-progress 1.8s ease forwards;
}

@keyframes preview-progress {
  from {
    width: 0;
  }

  to {
    width: 100%;
  }
}
```

## 6. 性能策略

自动预览最容易造成性能问题，必须控制资源。

### 6.1 加载策略

- 默认只加载封面图。
- 预览视频不要提前加载完整文件。
- hover 后再设置或渲染 `video`。
- 使用 `preload="metadata"`。
- 如果我们自己生成预览片段，建议控制在 `8-10s`，默认对齐为 `10 秒`，这样更接近参考站的体感。
- 视频体积尽量控制在 `300KB-1.5MB`。

### 6.2 播放数量控制

建议在全局维护一个当前播放项：

```ts
type PreviewController = {
  activeVideoId: string | null;
  setActiveVideoId: (id: string | null) => void;
};
```

规则：

- 同一时间只允许一个卡片播放预览。
- 新卡片开始预览时，通知旧卡片停止。
- 快速移动鼠标时不重复创建多个视频。

### 6.3 视口控制

使用 `IntersectionObserver`：

- 卡片进入视口附近才允许预览。
- 离开视口后停止播放。
- 不在视口附近的卡片不挂载预览视频。

### 6.4 清理规则

鼠标离开时：

```ts
video.pause();
video.removeAttribute("src");
video.load();
```

然后再卸载 video 节点。

这样可以让浏览器释放网络和解码资源。

## 7. 响应式布局

```css
.video-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

@media (min-width: 1440px) {
  .video-grid {
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }
}

@media (max-width: 1024px) {
  .video-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .video-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .video-card {
    padding: 6px;
  }

  .video-title {
    font-size: 13px;
  }

  .video-meta span:nth-child(n + 2) {
    display: none;
  }
}

@media (max-width: 380px) {
  .video-grid {
    grid-template-columns: 1fr;
  }
}
```

## 8. 技术栈

推荐使用：

- React
- Vite
- TypeScript
- CSS Modules 或普通 CSS
- lucide-react
- 本地 mock 数据

第一版可以不接真实后端，但数据模型要按真实接口设计，避免后续接入时重写组件。

建议前端先模拟三类页面：

- 首页：热词、今日排行、24 个视频卡片。
- 列表页：分类/搜索结果、排序工具条、36-48 个视频卡片。
- 详情页：播放器壳、操作区、信息区、评论占位、右侧推荐列。

## 9. 推荐目录结构

```txt
src/
  pages/
    HomePage.tsx
    ListingPage.tsx
    VideoDetailPage.tsx
  components/
    AppShell.tsx
    TopBar.tsx
    MainNav.tsx
    SubNav.tsx
    PromoStrip.tsx
    SearchPanel.tsx
    TagCloud.tsx
    SectionHeader.tsx
    SortToolbar.tsx
    VideoGrid.tsx
    VideoCard.tsx
    PreviewVideo.tsx
    VideoPlayer.tsx
    VideoActions.tsx
    VideoInfoPanel.tsx
    RecommendedRail.tsx
    CommentPanel.tsx
    Pagination.tsx
    BackToTop.tsx
    Footer.tsx
  data/
    videos.ts
    tags.ts
    categories.ts
  styles/
    tokens.css
    base.css
    layout.css
    navigation.css
    search.css
    video-card.css
    video-detail.css
  App.tsx
  main.tsx
```

## 10. 组件职责

### AppShell

负责页面整体布局。

包含：

- TopBar
- MainNav
- SubNav
- 主内容区域
- Footer

### SearchPanel

负责搜索表单状态。

功能：

- 输入关键词
- 选择搜索类型
- 点击搜索
- 回车搜索

第一版可以只在本地过滤 mock 数据。

### TagCloud

负责热门标签。

功能：

- 展示标签
- 点击标签后触发搜索或过滤
- 移动端横向滚动

### VideoGrid

负责接收视频数组并渲染卡片。

功能：

- 响应式网格
- 空状态
- 加载状态
- 首页、列表页、详情页侧栏都复用

### VideoCard

负责单个视频卡片。

功能：

- 展示封面、标题、元信息
- hover 自动预览
- 资源清理
- 键盘 focus 预览
- 移动端点击预览
- 元信息紧凑展示
- 徽标、时长、质量标签叠加

### ListingPage

负责分类页、排行页、搜索结果页。

功能：

- 读取当前分类或搜索参数
- 展示排序工具条
- 展示高密度视频网格
- 管理分页状态

### VideoDetailPage

负责视频详情页。

功能：

- 组织 `8/4` 双列布局
- 展示橙色标题条
- 展示 `16:9` 播放器壳
- 展示视频统计和操作按钮
- 展示作者、描述、嵌入链接等视频信息
- 展示评论占位和分页容器
- 展示右侧推荐列
- 移动端改为单列布局

### VideoPlayer

负责详情页播放器区域。

功能：

- 保持 `16:9` 容器比例
- 展示 poster
- 支持本地 mock 播放源
- 保留播放、暂停、进度、音量、全屏控件
- 预留前贴片或推荐插槽，但第一版不接广告

### VideoActions

负责详情页播放器下方的统计和动作区。

功能：

- 展示时长、观看量、评论数、收藏数、热度/积分
- 展示点赞和点踩按钮
- 展示收藏按钮
- 展示写评论按钮
- 展示下载/权限提示
- 第一版只做本地状态，不发真实请求

### VideoInfoPanel

负责视频信息区。

功能：

- 展示发布时间
- 展示作者和作者徽标
- 展示关注按钮
- 展示作者数据
- 展示描述折叠/展开
- 展示嵌入链接只读文本框

### RecommendedRail

负责详情页右侧推荐列。

功能：

- 展示推荐列标题
- 展示可选推荐横幅位
- 复用紧凑版 `VideoCard`
- 桌面端作为右侧栏
- 移动端下沉到主内容下方

### PreviewVideo

负责 hover 预览的底层播放节点。

功能：

- 只在需要时挂载 `<video>`
- `canplay` 后通知卡片切换状态
- 离开时暂停、清空 `src` 并卸载
- 支持统一的全局播放锁

## 11. 第一版功能范围

第一版完成：

- 首页 UI
- 列表页 UI
- 详情页静态骨架
- 顶部工具条
- 主导航
- 二级菜单
- 横幅推荐区
- 搜索框
- 热门标签
- 内容模块标题
- 视频网格
- 视频卡片
- hover 自动预览
- 排序工具条
- 分页样式
- 播放器外壳
- 视频详情标题条
- 视频统计和操作区
- 视频信息面板
- 嵌入链接框
- 评论占位区
- 详情页右侧推荐列
- 返回顶部按钮
- 移动端适配

第一版暂不做：

- 真实登录
- 真实注册
- 真实上传
- 会员支付
- 评论
- 私信
- 后台管理
- 真实视频播放鉴权
- 真实评论提交
- 真实接口

## 12. 后续扩展

第二阶段：

- 真实播放器接入
- 详情页真实数据
- 评论区真实分页
- 收藏功能
- 点赞/点踩接口

第三阶段：

- 接入真实 API
- 用户系统
- 上传流程
- 搜索分页
- 标签页

第四阶段：

- 播放历史
- 个性化推荐
- 创作者主页
- 性能监控
- 图片和视频 CDN 优化

## 13. 验收标准

页面验收：

- 首屏能看到导航、搜索和视频内容。
- 桌面端视频网格至少 4 列。
- 列表页能显示高密度目录、排序工具条和分页。
- 详情页能显示播放器壳、操作区、信息区和右侧推荐列。
- 详情页播放器保持 `16:9`，不会因标题、统计或广告位导致布局跳动。
- 详情页标题条、信息面板标题、评论标题使用一致的橙色分区样式。
- 详情页统计区能展示时长、观看量、评论数、收藏数和热度/积分。
- 详情页操作区有点赞、点踩、收藏、写评论等可点击状态。
- 详情页作者信息、描述折叠、嵌入链接和评论占位都能在移动端正常堆叠。
- 详情页右侧推荐列在桌面端保留 hover 预览，在移动端下沉显示。
- 视频卡片标题单行省略，不撑破布局。
- 鼠标悬停卡片后可以自动播放静音预览。
- 鼠标移开后预览停止并释放资源。
- 移动端布局不溢出。
- 搜索和标签有可见交互反馈。
- 页面没有成人内容依赖，使用合规占位素材。

性能验收：

- 默认不加载所有预览视频。
- hover 才加载当前卡片预览。
- 同一时间不播放多个预览视频。
- 快速划过多个卡片不会堆积 video 节点。
- 页面滚动保持流畅。

## 14. 实现备注

本节记录第一版代码实现时相对本 plan 的偏离点和补充决定，作为后续迭代的参考。所有条目都列出了当前实现、偏离原因和回归 plan 的方法。

### 14.1 偏离 plan 的实现决定

#### 14.1.1 移动端预览交互改为直接跳转详情

- plan 5.4 节写：移动端点击封面开始预览，再次点击停止或滑出视口停止。
- 当前实现：移动端不走 hover 预览，点击卡片直接跳转详情页。
- 原因：真实设备上"先预览再跳转"容易误判，用户习惯点一下就进详情。单击预览还会让卡片产生两种点击语义，交互学习成本高。
- 代码位置：`src/components/VideoCard.tsx` 使用 `onPointerEnter` 触发 hover，触屏设备不会稳定触发，自然回退为纯跳转。
- 回归方法：在 `VideoCard` 的点击处理里检测 `pointerType === 'touch'`，首次点击 `preventDefault` 并启动预览，浮层加"播放"按钮跳转。

#### 14.1.2 详情页移动端推荐列放在评论之后

- plan 3.9 节写：移动端建议把推荐列放在评论区前，提高回流。
- 当前实现：推荐列在 CSS grid 第二列，移动端单列堆叠时自然出现在评论后。
- 原因：当前使用 `grid-template-columns: 2fr 1fr` 的简单布局，DOM 顺序即视觉顺序。移到评论前需要用 `order` 重排或拆分 DOM。
- 代码位置：`src/pages/VideoDetailPage.tsx`，`src/styles/video-detail.css` 中 `.detail-layout` 的媒体查询。
- 回归方法：移动端断点下给 `.detail-side { order: -1 }`，或按窗口宽度在 JSX 里改子元素顺序。

#### 14.1.3 卡片边框色调亮

- plan 2 节写：`--color-card-border: #030303`。
- 当前实现：`--color-card-border: #2a2a2a`。
- 原因：`#030303` 和卡片背景 `#151515` 对比度太低，边框几乎不可见，卡片之间缺少分界。
- 代码位置：`src/styles/tokens.css`。
- 回归方法：改回 `#030303`，或者把边框换成更明显的阴影。

### 14.2 plan 未覆盖、实现时补充的决定

#### 14.2.1 引入 React Router

- plan 8 节技术栈未提路由。
- 当前实现：`react-router-dom` v6，`BrowserRouter` 包在 `main.tsx`。
- 路由表：
  - `/` → `HomePage`
  - `/list` → `ListingPage`（查询参数 `q` / `tag` / `cat`）
  - `/video/:id` → `VideoDetailPage`
- 代码位置：`src/App.tsx`、`src/main.tsx`。

#### 14.2.2 全局预览锁用模块级 store + useSyncExternalStore

- plan 6.2 节只给了 `PreviewController` 类型，未指定实现。
- 当前实现：模块级 singleton，`subscribe` 订阅，`useSyncExternalStore` 接入 React。
- 原因：Context 放 `activeVideoId` 会触发全树重渲染；模块级 store 只让新旧两张 active 卡片重渲染。
- 代码位置：`src/lib/previewController.ts`。

#### 14.2.3 IntersectionObserver 全局共享实例

- plan 6.3 节只说"使用 IntersectionObserver"，未指定粒度。
- 当前实现：单例 observer + `WeakMap` 映射 element 到回调。`rootMargin: '200px 0px'` 让靠近视口的卡片也允许挂载预览。
- 原因：列表页 24-48 张卡片各自 `new IntersectionObserver` 是重复开销。
- 代码位置：`src/lib/useInViewport.ts`。

#### 14.2.4 嵌入代码增加"复制"按钮

- plan 详情页说明写：只读 textarea，点击可全选。
- 当前实现：textarea 仍保留点击全选，同时加了"复制"按钮，用 `navigator.clipboard.writeText` 并以 `document.execCommand('copy')` 作为 fallback。复制后 1.6 秒内按钮文案切换为"已复制"。
- 原因：现代 UI 习惯是一键复制，textarea 点击全选再 `Ctrl+C` 的操作链条偏长。
- 代码位置：`src/components/VideoInfoPanel.tsx`。

#### 14.2.5 数据层用 Promise 模拟异步

- plan 8 节只说"本地 mock 数据"，未定义同步或异步。
- 当前实现：`src/data/videos.ts` 同时导出同步版（`getHomeVideos` 等）和异步版（`fetchHomeVideos` 等，带 120ms `setTimeout`）。页面实际使用异步版。
- 原因：接真实 API 时只需替换 `fetchXxx` 实现，组件的 `useEffect + setLoading` 模式不用改。

#### 14.2.6 Loading / Empty 状态规格

- plan 10 节 `VideoGrid` 职责提到空状态和加载状态，未给具体样式。
- 当前实现：
  - Loading 用 `.skeleton-card` 骨架屏，灰色 shimmer 动画。`skeletonCount` 默认 8，首页/列表页传 12。
  - Empty 用 `.video-grid-empty` 居中文字，文本通过 `emptyText` prop 覆盖。
  - Error 第一版仅覆盖预览失败，用 `.preview-error` 覆盖层显示"预览加载失败"。
- 代码位置：`src/components/VideoGrid.tsx`、`src/styles/video-card.css`。

#### 14.2.7 分页组件展示规则

- plan 未定义分页展示规则。
- 当前实现：总页数 ≤ 7 全显示；> 7 显示 `1 ... 当前-1 当前 当前+1 ... 末页`。每页 `PAGE_SIZE = 24`，和首页卡片数对齐。
- 代码位置：`src/components/Pagination.tsx`、`src/pages/ListingPage.tsx`。

#### 14.2.8 详情页双列用 CSS Grid

- plan 说主列占 `8/12`，侧栏占 `4/12`，但未定 CSS 实现。
- 当前实现：`grid-template-columns: 2fr 1fr`（≈ `67/33`），容器最大宽度 `1200px`，和 plan 数字对齐。移动端断点 `900px` 切单列。
- 代码位置：`src/styles/video-detail.css`。

#### 14.2.9 扩充了 CSS token 集合

- plan 2 节只定义了颜色 token。
- 当前实现补充了：
  - 间距：`--space-1` 到 `--space-8`
  - 圆角：`--radius-sm` / `--radius-md` / `--radius-lg`
  - 容器：`--container-max: 1200px`
  - 阴影：`--shadow-card` / `--shadow-elevated`
  - 颜色：`--color-muted-light` / `--color-accent-dark` / `--color-danger` / `--color-section`
- 代码位置：`src/styles/tokens.css`。
- 原因：实现复杂布局需要统一的间距和圆角尺度。

#### 14.2.10 published_at 一律用入库时刻，不取网盘 mtime（2026-05-23）

- plan 未明确 `published_at` 的取值规则。
- 早期实现：通用 scanner 取 `e.ModTime`（115/夸克/PikPak/沃盘/OneDrive 都是网盘端文件 mtime），spider91 用爬取时刻。结果：早期上传到网盘的视频 mtime 很老，扫到后被 spider91 当下时刻的视频压在首页"最新视频"之外，体验上"凌晨明明扫到了 115 新视频却看不到"。
- 当前实现：所有 driver（含 spider91 / 通用 scanner / 手动上传）`PublishedAt = now`，等于 `CreatedAt`。`scanner.go` 不再读 `e.ModTime`，相应的 `orDefault` 工具函数也删掉。
- 历史数据保持不变（不回填）；此后扫到的新视频按新规则。
- 代码位置：`backend/internal/scanner/scanner.go`。

### 14.3 mock 数据的临时代用

以下内容仅在 mock 阶段成立，接真实后端时需要一并替换。

#### 14.3.1 预览视频复用完整视频

- plan 5.2 节强调预览应为独立的 10 秒 teaser 文件。
- 当前 mock：`previewSrc === videoSrc`，都指向 Google 公开演示视频（`commondatastorage.googleapis.com/gtv-videos-bucket`）。
- 影响：只影响 mock 数据，组件按"只加载预览 URL"工作，后端生成好独立 teaser 后，只改 `data/videos.ts` 中 `previewSrc` 即可。

#### 14.3.2 "今日排行"和"最新视频"使用同一批数据

- 当前 mock：首页两个 section 用同一个 24 条数组，第二个 section 做了 `slice().reverse()`。
- 替换方案：真实接口会是两个 endpoint，换 `fetchXxx` 即可。

#### 14.3.3 封面图用 picsum.photos

- 当前 mock：`thumbnail: https://picsum.photos/seed/{seed}/480/270`，每个视频一张基于 seed 的稳定图片。
- 特点：和视频内容无关，但 seed 稳定不会每次刷新变。接真实 CDN 时一起替换。

### 14.4 待拍板的开放决定

影响度由高到低：

1. 详情页移动端推荐列位置（目前评论后，plan 要求评论前）。
2. 移动端单击预览行为（目前直接跳转，plan 要求先预览再跳转）。
3. 嵌入框复制按钮（目前保留，可回退为纯 textarea 全选）。
4. 卡片边框色（目前 `#2a2a2a`，可回退为 plan 原值 `#030303`）。

任何一项都可以在小改动内回归 plan 原设计，等统一确认后再动。

### 14.5 视频详情/播放页视觉重写（2026-05-21）

第一版详情页视觉过于"列表化"，标题 + 一行 `·` 分隔 meta + 上下灰线工具条 + 两张分离的简介/标签卡，缺少视觉重心和氛围感。本次按"沉浸感 + 信息层级清晰"的方向重写，仅改 UI，不动数据流和后端接口。

变化点：

- **Hero ambient 背景**：详情页根加 `.vd-ambient` 层，把当前视频海报作为模糊底色（桌面 48 px blur，opacity 0.42，高度 520 px；平板/手机依次降到 36/28 px blur 和 380/280 px 高度），叠加暖橙径向光晕和"过渡到页面色"的纵向渐变，仅顶部一段，不染整页。`.vd-page` 设为 `isolation: isolate` 形成新的 stacking context。
- **播放器外光环**：播放器外加 `.vd-player-wrap`，1 px 暖橙渐变描边 + 顶部柔光 + 24/72 重阴影；移动端去除外圈和圆角，让播放器顶到容器边缘。
- **标题升级**：从 `font-2xl` 升到 `font-3xl`（手机端依次降到 xl/lg），加文字阴影提升在 ambient 上的可读性。
- **作者 + meta 重组**：删除原 `·` 分隔列表。新增 `.vd-author`（首字大写圆形渐变头像 + 名字胶囊）和 `.vd-meta__chip` 列表（来源、画质、时长、观看数、发布时间）。每个胶囊有自己的 `data-tone`：`accent` 用于 HD；网盘 tone（`quark / p115 / pikpak / wopan / onedrive`）走对应品牌色。
- **操作工具条**：从"上下灰线"改为整体浮起的玻璃卡（毛玻璃 `backdrop-filter: blur(12px)` + 1 px 描边 + 阴影）。点赞 + 点踩组合胶囊在 hover/focus 时露出 accent 描边和 `accent-softer` 光环；"不再显示" 默认透明、hover 才露出 danger 红。点赞 burst 动画时长 280→320 ms，scale 1.18→1.20。
- **简介 + 标签合并卡**：原本两张分离卡合并成一张 `.vd-info` 大卡，子区用 `.vd-info__section-head` 小标题区分（"简介" / `#` 图标 + "标签"），中间细分隔线。整卡 hover border 变亮。标签编辑按钮图标从 `+` 改为铅笔 `Pencil`。标签编辑器改为卡内行内弹层。
- **推荐栏头部**：从单 div 升级为 header：左侧 4×28 渐变发光竖条 + 标题 + 副标题（"根据当前视频 · N 条"）。item hover 时缩略图微缩放 1.04 + 整 item 上浮 1 px。
- **响应式**：>=1024 双列；<=1024 折单列；<=768 平板触控目标 ≥44 px、meta 胶囊缩到 22 px 高、播放器顶到容器边缘；<=480 手机标题 3 行、操作栏点赞/点踩平分主行、"不再显示"折成 44×44 纯图标按钮（保留 `aria-label`）。

代码位置：

- `src/pages/VideoDetailPage.tsx`：外层结构从 `container page-section` 改为 `vd-page > vd-ambient + vd-page__inner`；删除 `.vd-toolbar` 包裹层（`VideoActions` 自身是工具条）。
- `src/components/VideoMetaHeader.tsx`：标题 + `.vd-header__row`（作者 + meta 胶囊）。
- `src/components/VideoActions.tsx`：图标 16→18 px；外层加 `role=toolbar`；类名拼接改为模板字符串。
- `src/components/VideoInfoPanel.tsx`：简介与标签合并到 `.vd-info`；section head 模式。
- `src/components/RecommendedRail.tsx`：仅改头部 JSX，预览 hooks（`previewController` / `previewIntent` / `useInViewport` / `PreviewVideo`）保持不变。
- `src/styles/video-detail.css`：全面重写。

不变项：

- 所有数据请求 (`fetchVideoDetail` / `fetchTags` / `recordView` / `updateVideoTags` / `hideVideo`) 和 `like` API 调用、点踩本地 state 都未改动。
- 不引入新依赖，颜色全部走 `tokens.css`，未使用 `!important`。
- `lint` (`tsc --noEmit`) 和 `build` (`tsc -b && vite build`) 均通过。

### 14.6 双主题系统（2026-05-21）

新增管理后台"外观"页，全站可在两套主题间切换：

- **暗黑 + 暖橙（dark，默认）**：原有的视觉系统
- **奶油白 + 樱花粉（pink）**：奶油白 `#fff5f7` 页底 + 樱花粉 `#ff5b8a` 主色 + 深咖紫 `#2a1820` 文本 + 粉色柔投影

切换语义为"全站统一"——管理员选什么，所有访客看到什么。前台普通用户看不到切换控件。

#### 实现方式

**1. CSS 变量分组**
- `src/styles/tokens.css` 重写：把 `:root` 按主题相关 / 无关拆分。
- 间距 / 圆角 / 字号 / 字重 / 行高 / 容器 / 过渡 / 层级挂在 `:root`，两套主题共享。
- 暗色色 token 挂在 `:root, :root[data-theme="dark"]`（既作为默认值，又作为显式 dark 兜底）。
- 粉白色 token 挂在 `:root[data-theme="pink"]`，所有 key 与暗色对齐，组件 CSS 不动。
- 粉白下额外覆盖 `body::before` 的暖光晕和滚动条颜色（`base.css` 那部分用了硬编码白色透明）。

切换不重新载样式表，只换 `<html data-theme>` 属性，浏览器原生重算 CSS 变量，性能可忽略。

**2. 防首屏闪烁**
`index.html` `<head>` 加一段同步 inline script：

```js
var t = localStorage.getItem("video-site:theme");
document.documentElement.setAttribute(
  "data-theme",
  t === "pink" || t === "dark" ? t : "dark"
);
```

样式表加载之前 `data-theme` 就已写到 `<html>`，避免"先黑后粉"的视觉跳变。

**3. 服务端权威同步**
- 后端在 SQLite `settings` 表里以 key=`ui.theme` 存当前主题。`backend/cmd/server/main.go` 加 `App.Theme()` / `App.SetTheme()` / `App.loadTheme()`。
- 公开端点 `GET /api/settings/theme` 在 `RegisterRoutes` 鉴权组之外（登录页本身要正确显示主题），只暴露 `theme` 一个字段。
- `src/main.tsx` 在 `ReactDOM.createRoot` 之前并行 fire `syncThemeFromServer()`，把服务端值覆盖本地（不 await）。

**4. 后台切换**
- `src/admin/ThemePage.tsx`：两张大主题预览卡，每张带迷你"页面骨架"（顶部色条 + 黑底播放器三角 + 文字行 + chips）。
- 卡内预览色用 `data-preview="dark|pink"` 强制锁定，不跟随当前主题，让用户能看到未选中主题的样子。
- 点击切换：先 `applyTheme()` 本地立即生效（写 `<html data-theme>` + `localStorage`），再 PUT `/admin/api/settings`；失败回滚。
- 复用现有 `Settings` DTO（`previewEnabled` + `theme`），后端 `handlePutSettings` 中对 `theme==""` 时不写 DB（局部更新友好）。
- `App.tsx` 路由加 `/admin/theme`；`AdminLayout.tsx` 侧栏加 Palette 图标 "外观" 菜单项。
- `src/admin/PreviewToggle.tsx` toggle 时显式带上当前 `theme` 一起 PUT，保险。

**5. 关键文件**
- 后端：`backend/internal/api/admin.go`、`backend/internal/api/api.go`、`backend/cmd/server/main.go`
- 前端：`index.html`、`src/main.tsx`、`src/lib/theme.ts`、`src/styles/tokens.css`、`src/styles/admin.css`、`src/admin/api.ts`、`src/admin/PreviewToggle.tsx`、`src/admin/ThemePage.tsx`、`src/admin/AdminLayout.tsx`、`src/App.tsx`

#### 已知限制

- 组件 CSS 里有 ~45 处硬编码的 `rgba(255, 255, 255, N)`（暗色下做提亮 hover 边框/底色）。这些在粉白下会变成几乎不可见的"白叠白"，导致 hover 反馈稍弱，但不破坏布局。后续若要彻底干净，可以批量替换成 `color-mix(in srgb, var(--text-strong) Nx100%, transparent)`，让两套主题各自反向叠加。当前优先级低。
- 配色已在主要页面（首页 / 列表 / 视频详情 / 管理后台）核验过整体不翻车；如有具体页面在粉白下视觉异常，单独修补即可。

#### 验证

- `gofmt`：本次改的 3 个 go 文件干净
- `go test ./... -count=1`：全部 PASS
- `npm run lint`：干净
- `npm run build`：CSS 80.48 kB / JS 246.33 kB / index.html 1.44 kB（含 inline theme 同步 script）

## 15. 后端集成方案（网盘驱动 + 元数据 + 预览生成）

本节记录接入真实网盘后端的架构和关键决策。

### 15.1 架构

```
VideoProject/
├─ src/                        React 前端
├─ backend/                    Go 单体服务
│  ├─ cmd/server/main.go
│  ├─ internal/
│  │  ├─ drives/               Drive 接口 + 多家实现
│  │  │  ├─ iface.go           List / Stat / StreamURL / RefreshAuth
│  │  │  ├─ quark/             自己实现（参考 OpenList quark_uc）
│  │  │  ├─ p115/              壳 + SheltonZhu/115driver
│  │  │  ├─ pikpak/            自己实现（参考 OpenList pikpak）
│  │  │  └─ wopan/             壳 + OpenListTeam/wopan-sdk-go
│  │  ├─ catalog/              SQLite + VideoItem 增删改查
│  │  ├─ scanner/              扫目录 → 落库 + 异步抽 teaser
│  │  ├─ preview/              ffmpeg 抽 10s teaser
│  │  ├─ proxy/                /p/<drive>/<id> 代理下载，注入 UA/Referer/Cookie
│  │  ├─ auth/                 管理后台鉴权
│  │  └─ api/                  REST 路由
│  ├─ admin/                   管理后台静态页（登录、盘管理、视频录入）
│  ├─ config.yaml
│  └─ go.mod
├─ 115driver-1.3.2/            SDK 本地镜像（go.mod replace 引用）
└─ wopan-sdk-go-0.2.0/         SDK 本地镜像（go.mod replace 引用）
```

### 15.2 技术选型

- **后端语言**：Go 1.23。一个二进制、交叉编译到 Linux 简单。
- **数据库**：SQLite（`modernc.org/sqlite` 纯 Go 驱动，无需 CGO，便于交叉编译）。
- **HTTP 框架**：标准库 `net/http` + `gorilla/mux` 或 `chi`。
- **SDK**：
  - 夸克：移植 OpenList `drivers/quark_uc` 的 HTTP 逻辑（纯 Cookie + resty）。
  - 115：`github.com/SheltonZhu/115driver`，通过 `replace` 指令指向 `../115driver-1.3.2`。
  - PikPak：移植 OpenList `drivers/pikpak` 的 HTTP 逻辑（用户名密码 / refresh_token + captcha_token + resty）；支持扫描和播放，teaser/封面生成产物只写本地。
  - 沃盘：`github.com/OpenListTeam/wopan-sdk-go`，`replace` 指向 `../wopan-sdk-go-0.2.0`。
- **视频处理**：ffmpeg / ffprobe，作为外部子进程调用。
- **部署**：本地 Windows 开发，最终部署到 Linux 服务器（二进制 + systemd + nginx 反代）。

### 15.3 关键决策（已拍板）

| 项 | 决定 |
|---|---|
| 登录方式 | **B**：管理后台做完整登录流程。115 扫码、夸克扫码或 Cookie 导入、沃盘手机号 + 短信验证。Token 持久化到 SQLite 并自动刷新。 |
| 元数据来源 | **默认文件名解析**：`标题.mp4`、`标题 - 作者.mp4`，或带前缀的 `[前缀] 标题 - 作者.mp4`；前缀只用于标题清理，不作为任意标签列表入库。标签来自系统 / 用户标签匹配和目录合集规则；同时提供后台录入 API 覆盖字段 |
| Hover teaser | **C 预生成**：scanner 发现新视频时异步生成 10s teaser 并存回网盘的 `previews/` 目录，详情页和列表页 hover 都秒开 |
| 部署目标 | Linux 服务器；本地 Windows 开发 |
| 扫描策略 | 启动时全量 + 每 6 小时增量 + 支持手动触发 |

### 15.4 Drive 接口

```go
// internal/drives/iface.go
type Drive interface {
    Name() string                     // "quark" / "p115" / "wopan"
    Init(ctx context.Context) error

    List(ctx context.Context, dirID string) ([]Entry, error)
    Stat(ctx context.Context, fileID string) (*Entry, error)

    // 返回一次性直链 + 必要的请求头。proxy 层据此回源。
    StreamURL(ctx context.Context, fileID string) (*StreamLink, error)

    // 上传用于 scanner 写回 teaser 文件
    Upload(ctx context.Context, parentID, name string, r io.Reader, size int64) (string, error)
}

type Entry struct {
    ID       string
    Name     string
    Size     int64
    IsDir    bool
    ParentID string
    MimeType string
    ModTime  time.Time
}

type StreamLink struct {
    URL     string
    Headers http.Header   // UA/Referer/Cookie
    Expires time.Time
}
```

三家实现都收敛到这套接口，上层不区分盘。

### 15.5 文件名解析规则

默认解析顺序（取第一个匹配），用于提取 `title` / `author`：

1. 带前缀和作者：`[前缀] 标题 - 作者.ext`
2. 带前缀：`[前缀] 标题.ext`
3. 带作者：`标题 - 作者.ext`
4. 最简单：`标题.ext`

开头的 `[前缀]` 只会从标题里剥离，不会按 `,` / `，` / `、` / 空格拆成任意标签入库。`tags[]` 由 scanner 另行生成：文件名、作者和目录名命中系统标签或已有标签的标签名 / 别名时自动打标；符合条件的目录名会创建 `collection` 合集标签；常见番号类文本会归并为 `AV`。当前内置系统标签是 `后入`、`奶子`、`口交`、`臀`、`人妻`、`女大`、`AV`。其余字段（`duration` / `views` / `favorites` 等）由 scanner 读取文件元数据或置默认值。

后台录入接口可用来覆盖解析结果：

```
POST /admin/api/videos/:id     # 更新元数据
PUT  /admin/api/videos         # 新建视频（跳过扫描）
```

### 15.6 Teaser 生成流程

scanner 每次发现新视频（catalog 里没有的 fileID）时：

1. 向对应 Drive 要一次性直链 `StreamURL`
2. 启动 ffmpeg 子进程：
   ```
   ffmpeg -ss 10 -i "<直链>" -t 10 -an -vf "scale=480:-2" -c:v libx264 -preset veryfast -crf 28 -movflags +faststart -y <tmp>.mp4
   ```
   - `-ss 10`：跳过片头
   - `-t 10`：固定 10 秒
   - `-an`：去音轨
   - `scale=480:-2`：目标宽 480，缩减体积到 300KB-1.5MB
   - `-movflags +faststart`：让 moov atom 在文件头部，支持边下边播
3. ffmpeg 需要带上 Drive 提供的 UA/Referer/Cookie（用 `-headers` 参数传递）
4. teaser 写入本地 `data/previews/<videoID>.mp4`
5. catalog 记录 `preview_local`，详情页/卡片返回 `previewSrc` 指向 `/p/preview/<videoID>`；旧版 `preview_file_id` 字段保留但不再用于读取

失败重试 3 次，间隔指数退避。失败的记录标记 `preview_status = failed`，不再自动重试，需要后台手动重扫。

### 15.7 直链代理

网盘直链不能直接喂给 `<video>`：

- 夸克：校验 `User-Agent` 为 `quark-cloud-drive`
- 115：IP + UA 绑定 + 30 分钟过期
- 沃盘：有效期短

代理路由：

```
GET /p/<drive>/<fileID>
```

backend 做的事：

1. 通过 `fileID` 查 catalog，确认授权（管理后台的视频才能被代理）
2. 向 Drive 要一次性 `StreamURL`（带缓存，30 秒 TTL，避免高频 hover 打爆网盘 API）
3. 反向代理到真实直链，透传 `Range` 请求头
4. 设置合理的响应头：`Accept-Ranges: bytes`、`Content-Type`、`Cache-Control: private, max-age=300`

### 15.8 REST API

前台（无需鉴权）：

```
GET  /api/home                     # 首页视频
GET  /api/list?q=&tag=&cat=&sort=&page=&size=
GET  /api/video/:id                # 详情 + relatedVideos
GET  /api/tags                     # 热门标签
```

管理后台（需 Cookie/Token 鉴权）：

```
POST /admin/api/login              # 管理员账号密码
POST /admin/api/logout

POST /admin/api/drives             # 新建盘
GET  /admin/api/drives
POST /admin/api/drives/:id/login   # 触发登录流程
GET  /admin/api/drives/:id/login/status
POST /admin/api/drives/:id/rescan

GET  /admin/api/videos
POST /admin/api/videos             # 手动新建
PUT  /admin/api/videos/:id         # 修改元数据
DELETE /admin/api/videos/:id
POST /admin/api/videos/:id/regen-preview

GET  /admin/api/tags               # 标签列表
POST /admin/api/tags               # 新增标签并自动归类历史视频
DELETE /admin/api/tags/:id         # 删除非系统标签，并从所有视频上移除
```

登录流程三家各不相同：

- **115 扫码**：`POST /admin/api/drives/:id/login` 返回二维码图片；前端轮询 `.../login/status` 直到成功
- **夸克**：最稳是让用户在电脑浏览器登录 pan.quark.cn 后 F12 复制 Cookie，后台粘贴保存。可选：实现扫码登录（OpenList 社区有方案）
- **PikPak**：参考 OpenList，后台粘贴 username/password 或 refresh_token；遇到 captcha URL 时手动验证后回填 captcha_token
- **沃盘**：手机号 → 后端请求短信 → 前端填验证码 → 登录

### 15.9 前端改动

仅改 `src/data/videos.ts`：把 `fetchXxx` 实现换成 `fetch('/api/...')`，保持签名不变。组件代码一行不改。

Vite dev server 加 proxy：

```ts
// vite.config.ts
server: {
  port: 5173,
  proxy: {
    '/api': 'http://localhost:8080',
    '/p':   'http://localhost:8080',
    '/admin': 'http://localhost:8080',
  },
}
```

生产部署用 nginx 把 `/`、`/api`、`/p`、`/admin` 都反代到 backend 或前端 dist 目录。

### 15.10 部署

Linux 服务器：

1. `go build -o video-server ./cmd/server` 交叉编译
2. 上传到服务器 `/opt/video-site/`
3. ffmpeg：`apt install ffmpeg`
4. systemd 单元：
   ```
   [Service]
   WorkingDirectory=/opt/video-site
   ExecStart=/opt/video-site/video-server
   Restart=always
   ```
5. nginx 反代 + 静态文件服务

本地开发同时跑：
- `npm run dev`（前端 5173）
- `go run ./backend/cmd/server`（后端 8080）

### 15.11 风险和待确认

- **三家协议变动风险**：协议是逆向出来的，网盘方改就得跟着改。SDK 社区更新到了就 `go get` 新版本。
- **网盘风控**：扫描频率太高、直链请求太密集可能被封。scanner 默认 QPS 限制 + 单次扫描目录数量上限。
- **teaser/封面本地存储**：生成产物只写入本地 `data/previews/`，不再依赖网盘写权限；部署时需要把该目录纳入持久化和备份策略。

### 15.12 Teaser 生成策略（已落地）

Teaser 不再是"固定从第 10 秒抽 10 秒"，改为按视频时长分段挑起点 + 三段拼接：

- **段数**：`Config.Segments`，默认 3。视频 `< 30s` 自动降级为单段。
- **每段时长**：`DurationSeconds / Segments`，下限 2 秒，默认 9 / 3 = 3 秒。
- **起点策略** `pickSegmentStarts(duration, n, eachSec)`：
  - `duration < 30s` → 单段，起点 `max(2, duration*0.1)`
  - `30s ≤ duration < 10min` → 在 `[5%, 85%]` 区间均匀分布 N 段
  - `duration ≥ 10min` → 在 `[20%, 80%]` 区间均匀分布 N 段
- **拼接**：每段 `scale=480:-2` 缩放，`fade-in 0.2s` + `fade-out 0.2s`，`concat` 滤镜合成单个 mp4，`libx264 crf 28 preset veryfast`，体积 500 KB - 1.5 MB。

封面独立于 teaser：
- `pickThumbnailOffset(duration)`：
  - `duration < 60s` → `duration * 0.3`
  - `duration ≥ 60s` → `clamp(duration * 0.2, 5, 120)` 秒
- 抽帧单独走 `ffmpeg -frames:v 1`，和 teaser 起点解耦。
- 输出 `data/previews/thumbs/<videoID>.jpg`，前端走 `/p/thumb/<videoID>` 路由。

前端展示（`VideoCard.tsx`）：
- 播放中底部显示橙色进度条，随 `<video>.currentTime / duration` 同步。
- 右上角"预览"角标 `.preview-tag`，与"HD/徽标"区分。
- 离开卡片时进度归零。

代码位置：
- `backend/internal/preview/ffmpeg.go` `pickSegmentStarts` / `pickThumbnailOffset` / `Generate` / `GenerateThumbnail`
- `backend/internal/config/config.go` `Preview.Segments` / `FFprobePath`
- `src/components/PreviewVideo.tsx` `onTimeUpdate`
- `src/components/VideoCard.tsx` progress state + DOM
- `src/styles/video-card.css` `.preview-progress` / `.preview-tag`

取舍说明：
- 第一段不选 `duration*0.1` 之前的起点，避免片头黑屏/logo。
- 最后一段末端留 1 秒余量，避免切到文件尾部导致 ffmpeg 读越界。
- 单段 fallback 原因：拼接滤镜对 < 30s 视频性价比低，直接整段取一次性 25% 位置。
- 选段未使用场景检测（`ffmpeg scdet`）：单次扫描 3000+ 视频时成本过高，留给后续 C3 按需开关。

### 15.13 孤儿 collection 标签清理（已落地）

**背景**：`scanner.EnsureCollectionTag` 会在扫描 115 等网盘时按目录名创建 `source='collection'` 的合集标签，并把同目录下视频自动打上。`internal/scanner/scanner.go shouldExcludeDir` 规则把名为 `影视` 的整棵子树标记为 `ExcludedFileIDs`，由 `cmd/server/main.go cleanupExcludedDriveVideos` 调 `Catalog.DeleteVideo` 把对应视频删掉。原 `DeleteVideo` 只清 `videos` + `video_tags`，`tags` 表里的合集标签会变成无引用孤儿，仍出现在标签云和管理后台（`ListTags` 用 LEFT JOIN，count=0）。

**实现**：

- `Catalog.DeleteVideo` 事务里：
  1. `collectVideoTagIDs(tx, videoID)` 先记录这次视频关联的 `tag_id`。
  2. 删 `video_tags` 和 `videos`。
  3. `pruneOrphanCollectionTagsByID(tx, tagIDs)` 仅对 `source='collection'` 且无引用的 tag_id 执行 `DELETE FROM tags`；其它 source 一律保留。
- `Catalog.migrate` 末尾追加 `pruneOrphanCollectionTags`：单条 `DELETE FROM tags WHERE source='collection' AND id NOT IN (SELECT tag_id FROM video_tags)`，作为启动自愈，吃掉历史遗留孤儿。

**为什么只动 collection**：
- `system`：固定标签（`AV` 等），即使临时无视频也要保留。
- `user`：管理员手动建的，语义由人维护，孤儿状态保留，让管理员自己删。
- `auto`/`legacy`：是基于内容/迁移的旧标签，理论上有视频在引用；保守起见不在此处自动删，避免一次启动误清掉用户依赖的标签。

**代码位置**：
- `backend/internal/catalog/catalog.go` `DeleteVideo`
- `backend/internal/catalog/tags.go` `migrate` / `pruneOrphanCollectionTags` / `pruneOrphanCollectionTagsByID` / `collectVideoTagIDs`
- 测试：`backend/internal/catalog/tags_test.go` `TestDeleteVideoPrunesOrphanCollectionTag` / `TestMigratePrunesPreexistingOrphanCollectionTags`

**手动删除标签**：
- `/admin/api/tags/{id}` 支持 `DELETE`。管理员可手动删除非系统标签，删除时同步清理 `video_tags` 并刷新相关视频的 `videos.tags` JSON。
- `system` 标签由固定标签池维护，不开放删除；`user` / `collection` / `legacy` 标签可由管理员按需删除。
- 历史孤儿 `collection` 标签仍由迁移自愈逻辑自动清理。

### 14.7 取消浏览器内本机转码，全部走 302 直链 + VLC 外部播放器按钮（2026-05-21）

**背景**：之前对 `.avi` / `.mkv` 视频前端会拿到 `/p/transcode/<videoID>` 路径，详情页轮询 `/status` + `POST /start`，后端用 ffmpeg 重编码（`-c:v libx264 -c:a aac -movflags +faststart`）整个文件落盘到 `backend/data/previews/transcodes/<videoID>.mp4`，再让 `<video>` 加载。问题：

- 完整重编码代价大，2 小时影片 `veryfast` 也要数分钟，期间用户只看到"正在准备可快进版本…"。
- transcodes 目录持续累积（实测 69 MB / 几百个文件），没有清理策略。
- 多用户并发触发不同 mkv 时会同时跑多个 ffmpeg 进程，没有并发上限保护。
- mkv 容器里 90% 是 H.264+AAC，本可 remux 秒级完成，但旧实现一律重编码，浪费严重。

**决策**：直接放弃浏览器内播放 mkv/avi 的尝试，所有视频统一走 `/p/stream/<driveID>/<fileID>`（即网盘直链 302）；浏览器原生不支持的格式（avi、部分 mkv）让用户用外部播放器（默认 VLC）打开。

**改动**：

- `backend/internal/api/api.go`
  - `videoSource(v)` 不再分支：所有非本地上传视频统一返回 `/p/stream/<driveID>/<fileID>`，本地上传仍走 `/p/upload/<videoID>`。
  - 删除 `/p/transcode/{videoID}`、`/p/transcode/{videoID}/start`、`/p/transcode/{videoID}/status` 三个路由及 `handleTranscode` / `handleTranscodeStart` / `handleTranscodeStatus`。
  - 删除 `startTranscode` / `generateTranscode` / `transcodeStatus` / `setTranscoding` / `transcodePath` / `transcodeTempPath` / `needsBrowserTranscode` / `buildFFmpegHeaders` 这一组辅助。
  - `Server` 结构体去掉 `transcodeMu` / `transcodeJobs` / `FFmpegPath`（preview worker 仍走 `preview.Config.FFmpegPath`）。
  - 清理 `log` / `os/exec` / `sync` import。
- `backend/cmd/server/main.go` `removeLocalVideoAssets` 不再清理 `transcodes/` 路径（目录已删，新建无需求）。
- `src/components/VideoPlayer.tsx` 删除 transcode 检测、`/status` 轮询、"正在准备可快进版本…"提示，简化为直接用 `src` 喂 `<video>`。
- `src/components/VideoActions.tsx` 在工具条内新增"VLC 打开"按钮：链接为 `vlc://${window.location.origin}${video.videoSrc}`，所有视频都显示，不区分扩展名（mp4 用户也可选择用 VLC 打开本地播放）。
- `src/styles/video-detail.css` 给 `.vd-actions__vlc` 加 accent 色 hover，与 `.vd-actions__hide`（danger）区分；`margin-left:auto` 从 hide 移到 vlc，hide 紧随 vlc 排列。

**测试同步**：

- `backend/internal/api/api_test.go`：`TestVideoSourceUsesTranscodeForAvi` 改名 `TestVideoSourceUsesDirectStreamForAvi`，断言 avi 也走 `/p/stream/`；删除 `TestTranscodeStatusReadyWhenCachedFileExists` / `TestTranscodeStatusProcessingWhenJobActive` / `TestTranscodeTempPathKeepsMp4Extension`。
- `backend/cmd/server/main_test.go` 清理用例里的 `obsoleteTranscode` / `obsoleteTranscodeTmp` 样本和断言。

**清理已有数据**：上线时执行一次 `rm -rf backend/data/previews/transcodes/`（本次回收 69 MB）。

**用户体验权衡**：

- mp4 / webm 等浏览器原生支持的容器：体验不变，仍是 `<video>` + 302 直链 Range。
- mkv（即使内编码是 H.264）/ avi：浏览器多数情况下显示无法播放，用户点"VLC 打开"调起本机 VLC（需用户允许浏览器调起 `vlc://` 协议）。手机端 VLC 可能不支持自定义 scheme。
- 如果将来要把这部分也吃下来，可以再做"先 ffprobe 探测，能 remux 就 `-c copy`"或 HLS 边转边播。本次保留"零本机转码"的简单方案。

**代码位置**：
- `backend/internal/api/api.go`、`backend/cmd/server/main.go`、`backend/internal/api/api_test.go`、`backend/cmd/server/main_test.go`
- `src/components/VideoPlayer.tsx`、`src/components/VideoActions.tsx`、`src/styles/video-detail.css`

**补丁：VLC 按钮改为 token 签名 + 强制 proxy 转发（2026-05-22）**

最初实现是 `vlc://${origin}/p/stream/<driveID>/<fileID>` 直跳，但有两个致命问题：

1. `/p/stream/...` 在鉴权 group 里要求 `vs_admin` cookie，VLC 没 cookie 永远 401。
2. 就算去掉鉴权，p115 走的是 302 → 115 CDN，VLC 的 UA `VLC/3.x` 不一定能通过 115 风控。

新链路：

- `backend/internal/api/playtoken.go`：新增进程内 `playTokenStore`，30 分钟 TTL，绑定 `(token -> videoID + expires)`，consume 不立即作废以兼容 VLC 多次 Range / seek 重连，懒清理。
- `backend/internal/api/api.go`
  - `Server` 加 `playTokensOnce sync.Once + playTokens *playTokenStore` 懒初始化。
  - 鉴权组里挂 `POST /api/play-token/{videoID}`，已登录用户 → 签发 `{url, expiresIn}`，url 形如 `/p/play/<id>?token=<48 字符 hex>`。
  - 鉴权组外挂 `GET /p/play/{videoID}`，handler 内部用 query token 校验，命中后强制走 `Proxy.ServeStreamProxied`（不 302），本地上传分支用 `http.ServeFile`。
- `backend/internal/proxy/proxy.go` 新增 `ServeStreamProxied`：复用 `getLink` + `serve`，但跳过 `shouldRedirect` 分支，让 115 等也走本机代理（携带正确的 cookie / UA / Referer），客户端只跟我们服务器对话。
- `src/components/VideoActions.tsx` `handleOpenInVlc`：preventDefault 后异步 `POST /api/play-token/<id>`（带 credentials），拿到 `{url}` 后跳 `vlc://${origin}${url}`；按钮文案在请求中显示"生成中…"。

测试：`backend/internal/api/playtoken_test.go` 覆盖 issue/consume、跨视频拒绝、过期拒绝、空输入拒绝四条。

**后续可选**：
- 如果要做"看一次就失效"的严格模式，把 `consume` 改成 delete-on-success；要权衡 VLC seek 行为。
- 如果担心进程内 map 在重启后丢失 token，可以挪到 SQLite 一张轻表里。当前每次启动 0 token，用户重新点"VLC 打开"即可。

**追加补丁：改回 302 直链，节省服务器带宽（2026-05-22）**

参考 OpenList `server/handles/down.go` 的 `Down` → `redirect` 路径以及 `drivers/115/driver.go` 的 `Link` 实现：

```go
// drivers/115/driver.go
userAgent := args.Header.Get("User-Agent")
downloadInfo, err := d.client.DownloadWithUA(file.(*FileObj).PickCode, userAgent)
```

OpenList 之所以能给外部播放器 302 直链，是因为它把请求方的 `User-Agent` 透传到 115 SDK 签链。115 直链是 UA 绑定的——必须用调链时使用的 UA 去拉，CDN 才认。本项目已经在 `internal/drives/p115/driver.go` 实现了 `StreamURLWithHeader(ctx, fileID, header)`，并由 `proxy.getLink` 把请求 header 一路传过去，所以 302 模式天然兼容 VLC：

- VLC 用自己的 UA `VLC/3.x LibVLC/3.x` 直接 GET 我们的 `/p/play/...?token=...`
- 后端校验 token 通过后，调 `StreamURLWithHeader`，115 SDK 用 VLC 的 UA 签链
- 我们 302 给 VLC，VLC 跟 302 后用同一个 UA 拉 CDN，CDN 校验通过

**改动**：

- `backend/internal/api/api.go` `handlePlayWithToken` 把 `Proxy.ServeStreamProxied(...)` 换回 `Proxy.ServeStream(...)`，对 115 而言会触发 `shouldRedirect → http.Redirect 302`。
- `backend/internal/proxy/proxy.go` 删掉短命的 `ServeStreamProxied`，没人用；future 想再做反代仍可一行恢复。

**收益**：服务器对外只传一个 302 头（约 320 字节）就退出，所有后续字节走 115 CDN，零中转流量。

**实测**（用 `curl -A "VLC/3.0.18 LibVLC/3.0.18" -L`）：
- `/p/play` → 302 → `cdnfhnfile.115cdn.net/...`
- CDN 返回 `206 Partial Content + Content-Type video/mp4`
- 前 16 字节正是 mp4 ftyp 盒子签名 `00 00 00 14 66 74 79 70 71 74 20 20 ...`

**继承的限制**：
- 115 直链有签名时效（约几十分钟），同一签出的 URL 长视频中途可能过期，VLC 表现是 stall / "无法继续播放"；用户需要重新点"VLC 打开"再签一次。
- 如果哪天 115 风控收紧，开始拒绝某些 UA，可以再恢复 `ServeStreamProxied`，或者在 token store 里加 `force_proxy` 标记，少数视频走代理。


**最终结果：14.7 整体撤销（2026-05-22）**

实测体验差：

- vlc:// 协议在多数浏览器/系统里无显式注册，按钮像没反应；
- 即便弹浮层让用户复制 URL 自行打开 VLC，仍属"换个工具看视频"，跟"在网站上看"的体验差距大；
- 用户对此普遍不接受。

最终选择**撤销 VLC 这一整套**，并选择保留 transcode 兼容层但改为智能模式（见 14.8）。本节保留作为决策记录与坑位说明。

撤销内容：

- 后端：删 `internal/api/playtoken.go` / `playtoken_test.go`；`Server` 的 `playTokens` 字段和懒初始化；`handleIssuePlayToken` / `handlePlayWithToken`；`/api/play-token/{videoID}` 和 `/p/play/{videoID}` 两个路由；不再需要的 `sync` import。
- 前端：`VideoActions.tsx` 删 VLC 按钮 + 浮层 + 相关 state（`vlcLoading` / `vlcUrl` / `vlcCopied`）+ 4 个 handler；`video-detail.css` 删 `.vd-actions__vlc` 和 `.vd-vlc-modal` 全部样式。

### 14.8 ffprobe 智能转码：能 remux 就 remux（2026-05-22）

**背景**：14.7 撤销前曾把 mkv / avi 一律走 ffmpeg `libx264 + aac` 完整重编码，2 小时电影要 3-10 分钟。但 mkv 容器里 80-90% 装的是 H.264 + AAC，本来只换个壳（mp4）就能播。

**决策**：保留 `/p/transcode/{videoID}` 兼容层，但 generateTranscode 流程改为：

1. `drv.StreamURL` 拿网盘直链
2. `probeCodecs` 用 ffprobe 探测 video / audio codec
3. `chooseTranscodeArgs` 决定 `-c:v` / `-c:a`：
   - 视频和音频都浏览器支持 → 都 `copy`（**remux**，几十秒）
   - 仅视频支持 → `-c:v copy -c:a aac`（**audio**，1-2 分钟）
   - 视频不支持 → `-c:v libx264 -c:a aac`（**encode**，几分钟）
4. `setTranscodeMode` 把模式写回 `transcodeJobs`，前端 status 接口能读到
5. ffmpeg 写 `.tmp.mp4` → rename 成 `.mp4`

**浏览器支持白名单**（保守）：

```go
browserSupportedVideo: h264, avc1
browserSupportedAudio: aac, mp4a, mp3
```

HEVC / VP9 / AV1 当作"不支持"（Chrome / Firefox 默认不行，宁可重编也不留黑屏风险）。
ac3 / dts / flac / opus / vorbis 一律重编 aac（音频码率小，1-2 分钟搞定）。

**新增文件**：`backend/internal/api/transcode.go`
- `probedCodecs` / `probeCodecs(ctx, ffprobePath, link)` - 用 `ffprobe -of json -show_streams -select_streams v:0,a:0` 解析 JSON，传 `link.Headers` 应付 115 等盘的请求头校验。
- `chooseTranscodeArgs(probedCodecs)` - 三档策略
- `browserSupportedVideo / Audio` - 白名单
- `buildFFmpegHeaders` - 把 http.Header 序列化成 ffmpeg `-headers` 字符串

**`Server` 改动**：
- 加 `FFprobePath string` 字段（main.go 从 `cfg.Preview.FFprobePath` 注入）
- 把 `transcodeJobs` 从 `map[string]bool` 改为 `map[string]*transcodeJob`，job 带 `mode` 字段供前端轮询读取
- 增加 `transcodeMode(videoID)` / `setTranscodeMode(videoID, mode)` / `clearTranscodeJob(videoID)` helpers
- handleTranscode / Status / Start 返回 `{status, mode}` 两字段

**前端 `VideoPlayer.tsx`**：
- 轮询 `${src}/status` 现在能拿到 `mode`，文案根据 mode 区分：
  - `remux` → "正在准备播放（仅换容器，几十秒）…"
  - `audio` → "正在重新封装并转换音轨（约 1-2 分钟）…"
  - `encode` → "正在重新编码（这个视频需要完整转码，可能要几分钟）…"
  - 空 / 未知 → "正在准备播放…"
- 流转：第一次 GET /status，看到非 ready 就 POST /start 触发后台任务，每 3 秒再 GET /status 直到 ready 切 src。

**容错**：
- ffprobe 失败（找不到二进制 / 网盘临时风控 / JSON 异常）→ log 一条，fallback 到完整重编（`libx264 + aac`），保证用户最终能看到视频。
- ffmpeg 失败 → 删 .tmp.mp4，goroutine 退出，下次客户端轮询再触发。

**测试**：
- `TestVideoSourceUsesTranscodeForAvi` / `TestVideoSourceUsesTranscodeForMkv`：分别断言 avi / mkv 走 `/p/transcode/<id>`
- `TestChooseTranscodeArgs` 6 个子用例覆盖：h264+aac、h264+mp3、h264+ac3、hevc+aac、vp9+opus、空 codec
- `TestTranscodeStatusReadyWhenCachedFileExists` / `TestTranscodeStatusReportsProcessingAndMode` / `TestTranscodeTempPathKeepsMp4Extension`：覆盖 transcodeStatus + transcodeMode + setTranscodeMode + clearTranscodeJob 全链路。
- `cmd/server/main_test.go` 把之前删掉的 obsoleteTranscode / Tmp 两条样本和断言加回。

**目前没做的**（标 backlog，看后续负载是否值得做）：
- transcodes/ 缓存清理：没有总量上限，没有 LRU。当前每次 mkv/avi 视频被请求都会落一个 .mp4，长期累积。简单方案：定期任务删除 N 天没访问的 .mp4，或当目录超过阈值时按 mtime 清最旧的。
- 并发上限：现在每个不同 videoID 启一个 ffmpeg；同时 5 个 mkv 会跑 5 个 ffmpeg。简单方案：semaphore 限制并发数（比如最多 2）。
- HEVC 用户体验：仍然是几分钟等待。可考虑 HLS 边转边播。
- 进度反馈：ffmpeg 输出 `time=00:01:23.45` 可解析，前端显示 X/Y。

**代码位置**：
- `backend/internal/api/transcode.go`、`backend/internal/api/api.go`、`backend/internal/api/api_test.go`
- `backend/cmd/server/main.go`、`backend/cmd/server/main_test.go`
- `src/components/VideoPlayer.tsx`

**最终结果：14.8 整体撤销（2026-05-22）**

实测下来对 2 核小机器（1.6 GB 内存）压力过大：

- HEVC mkv 重编码时 ffmpeg 单进程吃 160% CPU（占满双核），全机 100% busy；
- 加 `-threads 1` / `nice -n 19` / 并发 1 等限制可以缓解，但本质上"在小机器上转码 4K HEVC"这件事就不合适，再省也是几分钟全负载；
- 同时占用 30% 内存，影响 preview worker、115 扫描、SQLite WAL 等正常负载。

最终决策：**所有视频一律走 `/p/stream/<driveID>/<fileID>` 302 直链，浏览器原生不支持的 mkv/avi 用户自行选择能播的播放器**。这台机器不再做任何视频转码工作。

撤销内容（与本节描述的实现完全相反）：

- 删 `backend/internal/api/transcode.go`
- `backend/internal/api/api.go`：
  - `Server` 去掉 `FFmpegPath` / `FFprobePath` / `transcodeMu` / `transcodeJobs` 字段、`transcodeJob` 类型；
  - 删 3 个 transcode 路由、`handleTranscode` / `handleTranscodeStatus` / `handleTranscodeStart` handler；
  - 删 `startTranscode` / `generateTranscode`；
  - 删 6 个 transcode helper：`transcodeStatus` / `transcodeMode` / `setTranscodeMode` / `clearTranscodeJob` / `transcodePath` / `transcodeTempPath`；
  - 删 `needsBrowserTranscode`；
  - `videoSource` 恢复纯 302（本地上传 `/p/upload/`，其它 `/p/stream/`）；
  - 清理 `log` / `os/exec` / `sync` import。
- `backend/cmd/server/main.go`：去掉 `FFmpegPath` / `FFprobePath` 注入；`removeLocalVideoAssets` 不再清理 `transcodes/` 路径。
- `src/components/VideoPlayer.tsx`：删 transcode 检测 / `/status` 轮询 / mode 文案，简化为直接用 src 喂 `<video>`。
- 测试：`api_test.go` 把 avi/mkv 用例改为断言走 `/p/stream/`，删 `TestChooseTranscodeArgs` / `TestTranscodeStatus*` / `TestTranscodeTempPath*`；`main_test.go` 去掉 transcodes 资产清理样本和断言。
- 运行时：`pkill -9` 干掉残留 ffmpeg 进程，`rm -rf backend/data/previews/transcodes/`。

**用户体验**：
- mp4 / webm：体验不变。
- mkv / avi：浏览器原生 `<video>` 多数无法播放，用户看到原生"无法播放"提示；如有需要，用户可以右键复制视频地址（302 后的 115 直链需要登录态，但这部分网页地址是 `/p/stream/<driveID>/<fileID>`，cookie 校验通过的浏览器能直接下载或在另一个具备 cookie 转发的工具里打开）。这台机器不再为 mkv/avi 做任何额外服务。

至此 14.7（VLC 方案）和 14.8（ffprobe 智能转码）两条尝试都已撤销，回到"全部 302 直链"的最简实现。


## 16. 91 爬虫源接入（spider91，已落地，2026-05-22）

把 `91VideoSpider/spider_91porn.py` 包装成一种新的 drive 类型 `spider91`，每天凌晨自动跑一次爬虫"凑够 N 个新视频"，下载视频和封面到本地，作为视频源接入现有的列表/详情/标签/teaser 流水线。

### 16.1 设计取舍

- **保留 Python，子进程调用**：原脚本里 `strencode2` 解码视频直链的逻辑直接复用更稳；代价是部署机要装 Python 3 + bs4 + lxml。后续如果觉得多语言依赖太重，可以再翻译成 Go。
- **"凑够 N 个新"语义**：早先方案是固定爬 page=1，但 91porn 本月最热 page 1 内容相对稳定，每天能爬到的新视频很少。改成 "从 page 1 起翻页，跳过已知 viewkey，凑够 target_new（默认 15）个新视频后停止"。具体做法：backend 每次启动 Python 前，把 catalog 里已入库的 viewkey 写到 `<driveDir>/.crawl/seen-<时间戳>.txt`，作为 `--seen-viewkeys-file` 参数传入；Python 内部维护 `skip_viewkeys` set，列表页解析时直接过滤，命中即跳过详情页请求。
- **viewkey 做主键**：91porn 网站对每个视频的稳定标识，列表页/详情页 URL 都能拿到。`videos.id = "spider91-<driveID>-<viewkey>"`，`videos.file_id = "<viewkey>.<ext>"`，和 localupload 的 ID/FileID 解耦风格一致。
- **视频文件后缀按 URL 真实后缀**：原本 hardcode 写 `.mp4`，但 91porn 直链的格式不固定（`.mp4` / `.flv` / 个别 `.m3u8`），盲存 `.mp4` 会让 ffmpeg 拿到错的容器结构。`detectVideoExt(url)` 解析路径扩展名，命中白名单（mp4/webm/mkv/mov/m4v/flv/avi）就用真实后缀，`.m3u8` 等流媒体清单回退 `.mp4`。`videos.ext` 字段也跟实际后缀保持一致。
- **封面直接复用网站封面**：crawler 下载完封面后复制一份到 `data/previews/thumbs/<videoID>.jpg`，让 `/p/thumb/{videoID}` 路由不需要任何特例就能命中。同时 `videos.thumbnail_url` 设为 `/p/thumb/<videoID>`、`thumbnail_status = 'ready'`，让 thumb worker 自动跳过 spider91 视频。
- **teaser 仍走 ffmpeg 流水线**：crawler 调 `OnNewVideo` 回调把新视频塞进当前 drive 的 preview worker 队列。
- **走代理下载**：91porn CDN 节点在海外，国内直连只有几 KB/s。crawler 的 `http.Client` 用 `http.ProxyFromEnvironment`（读 `HTTPS_PROXY` 环境变量），并允许在 drive credentials 里通过 `proxy` 字段显式覆盖。**这是个重要修正**：自定义 `http.Transport` 默认不带 `Proxy: http.ProxyFromEnvironment`，必须显式加，否则会忽略 `HTTPS_PROXY` 环境变量直连——这是排查中花了最多时间的坑。
- **统一 `91porn` 标签**：所有 spider91 视频在入库时打上 `91porn` 标签。`attachSpider91Crawler` 启动时调 `Catalog.CreateTagAndClassify("91porn", nil, "system")` 同时建标签 + 给已入库的视频按 author 字段补打；新视频入库时 crawler 直接设置 `Tags: []string{"91porn"}` 让 `UpsertVideo` 自动同步 `video_tags` 表。
- **管理后台 UI 适配**：spider91 不属于"网盘"，但复用 `/admin/drives` 页有意义（视频源、teaser、本地占用等列都通用）。做了几处 surgical 修补：状态列对 spider91 直接看 `status` 字段不要求凭证（"已就绪"/"错误"，不会显示"未配置凭证"）；操作按钮对 spider91 显示 "立即抓取"（图标 Download）而非"重扫"；表单隐藏"根目录 ID" / "扫描起点目录 ID"两行；"扫描根"列对 spider91 显示 "上次抓取 N 小时前"（`lastCrawlAt` 字段从 `drive.credentials.last_crawl_at` 提取）。

### 16.2 文件改动

- `91VideoSpider/spider_91porn.py`：加 `--target-new N` `--seen-viewkeys-file FILE` `--page N` `--output FILE` `--no-resume` `--quiet` 等 CLI 参数；`--target-new` 模式下从 page 1 起翻页直到累计处理 N 个新 viewkey 后停止，配合 `--seen-viewkeys-file` 把 backend 已入库的 viewkey 注入 skip set；保留无参数的全量模式作向后兼容。
- `backend/internal/drives/spider91/driver.go`：实现 `drives.Drive`，存储结构 `<rootDir>/videos/<viewkey>.<ext>` + `<rootDir>/thumbs/<viewkey>.<ext>`，`StreamURL` 返回本地路径；`safeJoin` 防越界。
- `backend/internal/drives/spider91/crawler.go`：`Crawler.RunOnce(ctx, targetNew)` 完成"查 catalog 已知 viewkey → 写 seen 列表 → 跑 python（target-new 模式）→ 解析 JSON → 顺序下载视频和封面（按真实后缀）→ 复制封面到 standard thumbs 目录 → upsert（带 `91porn` 标签）+ UpdateVideoMeta(thumbnail_status=ready) → OnNewVideo"。`detectVideoExt` / `detectThumbExt` 处理后缀；`http.Client` 通过 `http.ProxyFromEnvironment` + 可选 `ProxyURL` 走代理。
- `backend/internal/catalog/catalog.go`：新增 `ListVideoFileIDsByDrive(driveID) ([]string, error)`，轻量查询某 drive 的所有 file_id，spider91 用它构造 seen 列表。
- `backend/cmd/server/main.go`：新增 `App.spider91Crawlers` map，`attachDrive` 多一条 `case spider91.Kind`，`detachDrive` 清理；`shouldScanDrive` 排除 spider91（不参与 02:00-07:00 网盘扫描循环）；新增 `crawlerLoop` 每分钟轮询，命中 `crawl_hour` 窗口 + 距离上次成功 ≥ 12 小时就触发；`OnScanRequested` 对 spider91 走 `runSpider91Crawl` 而不是 `runScan`；`attachSpider91Crawler` 异步调 `Catalog.CreateTagAndClassify("91porn", nil, "system")` 建系统标签 + 给已入库的 spider91 视频按 author 字段补打。
- `backend/internal/api/api.go`：`(s *Server).videoSource(v)` 通过 `Proxy.Registry` 检查 drive kind，spider91 视频回放路径 `/p/stream/...` 切到 `/p/spider91/<videoID>`；新增 `handleSpider91Video` 用 `http.ServeFile` 服务本地文件；`driveKindLabel` 加 `91 爬虫`。
- `backend/internal/api/admin.go`：`handleListDrives` 响应里加 `lastCrawlAt`（从 `drive.credentials.last_crawl_at` 提取，仅 spider91 用）；`hasCredential` 不把 `last_crawl_at` 这种"运行状态字段"算成凭证，spider91 直接强制 `hasCred=true`。
- `src/admin/api.ts`、`src/admin/DrivesPage.tsx`：`Kind` 联合类型加 `spider91`；表单 select 加选项；`credentialFields("spider91")` 返回 `target_new` / `crawl_hour` / `proxy` / `python_path` / `script_path` 五个字段；`defaultRootId("spider91") = "/"`；`StatusTag` 对 spider91 跳过凭证检查显示"已就绪"；操作按钮 spider91 变 "立即抓取" + Download 图标；表单隐藏"根目录 ID" / "扫描起点目录 ID"两行；"扫描根"列对 spider91 显示 "上次抓取 N 小时前"（用 `formatRelativeTime(lastCrawlAt)`）。
- `README.md`：加"91 爬虫源"专门章节，说明部署前置条件（含代理）、字段、目录结构、触发逻辑、UI 适配、风险。

### 16.3 凭证字段（写在 drive.Credentials）

| key | 默认值 | 说明 |
|---|---|---|
| `target_new` | `15` | 每次爬取的新视频数（spider91.DefaultTargetNew） |
| `crawl_hour` | `0` | 0-23，凌晨触发的小时 |
| `proxy` | `（空）` | 下载代理 URL（如 `http://127.0.0.1:7890`）；为空时回退到 backend 进程的 `HTTPS_PROXY` 环境变量 |
| `python_path` | `python3` | 解释器路径 |
| `script_path` | （`defaultSpider91ScriptPath()` 自动定位） | spider_91porn.py 绝对路径 |
| `last_crawl_at` | （自动写） | 最后一次成功完成的 unix 秒；通过 `lastCrawlAt` 字段暴露给 admin UI |

`last_crawl_at` 仅由后端写入。admin UI 不显示其它凭证（凭证字段只通过 `hasCredential` 抽象暴露布尔值）。

### 16.4 触发逻辑

- 每分钟轮询一次（`crawlerLoop` ticker，独立于 02:00-07:00 的 `scanLoop`）
- 当 `time.Now().Hour() == drive.Credentials["crawl_hour"]` 且 `now - last_crawl_at >= 12h` 时触发
- 管理后台点 "立即抓取" 按钮立刻触发，不受时间窗约束
- 多个 spider91 drive 可以挂不同 `crawl_hour`，错峰避免并发下载

### 16.5 GC 和清理

- 当前**不主动清理旧视频文件**。删 spider91 drive 不会删 `data/spider91/<driveID>/` 下的文件（和云盘 drive 删除时不动 teaser 一致）
- 已存在 viewkey 在 Python 端通过 `--seen-viewkeys-file` 跳过（不发详情页请求），Go 端再做 `Catalog.GetVideo` 二次去重防御
- 每次爬虫输出的 JSON 留在 `<driveDir>/.crawl/target-<N>-<UTC>.json`，对应的已知 viewkey 列表在 `<driveDir>/.crawl/seen-<UTC>.txt`，方便事后排查；磁盘吃紧可手动清理

### 16.6 测试覆盖

- `internal/drives/spider91/driver_test.go`：8 个用例覆盖 Init/safeJoin/List/Stat/StreamURL/BuildVideoID
- `internal/drives/spider91/crawler_test.go`：用 shell 脚本伪装成 python + httptest 服务器跑端到端流程，覆盖首次入库、文件落盘、封面副本、SeenSnapshot 计数、第二次跑跳过已存在 viewkey
- `internal/drives/spider91/ext_test.go`：`detectVideoExt` / `detectThumbExt` 的扩展名识别表驱动测试
- `internal/catalog/file_ids_test.go`：`ListVideoFileIDsByDrive` 的 drive 隔离 + 空字段过滤
- `cmd/server/main_spider91_test.go`：`spider91DueAt` 时间窗判断、`spider91IntCred` 凭证解析

### 16.7 已知风险

- 视频直链带过期 token，必须立刻下载；当前下载超时 30 分钟，单条视频典型 100 MB，正常网络 1 分钟内完成
- 91porn 有 Cloudflare，遇到 403 会让 Python 脚本退出非零，crawler 会写 last_crawl_at + drive.status=error，下次窗口仍会重试
- **代理是必需的**：91porn CDN 节点都在海外，国内服务器直连会变成几 KB/s 级别的慢速。backend 启动时必须能拿到 `HTTPS_PROXY` 环境变量，或在 drive credentials 里显式设 `proxy`；启动前在 `start.sh` 中导出 `HTTPS_PROXY` 是最方便的做法
- 单条视频平均 100 MB，每天 15 个新视频约占 1.5 GB；运行一段时间后注意磁盘容量，当前无自动清理
