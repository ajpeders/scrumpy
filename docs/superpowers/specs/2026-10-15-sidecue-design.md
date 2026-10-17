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
  `{inFavorites, inLibrary, shuffleMode, repeatMode}`: no title, position or duration.

### Not yet verified (each is the first step of the card that needs it)

Probing these requires starting playback, so they were deferred:

| Unknown | Card |
|---|---|
| no-body routes `playpause`, `next`, `previous`, `toggle-shuffle`, `toggle-repeat` | CID-03 |
| `seek.position` unit (absolute seconds expected) | CID-03 |
| now-playing fields for title, artist, position and duration while playing | CID-03 |
| `play-item` `type` values accepted for catalog (`songs`, `albums`, `playlists`) vs library (`library-songs`, `library-playlists`) items | CID-04 |
| how to start a playlist at a given track (`play-item` + start index, `play-item-href`, or queue a playlist then skip) | CID-06 |
| Cider's PipeWire node properties while playing | CID-08 |

Each card records its findings in this section before building on them. If a
route doesn't exist, the card drops that feature and says so in ROADMAP.md.

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
- **viz.Analyzer**: finds Cider's stream by parsing `pw-dump` JSON for a node whose
  `application.name` matches `cider` (case-insensitive), then starts
  `pw-record --target <node> --format f32 --channels 2 -`, reads PCM frames, runs an
  FFT (Hann window, log-spaced bands) and emits bar heights on a channel at ~30 fps.
  The stream node likely exists only while playing, so when `pw-record` exits or no
  node is found it re-scans every 2 s; meanwhile it captures the default sink
  monitor. Missing `pw-record` → blank strip, no retries. Spectrum and bar style may
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
  a retry every 2 s; no crash.
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
