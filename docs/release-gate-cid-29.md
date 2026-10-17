# CID-29 — Live release acceptance checklist

Run this when ready to tag a release. Record everything in
`docs/release-gate-cid-29-results.md` (create with the results template
below) and attach it to the release notes.

## Environment

| Item | Value |
|---|---|
| Terminal (kitty/wezterm/ghostty/...) | |
| `kitty`/`wezterm`/`TERM` (whichever applies) | |
| Resolution / cell size if non-default | |
| Distro and kernel (`uname -a`) | |
| Cider version (`Cider → About`) | |
| Apple Music signed-in account (region, or "US/UK/etc.") | |
| cider-tui build (`cider-tui --version`) | |
| Branch / commit SHA being tested | |

## Pre-flight

- [ ] Cider is running and signed in.
- [ ] `cider-tui --check` reports "Visualizer: ready" (pw-record found).
- [ ] Apple Music playback is unmuted at a sane volume (≥ 20%).
- [ ] A terminal multiplexer is **not** in front of cider-tui (it gates the
      visualizer to block-art mode; record this in the notes if you can't
      avoid it).
- [ ] `pidof cider-tui` is empty before start (clean state).
- [ ] `uptime` recorded; system has been up ≥ 5 min so the CPU baseline is
      stable.

## The soak (60+ minutes)

Start cider-tui against the real Cider. Record every transition.

```sh
# In one shell: cider-tui
cider-tui

# In another shell: log CPU/RSS for cider-tui every 5 seconds
pid=$(pidof cider-tui | awk '{print $1}')
( while kill -0 "$pid" 2>/dev/null; do
    ps -o %cpu= -o rss= -p "$pid"
    sleep 5
  done ) | tee cpu.log
```

| Time | Action | Result |
|---|---|---|
| 00:00 | Start cider-tui; capture `top -p $pid` baseline | |
| 00:01 | Press `space` (play) on whatever's in the queue | |
| 02:00 | `n` (next), `p` (prev) — verify skip works | |
| 04:00 | `j` `k` through the queue; verify cursor stays sane | |
| 05:00 | Press `space` to pause; let it sit for 60 s | |
| 06:00 | Resume play; verify the bar advances and the meter moves | |
| 08:00 | `s` (shuffle); `r` (cycle repeat: off→all→one→off) | |
| 10:00 | Type `/` and search for an artist; `enter` to drill into albums; `enter` on an album to play its first song | |
| 12:00 | `tab` `shift+tab` through all four tabs; verify each loads | |
| 14:00 | On Queue tab: `enter` to jump to a non-current row, `x` to remove a row past the cursor, `c` to clear (then confirm Cider advances) | |
| 16:00 | Add the same song twice to the queue (search → `a` twice); verify both copies appear and play in order | |
| 18:00 | `d` on a row; verify the detail panel opens beside the lyrics; `j` to follow selection; `d` to collapse | |
| 20:00 | `Y` to copy the row; verify clipboard has `Title — Artist · Album` (paste into a file to check) | |
| 22:00 | `,` to open settings; cycle theme (5 themes), toggle background, switch spectrum↔waveform, toggle artwork on/off, toggle lyrics off then on | |
| 25:00 | Close cider-tui (`q`); confirm cider-tui is gone; reopen; verify theme/background/visualizer/artwork/lyrics settings all persisted | |
| 30:00 | `v` to enter fullscreen viz; verify lyrics band still visible if lyrics is on; `?` to open help; `esc` to close help; `esc` again to leave fullscreen | |
| 32:00 | Hide Cider's network connection (or kill it) for 30 s; cider-tui should show "Cider isn't running"; restore Cider and verify auto-recovery | |
| 35:00 | `y` to hide lyrics; `y` again to restore; verify current line tracks playback | |
| 40:00 | Play a track with synced lyrics from the start; verify the current line follows (especially on long lines that wrap to 2–3 rows) | |
| 45:00 | Find a track with no lyrics (a small/indie label track); verify "No lyrics for this song" appears | |
| 48:00 | Find a track without album art; verify the band collapses cleanly (no blank rows in the layout) | |
| 50:00 | Press `space` to pause; leave cider-tui idle for 10 minutes (long enough to catch the CID-23 freeze pattern) | |
| 60:00 | Capture `top -p $pid`; CPU should be near 0% during the idle stretch | |
| 60:00 | Press `space` again; verify playback resumes and the bars move | |
| 60:00 | `q` to quit cleanly; verify no panic, no orphan goroutines (`pgrep -f cider-tui` returns empty) | |

## Acceptance

- [ ] CPU idle (paused, no lyrics) stays under 1% for the 10-minute stretch.
- [ ] No visible frame skips when typing in the search field (`/` + a long
      query) — keyboard responsiveness stays under ~16 ms / keystroke.
- [ ] Reconnect after Cider goes away works within ~2 s of restoration.
- [ ] Shuffle + repeat behave the way Cider does (no client-side shuffling
      regressions).
- [ ] Duplicate-track queue entries play in order; removal/clear while
      filtered does not crash and does not silently mutate the queue.
- [ ] Waveform rests to flat during a quiet passage (CID-11 fix holds).
- [ ] Lyric transitions don't visibly jump the panel (CID-26 fix holds).
- [ ] Missing-art fallback shows the layout collapse cleanly, not a blank
      band (CID-17 caveat).
- [ ] Settings persist across restarts (CID-15).
- [ ] Clipboard tool detection works on this host (X11 / Wayland / macOS).
- [ ] All cosmetic issues encountered are listed in the release notes.

## Results template

Save as `docs/release-gate-cid-29-results.md`:

```markdown
# CID-29 results — cider-tui <tag>

Run by: <name>
Date: <YYYY-MM-DD>
Commit: <sha>
Cider: <version>
Terminal: <name> <version>

## Environment
<filled-in env table>

## Soak summary
<one paragraph: did the soak pass, what broke, what was cosmetic only>

## CPU/memory
<peak RSS, peak %CPU idle, peak %CPU playing — paste a couple of top lines>

## Outstanding issues
- [ ] <issue 1>
- [ ] <issue 2>

## Notes for release
<anything that should be in the release notes>
```

After completing, mark `[x]` in ROADMAP and link the results file.