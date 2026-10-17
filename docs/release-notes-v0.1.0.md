# sidecue v0.1.0 — release notes (draft)

First public release. sidecue is a terminal player for Apple Music on Linux:
[Cider](https://cider.sh) does the playback, and sidecue controls it over
Cider's local API, adding search, library, playlists, queue, lyrics, album art
and spectrum/waveform visualizers.

## Install

Download `sidecue-linux-amd64` or `sidecue-linux-arm64` from this release,
check it against `SHA256SUMS`, then `chmod +x` and run it. Or build:
`go install github.com/ajpeders/sidecue@latest` (Go 1.27+).

## Requirements

- Cider (tested with 3.1.8), signed in to Apple Music
- A Cider API token (Settings → Connectivity → Manage External Application
  Access); sidecue finds it automatically
- PipeWire `pw-record` for the visualizer (optional)
- A kitty-graphics terminal (kitty, ghostty, wezterm) for full-resolution
  album art; other terminals get block-art covers

Run `sidecue --check` first; see HOWTO.md for troubleshooting.

## Highlights

- Transport, volume, shuffle/repeat, seek
- Search (songs/albums/artists), library, playlists, queue editing
- Synced lyrics, details panel, album art, copy row to clipboard (`Y`)
- Spectrum and waveform visualizers, fullscreen mode, five themes
- Settings persist in `~/.config/sidecue/settings.json`

## Known issues

- Linux only. macOS/Windows builds are untested, and the visualizer needs
  PipeWire.
- Under tmux/Screen, album art falls back to block art.
- If the playing song is queued more than once, Cider doesn't say which copy
  is playing, so the Queue marks neither copy and refuses position-based edits
  on them rather than guess.
- <!-- add cosmetic issues found in the CID-29 soak -->

## Validation

Live acceptance results: [docs/release-gate-cid-29-results.md](release-gate-cid-29-results.md).
