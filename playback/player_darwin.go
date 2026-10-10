package playback

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/chrchr/clanking/pan"
	"github.com/chrchr/clanking/synth"
)

// soxArgs plays raw little-endian float32 PCM from stdin through sox's CoreAudio output.
// StallTimeout is how long a write may wait on a player that stopped reading. CoreAudio can take over a second to start on a cold boot.
const StallTimeout = 5 * time.Second

var soxArgs = []string{
	"-q", "--buffer", strconv.Itoa(PipeBytes),
	"-t", "raw", "-e", "floating-point", "-b", "32", "-L",
	"-r", strconv.Itoa(synth.SampleRate),
	"--channels", strconv.Itoa(pan.Channels),
	"-",
}

// Command returns the sox play invocation that plays raw interleaved stereo float32 PCM from stdin.
func Command() *exec.Cmd { return exec.Command("play", soxArgs...) }

// shrink is a no-op: macOS has no F_SETPIPE_SZ, so the pipe keeps its default size.
func shrink(*os.File) error { return nil }
