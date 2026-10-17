# How to

## Create the Cider API token

1. Open Cider → Settings → Connectivity → Manage External Application Access.
2. Create a token. Cider saves it in `sh.cider.genten/spa-config.yml` under
   your config directory (`~/.config` on Linux); scrumpy reads it from there.
   If Cider keeps it elsewhere (e.g. a Flatpak install), set `CIDER_TOKEN`.
3. Check: `scrumpy --check` (or by hand:
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
go test -race ./...
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

1. Update the docs if behaviour changed.
2. Commit, then show the diff and wait for OK before pushing `main` to both
   `origin` (Forgejo, alex/scrumpy) and `github` (ajpeders/scrumpy).

## Cut a release

First complete the [live acceptance checklist](docs/release-gate-cid-29.md), including
the live soak test and the [demo rehearsal](demo/README.md). Tests alone do not
validate Cider playback, terminal graphics, or the release upload credentials.
Review the working tree (including untracked files), commit only the intended
changes, and get approval before pushing. Do not tag a dirty or unreviewed tree.

Versions follow semver tags (`v0.1.0`). `go install …@v0.1.0` reports that
version in `scrumpy --version` automatically; for a hand-built binary:

```sh
git tag -a v0.1.0 -m "v0.1.0" && git push origin v0.1.0 && git push github v0.1.0
go build -ldflags "-X main.version=v0.1.0" -o scrumpy .
```

Pushing a `v*` tag runs the `release` workflow on both remotes
(`.forgejo/workflows/release.yml`, `.github/workflows/release.yml`): it builds
linux amd64/arm64 binaries with the tag baked in and attaches them to a new
release. Forgejo needs a `RELEASE_TOKEN` repo secret (access token with
`write:repository`) if the job token cannot create releases.

After both workflows finish:

1. Download both architectures from each release, compare SHA-256 hashes across
   mirrors, and publish a checksum manifest with the assets.
2. Run `--version`, `--check`, and a short playback session on a clean Linux
   machine for each architecture (or record explicitly which was not tested).
3. Test `go install github.com/ajpeders/scrumpy@v0.1.0` outside the checkout.
4. Add release notes with prerequisites, supported platforms, known issues and
   the demo link. Verify download links anonymously before announcing.

The tag/push commands above publish a release; run them only when ready. Never
move a published tag to repair a release—fix forward with a new version.

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
- Visualizer blank: install `pw-record` and `pw-dump`, start playback in Cider,
  and check the Cider PipeWire node above. `--check` reports prerequisites, not
  proof that live PCM frames are arriving. There is no microphone/sink fallback.
- Artwork looks pixelated: Unicode blocks are the fallback, including inside
  tmux/Screen. Rehearse high-resolution artwork directly in a supported terminal.
- Clipboard unavailable: install `wl-copy` on Wayland or `xclip`/`xsel` on X11.
- Need a reproducible showcase: use the [full demo runbook](demo/README.md);
  the existing VHS tape is only a short teaser, not the release test matrix.
