#!/usr/bin/env python3
import asyncio
import json
import mimetypes
import os
import tempfile
import time
from pathlib import Path
from typing import Any
from urllib import request as urlrequest
from urllib.error import HTTPError

import requests
from pyrogram import Client, filters
from pyrogram.types import Message


def load_env_file(path: Path) -> None:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError:
        return
    for raw in lines:
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export ") :].strip()
        key, sep, value = line.partition("=")
        if not sep:
            continue
        key = key.strip()
        if not key or not (key[0].isalpha() or key[0] == "_"):
            continue
        if any(not (char.isalnum() or char == "_") for char in key):
            continue
        if key in os.environ:
            continue
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
            value = value[1:-1]
        os.environ[key] = value


def load_env_files() -> None:
    candidates: list[Path] = []
    for base in (Path.cwd(), Path(__file__).resolve().parent.parent, Path(__file__).resolve().parent):
        candidates.extend([base / ".env.import", base / ".env"])
    seen: set[Path] = set()
    for path in candidates:
        try:
            resolved = path.resolve()
        except OSError:
            resolved = path
        if resolved in seen:
            continue
        seen.add(resolved)
        load_env_file(resolved)


load_env_files()


def strip_inline_comment(value: str) -> str:
    quote = ""
    escaped = False
    for index, char in enumerate(value):
        if quote:
            if quote == '"' and char == "\\" and not escaped:
                escaped = True
                continue
            if char == quote and not escaped:
                quote = ""
            escaped = False
            continue
        if char in {"'", '"'}:
            quote = char
            continue
        if char == "#" and (index == 0 or value[index - 1].isspace()):
            return value[:index].rstrip()
    return value.strip()


def parse_yaml_scalar(value: str) -> Any:
    value = strip_inline_comment(value).strip()
    if not value:
        return ""
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {"'", '"'}:
        unquoted = value[1:-1]
        if value[0] == '"':
            unquoted = unquoted.replace(r"\"", '"').replace(r"\\", "\\")
        return unquoted
    lowered = value.lower()
    if lowered in {"true", "false"}:
        return lowered == "true"
    if lowered in {"null", "~"}:
        return None
    try:
        return int(value)
    except ValueError:
        return value


def load_simple_yaml(path: Path) -> dict[str, Any]:
    root: dict[str, Any] = {}
    stack: list[tuple[int, dict[str, Any]]] = [(-1, root)]
    for raw in path.read_text(encoding="utf-8").splitlines():
        if not raw.strip() or raw.lstrip().startswith("#"):
            continue
        indent = len(raw) - len(raw.lstrip(" "))
        line = raw.strip()
        if line.startswith("- ") or ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = strip_inline_comment(key).strip().strip("'\"")
        if not key:
            continue
        while len(stack) > 1 and indent <= stack[-1][0]:
            stack.pop()
        parent = stack[-1][1]
        value = value.strip()
        if value:
            parent[key] = parse_yaml_scalar(value)
            continue
        child: dict[str, Any] = {}
        parent[key] = child
        stack.append((indent, child))
    return root


def load_yaml_config(path: Path | None) -> dict[str, Any]:
    if path is None:
        return {}
    try:
        import yaml  # type: ignore

        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        return data if isinstance(data, dict) else {}
    except Exception:
        return load_simple_yaml(path)


def find_config_file() -> Path | None:
    candidates: list[Path] = []
    for key in ("VIDEO_CONFIG", "IMPORT_CONFIG"):
        raw = os.getenv(key)
        if raw:
            candidates.append(Path(raw))
    repo_root = Path(__file__).resolve().parent.parent
    candidates.extend(
        [
            Path.cwd() / "config.yaml",
            repo_root / "config.yaml",
            repo_root / "backend" / "config.yaml",
        ]
    )
    seen: set[Path] = set()
    for candidate in candidates:
        try:
            resolved = candidate.expanduser().resolve()
        except OSError:
            resolved = candidate.expanduser()
        if resolved in seen:
            continue
        seen.add(resolved)
        if resolved.exists():
            return resolved
    return None


def nested_config(root: dict[str, Any], *keys: str) -> dict[str, Any]:
    value: Any = root
    for key in keys:
        if not isinstance(value, dict):
            return {}
        value = value.get(key)
    return value if isinstance(value, dict) else {}


