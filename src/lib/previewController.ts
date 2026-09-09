// 全局预览控制器：同一时刻只允许一个卡片播放预览
// 使用模块级 singleton + 订阅模式，避免 Context 的重渲染开销

type Listener = (activeId: string | null) => void;

let activeId: string | null = null;
// Do not activate previews until the server policy has been loaded.
let enabled = false;
const listeners = new Set<Listener>();

export const previewController = {
  isEnabled(): boolean {
    return enabled;
  },

  setEnabled(next: boolean) {
    if (enabled === next) return;
    enabled = next;
    if (!enabled) activeId = null;
    listeners.forEach((fn) => fn(activeId));
  },

  getActiveId(): string | null {
    return activeId;
  },

  setActiveId(id: string | null) {
    if (id !== null && !enabled) return;
    if (activeId === id) return;
    activeId = id;
    listeners.forEach((fn) => fn(activeId));
  },

  subscribe(fn: Listener): () => void {
    listeners.add(fn);
    return () => {
      listeners.delete(fn);
    };
  },
};
