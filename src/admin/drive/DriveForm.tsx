import { useId, useMemo } from "react";
import { P123QRCodeLogin } from "./P123QRCodeLogin";
import { Spider91UploadTargetField } from "./Spider91UploadTargetField";
import { FormState, Kind, credentialFields, credentialHelp, usesRootDirectoryID, rootIdPlaceholder } from "./constants";
import * as api from "../api";

export function DriveForm({
  form,
  onChange,
  isEdit,
  uploadTargets,
  nameError,
  onNameBlur,
}: {
  form: FormState;
  onChange: (f: FormState) => void;
  isEdit: boolean;
  uploadTargets: api.AdminDrive[];
  nameError?: string;
  onNameBlur?: () => void;
}) {
  const idPrefix = useId();
  const fields = useMemo(() => credentialFields(form.kind), [form.kind]);
  const help = credentialHelp(form.kind, isEdit);
  const nameId = `${idPrefix}-drive-name`;
  const kindId = `${idPrefix}-drive-kind`;
  const rootId = `${idPrefix}-drive-root`;

  function set<K extends keyof FormState>(k: K, v: FormState[K]) {
    onChange({ ...form, [k]: v });
  }
  function setCred(k: string, v: string) {
    onChange({ ...form, creds: { ...form.creds, [k]: v } });
  }
  function setKind(v: Kind) {
    onChange({
      ...form,
      kind: v,
      rootId: "",
      creds: {},
    });
  }

  return (
    <div className="admin-form">
      <div className="admin-form__row">
        <label htmlFor={nameId}>名称 *</label>
        <input
          id={nameId}
          value={form.name}
          onChange={(e) => set("name", e.target.value)}
          onBlur={onNameBlur}
          placeholder="给这个盘起个名字"
          className={nameError ? "is-invalid" : undefined}
          aria-invalid={nameError ? "true" : undefined}
          aria-describedby={nameError ? `${nameId}-error` : undefined}
        />
        {nameError && (
          <div className="admin-form__error" id={`${nameId}-error`}>
            {nameError}
          </div>
        )}
      </div>
      <div className="admin-form__row">
        <label htmlFor={kindId}>类型</label>
        <select
          id={kindId}
          value={form.kind}
          onChange={(e) => setKind(e.target.value as Kind)}
          disabled={isEdit}
        >
          <option value="p115">115 网盘</option>
          <option value="p123">123 云盘</option>
          <option value="pikpak">PikPak</option>
          <option value="onedrive">OneDrive</option>
          <option value="googledrive">Google Drive</option>
          <option value="localstorage">本地存储</option>
          <option value="spider91">91 Spider</option>
          <option value="quark">夸克网盘</option>
          <option value="wopan">联通沃盘</option>
        </select>
      </div>
      {usesRootDirectoryID(form.kind) && (
        <div className="admin-form__row">
          <label htmlFor={rootId}>根目录 ID</label>
          <input
            id={rootId}
            value={form.rootId}
            onChange={(e) => set("rootId", e.target.value)}
            placeholder={rootIdPlaceholder(form.kind)}
          />
          <div className="admin-form__help">
            留空时使用该网盘类型的默认根目录，具体目录ID获取方式请参考OpenList文档
          </div>
        </div>
      )}

      {(help || fields.length > 0) && (
        <>
          <hr className="admin-form__divider" />

          {help && (
            <div className="admin-form__help admin-form__help--lead">
              {help}
            </div>
          )}

          {form.kind === "p123" && (
            <P123QRCodeLogin
              onToken={(token) => setCred("access_token", token)}
            />
          )}

          {fields.map((f) => (
            <div key={f.key} className="admin-form__row">
              <label htmlFor={`${idPrefix}-credential-${f.key}`}>
                {f.label}{f.required && " *"}
              </label>
              {f.multiline ? (
                <textarea
                  id={`${idPrefix}-credential-${f.key}`}
                  value={form.creds[f.key] ?? ""}
                  onChange={(e) => setCred(f.key, e.target.value)}
                  placeholder={f.placeholder}
                />
              ) : (
                <input
                  id={`${idPrefix}-credential-${f.key}`}
                  type={credentialInputType(f.key)}
                  value={form.creds[f.key] ?? ""}
                  onChange={(e) => setCred(f.key, e.target.value)}
                  placeholder={f.placeholder}
                />
              )}
              {f.help && <div className="admin-form__help">{f.help}</div>}
            </div>
          ))}
        </>
      )}

      {form.kind === "spider91" && (
        <>
          <hr className="admin-form__divider" />
          <Spider91UploadTargetField
            value={form.spider91UploadDriveId}
            onChange={(v) => set("spider91UploadDriveId", v)}
            uploadTargets={uploadTargets}
          />
        </>
      )}
    </div>
  );
}

function credentialInputType(key: string): string {
  return /password|token|secret/i.test(key) ? "password" : "text";
}
