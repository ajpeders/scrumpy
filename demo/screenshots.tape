# Render with: vhs demo/screenshots.tape   (Cider must be playing)
#
# README screenshots. Read-only: it searches but never changes playback, so
# start a song with lyrics and album art in Cider first. Shows catalog search
# results, not your library.
Output "/tmp/opencode/sidecue-screenshots.gif"

Set Shell zsh
Set FontSize 16
Set Width 1400
Set Height 850
Set Padding 16

# Off camera: isolated config with the real Cider token via symlink. VHS
# renders in a browser terminal that has no kitty graphics, so drop the
# kitty variables inherited from the host and use block-art covers.
Hide
Type `D=$(mktemp -d /tmp/opencode/cider-shots.XXXXXX) && mkdir -p $D/cfg/sidecue && ln -s "${XDG_CONFIG_HOME:-$HOME/.config}/sh.cider.genten" $D/cfg/sh.cider.genten`
Enter
Type `echo '{"theme":"Dark","background":true,"visualizer":"spectrum","artwork":true,"lyrics":true}' > $D/cfg/sidecue/settings.json`
Enter
Type "go build -o $D/bin . && clear && env -u KITTY_WINDOW_ID -u TERM_PROGRAM TERM=xterm-256color XDG_CONFIG_HOME=$D/cfg $D/bin"
Enter
Sleep 4s
Type "1"
Type "/"
Type "The Strokes"
Enter
Sleep 4s
Type "j"
Show

# Player: artwork, lyrics, spectrum and catalog search results
Sleep 3s
Screenshot demo/screenshot-player.png

# Waveform visualizer
Type "b"
Sleep 6s
Screenshot demo/screenshot-waveform.png

# Full-screen spectrum
Type "b"
Sleep 1s
Type "v"
Sleep 6s
Screenshot demo/screenshot-fullscreen.png
Sleep 1s

Hide
Type "q"