def config_value(section: dict[str, Any], key: str, env_names: tuple[str, ...] = (), default: str = "") -> str:
    value = section.get(key)
    if value is not None and str(value).strip():
        return str(value).strip()
    for env_name in env_names:
        value = os.getenv(env_name)
        if value is not None and value.strip():
            return value.strip()
    return default


def config_int(section: dict[str, Any], key: str, env_names: tuple[str, ...] = (), default: int = 0) -> int:
    raw = config_value(section, key, env_names, str(default))
    try:
        return int(raw)
    except (TypeError, ValueError):
        return default


CONFIG_PATH = find_config_file()
CONFIG = load_yaml_config(CONFIG_PATH)
CONFIG_DIR = CONFIG_PATH.parent if CONFIG_PATH else Path.cwd()
EXTERNAL_IMPORT = nested_config(CONFIG, "external_import")
TELEGRAM_CONFIG = nested_config(CONFIG, "external_import", "telegram")

API_BASE = config_value(EXTERNAL_IMPORT, "api_base", ("IMPORT_API_BASE",), "http://127.0.0.1:9191").rstrip("/")
API_TOKEN = config_value(EXTERNAL_IMPORT, "token", ("IMPORT_API_TOKEN", "VIDEO_IMPORT_TOKEN", "SYNC_TOKEN"))
BOT_TOKEN = config_value(TELEGRAM_CONFIG, "bot_token", ("TELEGRAM_BOT_TOKEN",))
API_ID = config_int(TELEGRAM_CONFIG, "api_id", ("API_ID",))
API_HASH = config_value(TELEGRAM_CONFIG, "api_hash", ("API_HASH",))
ADMIN_ID = config_int(TELEGRAM_CONFIG, "admin_id", ("ADMIN_ID",))
data_dir_raw = config_value(TELEGRAM_CONFIG, "data_dir", ("TG_IMPORT_DATA_DIR",), "data/tg-import")
DATA_DIR = Path(data_dir_raw)
if not DATA_DIR.is_absolute():
    DATA_DIR = CONFIG_DIR / DATA_DIR
WHITELIST_FILE = DATA_DIR / "whitelist.json"

VIDEO_EXTS = {".avi", ".flv", ".m4v", ".mkv", ".mov", ".mp4", ".mpeg", ".mpg", ".ts", ".webm", ".wmv"}


def load_whitelist() -> set[int]:
    if not WHITELIST_FILE.exists():
        return {ADMIN_ID} if ADMIN_ID else set()
    try:
        raw = json.loads(WHITELIST_FILE.read_text(encoding="utf-8"))
        users = {int(x) for x in raw.get("users", [])}
    except Exception:
        users = set()
    if ADMIN_ID:
        users.add(ADMIN_ID)
    return users


def save_whitelist(users: set[int]) -> None:
    DATA_DIR.mkdir(parents=True, exist_ok=True)
    WHITELIST_FILE.write_text(json.dumps({"users": sorted(users)}, ensure_ascii=False, indent=2), encoding="utf-8")


WHITELIST = load_whitelist()


def authorized(user_id: int | None) -> bool:
    return bool(user_id and user_id in WHITELIST)


def api_headers(content_type: str = "application/json") -> dict[str, str]:
    if not API_TOKEN:
        raise RuntimeError("external_import.token is required")
    return {"Content-Type": content_type, "X-Import-Token": API_TOKEN}


def http_json(method: str, path: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
    body = json.dumps(payload or {}).encode("utf-8") if payload is not None else None
    req = urlrequest.Request(API_BASE + path, data=body, method=method, headers=api_headers())
    try:
        with urlrequest.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="ignore")
        raise RuntimeError(detail or f"HTTP {exc.code}") from exc


def multipart_upload(path: Path, title: str = "") -> dict[str, Any]:
    content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
    data: dict[str, str] = {}
    if title:
        data["title"] = title
    with path.open("rb") as file:
        response = requests.post(
            API_BASE + "/api/imports/external-upload",
            headers={"X-Import-Token": api_headers()["X-Import-Token"]},
            data=data,
            files={"file": (path.name, file, content_type)},
            timeout=3600,
        )
    if response.status_code >= 400:
        raise RuntimeError(response.text or f"HTTP {response.status_code}")
    return response.json()


def submit_link(url: str) -> dict[str, Any]:
    return http_json("POST", "/api/imports/external", {"url": url})


def get_job(job_id: str) -> dict[str, Any]:
    return http_json("GET", f"/api/imports/external/{job_id}")


