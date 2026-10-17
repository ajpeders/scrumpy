# Demo

[Watch the kitty showcase](showcase.mp4): native album art and synced lyrics,
catalog search-to-play, playlist enqueueing, shuffle and skips, all five themes,
and spectrum/waveform plus fullscreen. The sequence comes from `showcase.tape`.

## Native-art showcase video (kitty)

The video is captured from a real **kitty** window using GPU Screen Recorder,
including rendered kitty-graphics images. `kitten @` drives the TUI, but is not
used for per-frame screenshots: that approach introduced visible lag. VHS
cannot capture native kitty graphics. The MP4 is muted (no system or microphone audio).

Requirements: kitty (recorded with 0.49.2), GPU Screen Recorder with region
capture support, Python 3, ffmpeg, Cider and the usual sidecue/PipeWire prerequisites. Prepare a
disposable queue: this run plays **Hard to Explain — The Strokes**, then
**I Wonder — Kanye West**, appends playlist **102026**, toggles shuffle twice,
and skips twice. It visibly seeks about one-third into **I Wonder** but does
not edit your library. Start with shuffle off.
Seek keys are paced to let Cider respond; the recorder verifies the displayed
position and adds a few visible five-second steps if rapid seeks landed short.
The script expects a fresh Dark/Spectrum appearance profile. Shell setup is
performed before launch; all TUI keys, theme order and holds follow
`showcase.tape`. Its `Screenshot` commands are deliberately skipped: README
screenshots are managed separately and are never overwritten by this recorder.

Build and launch off-camera from the repository root:

```sh
D=$(mktemp -d /tmp/opencode/kitty-demo.XXXXXX)
mkdir -p "$D/config/sidecue" "$D/cache"
ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" "$D/config/sh.cider.genten"
printf '%s\n' '{"theme":"Dark","background":true,"visualizer":"spectrum","artwork":true,"lyrics":true}' > "$D/config/sidecue/settings.json"
go build -o "$D/sidecue" .
XDG_CONFIG_HOME="$D/config" XDG_CACHE_HOME="$D/cache" kitty --detach --start-as normal \
  --config NONE --class sidecue-video --title sidecue-video \
  --listen-on "unix:$D/kitty.sock" -o allow_remote_control=socket-only \
  -o font_size=12 -o background_opacity=1 -o window_padding_width=12 \
  -o confirm_os_window_close=0 -e env -u TMUX -u STY "$D/sidecue"
```

Float/resize that window to **1400×850 pixels** using your window manager.
Wayland tiling compositors may ignore kitty's requested initial size. On the
Lua-based Hyprland used for this capture, register this rule **before launch**:

```sh
hyprctl eval 'hl.window_rule({ match = { class = "sidecue-video" }, float = true, size = "1400 850", center = true })'
```

Keep the capture window visible, unobscured and fixed in place throughout
recording. Hide notifications and turn off inactive-window dimming for the
capture window. Verify the sharp cover and synced lyrics are visible. On
Hyprland, get its client region and run in the original shell:

```sh
REGION=$(hyprctl clients -j | jq -r '.[] | select(.class=="sidecue-video") | "\(.size[0])x\(.size[1])+\(.at[0])+\(.at[1])"')
python3 demo/record-kitty.py --socket "unix:$D/kitty.sock" --region "$REGION"
```

To keep a song you have already selected in Cider for the opening instead of
starting the tape's Hard to Explain, add `--current-song`. Playback must already
be running; the rest of the tape, including the visible seek during I Wonder,
is unchanged.

This replaces **only `showcase.mp4`**. GPU region capture records at 60 fps
with the mouse pointer excluded. Anything overlapping that region, including
notifications or other windows, would be captured: keep it unobscured.
The script exports the MP4 only after playback of the tape completes; timed
keys do not prove successful actions. Review the full video before publishing,
including artwork, lyrics, track changes and personal queue names. UI text and
a timeline are saved beside the socket in `review/` for verification.
Close the capture window normally afterward. No commit or upload is performed.

Export the smaller full-length GIF after review:

