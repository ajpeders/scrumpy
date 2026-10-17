# Short showcase: album art, playlist queueing, shuffle, waveform in all five themes.
# Run from the repository root: vhs demo/showcase.tape
# Cider must be running and signed in; playlist 102026 must exist.
# Uses real playback/queue, with isolated appearance settings and visible seeking.
# Review the export: API sleeps are not acceptance assertions.
Output demo/showcase.mp4
Output demo/showcase.gif

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 850
Set Padding 16
Set TypingSpeed 60ms

# Build and start a known song off camera. Never display credentials.
Hide
Type `D=$(mktemp -d /tmp/opencode/cider-showcase.XXXXXX) && mkdir -p "$D/config/sidecue" "$D/cache" && ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" "$D/config/sh.cider.genten"`
Enter
Type `printf '%s\n' '{"theme":"Dark","background":true,"visualizer":"spectrum","artwork":true,"lyrics":true}' > "$D/config/sidecue/settings.json"`
Enter
Type `go build -o "$D/sidecue" . && clear && XDG_CONFIG_HOME="$D/config" XDG_CACHE_HOME="$D/cache" "$D/sidecue"`
Enter
Sleep 5s
Type "1/"
Type "Hard to Explain The Strokes"
Enter
Sleep 4s
Enter
Sleep 4s
Type "4"
Sleep 6s
Show
Sleep 4s
Screenshot demo/screenshot-player.png

# A short catalog search-to-play, before adding the playlist to its queue.
Type "1/"
Backspace 30
Type "I Wonder Kanye West"
Enter
Sleep 4s
Enter
Sleep 4s

# Visible scrubbing: fifteen 5-second seeks land around one-third of I Wonder.
Right@500ms 15
Sleep 3s

# Queue the playlist itself (a on the container, not on a song).
Type "3"
Sleep 3s
Type "/102026"
Enter
Sleep 2s
Type "a"
Sleep 4s
Enter
Sleep 3s
Escape
Type "4"
Sleep 4s

# Show shuffle enabled, then hold two skips so new titles and queue markers
# are visible. Review the export: a position reset alone is not a song change.
Type "s"
Sleep 3s
Type "n"
Sleep 6s
Type "n"
Sleep 6s
Type "s"
Sleep 3s

# Hold waveform in the normal player for every theme, with Settings closed.
# Briefly show each selected theme label before returning to the waveform.
Type "b"
Type ","
Up
Enter
Sleep 1s
Escape
Sleep 4s
Screenshot demo/screenshot-waveform-terminal.png
Type ","
Down
Enter
Sleep 1s
Escape
Sleep 4s
Screenshot demo/screenshot-waveform.png
Type ","
Down
Enter
Sleep 1s
Escape
Sleep 4s
Screenshot demo/screenshot-waveform-light.png
Type ","
Down
Enter
Sleep 1s
Escape
# Paper's normal layout gets the longest theme hold; palette is unchanged.
Sleep 7s
Screenshot demo/screenshot-paper.png
Type ","
Down
Enter
Sleep 1s
Escape
Sleep 4s
Screenshot demo/screenshot-waveform-contrast.png

# Finish in Paper (3), rather than changing the user's preferred palette.
Type ","
Up
Enter
Sleep 1s
Escape
Sleep 3s

# Return briefly to spectrum, then hold fullscreen with the album band visible.
Type "b"
Sleep 5s
Type "b"
Type "v"
Sleep 6s
Screenshot demo/screenshot-fullscreen.png
Type "v"
Sleep 3s
# Leave fullscreen so the album band is visible in the normal layout too.
Screenshot demo/screenshot-player.png
Hide
Type "q"
Enter
