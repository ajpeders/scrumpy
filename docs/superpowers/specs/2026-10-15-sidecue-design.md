# sidecue — design

Date: 2026-10-15 · Status: approved in chat, pending written review

## Goal

A cliamp-style terminal player for Apple Music on the Arch desktop. Cider does the
playback (Apple Music streams are DRM-protected, so only Cider can decode them);
sidecue is a remote control with a now-playing view, search, library, playlists,
queue and a spectrum visualizer.

Non-goals: decoding audio, an equalizer, gapless control, running without Cider,
any GUI. A fork of cliamp was rejected: cliamp decodes every source itself
(~174k lines of Go built around its own audio pipeline), and Cider cannot hand it audio.

## Verified Cider facts (Cider 3.1.8, probed 2026-10-15)

- Server: `http://localhost:10767`, binds `0.0.0.0` (LAN-reachable).
- Auth: header `apptoken: <token>`. Without it every route returns
  `403 {"error":"UNAUTHORIZED_APP_TOKEN"}`.
- Token lives in `~/.config/sh.cider.genten/spa-config.yml` under
  `connectivity.apiTokens[].token` (with `apiTokensRequired: true`).
- GET `/api/v1/playback/{active,is-playing,now-playing,volume,queue,repeat-mode,shuffle-mode,autoplay}`
  return `{"status":"ok", ...}` JSON (`queue` returns a bare array).
- POST bodies are validated (zod); confirmed field names:
  `play-item {type,id}`, `play-item-href {href}`, `play-next {type,id}`,
  `play-later {type,id}`, `seek {position}`, `volume {volume}`.
  Unknown routes return 404, so route existence is cheap to check.
- POST `/api/v1/amapi/run-v3 {"path": "/v1/..."}` proxies the Apple Music API with
  the signed-in user's token; response is `{"data": <Apple response>}`.
  Verified: `/v1/me/library/songs` (13,274 songs, paged via `next`),
  `/v1/me/library/playlists`, `/v1/catalog/us/search`, `/v1/me/storefront`.
- socket.io (EIO 4) is served at `/socket.io/`; event names not yet mapped.
- Volume is a float 0–1 (GET returned `0.82402`). Library lists carry
  `meta.total`, so counts need only `?limit=1`.
- Idle `now-playing` (nothing loaded) returns only
  `{"status":"ok","info":{inFavorites, inLibrary, shuffleMode, repeatMode}}`:
  no title, position or duration. Every `now-playing` response nests its data
  under `info` (see "Verified live").

### Verified live (Cider 3.1.8, probed 2026-10-16)

Probing with real playback resolved the earlier unknowns:

- **now-playing while playing** nests everything under `info` (idle returns only
  `{inFavorites, inLibrary, shuffleMode, repeatMode}` there too). Playing fields:
  `info.name` (title), `info.artistName`, `info.albumName`,
  `info.currentPlaybackTime` (seconds), `info.remainingTime` (seconds),
  `info.durationInMillis`, `info.shuffleMode` (int), `info.repeatMode` (int),
  `info.playParams.id` (track id). The client and UI decode this shape.
- **No-body control routes** `playpause`, `next`, `previous`, `toggle-shuffle`,
  `toggle-repeat` all return `{"status":"ok"}`.
- **`seek.position` is absolute seconds** (seek to 30 → `currentPlaybackTime`
  ≈30.5, seek to 5 → ≈5.5).
- **`play-item` `type` values**: `library-songs`, `songs` (catalog), and
  `library-albums` all start playback and populate the queue. `library-playlists`
  and `playlists` return `{"status":"ok"}` but leave the queue empty — no
  playlist-by-id playback route was found.
- **Playlist tracks**: `/v1/me/library/playlists/{id}/tracks` works for
  user-created playlists (e.g. 600 tracks) but Apple-curated library playlists
  return `40403 No related resources`. Because playlist-by-id playback does not
  work, the Playlists tab lists tracks and plays the **selected track only**;
  playlist-level playback is a known gap (recorded in ROADMAP.md CID-06).
  Update 2026-10-16: `play-item` on a song **replaces the queue** with that one
  song, and `autoplay` is off by default, so playback stopped after it. `enter`
  now plays the song and queues up to 100 following songs with `play-later`
  (verified: `play-next` inserts right after the current track); `P` queues a
  playlist's loaded tracks the same way, shuffling them locally when shuffle is
  on. Cider's queue entries carry no current flag (`_state.current` marks loaded
  items); the playing entry is found by matching now-playing's `playParams.id`.
- **Queue routes** (found 2026-10-16 by POSTing `{}` and reading the zod
  errors): `queue/move-to-position {startIndex, destinationIndex}`,
  `queue/remove-by-index {index}`, `queue/change-to-index {index}`, and
  `queue/clear-queue` (needs a `{}` body; a bare POST is a no-op, and the
  current song keeps playing while the queue empties). Indexes
  match the GET `queue` array. That array is the **whole** queue, including
  songs already played, and can list the same song twice, so entry 0 is not
  the current song. Edits locate the playing entry by its ID and refuse when
  it is missing or duplicated; moving entries around the wrong position left
  Cider stuck (loaded the next song, never played it) on 2026-10-16. Moving
  upcoming entries does not interrupt playback (verified with a reversible
  swap). The Queue tab exposes jump/remove/clear. `s` toggles Cider's shuffle
  mode, `r` cycles repeat, and `a` outside Queue appends with `play-later`.
  The time bar shows Cider's polled shuffle/repeat modes; queue-reordering
  bindings have been removed.