```sh
ffmpeg -y -i demo/showcase.mp4 -filter_complex \
  '[0:v]fps=8,scale=700:-1:flags=lanczos,split[a][b];[a]palettegen[p];[b][p]paletteuse' \
  -loop 0 demo/showcase.gif
```

### Latest kitty export review (2026-10-17)

`showcase.mp4` is **106.92 seconds**, 1400×850, H.264 at **60 fps**, with no
audio. The full stream decodes without errors; one-second samples across the
entire cut were visually reviewed. Confirmed: native album art, Headlines as
the opening track, catalog playback of I Wonder, visible scrubbing to
**1:20 / 4:03**, `Queued: 102026`, both shuffle states, two skips advancing to
I Like It Like That and Heaven Is a Bedroom, all five labeled themes, both
visualizers and fullscreen. The full-length 700-pixel GIF was regenerated.
The video-only recorder did not overwrite any README screenshots. This is a
demo review, not sign-off of the full release acceptance checklist.

### README screenshots (kitty)

The seven README PNGs show **Hard to Explain — The Strokes** playing in the
catalog Search view. They were captured directly from a 1400×900 kitty window
with `kitten @ --to "unix:$D/kitty.sock" screenshot --match id:1
demo/screenshot-player.png` (substitute each screenshot filename while its
theme or fullscreen view is visible). The isolated profile keeps theme settings
out of the user's normal configuration; screenshots include kitty-native album
art, unlike the legacy VHS captures below. Review every image before publishing.

### Legacy VHS captures (block-art fallback only)

Play a song with lyrics in Cider, then run `vhs demo/screenshots.tape`. It
only searches (never changes playback) and shows catalog results rather than
your library. VHS has no kitty graphics, so covers render as block art.

## Re-record the legacy VHS teaser

1. Start Cider and enable its local API. The tape starts Hard to Explain
   itself (off camera) and seeks to about the middle, so whatever is playing
   will be replaced.
2. Make sure the library has a playlist named **102026**, and that the first
   catalog results for "Hard to Explain The Strokes" and "I Wonder Kanye West"
   are those songs.
3. From the repository root, run `go run . --check`.
4. With VHS installed, run `vhs demo/demo.tape`.

The tape builds the current working tree, uses an isolated settings directory
under `/tmp/opencode`, and reads the existing Cider token without displaying it.
It changes playback but does not remove queue entries or modify the library.
Review the recording for visible private information before publishing.

`demo.mp4` remains the older block-art teaser. The generated `demo.gif` is
ignored to avoid large Git history. Prefer `showcase.mp4` for native artwork.

## Legacy VHS feature showcase (under two minutes)

For a block-art fallback recording only, run `vhs demo/showcase.tape` from the repository root with Cider signed in
and playlist **102026** available. This builds the working tree and exports
`showcase.mp4` and `showcase.gif`, using isolated appearance settings and the
existing token without showing it. It changes real playback and queues that
playlist; its real track names are visible. The updated tape includes visible
scrubbing during I Wonder.

The cut shows the playlist's enqueue confirmation, tracks and populated Queue,
both shuffle states with a next-track skip between them, catalog search-to-play,
all five labeled themes, and spectrum/waveform plus fullscreen. Light and Paper
are also held in normal view; Paper has the longest hold and closes the cut.
The tape captures `screenshot-paper.png` for the README. Settings navigation
accounts for the active theme when reopening. Verify the frames and duration
(maximum 120 seconds); timed keystrokes alone do not prove successful actions.
This is focused feature coverage, not completion of the release gates below.

### Previous VHS export review (2026-10-17; superseded by kitty capture)

`showcase.mp4` is **95.88 seconds** at 1400×850. One-second contact sheets
and full-size README screenshots were reviewed. Confirmed: catalog playback,
`Queued: 102026`, playlist tracks in Queue, two post-shuffle skips advancing
to a different track (Hard to Explain → No Time (feat. Gunna)), all five
labeled themes held in the waveform view, both visualizers and fullscreen.
Album art is enabled in the isolated settings but VHS still uses the block
fallback, so cover quality is not demonstrated in this recording.

Remaining observations (not passed acceptance checks):

- The opening 0–2 seconds show a queue timeout/retry before rows recover.
- This VHS export did not demonstrate high-resolution covers. Use the kitty
  capture procedure above for the README assets instead.
