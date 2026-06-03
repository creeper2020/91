import { useEffect, useState } from "react";
import { NavLink, Outlet, useNavigate } from "react-router-dom";
import {
  HardDrive,
  Film,
  LogOut,
  Play,
  Home,
  Tags,
  Palette,
  RefreshCw,
  MoreVertical,
} from "lucide-react";
import * as api from "./api";
import { useAuth } from "./AuthContext";
import { useToast } from "./ToastContext";

export function AdminLayout() {
  const { logout } = useAuth();
  const navigate = useNavigate();
  const { show } = useToast();
  const [checkingUpdate, setCheckingUpdate] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  useEffect(() => {
    if (!mobileMenuOpen) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setMobileMenuOpen(false);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [mobileMenuOpen]);

  async function handleCheckUpdate() {
    if (checkingUpdate) return;
    setCheckingUpdate(true);
    try {
      const result = await api.checkUpdate();
      if (result.hasUpdate) {
        show(
          `发现新版本 ${result.latestVersion}，当前 ${result.currentVersion}`,
          "success"
        );
        return;
      }
      if (result.currentVersion === "unknown") {
        show(`当前版本未知，GitHub 最新版本为 ${result.latestVersion}`, "info");
        return;
      }
      show(`当前已是最新版本：${result.currentVersion}`, "success");
    } catch {
      show("检查更新失败，请稍后重试", "error");
    } finally {
      setCheckingUpdate(false);
    }
  }

  async function handleLogout() {
    try {
      await logout();
      show("已退出登录", "success");
      navigate("/login", { replace: true });
    } catch {
      show("退出失败", "error");
    }
  }

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <div className="admin-sidebar__brand">
          <span className="admin-sidebar__brand-mark">
            <Play size={14} fill="#000" />
          </span>
          <span className="admin-sidebar__brand-text">91后台</span>
        </div>
        <nav className="admin-nav">
          <NavLink to="/" className="admin-nav__link">
            <Home size={16} /> 返回主站
          </NavLink>
          <NavLink
            to="/admin/drives"
            className={({ isActive }) =>
              `admin-nav__link ${isActive ? "is-active" : ""}`
            }
          >
            <HardDrive size={16} /> 网盘管理
          </NavLink>
          <NavLink
            to="/admin/videos"
            className={({ isActive }) =>
              `admin-nav__link ${isActive ? "is-active" : ""}`
            }
          >
            <Film size={16} /> 视频管理
          </NavLink>
          <NavLink
            to="/admin/tags"
            className={({ isActive }) =>
              `admin-nav__link ${isActive ? "is-active" : ""}`
            }
          >
            <Tags size={16} /> 标签管理
          </NavLink>
          <NavLink
            to="/admin/theme"
            className={({ isActive }) =>
              `admin-nav__link ${isActive ? "is-active" : ""}`
            }
          >
            <Palette size={16} /> 主题外观
          </NavLink>
        </nav>
        <div className="admin-sidebar__footer">
          <button
            className="admin-sidebar__check-update"
            onClick={handleCheckUpdate}
            disabled={checkingUpdate}
          >
            <RefreshCw size={14} />
            {checkingUpdate ? "检查中" : "检查更新"}
          </button>
          <button className="admin-sidebar__logout" onClick={handleLogout}>
            <LogOut size={14} />
            退出登录
          </button>
        </div>
        <button
          className="admin-sidebar__mobile-menu"
          onClick={() => setMobileMenuOpen((v) => !v)}
          aria-label="更多操作"
        >
          <MoreVertical size={18} />
        </button>
      </aside>
      {mobileMenuOpen && (
        <div className="admin-sidebar__mobile-overlay" onClick={() => setMobileMenuOpen(false)} />
      )}
      <div className={`admin-sidebar__mobile-panel${mobileMenuOpen ? " is-open" : ""}`}>
        <button
          className="admin-sidebar__check-update"
          onClick={() => { handleCheckUpdate(); setMobileMenuOpen(false); }}
          disabled={checkingUpdate}
        >
          <RefreshCw size={14} />
          {checkingUpdate ? "检查中" : "检查更新"}
        </button>
        <button className="admin-sidebar__logout" onClick={() => { handleLogout(); setMobileMenuOpen(false); }}>
          <LogOut size={14} />
          退出登录
        </button>
      </div>
      <main className="admin-main">
        <Outlet />
      </main>
    </div>
  );
}
