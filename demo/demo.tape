# Render with: vhs demo/demo.tape   (Cider must be playing)
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

# Opens on Queue: visualizer + synced lyrics
Sleep 5s

# Tour the tabs
Type "2"
Sleep 2s
Type "jjj"
Sleep 1s
Type "d"
Sleep 2s
Type "d"
Type "3"
Sleep 2s

# Search the catalog (no playback change)
Type "1"
Sleep 1s
Type "/"
Type "daft punk"
Enter
Sleep 3s
Type "jj"
Sleep 2s
Escape

# Switch visualizer style, then full-screen
Type "b"
Sleep 3s
Type "v"
Sleep 4s
Type "v"

# Expanded lyrics
Type "y"
Sleep 4s
Type "y"

# Key help
Type "?"
Sleep 3s
Type "?"
Sleep 1s

Type "q"
