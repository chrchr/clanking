package playback

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"github.com/chrchr/clanking/pan"
	"github.com/chrchr/clanking/synth"
)

// StallTimeout is how long a write may wait on a player that stopped reading.
const StallTimeout = time.Second

var pwCatArgs = []string{
	"--playback", "--raw",
	"--format", "f32",
	"--rate", strconv.Itoa(synth.SampleRate),
	"--channels", strconv.Itoa(pan.Channels),
	"--properties", "application.name=clanking",
	"-",
}

// Command returns the pw-cat invocation that plays raw interleaved stereo float32 PCM from stdin.
func Command() *exec.Cmd { return exec.Command("pw-cat", pwCatArgs...) }

func shrink(pipe *os.File) error {
	raw, err := pipe.SyscallConn()
	if err != nil {
		return err
	}
	var fcntlErr error
	if err := raw.Control(func(fd uintptr) { _, fcntlErr = unix.FcntlInt(fd, unix.F_SETPIPE_SZ, PipeBytes) }); err != nil {
		return err
	}
	return fcntlErr
}
