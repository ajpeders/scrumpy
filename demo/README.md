# Demo

[Watch the short demo](demo.mp4) (under a minute): it opens mid-song on
**Hard to Explain — The Strokes** with the visualizer and synced lyrics, then
plays **I Wonder — Kanye West** from catalog search, opens the **102026**
playlist, the details panel and the Queue, and ends on the waveform and
full-screen visualizer.

## Re-shoot the README screenshots

Play a song with lyrics in Cider, then run `vhs demo/screenshots.tape`. It
only searches (never changes playback) and shows catalog results rather than
your library. VHS has no kitty graphics, so covers render as block art.

## Re-record the teaser

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

`demo.mp4` is tracked; the generated `demo.gif` is ignored to avoid large Git
history. Prefer the MP4 for the full demo and a short GIF cutdown for embeds.

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
printf '%s\n' '{"theme":"Dark","background":true,"visualizer":"spectrum","artwork":false,"lyrics":true}' > "$D/config/sidecue/settings.json"
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
