# Roadmap

Status legend: `[ ]` not started · `[~]` started · `[x]` done.

Design: [docs/superpowers/specs/2026-10-15-cider-tui-design.md](docs/superpowers/specs/2026-10-15-cider-tui-design.md)

## Public release gates (reviewed 2026-10-17)

**Not ready to tag yet.** Implementation is ahead of some historical cards;
completed features below do not imply release sign-off. Linux is the release
target. macOS/Windows playback and non-PipeWire capture remain out of scope.

- [x] **CID-33 Playlist next-track / Queue desynchronization (release blocker):**
  user reports that skipping to the next song in a playlist is not reflected
  in the displayed Queue. Root cause not yet verified; investigate playback
  order as well as stale queue rows/current-track marking (related: CID-14).
  Capture reproduction steps, shuffle/repeat state, now-playing and queue
  responses before/after a skip, including whether the skip was initiated in
  sidecue or Cider. Acceptance: next-track skips and natural advancement
  update now-playing and the Queue's current-track marker consistently with
  Cider's actual queue; verify playlist playback with shuffle on/off and
  duplicate tracks. Add regression coverage for track-change queue refresh and
  stale responses, plus a live reproduction/fix check before closing.
  Live check 2026-10-17 (Cider, 22-row playlist queue): 3 skips with shuffle on
  and 3 with shuffle off issued via the API; after each, sidecue's now-playing
  and Queue marker named the new track. Cider's queue response carried no
  isCurrent flag, so the marker came from the now-playing ID (unique in the
  queue each time). Natural advancement also verified (seeked to 5 s before
  the end; marker followed "Caustic" → "Fry"). Skips made in Cider's own
  window followed too (checked by the user). With the playing song queued
  twice, the Queue marked both copies; fixed so an ambiguous duplicate marks
  neither (currentQueueID now uses playingIndex), with a regression test.
  Fix verified live: with the duplicate queued, neither copy is marked.

- [~] **CID-34 Bottom-panel album metadata and compact layout:** include the
  **album name** alongside title/artist for song rows in the bottom browsing
  panel, and tighten metadata spacing so more useful information fits without
  increasing row height. This is metadata/layout polish, not additional album
  artwork or a change to playback behavior.
  - Agent instructions: inspect the shared row renderer and item metadata
    mapping first. Cover Queue, Search, Library and playlist/album track lists;
    use already-loaded album metadata, with no per-row API requests. Preserve
    current-track/selection indicators and existing details-panel behavior.
  - Propose a compact wide-terminal row and a narrow-terminal fallback before
    implementation. Prefer readable title/artist/album grouping; avoid repeating
    album names where the panel already supplies that context. Keep title and
    navigation usable when width is tight; gracefully omit missing metadata
    without empty separators. Do not invent album names from unrelated fields.
  - Use terminal-cell-aware truncation, including Unicode and styled text.
    Preserve borders, footer, artwork/lyrics layouts and selected-row contrast
    in all five themes. Do not silently change keybindings, sort or queue logic.
  - Acceptance: tests cover album present/absent, long titles/artists/albums,
    Unicode, selected/current rows and each affected tab at wide (120 columns),
    standard (80), narrow (40) and tiny widths. No overflow or extra row height;
    album is visible when room permits, and compact fallback remains readable.
    Provide before/after screenshots or rendered fixtures for layout review,
    update README/demo guidance, and run vet, unit and race tests. Record manual
    review separately; do not mark visual acceptance complete from tests alone.

  Implementation complete; visual review pending. Renderer at
  `ui/view.go:listLines` plus helpers `rowSubtitle` and
  `shouldShowAlbum`. Tests: `ui/cid34_test.go` (17 cases — wide,
  standard, narrow, tiny widths; album present/absent; Unicode;
  selected/current rows; every affected tab). Rendered fixtures:
  `docs/cid-34-examples.txt`. README + demo/README updated. vet, unit,
  race tests green as of 2026-10-17.

- [x] **CID-27 Restore a green baseline:** `go vet ./...` passes, but both
  `go test ./...` and `go test -race ./...` fail in
  `TestBottomBorderEnclosesAllTabsAndFooter`,
  `TestLyricsFetchAutomaticallyAndFollowSong`, and
  `TestYTogglesLyricsBandAndDoesNotHideQueue`. Resolve the missing border,
  lyrics rendering expectation, and tiny-terminal list visibility; distinguish
  regressions from stale assertions rather than deleting coverage.
