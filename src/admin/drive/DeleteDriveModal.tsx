import { AlertTriangle, Trash2 } from "lucide-react";
import * as api from "../api";
import { Modal } from "../Modal";

export function DeleteDriveModal({
  drive,
  deleting,
  onCancel,
  onConfirm,
}: {
  drive: api.AdminDrive | null;
  deleting: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const name = drive?.name || drive?.id || "";
  const isSpider91 = drive?.kind === "spider91";
  const isLocalStorage = drive?.kind === "localstorage";
  const title = isSpider91 ? "删除 91Spider" : "删除存储";
  const primaryText = deleting ? "删除中..." : "确认删除并清理";

  return (
    <Modal
      open={!!drive}
      title={title}
      onClose={onCancel}
      footer={
        <>
          <button className="admin-btn" onClick={onCancel} disabled={deleting}>
            取消
          </button>
          <button className="admin-btn is-danger" onClick={onConfirm} disabled={deleting}>
            <Trash2 size={13} />
            {primaryText}
          </button>
        </>
      }
    >
      <div className="admin-delete-confirm">
        <div className="admin-delete-confirm__icon">
          <AlertTriangle size={20} />
        </div>
        <div className="admin-delete-confirm__content">
          <p className="admin-delete-confirm__title">
            {isSpider91
              ? `确定删除「${name}」吗？`
              : `确定删除「${name}」并清理该存储的视频数据吗？`}
          </p>
          <p className="admin-delete-confirm__text">
            取消或关闭此弹窗不会删除存储配置，也不会清理任何文件。
          </p>
          <ul className="admin-delete-confirm__list">
            <li>删除该存储配置</li>
            <li>删除数据库中的相关视频记录，网站首页、列表、标签页和详情页不再展示这些视频</li>
            <li>删除本机保存的封面图和预览视频</li>
            {isSpider91 && (
              <li>删除通过 91Spider 爬取到的本机 91 视频文件</li>
            )}
            {isLocalStorage && (
              <li>不会删除用户配置的本地目录中的原始视频文件</li>
            )}
          </ul>
          {!isSpider91 && !isLocalStorage && (
            <p className="admin-delete-confirm__text">
              此操作只清理本项目生成和记录的数据，不会删除云盘上的原始视频文件。
            </p>
          )}
        </div>
      </div>
    </Modal>
  );
}