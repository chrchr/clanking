package playback

import (
	"slices"
	"testing"
)

func TestCommandPlaysStereoLayout(t *testing.T) {
	args := Command().Args
	i := slices.Index(args, "-ch_layout")
	if i < 0 || args[i+1] != "stereo" {
		t.Errorf("args = %q, want -ch_layout stereo", args)
	}
}
