package playback

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/chrchr/clanking/pan"
	"github.com/chrchr/clanking/synth"
)

// StallTimeout is how long a write may wait on a player that stopped reading.
// Windows pipes take no deadlines, so this applies only where the pipe supports one.
const StallTimeout = 5 * time.Second

// ffplayArgs plays raw little-endian float32 PCM from stdin with no window and minimal buffering.
var ffplayArgs = []string{
	"-hide_banner", "-loglevel", "error", "-nodisp",
	"-fflags", "nobuffer",
	"-f", "f32le",
	"-ar", strconv.Itoa(synth.SampleRate),
	"-ch_layout", channelLayout(pan.Channels),
	"-i", "-",
}

func channelLayout(channels int) string {
	if channels == 1 {
		return "mono"
	}
	return "stereo"
}

// Command returns the ffplay invocation that plays raw interleaved stereo float32 PCM from stdin.
func Command() *exec.Cmd { return exec.Command("ffplay", ffplayArgs...) }

// shrink is a no-op: Windows pipes keep their default size.
func shrink(*os.File) error { return nil }