- No “Nothing playing” flash was seen in the sampled frames. Its original live
  cause remains unconfirmed; regression tests cover explicitly confirmed playback
  with temporary empty metadata, including fresh shuffle/repeat labels.

Vet, unit tests and race tests pass for the fixes. Real-terminal Background
on/off checks remain outside this VHS review. Updated player, terminal,
Dark, Light, Paper, High contrast and fullscreen PNGs are embedded in
the main README.

## Full public-release showcase (5–7 minutes)

The existing tape is a short teaser, **not** coverage of all themes/options.
Use this manual runbook for the full showcase: live API timing, search results
and graphics support make blind timed keystrokes unsuitable for acceptance.
Record a separate `demo/full-demo.mp4` after rehearsal; it has not been produced
as part of the release review.

### Prepare

1. Run `go vet ./...`, `go test ./...`, and `go test -race ./...` on the exact
   candidate. Build once; do not record a compile or a token setup screen.
2. Use Linux with Cider signed in, `pw-record`/`pw-dump` installed and a working
   clipboard helper. Run `--check` off-camera. Confirm bars actually move with
   Cider and settle on pause; a successful prerequisite check is not enough.
3. Pick a track with verified synced lyrics and cover art, a second track for
   skipping, a small demo playlist, and a known search query. Verify the exact
   result before pressing Enter—do not assume the first result is the right one.
   Use music you have permission to publish; an Apple Music subscription alone
   does not establish recording/distribution rights. A muted recording is an
   option, but still review visible lyrics and artwork before publishing.
4. Save your queue externally if needed, note shuffle/repeat/volume, and use a
   disposable demo queue/playlist. Play-from-here and `P` replace playback;
   queue clear/remove are real mutations, not a sandbox. Do not edit the library.
5. Hide notifications, personal playlist names, account details and shell
   history. Keep API credentials off-camera. Never print or copy config files
   containing tokens into the recording or a bug report.
6. Start with a stable wide terminal (roughly 120×40 cells or larger), a readable
   font, and the same size for each theme. Use a real kitty-protocol terminal
   outside tmux/Screen for high-resolution artwork. Record the block fallback
   separately; do not claim the VHS browser terminal proves kitty support.

For an isolated Linux appearance/cache profile, run this **off-camera** from
the repository root. It does not copy tokens or modify Cider's config; the
symlink allows normal token discovery. If using `CIDER_TOKEN`, keep that existing
environment variable private instead. This still controls your real Cider.

```sh
D=$(mktemp -d /tmp/opencode/cider-showcase.XXXXXX)
mkdir -p "$D/config/sidecue" "$D/cache"
ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" "$D/config/sh.cider.genten"
printf '%s\n' '{"theme":"Dark","background":true,"visualizer":"spectrum","artwork":true,"lyrics":true}' > "$D/config/sidecue/settings.json"
go build -o "$D/sidecue" .
XDG_CONFIG_HOME="$D/config" XDG_CACHE_HOME="$D/cache" "$D/sidecue" --check
XDG_CONFIG_HOME="$D/config" XDG_CACHE_HOME="$D/cache" "$D/sidecue"
```

Keep `$D` in that shell for the restart test. Leave the temporary profile out of
Git. Exit normally to restore the terminal; return Cider's playback settings to
your preferences after recording.

### Shot list

