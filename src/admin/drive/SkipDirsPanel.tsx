import { useCallback, useEffect, useMemo, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import * as api from "../api";
import { useToast } from "../ToastContext";
import { kindLabel } from "./constants";

type SkipDirsPanelProps = {
  drive: api.AdminDrive;
  onSaved: (saved: { id: string; skipDirIds: string[] }) => void;
};

export function SkipDirsPanel({ drive, onSaved }: SkipDirsPanelProps) {
  const { show } = useToast();
  const [selected, setSelected] = useState<Set<string>>(
    () => new Set(drive.skipDirIds ?? [])
  );
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setSelected(new Set(drive.skipDirIds ?? []));
  }, [drive.id, drive.skipDirIds]);

  const toggle = useCallback((id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  async function handleSave() {
    setSaving(true);
    try {
      const ids = Array.from(selected);
      const resp = await api.setDriveSkipDirIds(drive.id, ids);
      onSaved({ id: drive.id, skipDirIds: resp.skipDirIds });
    } catch (e) {
      show(e instanceof Error ? e.message : "保存失败", "error");
    } finally {
      setSaving(false);
    }
  }

  const selectedList = useMemo(() => Array.from(selected), [selected]);

  return (
    <div className="admin-detail-card">
      <header className="admin-detail-card__title">
        <div className="admin-detail-card__title-left">
          <span>扫描跳过目录</span>
        </div>
        <button
          className="admin-btn is-primary"
          onClick={handleSave}
          disabled={saving}
          style={{ padding: "4px 10px", fontSize: "12px", height: "auto" }}
        >
          {saving ? "保存中..." : `保存更改 (${selectedList.length})`}
        </button>
      </header>

      <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
        <p className="admin-text-faint" style={{ margin: 0, fontSize: "12px", lineHeight: "1.5" }}>
          勾选要在扫描时跳过的目录。命中目录及其全部子目录都不会被递归扫描。下次扫描生效。
        </p>

        <SelectedDirsChips
          drive={drive}
          selected={selectedList}
          onRemove={toggle}
        />

        <div className="admin-detail-tree-container">
          <DirTreeNode
            driveId={drive.id}
            id=""
            name={drive.name || drive.id}
            depth={0}
            initiallyOpen
            ancestorSkipped={false}
            selected={selected}
            onToggle={toggle}
          />
        </div>
      </div>
    </div>
  );
}

function SelectedDirsChips({
  drive,
  selected,
  onRemove,
}: {
  drive: api.AdminDrive;
  selected: string[];
  onRemove: (id: string) => void;
}) {
  if (selected.length === 0) {
    return (
      <div
        className="admin-text-faint"
        style={{ fontSize: "13px", padding: "6px 0" }}
      >
        当前未勾选任何跳过目录（{kindLabel[drive.kind] ?? drive.kind}{" "}
        将完整扫描）。
      </div>
    );
  }
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: "6px" }}>
      {selected.map((id) => (
        <span
          key={id}
          className="admin-mono-cell"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: "6px",
            padding: "3px 10px",
            border: "1px solid var(--border-subtle)",
            borderRadius: "999px",
            fontSize: "12px",
          }}
          title="点击 × 移除"
        >
          {id}
          <button
            type="button"
            onClick={() => onRemove(id)}
            style={{
              border: "none",
              background: "transparent",
              cursor: "pointer",
              color: "var(--text-secondary)",
              padding: 0,
              lineHeight: 1,
              fontSize: "14px",
            }}
            aria-label={`移除 ${id}`}
          >
            ×
          </button>
        </span>
      ))}
    </div>
  );
}

type DirTreeNodeProps = {
  driveId: string;
  id: string;
  name: string;
  depth: number;
  initiallyOpen?: boolean;
  ancestorSkipped: boolean;
  selected: Set<string>;
  onToggle: (id: string) => void;
};

function DirTreeNode({
  driveId,
  id,
  name,
  depth,
  initiallyOpen,
  ancestorSkipped,
  selected,
  onToggle,
}: DirTreeNodeProps) {
  const [open, setOpen] = useState(!!initiallyOpen);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [children, setChildren] = useState<api.DriveDirEntry[]>([]);
  const [error, setError] = useState("");

  const isRoot = depth === 0;
  const isSelected = id !== "" && selected.has(id);
  const dimmed = ancestorSkipped;

  const loadChildren = useCallback(async () => {
    if (loaded || loading) return;
    setLoading(true);
    setError("");
    try {
      const data = await api.listDriveDirChildren(driveId, id || undefined);
      setChildren(data ?? []);
      setLoaded(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [driveId, id, loaded, loading]);

  useEffect(() => {
    if (open && !loaded) {
      void loadChildren();
    }
  }, [open, loaded, loadChildren]);

  function handleToggleOpen() {
    setOpen((v) => !v);
  }

  return (
    <div
      style={{
        paddingLeft: depth === 0 ? 0 : 16,
        opacity: dimmed && !isSelected ? 0.55 : 1,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "6px",
          padding: "4px 6px",
          borderRadius: "4px",
          background: isSelected ? "var(--accent-soft, rgba(255,140,0,0.12))" : "transparent",
        }}
      >
        {!isRoot ? (
          <button
            type="button"
            onClick={handleToggleOpen}
            style={{
              border: "none",
              background: "transparent",
              cursor: "pointer",
              padding: 0,
              display: "inline-flex",
              alignItems: "center",
            }}
            aria-label={open ? "折叠" : "展开"}
          >
            {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </button>
        ) : (
          <span style={{ width: 14, display: "inline-block" }} />
        )}

        {!isRoot && (
          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => onToggle(id)}
            aria-label={`跳过目录 ${name}`}
          />
        )}

        <span
          style={{
            fontSize: "13px",
            cursor: isRoot ? "default" : "pointer",
            userSelect: "none",
            fontWeight: isRoot ? 600 : 400,
          }}
          onClick={isRoot ? undefined : handleToggleOpen}
        >
          {name}
          {isRoot ? " (根目录)" : ""}
        </span>
        {!isRoot && (
          <span
            className="admin-mono-cell admin-text-faint"
            style={{ fontSize: "11px", marginLeft: "6px" }}
          >
            {id}
          </span>
        )}
      </div>

      {open && (
        <div>
          {loading && (
            <div className="admin-text-faint" style={{ fontSize: "12px", padding: "4px 28px" }}>
              加载中...
            </div>
          )}
          {error && (
            <div style={{ fontSize: "12px", padding: "4px 28px", color: "var(--danger, #d33)" }}>
              {error}
            </div>
          )}
          {loaded && !error && children.length === 0 && (
            <div className="admin-text-faint" style={{ fontSize: "12px", padding: "4px 28px" }}>
              （无子目录）
            </div>
          )}
          {children.map((child) => (
            <DirTreeNode
              key={child.id}
              driveId={driveId}
              id={child.id}
              name={child.name}
              depth={depth + 1}
              ancestorSkipped={ancestorSkipped || isSelected}
              selected={selected}
              onToggle={onToggle}
            />
          ))}
        </div>
      )}
    </div>
  );
}