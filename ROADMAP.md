# Roadmap

Status legend: `[ ]` not started · `[~]` started · `[x]` done.

Design: [docs/superpowers/specs/2026-10-15-cider-tui-design.md](docs/superpowers/specs/2026-10-15-cider-tui-design.md)

## Done (2026-10-15)

- [x] Probed Cider 3.1.8's local API: auth header, token location, playback routes,
  POST body fields, Apple Music passthrough (`amapi/run-v3`), socket.io endpoint
- [x] Design spec and initial docs

## Next

Acceptance for any card: `go vet ./... && go test ./...` green, plus a new test
covering the change. Push to `main` only after review (see HOWTO.md).

- [x] **CID-00 Create the repos**: public `alex/cider-tui` on git.thelunadog.com
  is `origin` (SSH, port 2222); public `ajpeders/cider-tui` on GitHub is `github`.
  Both repos and remotes are set up. The reviewed `main` was pushed to both.
  (1) Acceptance: `git ls-remote origin` and `git ls-remote github` both show
  `main` at the local HEAD.
- [x] **CID-01 Scaffold + Cider client**: Go module, `cider` package with token
  discovery (`CIDER_TOKEN` → Cider's `spa-config.yml`), `CIDER_URL` override,
  `Get`/`Post`/`AMAPI`/paging, typed errors for not-running (connection refused)
  and bad token (403). `cider-tui --check` prints Cider status, volume and library
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
  playlists) in the user's storefront; `enter` play, `a` play next, `A` play later.
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

## Later

- [~] **CID-09 Live updates over socket.io**: map Cider's socket events and replace
  polling where they cover it. (2) Acceptance: event names documented in the
  spec; a test feeds recorded events into the model; polling stays as the fallback
  when the socket drops.
- [~] **CID-10 Spotify export** (one-shot subcommand, not part of the TUI): `cider-tui export-spotify` moves library songs to
  Liked Songs and recreates playlists, matching by ISRC with a title/artist search
  fallback; writes an unmatched report; resumable. Needs a Spotify developer app.
  (3) Acceptance: tests for ISRC/fallback matching and resume; a dry run on the
  live library reports the match rate. **Parked 2026-10-16**: removed from
  `main` for the public release; the code lives on the local `spotify-export`
  branch.
- [ ] **CID-14 Reliable shuffle and queue editing**: shuffle and the Queue
  tab's move/remove keys are unreliable in practice (2026-10-16). Known
  causes: Cider's GET `queue` lists played songs and duplicates, so the
  current position is guessed from the playing song's ID (edits now refuse
  when it's ambiguous); moving entries around the wrong position wedged
  Cider; `s` reorders the queue client-side with `move-to-position`, and the
  `shuffle` indicator is cider-tui's own flag, not state read from Cider.
  To investigate: a reliable source for the current index (socket.io
  events, CID-09), whether Cider's own shuffle mode plus `P` covers the need
  instead, and recovery when Cider stops advancing. (3) Acceptance: on a live
  queue, `s` reorders only upcoming songs, the indicator matches what plays,
  and `n` keeps working after shuffles and moves.
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
  Cider can't be relied on for this, so cider-tui drives its own player:
  `mpv` over its JSON IPC socket (play, queue, seek, volume), with a library
  built by scanning tags. Same player-backend interface as CID-12; the
  visualizer captures mpv's PipeWire stream. (3) Acceptance: scan test on a
  fixture folder (tags, missing tags, non-audio files skipped); backend tests
  against a fake mpv socket; manual playback check.
- [x] **CID-15 Public release readiness**: `.gitignore` (build output), token discovery via `os.UserConfigDir()` (XDG, macOS,
  Windows), `-h` lists flags and environment, README install and
  platform notes, MIT LICENSE, `--version` (ldflags or module version),
  `--check` reports visualizer readiness, `go 1.27.0` minimum, GitHub CI
  mirroring Forgejo's; Spotify export parked (CID-10). (1) Acceptance: tests for XDG token
  discovery and help text; `--check` works against live Cider.
- [ ] **CID-16 Release binaries**: on a `v*` tag, build Linux amd64/arm64
  binaries with `-ldflags "-X main.version=<tag>"` and attach them to the
  Forgejo and GitHub releases, so trying cider-tui doesn't need Go.
  (2) Acceptance: tagging produces both binaries on both releases, and each
  prints the tag from `--version`.
