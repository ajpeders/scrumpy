#!/usr/bin/env python3
"""Replay showcase.tape in a prepared kitty window; record video, never PNGs.

Requires kitten, gpu-screen-recorder and ffmpeg. Uses real playback/queue.
"""

import argparse
import json
import pathlib
import re
import signal
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--socket", required=True)
    parser.add_argument("--region", required=True, help="Kitty client geometry: WIDTHxHEIGHT+X+Y")
    parser.add_argument("--current-song", action="store_true",
                        help="Keep Cider's current song instead of the tape's off-camera track setup")
    args = parser.parse_args()
    root = pathlib.Path(__file__).resolve().parent.parent
    remote = ["kitten", "@", "--to", args.socket]

    def call(*command):
        return subprocess.check_output([*remote, *command], timeout=15)

    def send(value):
        # stdin sends literal bytes, without another layer of escape parsing.
        subprocess.run(
            [*remote, "send-text", "--match", "id:1", "--stdin"],
            input=value.encode(), check=True, timeout=15,
        )
        # Let Bubble Tea consume keys before the next command.
        time.sleep(0.01)

    def position():
        text = call("get-text", "--match", "id:1").decode()
        match = re.search(r"\]\s+(\d+):(\d+) / (\d+):(\d+)", text)
        if match is None:
            raise RuntimeError("Cannot verify the visible seek position")
        minutes, seconds, total_minutes, total_seconds = map(int, match.groups())
        return minutes * 60 + seconds, total_minutes * 60 + total_seconds

    encoder = None
    keys = {"Enter": "\r", "Escape": "\x1b", "Up": "\x1b[A", "Down": "\x1b[B"}

    with tempfile.TemporaryDirectory(prefix="kitty-video-", dir="/tmp/opencode") as tmp:
        output = pathlib.Path(tmp) / "showcase.mp4"
        timeline = []
        start = None
        capture_log = open(pathlib.Path(tmp) / "capture.txt", "wb")

        try:
            # Shell setup is done before launch, not typed into the app.
            # Replay all TUI steps after the setup's initial five-second wait.
            ready = False
            for number, raw in enumerate((root / "demo/showcase.tape").read_text().splitlines(), 1):
                line = raw.strip()
                if not ready:
                    if args.current_song:
                        if line != "Show":
                            continue
                        send("4")
                        time.sleep(6)
                        ready = True
                    elif line == "Sleep 5s":
                        ready = True
                        time.sleep(5)
                        continue
                    else:
                        continue
                if not line or line.startswith("#") or line.startswith("Screenshot "):
                    continue
                if encoder is not None and encoder.poll() is not None:
                    raise RuntimeError("GPU recorder exited before the tape completed")
                if line == "Show":
                    encoder = subprocess.Popen([
                        "gpu-screen-recorder", "-w", "region", "-region", args.region,
                        "-f", "60", "-fm", "cfr", "-k", "h264", "-q", "very_high",
                        "-cursor", "no", "-o", str(output),
                    ], stdout=capture_log, stderr=subprocess.STDOUT)
                    start = time.monotonic()
                    print("Recording visible portion of showcase.tape", flush=True)
                elif line == "Hide":
                    break  # Stop before q; leave the app open on the final frame.
                elif line.startswith("Sleep "):
                    duration = line.split()[1]
                    time.sleep(float(duration[:-2]) / 1000 if duration.endswith("ms") else float(duration[:-1]))
                elif line.startswith("Type "):
                    value = json.loads(line[5:])
                    for character in value:
                        began = time.monotonic()
                        send(character)
                        time.sleep(max(0, 0.06 - (time.monotonic() - began)))
                elif line.startswith("Backspace "):
                    for _ in range(int(line.split()[1])):
                        send("\x7f")
                elif line.startswith("Right@500ms "):
                    _, duration = position()
                    for _ in range(int(line.split()[1])):
                        began = time.monotonic()
                        send("\x1b[C")
                        time.sleep(max(0, 0.5 - (time.monotonic() - began)))
                    # Live Cider reads can overwrite optimistic rapid seeks.
                    # Verify the displayed position, topping up visibly if needed.
                    time.sleep(1)
                    for _ in range(10):
                        current, _ = position()
                        if current >= duration / 3 - 5:
                            break
                        send("\x1b[C")
                        time.sleep(0.75)
                    current, _ = position()
                    if not duration / 3 - 5 <= current <= duration / 3 + 12:
                        raise RuntimeError(f"Seek landed at {current}s, expected about {duration / 3:.0f}s")
                    print(f"Verified scrub: {current}s / {duration}s", flush=True)
                elif line in keys:
                    send(keys[line])
                else:
                    raise RuntimeError(f"Unsupported tape command at line {number}: {line}")
                if start is not None and not line.startswith("Sleep "):
                    at = time.monotonic() - start
                    timeline.append({"seconds": round(at, 2), "command": line})
                    print(f"{at:6.2f}s {line}", flush=True)
                    # Save text for review without touching any README images.
                    (pathlib.Path(tmp) / f"{number}.txt").write_bytes(call("get-text", "--match", "id:1"))
        finally:
            if encoder is not None:
                if encoder.poll() is None:
                    encoder.send_signal(signal.SIGINT)
                result = encoder.wait()
            capture_log.close()
        if encoder is None or result:
            raise RuntimeError("No successful video export")
        # /tmp may be a different filesystem: copy beside the destination
        # before replacing it, leaving the previous export intact on failure.
        with tempfile.NamedTemporaryFile(dir=root / "demo", suffix=".mp4", delete=False) as staged:
            staged_path = pathlib.Path(staged.name)
        try:
            subprocess.run([
                "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(output),
                "-an", "-c:v", "copy", "-movflags", "+faststart", str(staged_path),
            ], check=True)
            staged_path.chmod(0o644)
            staged_path.replace(root / "demo/showcase.mp4")
        finally:
            staged_path.unlink(missing_ok=True)
        # Review metadata belongs outside the repository.
        review = pathlib.Path(args.socket.removeprefix("unix:")).parent / "review"
        review.mkdir(exist_ok=True)
        for source in pathlib.Path(tmp).glob("*.txt"):
            source.replace(review / source.name)
        (review / "timeline.json").write_text(json.dumps(timeline, indent=2))
        print("Saved demo/showcase.mp4; README screenshots were not modified.")


if __name__ == "__main__":
    main()
