#!/usr/bin/env python3
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path
from urllib.parse import urlparse, unquote

USER_AGENT = (
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
    "(KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
)

VIDEO_EXTS = {
    ".avi",
    ".flv",
    ".m4v",
    ".mkv",
    ".mov",
    ".mp4",
    ".mpeg",
    ".mpg",
    ".ts",
    ".webm",
    ".wmv",
}


def log(message: str) -> None:
    print(message, file=sys.stderr, flush=True)


def sanitize_filename(name: str) -> str:
    clean = re.sub(r'[\\/*?:"<>|]', "", name or "")
    clean = re.sub(r"\s+", " ", clean).strip()
    return clean[:160] or "downloaded-video"


def ensure_tools(*names: str) -> None:
    missing = [name for name in names if shutil.which(name) is None]
    if missing:
        raise RuntimeError("missing required command: " + ", ".join(missing))


def is_video_url(url: str) -> bool:
    path = urlparse(url).path.lower()
    return Path(unquote(path)).suffix in VIDEO_EXTS


def unique_path(path: Path) -> Path:
    if not path.exists():
        return path
    stem = path.stem
    suffix = path.suffix
    for index in range(2, 1000):
        candidate = path.with_name(f"{stem}-{index}{suffix}")
        if not candidate.exists():
            return candidate
    raise RuntimeError(f"cannot create unique path for {path}")


def collect_video_files(output_dir: Path) -> list[Path]:
    files: list[Path] = []
    for root, _, names in os.walk(output_dir):
        for name in names:
            path = Path(root) / name
            if path.is_file() and path.stat().st_size > 0 and path.suffix.lower() in VIDEO_EXTS:
                files.append(path)
    return sorted(files, key=lambda p: p.stat().st_size, reverse=True)


def trackers_from_file() -> str:
    tracker_file = Path(__file__).with_name("trackers.txt")
    if not tracker_file.exists():
        return ""
    trackers = [
        line.strip()
        for line in tracker_file.read_text(encoding="utf-8", errors="ignore").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]
    return ",".join(trackers)


def aria2_download(url: str, output_dir: Path, is_magnet: bool = False) -> list[Path]:
    ensure_tools("aria2c")
    output_dir.mkdir(parents=True, exist_ok=True)
    cmd = [
        "aria2c",
        "--dir",
        str(output_dir),
        "--max-connection-per-server=16",
        "--split=16",
        "--min-split-size=1M",
        "--summary-interval=0",
        "--download-result=hide",
        "--console-log-level=error",
        "--allow-overwrite=false",
        "--auto-file-renaming=true",
    ]
    if is_magnet:
        cmd += [
            "--seed-time=0",
            "--bt-stop-timeout=300",
            "--enable-dht=true",
            "--bt-tracker-connect-timeout=10",
            "--bt-require-crypto=true",
            "--bt-min-crypto-level=arc4",
            "--peer-id-prefix=-qB4500-",
            "--peer-agent=qBittorrent/4.5.0",
            "--bt-exclude-tracker=*xunlei*,*sandai*,*thunder*",
        ]
        trackers = trackers_from_file()
        if trackers:
            cmd.append(f"--bt-tracker={trackers}")
    cmd.append(url)
    log("[import] running aria2c")
    subprocess.run(cmd, check=True)
    files = collect_video_files(output_dir)
    if not is_magnet and not files:
        # Direct video links may preserve an extension not in VIDEO_EXTS if the
        # server sends a generated file name. Keep the largest non-empty file.
        candidates = [p for p in output_dir.iterdir() if p.is_file() and p.stat().st_size > 0]
        if candidates:
            files = [max(candidates, key=lambda p: p.stat().st_size)]
    return files


def ffprobe_duration(media_url: str, headers: str) -> float:
    try:
        out = subprocess.check_output(
            [
                "ffprobe",
                "-headers",
                headers,
                "-v",
                "error",
                "-show_entries",
                "format=duration",
                "-of",
                "default=noprint_wrappers=1:nokey=1",
                media_url,
            ],
            timeout=20,
            stderr=subprocess.DEVNULL,
        )
        return float(out.decode().strip())
    except Exception:
        return 0.0


