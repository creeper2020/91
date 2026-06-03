import { useEffect, useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { Play } from "lucide-react";
import { useAuth } from "./AuthContext";
import { useToast } from "./ToastContext";
import * as api from "./api";

export function LoginPage() {
  const { status, login, refresh } = useAuth();
  const [u, setU] = useState("");
  const [p, setP] = useState("");
  const [p2, setP2] = useState("");
  const [setupRequired, setSetupRequired] = useState<boolean | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();
  const { show } = useToast();
  const passwordMismatch = setupRequired === true && p2.length > 0 && p !== p2;

  useEffect(() => {
    let active = true;
    api.setupStatus()
      .then((res) => {
        if (active) setSetupRequired(res.required);
      })
      .catch((e) => {
        if (active) {
          setSetupRequired(false);
          setErr(e instanceof Error ? e.message : "初始化状态检查失败");
        }
      });
    return () => {
      active = false;
    };
  }, []);

  if (status === "loading" || setupRequired === null) {
    return (
      <div className="admin-loading-screen">
        检查登录状态...
      </div>
    );
  }

  // 已登录：回到来源页，或默认去首页
  if (status === "authed") {
    const from = (location.state as { from?: string } | null)?.from ?? "/";
    return <Navigate to={from} replace />;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    if (setupRequired && p !== p2) {
      setErr("两次输入的密码不一致");
      return;
    }
    setLoading(true);
    try {
      if (setupRequired) {
        await api.setupAdmin(u, p);
        await refresh();
        setSetupRequired(false);
        show("管理员账号已设置", "success");
      } else {
        await login(u, p);
        show("登录成功", "success");
      }
      const from = (location.state as { from?: string } | null)?.from ?? "/";
      navigate(from, { replace: true });
    } catch (e) {
      setErr(e instanceof Error ? e.message : "登录失败");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="admin-login">
      <form className="admin-login__card" onSubmit={handleSubmit}>
        <h1 className="admin-login__title">
          <Play size={18} fill="currentColor" /> {setupRequired ? "首次设置管理员" : "登录"}
        </h1>
        <div className="admin-form">
          <div className="admin-form__row">
            <label htmlFor="admin-login-username">用户名</label>
            <input
              id="admin-login-username"
              autoFocus
              value={u}
              onChange={(e) => setU(e.target.value)}
              autoComplete="username"
            />
          </div>
          <div className="admin-form__row">
            <label htmlFor="admin-login-password">密码</label>
            <input
              id="admin-login-password"
              type="password"
              value={p}
              onChange={(e) => setP(e.target.value)}
              autoComplete={setupRequired ? "new-password" : "current-password"}
            />
          </div>
          {setupRequired && (
            <div className="admin-form__row">
              <label htmlFor="admin-login-password-confirm">确认密码</label>
              <input
                id="admin-login-password-confirm"
                type="password"
                value={p2}
                onChange={(e) => setP2(e.target.value)}
                autoComplete="new-password"
                className={passwordMismatch ? "is-invalid" : undefined}
                aria-invalid={passwordMismatch ? "true" : undefined}
                aria-describedby={passwordMismatch ? "admin-login-password-confirm-error" : undefined}
              />
              {passwordMismatch && (
                <div className="admin-form__error" id="admin-login-password-confirm-error">
                  密码不一致
                </div>
              )}
            </div>
          )}
          <button
            className="admin-btn is-primary"
            type="submit"
            disabled={loading || !u || !p || (setupRequired && (!p2 || passwordMismatch))}
          >
            {loading
              ? setupRequired
                ? "保存中..."
                : "登录中..."
              : setupRequired
              ? "保存并进入"
              : "登录"}
          </button>
          {err && <div className="admin-login__error">{err}</div>}
        </div>
      </form>
    </div>
  );
}
