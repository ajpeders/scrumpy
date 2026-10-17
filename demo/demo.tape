# Render with: vhs demo/demo.tape   (Cider must be running)
#
# Under a minute: opens mid-song on Hard to Explain by The Strokes with the
# visualizer and lyrics, searches the catalog and plays I Wonder by Kanye
# West, opens the 102026 playlist, the details panel and the Queue, then
# waveform and full-screen visualizer.
Output demo/demo.gif
Output demo/demo.mp4

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 850
Set Padding 16
Set TypingSpeed 80ms

# Off camera: isolated config with the real Cider token via symlink, then
# start Hard to Explain and seek to about its middle (22 × 5 s ≈ 1:50).
Hide
Type `D=$(mktemp -d /tmp/opencode/cider-demo.XXXXXX) && mkdir -p $D/cfg/sidecue && ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" $D/cfg/sh.cider.genten`
Enter
Type `echo '{"theme":"Dark","background":true,"visualizer":"spectrum"}' > $D/cfg/sidecue/settings.json`
Enter
Type "go build -o $D/bin . && clear && XDG_CONFIG_HOME=$D/cfg $D/bin"
Enter
Sleep 3s
Type "1"
Type "/"
Type "Hard to Explain The Strokes"
Enter
Sleep 3s
Enter
Sleep 3s
Right@100ms 22
Type "4"
Sleep 2s
Show

# Hook: mid-song, spectrum animating with synced lyrics
Sleep 7s

# Catalog search: play I Wonder by Kanye West
Type "1"
Sleep 500ms
Type "/"
# "/" pre-fills the last query (Hard to Explain…); clear it first
Backspace@5ms 27
Type "I Wonder Kanye West"
Enter
Sleep 3s
Enter
Sleep 5s

# Playlists: filter to 102026 and open it
Type "3"
Sleep 2s
Type "/"
Type "102026"
Enter
Sleep 1s
Enter
Sleep 3s
Type "jj"
Sleep 1s

# Details panel for the selected track
Type "d"
Sleep 3s
Type "d"

# Queue
Type "4"
Sleep 3s

# Payoff: waveform, then full-screen visualizer
Type "b"
Sleep 3s
Type "v"
Sleep 6s
Type "v"

# End on the player rather than a shell prompt
Sleep 2s
Hide
Type "q"
