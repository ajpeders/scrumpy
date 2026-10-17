//go:build linux || darwin

package ui

import (
	"os"

	"golang.org/x/sys/unix"
)

func terminalArtworkSize() (cols, rows int) {
	for _, f := range []*os.File{os.Stdout, os.Stdin} {
		ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
		if err == nil && ws.Col > 0 && ws.Row > 0 && ws.Xpixel > 0 && ws.Ypixel > 0 {
			return artworkSizeForCells(int(ws.Xpixel)/int(ws.Col), int(ws.Ypixel)/int(ws.Row))
		}
	}
	return artworkCols, artworkRows
}
