# How to

## Create the Cider API token

1. Open Cider → Settings → Connectivity → Manage External Application Access.
2. Create a token. Cider saves it in `sh.cider.genten/spa-config.yml` under
   your config directory (`~/.config` on Linux); cider-tui reads it from there.
   If Cider keeps it elsewhere (e.g. a Flatpak install), set `CIDER_TOKEN`.
3. Check: `cider-tui --check` (or by hand:
   `curl -H "apptoken: $TOKEN" localhost:10767/api/v1/playback/active` → `{"status":"ok"}`).

## Call the Apple Music API through Cider

```sh
curl -s -X POST -H "apptoken: $TOKEN" -H 'Content-Type: application/json' \
  -d '{"path":"/v1/me/library/songs?limit=1"}' \
  localhost:10767/api/v1/amapi/run-v3
```

Any path from Apple's [Music API docs](https://developer.apple.com/documentation/applemusicapi/)
works; catalog paths need the storefront (`/v1/me/storefront`, currently `us`).

## Run the tests

```sh
go vet ./... && go test ./...
CIDER_LIVE=1 go test ./cider -run Live   # read-only smoke test against running Cider
```

## Find Cider's PipeWire node (visualizer)

Play something in Cider, then:

```sh
# Cider is Electron: its stream reports application.name "Chromium" but
# application.process.binary "Cider".
pw-dump | jq '.[] | select(.info.props["application.process.binary"]? // "" | test("cider";"i")) | .info.props["object.serial"]'
```

## Ship a change

1. Tick the card in ROADMAP.md and update the other docs if behaviour changed.
2. Commit, then show the diff and wait for OK before pushing `main` to both
   `origin` (Forgejo, alex/cider-tui) and `github` (ajpeders/cider-tui).

## Cut a release

Versions follow semver tags (`v0.1.0`). `go install …@v0.1.0` reports that
version in `cider-tui --version` automatically; for a hand-built binary:

```sh
git tag -a v0.1.0 -m "v0.1.0" && git push origin v0.1.0 && git push github v0.1.0
go build -ldflags "-X main.version=v0.1.0" -o cider-tui .
```

## Troubleshooting

- `UNAUTHORIZED_APP_TOKEN` (403): the token was deleted or regenerated in Cider;
  create a new one.
- Connection refused: Cider isn't running. `cider` starts it. If Cider is
  open and this persists, check that its local API is enabled
  (`connectivity.webSockets.enabled: true` in `spa-config.yml`, a toggle
  under Settings → Connectivity), then restart Cider.
- Cider runs on another port or machine: set `CIDER_URL`, e.g.
  `CIDER_URL=http://localhost:10767`.
- Token not found (Flatpak or custom install): copy the token from Cider's
  token list and `export CIDER_TOKEN=...`.
