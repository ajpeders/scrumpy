# Render with: vhs demo/demo.tape   (Cider must be playing)
#
# Concise cut (~35 s): cold open on the animating visualizer + lyrics, one
# theme hop, one search→enqueue→play interaction, then the full-screen
# visualizer as the payoff and a key-help closer. No dead air, no private
# library rows, and a neutral search query.
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
Type "D=/tmp/cider-tui-vhs && mkdir -p $D/cfg/cider-tui && ln -sfn ~/.config/sh.cider.genten $D/cfg/sh.cider.genten"
Enter
Type `echo '{"theme":"Dark","background":true,"visualizer":"spectrum"}' > $D/cfg/cider-tui/settings.json`
Enter
Type "go build -o $D/bin . && clear && XDG_CONFIG_HOME=$D/cfg $D/bin"
Enter
Sleep 3s
Show

# Hook: opens on Queue with the spectrum animating and synced lyrics
Sleep 6s

# One theme hop, recoloring live
Type ","
Type "j"
Enter
Type ","
Sleep 4s

# Search the catalog, land on a result, enqueue it
Type "1"
Sleep 1s
Type "/"
Type "fleetwood mac"
Enter
Sleep 3s
Type "j"
Sleep 2s
Type "a"
Sleep 3s

# Land back on Queue and play from the enqueued song
Type "4"
Sleep 2s
Type "jjjjjjjjjj"
Sleep 1s
Enter
Sleep 3s

# Payoff: switch to waveform, then full-screen visualizer
Type "b"
Sleep 4s
Type "v"
Sleep 6s
Type "v"

# Closer: key help, then quit
Type "?"
Sleep 3s
Type "?"
Sleep 1s

Type "q"