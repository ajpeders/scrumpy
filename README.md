# sidecue

A cliamp-style terminal player for Apple Music on Linux. [Cider](https://cider.sh)
does the playback; sidecue controls it over Cider's local API and adds search,
library, playlists, queue and spectrum/waveform visualizers.

Status: preparing the first public release; core playback has been live-verified
against Cider 3.1.8, but release artifacts and the remaining manual checks are
not yet signed off. Repositories on
[Forgejo](https://git.thelunadog.com/alex/sidecue) and
[GitHub](https://github.com/ajpeders/sidecue).
See [ROADMAP.md](ROADMAP.md).

## Requirements

To run:

- [Cider](https://cider.sh) (tested with 3.1.8), signed in to Apple Music.
  A paid Apple Music subscription is required; sidecue can't play anything on
  its own. On Arch: `yay -S cider` (AUR), or use Cider's AppImage/Flatpak.
- An API token created in Cider: Settings → Connectivity → Manage External
  Application Access (see [HOWTO.md](HOWTO.md)).
- PipeWire with `pw-record` on `PATH` for the visualizer (optional; the rest of
  the app works without it).

To build from source: Go 1.27 or newer. The release binaries need nothing extra.

Linux is the supported platform. It also builds for macOS and Windows and the
player should work there, but that is untested, and the visualizer needs
PipeWire, so it stays blank elsewhere.

## Quick start

1. In Cider, open Settings → Connectivity → Manage External Application
   Access and create a token (any name, e.g. `sidecue`). sidecue finds
   it automatically; nothing to copy.
2. Install and check the connection:

```sh
go install github.com/ajpeders/sidecue@latest
sidecue --check   # Cider status, volume, library size, visualizer
sidecue
```

If `--check` fails, see [HOWTO.md](HOWTO.md#troubleshooting).

Once a release is published, download `sidecue-linux-amd64` /
`sidecue-linux-arm64` from its Forgejo or GitHub release page, `chmod +x`
it and run it. Until then, build from a clone:
`go build -o sidecue . && ./sidecue`.

The token is read from Cider's own config (`sh.cider.genten/spa-config.yml` in
your config directory) automatically. Override with `CIDER_TOKEN`, and the
server with `CIDER_URL` (default `http://localhost:10767`). `sidecue -h`
lists all options; `sidecue --version` prints the version.

## Keys

`space` play/pause · `n`/`p` next/previous song · `←`/`→` seek · `+`/`-` volume ·
`s` toggle Cider shuffle · `r` cycle repeat · `tab`/`shift+tab`/`1-4` tabs · `j`/`k` move ·
`enter` play from here (the rest of the list is queued) · `P` play the whole
playlist/album (shuffled when shuffle is on) · `?` all keys ·
`a` enqueue at the end (outside Queue) · on the Queue tab: `enter`
jump to a song, `x` remove it, `c` clear the queue · `o` sort (A→Z, Z→A, newest, oldest
added) · `l` cycle songs/albums/artists · `d` expand selected item details · `Y` copy `Title — Artist · Album` of the selected row to the clipboard (pbcopy, clip.exe, wl-copy, xclip or xsel) · `y` hide/show lyrics · `v` full-screen visualizer · `b` switch spectrum/waveform · `,` settings · `q` quit

Each bottom-panel row shows the title on the left and `Artist · Album`
on the right; the album is suppressed on tabs that already name the
container (Library albums/artists, Playlists tab, album-tracks
drill-down). See `docs/cid-34-examples.txt` for rendered examples at
120, 80 and 40 columns.

After connecting, brief API failures show a retrying status without hiding the
player. Three consecutive failed reads show the offline screen, which retries
every 2 s; `r` retries immediately and `q` quits. Startup connection failures
show the offline screen immediately.

The footer shows essential contextual controls; `?` shows the full key list.
Shuffle and Repeat labels beside the time bar reflect Cider's playback settings:
dim when off, highlighted when on; `Repeat 1` means repeat one.

The app opens on Queue. Synced lyrics are shown in a full-width box between
the now-playing metadata and seek bar when space permits, with browsing below.
Word-timed lyrics highlight progressively
and animate even without the visualizer. Apple's supplied timing determines the
available lyric precision.

When album art is on, the cover sits to the left of the now-playing metadata,
with the lyrics boxed in a smaller form under the song and artist, so art, song
and words share one band and the list keeps the full width.

### High-resolution album art

The cover renders as a true high-resolution image only on terminals that speak
the **kitty graphics protocol**. You need one of:

- **kitty** (Linux/macOS; `kitty` ≥ 0.20),
- **WezTerm**,
- **Ghostty**,
- **iTerm2** (recent versions implement the kitty protocol),
- **Konsole** (with graphics support).

sidecue detects your terminal from its `TERM` / `TERM_PROGRAM` / `KITTY_WINDOW_ID`
environment variables. Note the limitation: the image is drawn directly into the
scrolling area, so it requires the **alternate screen** (enabled by default) and
a terminal that keeps graphics across partial redraws. Inside `tmux` or GNU
Screen, sidecue conservatively uses blocks rather than forwarding graphics.

On any other terminal the cover falls back to **Unicode quadrant blocks** — a
small coloured mosaic that works everywhere but is not true image resolution.

`y` temporarily hides/shows the lyrics band without replacing the browsing
list. It has no effect when Lyrics is off in Settings; `esc` does not undo it.
Playback controls remain available. Leave the full-screen visualizer with
`v` or `esc`; opening `?` help from full-screen shows the key list in place
of the bars (`esc` closes help, then `esc` again leaves full-screen).

While search input is open, letters and spaces type into the query rather than
control playback. In Settings, Up/Down select rows; Left/Right still seek and
other transport keys remain live. Help is modal: while it's open, list
actions (`j`/`k`, `d`, `enter`, `a`, `x`, `o`, etc.) are blocked so a stray
press cannot mutate state behind the overlay; transport keys still work.
On a filtered Queue, clear the filter with `esc` before jumping to or
removing a track.

`d` opens a bordered details panel beneath the selected row, with album,
duration, genre, release date and track/disc numbers when supplied by the API.
When Album art is on in Settings, the panel also shows the item's cover
beside the metadata (songs, albums, playlists, music videos — fetched on
demand and cached on disk for next time). Move with `j`/`k` to inspect
another item. Press `d` again or `esc` to collapse it without starting
playback or making extra network requests.

Open Settings with `,`, select a theme with `j`/`k` or arrows, and press `enter`
to apply and save. Terminal, Dark, Light, Paper and High contrast presets are available;
`esc` returns. The selection is stored in `sidecue/settings.json` under your
config directory: `$XDG_CONFIG_HOME` (normally `~/.config`) on Linux,
`~/Library/Application Support` on macOS, `%AppData%` on Windows. Delete it to
reset to defaults.

Select **Background** in Settings and press `enter` to toggle it. **Off** leaves
your terminal's background untouched while keeping the selected theme's text
and visualizer colors. The choice is saved and kept when changing themes.

Select **Visualizer** in Settings and press `enter` to choose Spectrum or
Waveform. This saves the startup view; `b` switches views during playback.

Select **Lyrics** in Settings and press `enter` to turn lyrics on or off.
The choice is saved. Off hides both the regular lyrics band and the boxed lyrics
beside artwork, frees layout space, and disables `y`.

Select **Album art** in Settings and press `enter` to show the current track's
cover to the left of the now-playing title, with compact bordered lyrics beneath
the metadata, to the right of the cover. Narrow or short windows use the regular
layout to keep browsing and controls accessible. On terminals with the kitty graphics protocol (kitty, WezTerm, Ghostty,
iTerm2, Konsole) the cover renders as a true high-resolution image; elsewhere it
falls back to coloured Unicode quadrant blocks. Artwork comes from Apple Music
through Cider and is cached on disk under your cache directory
(`sidecue/artwork/`), so a track is downloaded once and the cache stays
bounded.

Paper is monochrome: black text, controls, borders and visualizer on a light
paper-colored background.

`/` filters what you're looking at as you type, matching a song's title, artist
or album: the loaded rows on Search, Library, Playlists and the Queue, and the
tracks inside a playlist/album. Search also returns artists; `enter` on an artist
row opens its top songs (catalog) or albums (library), and on the Library tab `l`
cycles songs → albums → artists. On Search, `l` also cycles Songs → Albums →
Artists and reruns the same query for that result type. While the search input is open, `enter` runs
Apple's catalog/library search and `esc` clears the filter (or restores the
library).

## Docs

[ARCHITECTURE.md](ARCHITECTURE.md) — how the pieces fit together ·
[ROADMAP.md](ROADMAP.md) — what's built and what's next ·
[HOWTO.md](HOWTO.md) — tokens, the Cider API, tests, troubleshooting ·
[CONTRIBUTING.md](CONTRIBUTING.md) — how to pick up a card and send a change ·
[Demo and recording checklist](demo/README.md)

### Before publishing or reporting a bug

Linux is the release target; other platforms are build-only, not validated.
Known issues awaiting live verification include long-session CPU/freezes,
waveform spikes in quiet passages, missing covers on some tracks, and lyric
layout jumps. See the release gates in [ROADMAP.md](ROADMAP.md).

Keep Cider's API token authentication enabled and its API off the public
Internet. Remote `CIDER_URL` connections over plain HTTP expose the token on
the network; use a trusted local connection or an encrypted tunnel.

For bugs, include `sidecue --version`, Cider version, OS, terminal (and whether
tmux/Screen is involved), reproduction steps, and sanitized `--check` output.
Remove tokens, private library/playlist names and local paths from logs and
recordings. Never attach Cider's config or your API token.

## License

MIT — see [LICENSE](LICENSE). Visualizer approach adapted from
[cliamp](https://github.com/bjarneo/cliamp) (MIT).