- [x] **CID-28 Harden release/CI workflows:** replace Forgejo release ID parsing
  via greedy `sed` with a JSON parser and validate the top-level ID before
  uploading. Remove checkout credentials from `.git/config` before PR code runs
  and verify untrusted-PR token permissions. Run vet/tests in both tag workflows
  before publishing; add checksums. Acceptance: an approved release-candidate
  tag produces downloadable, versioned assets on both mirrors (CID-16).
- [ ] **CID-29 Live release acceptance:** run at least a 60-minute playing/paused
  soak covering CID-23, record CPU/memory and terminal responsiveness, exercise
  reconnect, shuffle/repeat and duplicate-track queue behavior, and verify
  quiet waveform, lyric transitions and missing-art fallback. Record terminal,
  Cider version and results; disclose unresolved cosmetic issues in release notes.
  Checklist and results template: [`docs/release-gate-cid-29.md`](docs/release-gate-cid-29.md).
- [x] **CID-30 Keybind context polish (proposal; no remap yet):** retain current
  bindings, make Escape leave fullscreen, make help visible there and prevent
  hidden-list actions while help is open, suppress unavailable search/settings
  hints, and explain filtered-Queue restrictions. Correct help's stale `y`
  label. Add mode-specific tests. Alternative: a configurable action map
  (CID-19), with more scope and migration risk; defer unless selected.
- [ ] **CID-31 Demo and docs sign-off:** follow [demo/README.md](demo/README.md)
  for all five themes and four appearance controls, persistence, browsing and
  transport. Record a full showcase separately from the existing teaser; verify
  artwork in a real graphics terminal and the block fallback separately. Review
  all media for private data and music permissions before publication. (The
  shot list, coverage matrix and off-camera acceptance checks in
  [demo/README.md](demo/README.md) are the runbook; nothing extra to author
  here, just run it.)
- [x] **CID-32 Artwork URL trust:** review scheme/host/redirect restrictions for
  API-supplied artwork URLs (`ui/artwork.go`). A compromised/custom Cider endpoint
  can currently cause requests to local/private addresses; artwork requests do
  not include the Cider token. Decide and test the supported trust boundary.
- [ ] **CID-35 Hide album art without kitty graphics:** when the terminal
  doesn't speak the kitty graphics protocol (including inside tmux/Screen),
  hide the cover entirely instead of drawing the Unicode quadrant-block
  fallback. The layout should collapse the art band cleanly, the way it does
  for a track with no art, and the Album art setting should say it's
  unavailable rather than toggle nothing. Acceptance: tests cover supported,
  unsupported and tmux terminals, with no blank band; README (High-resolution
  album art and Album art setting), HOWTO troubleshooting and ARCHITECTURE
  describe the new behaviour, and the block-art fallback mentions are removed.

Validation here was local and automated only: no live playback, recording,
release upload, or clean-machine artifact test was performed by this review.

## Done (2026-10-15)

- [x] Probed Cider 3.1.8's local API: auth header, token location, playback routes,
  POST body fields, Apple Music passthrough (`amapi/run-v3`), socket.io endpoint
- [x] Design spec and initial docs

## Next

Acceptance for any card: `go vet ./... && go test ./...` green, plus a new test
covering the change. Push to `main` only after review (see HOWTO.md).

- [x] **CID-00 Create the repos**: public `alex/sidecue` on git.thelunadog.com
  is `origin` (SSH, port 2222); public `ajpeders/sidecue` on GitHub is `github`.
  Both repos and remotes are set up. The reviewed `main` was pushed to both.
  (1) Acceptance: `git ls-remote origin` and `git ls-remote github` both show
  `main` at the local HEAD.