| Time | Demonstration | Keys and success check |
| --- | --- | --- |
| 0:00–0:25 | Cold open: Queue, spectrum, song and synced lyrics | Start already playing; no shell prompt or setup. Show lyrics tracking the song. |
| 0:25–1:00 | Playback controls | Space pause/resume; Left/Right seek; small `+`/`-` volume adjustment; `n`/`p` skip; `s` and `r` show state labels. Restore original shuffle/repeat before continuing. |
| 1:00–2:00 | Browse and discover | `1` Search, `/` known query, Enter search; inspect result, Enter play/open. `l` cycles Songs/Albums/Artists. `2` Library, `l` types, `o` sort; `3` Playlists and Enter drill-down; Escape back. Wait for results between actions. The bottom panel renders rows as `Title    Artist · Album`; the album is suppressed on tabs where the container is already named (Library albums/artists, Playlists tab, album-tracks drill-down). |
| 2:00–2:40 | Details and queue | `d` details, `Y` copy into a clean editor, `d` close. `a` enqueue from outside Queue; `4` inspect. Demonstrate Enter jump only unfiltered. Explain `P` plays the whole playlist/album, and demonstrate only on the disposable playlist. |
| 2:40–4:10 | Theme carousel | `,` Settings; Up/Down select each theme and Enter apply. Show Terminal → Dark → Light → Paper → High contrast, holding each 5–8 seconds with the same song/layout. Escape returns. Do not assume Settings starts at row zero: it selects the current theme. |
| 4:10–5:10 | Every appearance option | Follow the coverage matrix below: Background, Visualizer, Album art, Lyrics. Use settings labels rather than counting blind keypresses. |
| 5:10–5:40 | Visualizer payoff | Escape out of Settings; `b` compare spectrum/waveform, `v` fullscreen, `v` return. `y` hide/show lyrics while the list remains visible. |
| 5:40–6:10 | Discoverability and persistence | `?` help in normal view, then close with `?`. Quit and relaunch with the same isolated profile; show saved appearance. End on the player, not the shell. |

### Themes/options coverage matrix

Check off both directions of each toggle during rehearsal, not just the pretty
state. Hold each state long enough to see lyrics, selection and the visualizer.

- [ ] **Terminal:** inherits terminal colors; test readability on your actual
  terminal palette (it does not force a dark/light background).
- [ ] **Dark:** metadata, selected row, borders and spectrum readable.
- [ ] **Light:** same checks, including dim text.
- [ ] **Paper:** monochrome controls, borders and visualizer on paper background.
- [ ] **High contrast:** selection, muted text and active shuffle/repeat distinct.
- [ ] **Background on/off:** for every theme, verify off preserves the terminal
  background; change themes while off and confirm it stays off. Restore on.
- [ ] **Visualizer Spectrum/Waveform:** select both in Settings. Restart to
  verify the saved choice. `b` is a session-only switch, not a settings save.
- [ ] **Album art off/on:** show now-playing and selected-item details artwork;
  separately check a missing cover, narrow layout and block fallback. A warm
  cache is useful for recording, but also rehearse a cold fetch off-camera.
- [ ] **Lyrics on/off:** show space reclaimed when off and that `y` is disabled;
  restore on, then use `y` to hide/show temporarily. Escape does not undo `y`.
- [ ] **Persistence:** save a distinctive theme/options combination, quit,
  restart using the same `$D/config`, and verify all five saved fields.

### Off-camera acceptance checks

- Check wide, 80×24 and tiny terminals for clipping/borders; resize during
  playback and while lyrics change. The tiny-terminal test
  (`TestYTogglesLyricsBandAndDoesNotHideQueue` at 8×5) is green in the suite as
  of CID-27 — still rehearse it on the actual candidate so you see what users
  see at that size.
- Test queue remove (`x`) and clear (`c`) only against the disposable queue.
  Filter with `/`, clear the filter with Escape before jump/remove, and verify
  duplicate-track/current-track safeguards. Capture unexpected results.
- Test missing clipboard helper, no lyrics, no artwork and no PipeWire tools;
  playback should remain usable. Record the actual result, not an assumption.
- Stop/restart Cider to rehearse offline/reconnect; run the 60-minute playing/
  paused soak separately from the short recording.
- Current keybind caveats: help is modal — list actions (`j`/`k`, `d`,
  `enter`, `a`, `x`, `o`, `tab`, …) are blocked while help is open; transport
  keys still work. Settings keeps transport live (Left/Right seek; Up/Down
  select). Search input captures typed playback keys.

### Publish only after review

Watch the entire export for secrets/private names, wrong tracks, blank art,
lyric jumps, unreadable themes and clipping. Add captions naming each theme and
option, plus Cider/app version, Linux/terminal and known limitations. Verify
music/media permissions, link the final video from the README/release notes,
and keep a short muted teaser for fast loading. Do not replace the existing
teaser until the new recording has been reviewed.