def capture_media_links(webpage_url: str) -> tuple[str, list[str], list[str]]:
    try:
        from playwright.sync_api import sync_playwright
    except Exception as exc:
        raise RuntimeError("playwright is required for webpage import") from exc

    m3u8_links: list[str] = []
    video_links: list[str] = []

    def handle_request(request) -> None:
        req_url = request.url
        if ".m3u8" in req_url:
            if req_url not in m3u8_links:
                log(f"[import] captured m3u8: {req_url[:120]}")
                m3u8_links.append(req_url)
            return
        if is_video_url(req_url) and req_url not in video_links:
            log(f"[import] captured video: {req_url[:120]}")
            video_links.append(req_url)

    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(
            user_agent=USER_AGENT,
            extra_http_headers={"Referer": webpage_url},
        )
        page = context.new_page()
        page.on("request", handle_request)
        title = "downloaded-video"
        try:
            page.goto(webpage_url, wait_until="domcontentloaded", timeout=25000)
            raw_title = page.title()
            if raw_title:
                title = sanitize_filename(raw_title)
            deadline = time.time() + 20
            while time.time() < deadline:
                if m3u8_links or video_links:
                    time.sleep(1)
                    break
                time.sleep(0.5)
        finally:
            browser.close()
    return title, m3u8_links, video_links


def download_webpage_video(url: str, output_dir: Path) -> list[Path]:
    ensure_tools("ffmpeg", "ffprobe")
    output_dir.mkdir(parents=True, exist_ok=True)
    title, m3u8_links, video_links = capture_media_links(url)
    targets = m3u8_links or video_links
    if not targets:
        return []
    files: list[Path] = []
    headers = f"Referer: {url}\r\nUser-Agent: {USER_AGENT}\r\n"
    for index, target in enumerate(targets):
        suffix = ".mp4"
        if not m3u8_links:
            target_suffix = Path(unquote(urlparse(target).path)).suffix.lower()
            if target_suffix in VIDEO_EXTS:
                suffix = target_suffix
        name = f"{title}{suffix}" if len(targets) == 1 else f"{title}-{index + 1}{suffix}"
        output = unique_path(output_dir / name)
        cmd = [
            "ffmpeg",
            "-headers",
            headers,
            "-i",
            target,
            "-c",
            "copy",
            "-y",
            str(output),
        ]
        log(f"[import] running ffmpeg for {output.name}")
        duration = ffprobe_duration(target, headers)
        if duration:
            log(f"[import] media duration: {duration:.1f}s")
        subprocess.run(cmd, check=True)
        if output.exists() and output.stat().st_size > 0:
            files.append(output)
    return files


def files_payload(files: list[Path]) -> dict:
    return {
        "files": [
            {
                "path": str(path.resolve()),
                "title": path.stem,
                "size": path.stat().st_size,
            }
            for path in files
            if path.exists() and path.stat().st_size > 0
        ]
    }


def run(url: str, output_dir: Path) -> dict:
    if url.lower().startswith("magnet:?"):
        files = aria2_download(url, output_dir, is_magnet=True)
    elif is_video_url(url):
        files = aria2_download(url, output_dir, is_magnet=False)
    else:
        files = download_webpage_video(url, output_dir)
    if not files:
        raise RuntimeError("no video files downloaded")
    return files_payload(files)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    try:
        payload = run(args.url.strip(), Path(args.output_dir))
        if args.json:
            print(json.dumps(payload, ensure_ascii=False))
        else:
            for item in payload["files"]:
                print(item["path"])
        return 0
    except Exception as exc:
        print(json.dumps({"files": [], "error": str(exc)}, ensure_ascii=False), file=sys.stdout)
        log(f"[import] error: {exc}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