DATA_DIR.mkdir(parents=True, exist_ok=True)
app = Client("video_site_import_bot", workdir=str(DATA_DIR), api_id=API_ID, api_hash=API_HASH, bot_token=BOT_TOKEN)


@app.on_message(filters.command("start"))
async def start(_, message: Message) -> None:
    if not authorized(message.from_user.id if message.from_user else None):
        await message.reply_text("没有权限。")
        return
    await message.reply_text("发送视频链接、磁力链接或视频文件即可导入。")


@app.on_message(filters.command("me"))
async def me(_, message: Message) -> None:
    user_id = message.from_user.id if message.from_user else 0
    await message.reply_text(f"ID: {user_id}\n授权: {'yes' if authorized(user_id) else 'no'}")


@app.on_message(filters.command("add"))
async def add_user(_, message: Message) -> None:
    if not ADMIN_ID or not message.from_user or message.from_user.id != ADMIN_ID:
        await message.reply_text("只有管理员可以授权。")
        return
    parts = (message.text or "").split()
    if len(parts) < 2:
        await message.reply_text("用法: /add 123456")
        return
    WHITELIST.add(int(parts[1]))
    save_whitelist(WHITELIST)
    await message.reply_text("已授权。")


@app.on_message(filters.command("del"))
async def del_user(_, message: Message) -> None:
    if not ADMIN_ID or not message.from_user or message.from_user.id != ADMIN_ID:
        await message.reply_text("只有管理员可以移除授权。")
        return
    parts = (message.text or "").split()
    if len(parts) < 2:
        await message.reply_text("用法: /del 123456")
        return
    user_id = int(parts[1])
    if user_id != ADMIN_ID:
        WHITELIST.discard(user_id)
        save_whitelist(WHITELIST)
    await message.reply_text("已更新。")


@app.on_message(filters.command("list"))
async def list_users(_, message: Message) -> None:
    if not authorized(message.from_user.id if message.from_user else None):
        await message.reply_text("没有权限。")
        return
    await message.reply_text("\n".join(str(x) for x in sorted(WHITELIST)) or "空")


@app.on_message(filters.text & ~filters.command(["start", "me", "add", "del", "list"]))
async def import_link(_, message: Message) -> None:
    if not authorized(message.from_user.id if message.from_user else None):
        await message.reply_text("没有权限。")
        return
    url = (message.text or "").strip()
    if not (url.startswith("magnet:?") or url.startswith("http://") or url.startswith("https://")):
        return
    status = await message.reply_text("已提交，等待下载。")
    try:
        job = await asyncio.to_thread(submit_link, url)
        last = ""
        while job.get("status") not in {"done", "failed"}:
            label = job.get("message") or job.get("status") or "running"
            if label != last:
                last = label
                await status.edit_text(f"导入中: {label}")
            await asyncio.sleep(5)
            job = await asyncio.to_thread(get_job, job["id"])
        if job.get("status") == "failed":
            await status.edit_text(f"导入失败: {job.get('error') or 'unknown error'}")
            return
        titles = [v.get("title", v.get("id", "")) for v in job.get("videos", [])]
        await status.edit_text("导入完成\n" + "\n".join(titles))
    except Exception as exc:
        await status.edit_text(f"导入失败: {exc}")


@app.on_message(filters.video | filters.document)
async def import_media(_, message: Message) -> None:
    if not authorized(message.from_user.id if message.from_user else None):
        await message.reply_text("没有权限。")
        return
    media = message.video or message.document
    file_name = getattr(media, "file_name", "") or f"telegram-{int(time.time())}.mp4"
    if Path(file_name).suffix.lower() not in VIDEO_EXTS:
        await message.reply_text("不是支持的视频文件。")
        return
    status = await message.reply_text("正在下载 Telegram 文件。")
    with tempfile.TemporaryDirectory(prefix="tg-import-") as tmp:
        path = Path(tmp) / file_name
        try:
            downloaded = await message.download(file_name=str(path))
            await status.edit_text("正在提交入库。")
            video = await asyncio.to_thread(multipart_upload, Path(downloaded), Path(file_name).stem)
            await status.edit_text(f"导入完成: {video.get('title') or video.get('id')}")
        except Exception as exc:
            await status.edit_text(f"导入失败: {exc}")


def main() -> int:
    if not BOT_TOKEN or not API_ID or not API_HASH:
        raise RuntimeError("external_import.telegram.bot_token, api_id and api_hash are required")
    app.run()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