- [x] **CID-01 Scaffold + Cider client**: Go module, `cider` package with token
  discovery (`CIDER_TOKEN` → Cider's `spa-config.yml`), `CIDER_URL` override,
  `Get`/`Post`/`AMAPI`/paging, typed errors for not-running (connection refused)
  and bad token (403). `sidecue --check` prints Cider status, volume and library
  song count. (2) Acceptance: httptest-based unit tests for token discovery, each
  error type and paging over a `next` chain; `--check` works against live Cider.
- [~] **CID-02 CI**: `.forgejo/workflows/ci.yml` running `go vet` + `go test ./...`
  in a `golang` job container with the manual checkout over `forgejo:3000` (same
  pattern as snapmd). (1) Acceptance: pipeline green on the first push.
- [x] **CID-03 Now playing + controls**: Bubble Tea app with header (title, artist,
  album, progress bar, volume, shuffle/repeat state), 500 ms polling, keys for
  play/pause, next/prev, seek ±5 s, volume, shuffle, repeat, quit. First verify on
  live Cider the no-body toggle routes, the `seek.position` unit and the
  now-playing fields while playing; record them in the spec's "Not yet verified" table.
  "Cider isn't running" screen with auto-retry. (3) Acceptance: model tests map
  each key to the expected client call (fake client); manual check against live Cider.
- [x] **CID-04 Search tab**: `/` opens a query; catalog search (songs, albums,
   playlists) in the user's storefront; `enter` play, `a` enqueue at the end.
  First verify which `play-item` `type` values Cider accepts for catalog and
  library items and record them in the spec.
  (2) Acceptance: tests for result parsing and the key → `PlayItem`/`PlayNext`/
  `PlayLater` mapping.
- [x] **CID-05 Library tab**: songs and albums with lazy paging (13k+ songs; load
  the next page near the list end), album drill-down to tracks. (2) Acceptance:
  tests show paging requests only when scrolling near the end; drill-down loads
  the album's tracks.
- [x] **CID-06 Playlists tab**: library playlists → tracks; play a playlist from the
  selected track. First find a route that starts a playlist at a given track and
  record it in the spec; if none exists, play the selected track only and note it. (2) Acceptance: tests for playlist and track listing and the
  play-from-track call.
- [x] **CID-07 Queue tab**: show Cider's queue with the current item marked; refresh
  on track change. (1) Acceptance: test renders a recorded queue response.
- [x] **CID-08 Spectrum visualizer**: `viz` package, `pw-record` targeting Cider's
  PipeWire node (find the node name while playing and record it in the spec),
  no fallback (a sink target recorded the mic; removed 2026-10-16), FFT into log-spaced bands,
  cliamp-style rendering (MIT, credited). Missing `pw-record` disables it quietly.
  (3) Acceptance: FFT test with a synthetic sine lands in the right band; manual
  check that bars move with Cider audio and controls stay responsive.

- [~] **CID-11 Visualizer-first UI** (POC done 2026-10-16): cliamp-style look with
  the spectrum as the centrepiece. Spec:
  `docs/superpowers/specs/2026-10-16-visualizer-ui-design.md`. (2) Acceptance:
  view fits any terminal size (tested); bars move at ~60 fps with audio
  (verified with a test stream on a null sink) and rest when Cider is paused.
  - Reported 2026-10-17: the waveform (`b`) feels too reactive during quiet
    passages — single high spikes persist even when the playing audio is
    essentially silent. `renderWaveform` tracks full min/max per column and
    scales by `(height-1)/2`, so any spurious PCM sample (capture jitter,
    codec warm-up, DC offset) makes a column go full-height. Fixed in
    `ui/view.go` `renderWaveform` via a noise gate (samples below 0.01
    count as zero) and a per-column percentile clamp (drop top/bottom 10%
    on long columns; drop the single extreme sample per side on short
    columns). Tests: `TestWaveformFlattensSynthSilence` (synth silence with
    a DC offset, sub-floor jitter and one outlier sample renders as a flat
    centre line) and `TestWaveformPreservesRealSignal` (a half-sine signal
    still paints above and below the centre line).

- [x] **CID-22 Artist search**: the Search tab returns catalog artists
  (`types=…,artists`) and `enter` on one opens its top songs
  (`…/artists/{id}/view/top-songs`); the Library tab's `l` cycles
  songs/albums/artists (`library-artists`), with `enter` opening an artist's
  albums (`…/me/library/artists/{id}/albums`). (2) Acceptance: tests for the
   artist result type, the library-artists browse/search path and both
  drill-downs.

- [~] **CID-23 Freeze / high CPU**: on 2026-10-17 a `go run .` session sat at
  ~41% CPU after ~40 min and the terminal stopped responding (kitty itself was
  idle, so the app was the busy one). Root cause (by code reading, no profile):
  the 16 ms `animMsg` chain and the 50 ms `lyricsFrameMsg` chain re-queued
  themselves forever, so every idle/paused second still ran ~80 Update+View
  cycles. No duplicate chains were found (poll/queue/viz chains are single).
  Fix: both chains now rest when paused/settled (or no lyrics) and `wake()`
  restarts them idempotently via `animating`/`lyricsTicking` flags; tests in
  `ui/rest_test.go` cover all four "nothing animates" states (paused+settled,
  paused+lyrics, viz closed, empty lyrics) and `TestIdleLoopRunsZeroCycles`
  drives 50 ticks with no chain re-queueing. **Unconfirmed against a live
  40-min run**; pprof repro still pending. (2) Acceptance: reproduce with a CPU profile (`pprof`), fix,
  and a test that the redraw tick stops when there is nothing to animate.

## Later

- [~] **CID-09 Live updates over socket.io**: map Cider's socket events and replace
  polling where they cover it. (2) Acceptance: event names documented in the
  spec; a test feeds recorded events into the model; polling stays as the fallback
  when the socket drops.
- [~] **CID-10 Spotify export** (one-shot subcommand, not part of the TUI): `sidecue export-spotify` moves library songs to
  Liked Songs and recreates playlists, matching by ISRC with a title/artist search
  fallback; writes an unmatched report; resumable. Needs a Spotify developer app.
  (3) Acceptance: tests for ISRC/fallback matching and resume; a dry run on the
  live library reports the match rate. **Parked 2026-10-16**: removed from
  `main` for the public release; the code lives on the local `spotify-export`
  branch.
- [~] **CID-14 Reliable shuffle and queue editing**:
  **Active release-blocking report: CID-33**, playlist next-track changes not
  reflected in Queue. Current controls use Cider's
  shuffle/repeat state, not the historical client-side shuffle flag. Queue
  exposes jump, remove and clear, not move keys. Jump/remove are disabled while
  filtered; removal protects current/played entries and ambiguous positions.
  Live validation remains required: duplicate tracks, shuffle plus `P`, next
  after removal, clear behavior and recovery when Cider stops advancing. Do not
  promise queue reordering or reliable duplicate-track positioning yet.
- [ ] **CID-12 Spotify playback**: browse, search and control Spotify from the
  same UI, switching source with a key. Needs a player-backend interface first:
  the UI currently speaks Cider's HTTP routes directly. Spotify control goes
  through the Web API (Premium + a Spotify developer app, OAuth) and plays on
  a Spotify Connect device (the official client, or `spotifyd`/librespot
  headless). The visualizer captures that player's PipeWire stream (generalise
  discovery beyond Cider's binary). Open question: Spotify's queue API only
  appends and lists, so the Queue tab's move/remove keys won't carry over.
  (3) Acceptance: backend interface with Cider and Spotify implementations
  behind the existing model tests; manual check of play/skip/queue and
  visualizer on a live Spotify session.
- [ ] **CID-13 Local files**: play a local music folder (default `~/Music`).
  Cider can't be relied on for this, so sidecue drives its own player:
  `mpv` over its JSON IPC socket (play, queue, seek, volume), with a library
  built by scanning tags. Same player-backend interface as CID-12; the
  visualizer captures mpv's PipeWire stream. (3) Acceptance: scan test on a
  fixture folder (tags, missing tags, non-audio files skipped); backend tests
  against a fake mpv socket; manual playback check.
- [x] **CID-15 Public release readiness**: implementation groundwork done; the
  release gates above remain open. `.gitignore` (build output), token discovery via `os.UserConfigDir()` (XDG, macOS,
  Windows), `-h` lists flags and environment, README install and
  platform notes, MIT LICENSE, `--version` (ldflags or module version),
  `--check` reports visualizer readiness, `go 1.27.0` minimum, GitHub CI
  mirroring Forgejo's; Spotify export parked (CID-10). (1) Acceptance: tests for XDG token
  discovery and help text; `--check` works against live Cider.
- [~] **CID-16 Release binaries**: on a `v*` tag, build Linux amd64/arm64
  binaries with `-ldflags "-X main.version=<tag>"` and attach them to the
  Forgejo and GitHub releases, so trying sidecue doesn't need Go.
  (2) Acceptance: tagging produces both binaries on both releases, and each
  prints the tag from `--version`.
  Implemented (release workflows on both remotes); untested until the first tag.
  2026-10-17: fixed the Forgejo create-release URL (was `.../releases/releases`)
  and switched the GitHub workflow to `gh release` (GitHub's upload API differs).
- [~] **CID-17 Album artwork**: implemented for now-playing and selected-item
  details, using the kitty graphics protocol with Unicode quadrant-block
  fallback (including tmux/Screen), on-demand fetching and bounded disk caching.
  This is not a sixel implementation. Capability/cache tests exist; live
  terminal compatibility and missing-cover investigation remain open.
  - Reported 2026-10-17: covers are missing for some indie/small-label bands
    (e.g. alexsucks, "Small Pond"). Could be Apple's `artwork.url` being
    absent or non-resolvable for these artists, or our handling of the
     response shape. To investigate: capture the
    raw AMAPI response for `/v1/catalog/<sf>/songs/<id>` on a failing track
    and check whether the URL is empty vs present-but-broken.
  - Reported 2026-10-17: in kitty mode with album art on, the cover column's
    border disappears when nothing is playing. `artBand` sets `art = nil`
    when `m.now.Title == ""`, and the kitty branch skips the `panelBox`
    wrapper that block mode uses, so the left column becomes a column of
    blanks next to the bordered lyrics box on the right. Fix: render the
    same bordered placeholder as the block path does, just without a kitty
    image behind it (a centred "No art" line, or a min `m.coverRows`-tall
    dimStyle frame). The block-mode path is fine because `panelBox("", nil,
    w, n)` already draws the borders; need a regression test that asserts
    the borders are present in both modes at idle.
- [ ] **CID-18 macOS and Windows support**: the player builds on both but is
  untested (see README), and the visualizer is PipeWire-only, so it stays
  blank. Confirm play/search/library/queue on each platform, and add a second
  capture path (CoreAudio tap on macOS, WASAPI loopback on Windows) or make
  `--check` report clearly why the visualizer is unavailable. (3) Acceptance:
  manual playback check on at least one of the two OSes; tests cover the
  capture-backend selection; `--check` states the visualizer status on each
  platform.
- [ ] **CID-19 Customizable keybindings**: move the hardcoded key map out of
  `ui/model.go` into a small config file persisted next to `settings.json`
  (XDG/macOS/Windows paths from CID-15), with defaults, per-action overrides
  and a `?` help view generated from the live map. Unknown actions and
  conflicting keys are reported on load, not silently ignored. (2) Acceptance:
  tests for parsing, overriding and conflict detection; a changed binding
  takes effect without a rebuild.
- [ ] **CID-21 Desktop notifications**: notify on track change through the
  platform's notification service, with a toggle in Settings next to the
  theme and background options. Stays silent when no notification daemon is
  present. (2) Acceptance: a test shows a track change emits exactly one
  notification with title and artist; the toggle persists and suppresses it.
- [~] **CID-24 Copy selected item to clipboard**: a key (`Y`; `c` is taken by clear queue; implemented, manual cross-platform check pending)
  copies the highlighted row's metadata — title, artist, album — to the system
  clipboard, formatted as plain text (`Title — Artist · Album`). Works on the
  Queue, Library, Playlists and Search tabs; on the Queue tab it copies the row
  under the cursor, not the now-playing song (matching how `enter`/`x`/`a` key
  off that cursor). Backend: detect the host's clipboard tool at startup —
  macOS `pbcopy`, Windows `clip.exe`, Linux `wl-copy` (Wayland) or `xclip`/`xsel`
  (X11), in that preference order — and fall back to a clear "no clipboard
  tool found" status line if none is available. `os/exec` only; no CGO. (2)
  Acceptance: tests stub the clipboard backend and assert the formatted string
  and which tool was invoked; manual check on Linux/X11, Linux/Wayland, macOS
  and Windows copies the text into a real editor.
- [ ] **CID-26 Layout jump when lyrics change**: reported 2026-10-17 — the screen
  visibly jumps when a particular lyric line comes up (or transitions from
  placeholder to a real line). The outer row count is stable in unit tests
  (`lyricsBandRows()` is a pure function of `m.height` + `m.fullScreenViz()`,
  and `fixedRows()`/`layout()` always agree on it), so the jump is not a
  band-height reflow. Suspects, in order: (1) inner-panel scroll — a long
  lyric that wraps to 2–3 rows pushes the centered current line down by a row
  mid-frame; (2) width wobble — different `selectedStyle`/`dimStyle` styling
  for the current line and its neighbours changes the cell width of the same
  text and the terminal re-renders; (3) the lyrics fetch landing — replacing
  the centred "Loading lyrics…" placeholder with the real scroll jumps the
  panel's contents in one frame. To investigate: log `len(View().split('\n'))`
  and `lipgloss.Width` of each row around a suspected lyric to see whether the
  cell count truly is constant; if it is, the fix is band-internal (e.g. cap
  wrap to a fixed number of inner rows so the centred line never slides more
  than one row per lyric change). Test: render the same model at `n=4` and
  `n=6` body rows for the same lyric list and assert the centred current line
  stays on the same row offset for every lyric.