- **Cider's PipeWire node**: Cider is Electron, so its output stream reports
  `application.name = "Chromium"` and `application.process.binary = "Cider"`.
  The visualizer identifies the node by `application.process.binary` (falling
  back to `application.name`) and targets it by `object.serial`; `pw-record
  --target <serial>` captures real f32 audio.
- **socket.io**: the EIO4 polling transport is implemented (`live`), but Cider's
  application event names are still unobserved, so HTTP polling remains the
  source of truth (CID-09 stays partial).

## Architecture

Go + Bubble Tea/Lip Gloss, one static binary `sidecue`.

```
sidecue (Bubble Tea)
 ├─ ui/        tabs: Search · Library · Playlists · Queue; header: now playing + visualizer
 ├─ cider/     HTTP client: token discovery, playback GET/POST, amapi run-v3, paging
 ├─ viz/       pw-record child process → f32 PCM → FFT → N bar heights
 └─ main.go    flags, wiring
        │ HTTP (poll 500 ms)            │ PCM over stdout pipe
        ▼                               ▼
     Cider :10767  ── plays audio ──►  PipeWire
```

### Units

- **cider.Client**: `New(baseURL, token)`, `Get(path, out)`, `Post(path, body, out)`,
  `AMAPI(path, out)`, `Pages(path) iterator` (follows `next`). Base URL: `CIDER_URL`
  env, default `http://localhost:10767`. Typed helpers:
  - playback: `NowPlaying()`, `PlayPause()`, `Next()`, `Previous()`, `Seek(sec)`,
    `SetVolume(0..1)`, `ToggleShuffle()`, `ToggleRepeat()`, `Queue()`
  - items: `PlayItem(type,id)`, `PlayNext(type,id)`, `PlayLater(type,id)`
  - Apple Music: `Storefront()` (fetched once, cached), `Search(term)`,
    `LibrarySongs/Albums/Playlists(offset)`, `AlbumTracks(id)`,
    `PlaylistTracks(id)`, `LibraryCount()` (`?limit=1` → `meta.total`)
- **Token discovery**: `CIDER_TOKEN` env wins. Otherwise parse
  `~/.config/sh.cider.genten/spa-config.yml` and use the first entry of
  `connectivity.apiTokens`. Missing file, unparseable YAML or an empty list →
  `ErrNoToken`, shown as the "create a token in Cider" message. If Cider's
  `apiTokensRequired` is `false`, run without a token.
- **viz.Analyzer**: finds Cider's stream by parsing `pw-dump` JSON for a
  `Stream/Output/Audio` node whose `application.process.binary` matches `cider`
  (case-insensitive; `application.name` is only a fallback because Cider is
  Electron and reports `Chromium`), then starts
  `pw-record --target <object.serial> --format f32 --channels 2 -`, reads PCM frames, runs an
  FFT (Hann window, log-spaced bands) and emits band levels on a channel at ~60 fps.
  The stream node exists only while Cider is playing (and for a while after a
  pause), so when `pw-record` exits or no node is found it re-scans every 2 s and
  records nothing in between. A default-sink fallback was removed on 2026-10-16:
  `pw-record --target <sink>` recorded the headset microphone instead. Missing `pw-record` → blank strip, no retries. Spectrum and bar style may
  be adapted from cliamp (MIT, credited).
- **ui.Model**: one Bubble Tea model; a tick message polls now-playing every 500 ms;
  viz messages redraw the header. List views load pages lazily (library is 13k+ songs).
- **`--check`**: prints Cider reachability, token source, volume, now-playing
  title (or "idle") and `LibraryCount()`, then exits 0; exits 1 on any error.

### Keys (cliamp-like)

`space` play/pause · `n`/`p` next/prev · `←`/`→` seek ±5 s · `+`/`-` volume ±5% ·
`s` shuffle · `r` repeat · `tab`/`1-4` switch tab · `/` search · `j`/`k` move ·
`enter` play · `a` play next · `A` play later · `q` quit.

While the search input has focus, every key goes to the input except `enter`
(run the search) and `esc` (leave the input).

## Error handling

- Cider not running / connection refused → full-screen "Cider isn't running" with
  a retry every 2 s; `r` retries at once and only `q`/`ctrl+c` do anything else
  while it is shown; no crash.
- 403 or `ErrNoToken` → message naming the token source and how to create one in Cider.
- Idle (now-playing has no title) → header shows "Nothing playing"; controls stay active.
- amapi errors → shown in a status line; the current list keeps its data.
- Visualizer failures never affect playback controls.

## Testing

- `cider` package: unit tests against `httptest` servers using the JSON shapes
  recorded above (no live Cider needed).
- `viz`: FFT test on a synthetic sine wave lands in the expected band.
- `ui`: model update tests (key → expected client call via a fake client).
- One opt-in live smoke test (`CIDER_LIVE=1 go test ./cider -run Live`) against the
  running desktop Cider, read-only routes only.
- CI: Forgejo Actions on `main`/PRs, `golang` job container, `go vet` + `go test ./...`.

## Later (separate cards)

- socket.io live updates instead of polling.
- `sidecue export-spotify`: library + playlists → Spotify via ISRC match, reusing
  `cider.Client` paging. Replaces paid transfer services (13k songs exceeds free tiers).
  Kept in this repo on purpose: alex wants the Spotify move too, and it needs
  exactly the client this project builds. It is a one-shot subcommand, not part of the TUI.
