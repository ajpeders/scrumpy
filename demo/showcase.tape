# Short showcase: playlist queueing, shuffle, five themes, both visualizers.
# Run from the repository root: vhs demo/showcase.tape
# Cider must be running and signed in; playlist 102026 must exist.
# Uses real playback/queue, with isolated appearance settings. No seeking.
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
Type `D=$(mktemp -d /tmp/opencode/cider-showcase.XXXXXX) && mkdir -p "$D/config/scrumpy" "$D/cache" && ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" "$D/config/sh.cider.genten"`
Enter
Type `printf '%s\n' '{"theme":"Dark","background":true,"visualizer":"spectrum","artwork":false,"lyrics":true}' > "$D/config/scrumpy/settings.json"`
Enter
Type `go build -o "$D/scrumpy" . && clear && XDG_CONFIG_HOME="$D/config" XDG_CACHE_HOME="$D/cache" "$D/scrumpy"`
Enter
Sleep 5s
Type "1/"
Type "Hard to Explain The Strokes"
Enter
Sleep 4s
Enter
Sleep 4s
Type "4"
Sleep 2s
Show
Sleep 4s

# A short catalog search-to-play, before adding the playlist to its queue.
Type "1/"
Backspace 30
Type "I Wonder Kanye West"
Enter
Sleep 4s
Enter
Sleep 4s

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

# Show both shuffle states, ending where we began. Skip in shuffled order.
Type "s"
Sleep 3s
Type "n"
Sleep 6s
Type "s"
Sleep 3s

# Cursor starts at Dark (1), Up reaches Terminal (0).
# Reopening Settings selects the active theme, not the last cursor position.
Type ","
Up
Enter
Sleep 4s
Down
Enter
Sleep 4s
Down
Enter
Sleep 2s
Escape
Sleep 4s
Type ","
Down
Enter
Sleep 2s
Escape
# Paper's normal layout gets the longest theme hold; palette is unchanged.
Sleep 7s
Screenshot /tmp/opencode/scrumpy-legacy-paper.png
Type ","
Down
Enter
Sleep 4s

# Finish in Paper (3), rather than changing the user's preferred palette.
Up
Enter
Sleep 1s
Escape
Sleep 3s

# Spectrum, waveform, waveform fullscreen, then normal layout.
Type "b"
Sleep 5s
Type "v"
Sleep 6s
Type "v"
Sleep 3s
Hide
Type "q"
Enter
