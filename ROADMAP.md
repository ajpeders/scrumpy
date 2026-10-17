# Roadmap

Status legend: `[ ]` not started · `[~]` started · `[x]` done.

Design: [docs/superpowers/specs/2026-10-15-sidecue-design.md](docs/superpowers/specs/2026-10-15-sidecue-design.md)

## Done (2026-10-15)

- [x] Probed Cider 3.1.8's local API: auth header, token location, playback routes,
  POST body fields, Apple Music passthrough (`amapi/run-v3`), socket.io endpoint
- [x] Design spec and initial docs

## Next

Acceptance for any card: `go vet ./... && go test ./...` green, plus a new test
covering the change. Push to `main` only after review (see HOWTO.md).

- [x] **CID-00 Create the repos**: public `alex/sidecue` on git.thelunadog.com
  is `origin` (SSH, port 2222); public `ajpeders/sidecue` on GitHub is `github`.
  Both repos and remotes are set up. Push `main` to both only after alex OKs the diff.
  (1) Acceptance: `git ls-remote origin` and `git ls-remote github` both show
  `main` at the local HEAD.
- [ ] **CID-01 Scaffold + Cider client**: Go module, `cider` package with token
  discovery (`CIDER_TOKEN` → Cider's `spa-config.yml`), `CIDER_URL` override,
  `Get`/`Post`/`AMAPI`/paging, typed errors for not-running (connection refused)
  and bad token (403). `sidecue --check` prints Cider status, volume and library
  song count. (2) Acceptance: httptest-based unit tests for token discovery, each
  error type and paging over a `next` chain; `--check` works against live Cider.
- [ ] **CID-02 CI**: `.forgejo/workflows/ci.yml` running `go vet` + `go test ./...`
  in a `golang` job container with the manual checkout over `forgejo:3000` (same
  pattern as snapmd). (1) Acceptance: pipeline green on the first push.
- [ ] **CID-03 Now playing + controls**: Bubble Tea app with header (title, artist,
  album, progress bar, volume, shuffle/repeat state), 500 ms polling, keys for
  play/pause, next/prev, seek ±5 s, volume, shuffle, repeat, quit. First verify on
  live Cider the no-body toggle routes, the `seek.position` unit and the
  now-playing fields while playing; record them in the spec's "Not yet verified" table.
  "Cider isn't running" screen with auto-retry. (3) Acceptance: model tests map
  each key to the expected client call (fake client); manual check against live Cider.
- [ ] **CID-04 Search tab**: `/` opens a query; catalog search (songs, albums,
  playlists) in the user's storefront; `enter` play, `a` play next, `A` play later.
  First verify which `play-item` `type` values Cider accepts for catalog and
  library items and record them in the spec.
  (2) Acceptance: tests for result parsing and the key → `PlayItem`/`PlayNext`/
  `PlayLater` mapping.
- [ ] **CID-05 Library tab**: songs and albums with lazy paging (13k+ songs; load
  the next page near the list end), album drill-down to tracks. (2) Acceptance:
  tests show paging requests only when scrolling near the end; drill-down loads
  the album's tracks.
- [ ] **CID-06 Playlists tab**: library playlists → tracks; play a playlist from the
  selected track. First find a route that starts a playlist at a given track and
  record it in the spec; if none exists, play the selected track only and note it. (2) Acceptance: tests for playlist and track listing and the
  play-from-track call.
- [ ] **CID-07 Queue tab**: show Cider's queue with the current item marked; refresh
  on track change. (1) Acceptance: test renders a recorded queue response.
- [ ] **CID-08 Spectrum visualizer**: `viz` package, `pw-record` targeting Cider's
  PipeWire node (find the node name while playing and record it in the spec),
  fallback to the default sink monitor, FFT into log-spaced bars at ~30 fps,
  cliamp-style rendering (MIT, credited). Missing `pw-record` disables it quietly.
  (3) Acceptance: FFT test with a synthetic sine lands in the right band; manual
  check that bars move with Cider audio and controls stay responsive.

## Later

- [ ] **CID-09 Live updates over socket.io**: map Cider's socket events and replace
  polling where they cover it. (2) Acceptance: event names documented in the
  spec; a test feeds recorded events into the model; polling stays as the fallback
  when the socket drops.
- [ ] **CID-10 Spotify export** (one-shot subcommand, not part of the TUI): `sidecue export-spotify` moves library songs to
  Liked Songs and recreates playlists, matching by ISRC with a title/artist search
  fallback; writes an unmatched report; resumable. Needs a Spotify developer app.
  (3) Acceptance: tests for ISRC/fallback matching and resume; a dry run on the
  live library reports the match rate.
