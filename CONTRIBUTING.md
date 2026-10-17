# Contributing

Pick up the next unticked card in [ROADMAP.md](ROADMAP.md). Each card states
its own acceptance criteria. Cards marked `[~]` are implemented and waiting on
a manual check.

## Getting set up

1. Install Cider and sign in to Apple Music (a paid subscription is needed),
   then create an API token (Settings → Connectivity → Manage External
   Application Access). The exact steps and a `curl` check are in
   [HOWTO.md](HOWTO.md).
2. Install Go 1.27+.
3. Clone the repo, then `go build -o sidecue . && ./sidecue --check`.

Without Cider running you can still work on anything with tests: the client is
exercised through `httptest`, and the UI through a fake client. Only the
`CIDER_LIVE=1` smoke test and manual checks need live Cider.

## Working on a card

- Read the [design spec](docs/superpowers/specs/2026-10-15-sidecue-design.md)
  for the card's area first. Its "Verified" sections record what has been
  probed against Cider 3.1.8; add to them when you verify something new.
- The baseline for every card: `go vet ./... && go test ./...` green, plus a
  new test covering the change. Run `go test -race ./...` for anything
  touching the event loop or the visualizer.
- Keep the layout in [ARCHITECTURE.md](ARCHITECTURE.md) (`cider/`, `ui/`,
  `viz/`, `main.go`) and update that file if a decision changes.

## Sending a change

1. Branch off `main`.
2. Tick the card in ROADMAP.md and update the other docs if behaviour changed.
3. Open a pull request on GitHub or Forgejo. `main` is pushed only after
   review; see "Ship a change" in [HOWTO.md](HOWTO.md).

## Scope

Deliberately out of scope, so you don't spend time on them: decoding audio
ourselves (DRM means only Cider can), bundling an EQ, and anything that needs
`apiTokensRequired` disabled in Cider. See the Key decisions and Security
sections of [ARCHITECTURE.md](ARCHITECTURE.md).
