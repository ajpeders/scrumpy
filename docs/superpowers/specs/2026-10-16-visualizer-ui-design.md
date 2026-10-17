# Visualizer-first UI — design

Date: 2026-10-16 · Status: approved

## Goal

Make cider-tui look like the "cliamp-style" player the README promises. The
spectrum visualizer is the centrepiece. The first UI was unstyled plain text: a
one-row 0–8 bar strip, ragged columns, no colour, no key hints.

Non-goals: more than one visualizer style (others may come later behind a
cycle key), themes or config files, mouse support.

## Layout

Top to bottom, at any terminal size:

| Rows | Content |
|---|---|
| 1 | `cider-tui` title, volume meter (`vol ████░ 82%`) |
| ~60% of the remainder | **Visualizer** |
| 1 | `▶ Title · Artist · Album`, `⏸` when Cider's `is-playing` is false (dim "Nothing playing" when idle) |
| 1 | Seek bar `━━━━●─────`, `1:02 / 2:17`, shuffle `⤮` / repeat `↻`/`↻1` (dim when off) |
| 1 | Tabs: `Search [Library] Playlists Queue`, plus context (`Songs · l albums`, search prompt, or detail title) |
| rest, min 3 | List: cursor `>`, queue current `▶`, aligned title and artist columns |
| 1 | Status/error line if any, otherwise key hints |

- `v` toggles full-screen visualizer: the tabs and list are hidden, and the
  visualizer takes their rows. List keys still work.
- If the visualizer channel is closed (no `pw-record`), its rows go to the list.
  Silence is not the same as unavailable: silence draws flat bars.
- Every line is truncated to the terminal width (ANSI-aware, `…`), so nothing
  wraps. View output never exceeds the terminal's width or height.
- The offline screen keeps the same frame and shows the retry message centred,
  with `r` to retry at once and `q` to quit.

## Visualizer

- `viz.Analyzer` emits `[]float64` levels in [0,1] instead of `[]int` 0–8. It
  uses 48 log-spaced bands over 40 Hz–16 kHz and 60 analyses per second (hop
  800 samples). Each band is its mean energy in dB.
- Automatic gain (`viz.gain`): the loudest band sets a reference that rises
  instantly and falls at ~5 dB/s, never below −45 dB. Levels span the 42 dB
  below it. Frames whose loudest band is under −70 dB are silence. Before this,
  a fixed −60 dB scale kept loud music at a static half height.
- Only Cider's own stream is captured; there is no default-sink fallback (it
  recorded the microphone). A capture that delivers no audio for 3 s is
  restarted: after a Cider restart, PipeWire sometimes never links a
  `pw-record` started as the new stream appears.
- `ui.meter` (pure, in `ui/`) eases bars towards the latest frame on a 60 fps
  animation tick, independent of frame arrival (pw-record delivers in uneven
  21/42 ms bursts). It uses cliamp's rates (rise 34/s, fall 10/s); peak caps
  hold 0.5 s, then fall at 1.2 heights/s. When Cider reports not playing, the
  target is empty and the bars settle to rest.
- Rendering: bands are reduced (by group maximum) to at most `(width+1)/3`
  bars, each at least 2 columns wide with a one-column gap, and leftover
  columns are spread evenly. The bottom row shows a `▁` floor during silence. Each cell uses fractional blocks `▁▂▃▄▅▆▇█` for the bar top and `▀` for
  the peak cap.
- Colour comes from the row's height: bottom 30% green (ANSI 10), up to 60%
  yellow (11), above that red (9). Styles are pre-rendered once per row, not per
  cell.

## Styling

- `lipgloss` v1 becomes a direct dependency. Its version matches the Bubble Tea
  v1 already in use.
- Colours use the ANSI 16 palette, so the UI follows the user's terminal theme:
  title/playing green (10), accent and seek yellow (11), dim (7/8), error red
  (9), key hints reversed.
- Styles live in `ui/styles.go`. The view is split into `ui/view.go`
  functions, one per region, each taking a width (and height where needed).

## Testing

- Bar rendering: a known level vector renders the expected glyph per cell, and
  gradient tiers land on the right rows.
- Meter: the bar rises quickly, the fall is monotonic, and the peak holds and then
  drops.
- Layout: at 80×24, 120×40 and 40×10, with viz on, off and unavailable, every
  line is at most the width (`lipgloss.Width`) and there are at most `height`
  lines. Existing model tests keep passing.
- Manual check: run in tmux against live Cider and capture the screen.

## Credits

The bar rendering approach (fractional blocks, three-tier gradient, peak caps)
is adapted from cliamp (MIT), which is already credited in the README.
