# Render with: vhs demo/demo.tape   (Cider must be playing)
#
# Concise cut (~30 s): cold open on the animating visualizer + lyrics,
# one search→play interaction, then the full-screen
# visualizer as the payoff. Start Cider on I Wonder by Kanye West.
Output demo/demo.gif
Output demo/demo.mp4

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 850
Set Padding 16
Set TypingSpeed 80ms

# Isolated config: demo theme, real Cider token via symlink
Hide
Type `D=$(mktemp -d /tmp/opencode/cider-demo.XXXXXX) && mkdir -p $D/cfg/sidecue && ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" $D/cfg/sh.cider.genten`
Enter
Type `echo '{"theme":"Dark","background":true,"visualizer":"spectrum"}' > $D/cfg/sidecue/settings.json`
Enter
Type "go build -o $D/bin . && clear && XDG_CONFIG_HOME=$D/cfg $D/bin"
Enter
Sleep 3s
Show

# Hook: opens on Queue with the spectrum animating and synced lyrics
Sleep 6s

# Search the catalog and play the first result: I Wonder by Kanye West
Type "1"
Sleep 1s
Type "/"
Type "I Wonder Kanye West"
Enter
Sleep 3s
Sleep 2s
Enter
Sleep 3s

# Show the resulting Queue without guessing the selected track's position
Type "4"
Sleep 3s

# Payoff: switch to waveform, then full-screen visualizer
Type "b"
Sleep 3s
Type "v"
Sleep 6s
Type "v"

# End on the player rather than a shell prompt
Sleep 2s
Hide
Type "q"
