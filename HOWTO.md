# How to

## Create the Cider API token

1. Open Cider → Settings → Connectivity → Manage External Application Access.
2. Create a token. Cider saves it in `~/.config/sh.cider.genten/spa-config.yml`;
   cider-tui reads it from there.
3. Check: `./cider-tui --check` (or by hand:
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
pw-dump | jq '.[] | select(.info.props["application.name"]? // "" | test("cider";"i")) | .info.props["node.name"]'
```

## Ship a change

1. Tick the card in ROADMAP.md and update the other docs if behaviour changed.
2. Commit, then show the diff and wait for OK before pushing `main` to both
   `origin` (Forgejo, alex/cider-tui) and `github` (ajpeders/cider-tui).

## Troubleshooting

- `UNAUTHORIZED_APP_TOKEN` (403): the token was deleted or regenerated in Cider;
  create a new one.
- Connection refused: Cider isn't running. `cider` starts it.
