# cider-tui

A cliamp-style terminal player for Apple Music on Linux. [Cider](https://cider.sh)
does the playback; cider-tui controls it over Cider's local API and adds search,
library, playlists, queue and a spectrum visualizer.

Status: design done, no code yet. Repositories exist on
[Forgejo](https://git.thelunadog.com/alex/cider-tui) and
[GitHub](https://github.com/ajpeders/cider-tui); the first push is pending (CID-00).
See [ROADMAP.md](ROADMAP.md).

## Requirements

- Cider (tested with 3.1.8), signed in to Apple Music
- An API token created in Cider: Settings → Connectivity → Manage External Application Access
- Go 1.27+ to build
- PipeWire `pw-record` for the visualizer (optional)

## Quick start

```sh
go build -o cider-tui .
./cider-tui --check   # Cider status, volume, library size
./cider-tui
```

The token is read from Cider's own config automatically. Override with
`CIDER_TOKEN`, and the server with `CIDER_URL` (default `http://localhost:10767`).

## Keys

`space` play/pause · `n`/`p` next/prev · `←`/`→` seek · `+`/`-` volume ·
`s` shuffle · `r` repeat · `tab`/`1-4` tabs · `/` search · `j`/`k` move ·
`enter` play · `a` play next · `A` play later · `q` quit

## Docs

[ARCHITECTURE.md](ARCHITECTURE.md) · [ROADMAP.md](ROADMAP.md) · [HOWTO.md](HOWTO.md)

Visualizer approach adapted from [cliamp](https://github.com/bjarneo/cliamp) (MIT).
