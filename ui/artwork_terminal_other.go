//go:build !linux && !darwin

package ui

func terminalArtworkSize() (cols, rows int) {
	return artworkCols, artworkRows
}
