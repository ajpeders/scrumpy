package viz

import (
	"context"
	"testing"
)

func TestMissingPipeWireExecutableDisablesAnalyzer(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	frames := NewAnalyzer(12).Run(context.Background())
	if _, open := <-frames; open {
		t.Fatal("expected closed bar channel without pw-record")
	}
}
