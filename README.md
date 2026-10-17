# cider-tui

A cliamp-style terminal player for Apple Music on Linux. [Cider](https://cider.sh)
does the playback; cider-tui controls it over Cider's local API and adds search,
library, playlists, queue and spectrum/waveform visualizers.

Status: implemented and live-verified against Cider 3.1.8. Repositories on
[Forgejo](https://git.thelunadog.com/alex/cider-tui) and
[GitHub](https://github.com/ajpeders/cider-tui).
See [ROADMAP.md](ROADMAP.md).

## Requirements

- Cider (tested with 3.1.8), signed in to Apple Music
- An API token created in Cider: Settings → Connectivity → Manage External Application Access
- Go 1.27+ to build
- PipeWire `pw-record` for the visualizer (optional)

Linux is the supported platform. It also builds for macOS and Windows and the
player should work there, but that is untested, and the visualizer needs
PipeWire, so it stays blank elsewhere.

## Quick start

1. In Cider, open Settings → Connectivity → Manage External Application
   Access and create a token (any name, e.g. `cider-tui`). cider-tui finds
   it automatically; nothing to copy.
2. Install and check the connection:

```sh
go install github.com/ajpeders/cider-tui@latest
cider-tui --check   # Cider status, volume, library size, visualizer
cider-tui
```

If `--check` fails, see [HOWTO.md](HOWTO.md#troubleshooting).

Or from a clone: `go build -o cider-tui . && ./cider-tui`.

The token is read from Cider's own config (`sh.cider.genten/spa-config.yml` in
your config directory) automatically. Override with `CIDER_TOKEN`, and the
server with `CIDER_URL` (default `http://localhost:10767`). `cider-tui -h`
lists all options; `cider-tui --version` prints the version.

## Keys

`space` play/pause · `n`/`p` next/previous song · `←`/`→` seek · `+`/`-` volume ·
`s` toggle Cider shuffle · `r` cycle repeat · `tab`/`shift+tab`/`1-4` tabs · `j`/`k` move ·
`enter` play from here (the rest of the list is queued) · `P` play the whole
playlist/album (shuffled when shuffle is on) · `?` all keys ·
`a` enqueue at the end (outside Queue) · on the Queue tab: `enter`
jump to a song, `x` remove it, `c` clear the queue · `o` sort (A→Z, Z→A, newest, oldest
added) · `d` expand selected item details · `y` expand lyrics · `v` full-screen visualizer · `b` switch spectrum/waveform · `,` settings · `q` quit

When Cider can't be reached the screen says so and retries every 2 s; `r`
retries immediately and `q` quits.

The footer shows essential contextual controls; `?` shows the full key list.
Shuffle and Repeat labels beside the time bar reflect Cider's playback settings:
dim when off, highlighted when on; `Repeat 1` means repeat one.

The app opens on Queue. Synced lyrics are always shown in a box to the right of
the menu (stacked below on narrow terminals, hidden only when space is too tight
or another full-screen view is open). Word-timed lyrics highlight progressively
and animate even without the visualizer. Apple's supplied timing determines the
available lyric precision. Album artwork is not displayed or downloaded.

`y` expands lyrics across the entire bottom panel, replacing the menu and tabs
while keeping the visualizer and song metadata above. Press `y` again or `esc`
to restore the menu and lyrics sidebar. Playback controls remain available.

`d` expands the selected row's available metadata; move with `j`/`k` to inspect
another item. Press `d` again or `esc` to collapse it without starting playback.

Open Settings with `,`, select a theme with `j`/`k` or arrows, and press `enter`
to apply and save. Terminal, Dark, Light, Paper and High contrast presets are available;
`esc` returns. The selection is stored in `cider-tui/settings.json` under your
config directory: `$XDG_CONFIG_HOME` (normally `~/.config`) on Linux,
`~/Library/Application Support` on macOS, `%AppData%` on Windows. Delete it to
reset to defaults.

Select **Background** in Settings and press `enter` to toggle it. **Off** leaves
your terminal's background untouched while keeping the selected theme's text
and visualizer colors. The choice is saved and kept when changing themes.

Select **Visualizer** in Settings and press `enter` to choose Spectrum or
Waveform. This saves the startup view; `b` switches views during playback.

Paper is monochrome: black text, controls, borders and visualizer on a light
paper-colored background.

`/` filters what you're looking at as you type, matching a song's title, artist
or album: the loaded rows on Search, Library, Playlists and the Queue, and the
tracks inside a playlist/album. On Search and Library, `enter` still runs Apple's
catalog/library search and `esc` clears the filter (or restores the library).

## Docs

[ARCHITECTURE.md](ARCHITECTURE.md) · [ROADMAP.md](ROADMAP.md) · [HOWTO.md](HOWTO.md)

MIT licensed (see [LICENSE](LICENSE)). Visualizer approach adapted from [cliamp](https://github.com/bjarneo/cliamp) (MIT).
