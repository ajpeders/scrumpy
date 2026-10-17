# Architecture

One Go binary. It never touches audio data from Apple Music: Cider plays it
(DRM means only Cider can decode it), and sidecue sends commands over HTTP and
reads Cider's output from PipeWire for the visualizer.

```
sidecue (Bubble Tea)
 ├─ ui/        tabs: Search · Library · Playlists · Queue; header: now playing + visualizer
 ├─ cider/     HTTP client: token discovery, playback GET/POST, amapi run-v3, paging
 ├─ viz/       pw-record child process → f32 PCM → FFT → bar heights
 └─ main.go    flags, wiring
        │ HTTP (poll 500 ms)            │ PCM over stdout pipe
        ▼                               ▼
     Cider :10767  ── plays audio ──►  PipeWire
```

## Cider API (verified on 3.1.8)

- Base `http://localhost:10767`, header `apptoken: <token>`.
- Token source: `CIDER_TOKEN`, else `~/.config/sh.cider.genten/spa-config.yml`
  → `connectivity.apiTokens[].token`.
- Playback: `/api/v1/playback/*` — GET state routes return `{"status":"ok",...}`;
  POST routes take JSON (`play-item {type,id}`, `seek {position}`, `volume {volume}`, ...).
- Apple Music API: POST `/api/v1/amapi/run-v3 {"path":"/v1/..."}` runs any Apple
  Music API request as the signed-in user; the response is wrapped in `{"data": ...}`.
  Lists page through `next`.

Full verified details are in the [design spec](docs/superpowers/specs/2026-10-15-sidecue-design.md).

## Key decisions

- **Own app, not a cliamp fork**: cliamp decodes every source itself and its
  visualizer and EQ depend on that; Cider can't hand it audio.
- **Polling first, socket.io later**: 500 ms polling is simple and good enough
  for a progress bar; socket.io is mapped later (CID-09).
- **Visualizer via PipeWire**: capture Cider's stream with `pw-record` rather than
  linking libpipewire, so the binary stays pure Go and the feature can fail alone.
- **Token from Cider's config**: no separate secret file to keep in sync.

## Security

Cider's API binds `0.0.0.0:10767`, so the token is the only thing keeping other
LAN devices from controlling playback. Don't disable `apiTokensRequired`.
