// Package playback streams float32 PCM to an audio player: pw-cat on Linux, sox on macOS, ffplay on Windows.
package playback

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"time"
)

// PipeBytes bounds the pipe to the player so that queued audio stays short.
const PipeBytes = 4096

// Encode appends samples to dst as little-endian float32 bytes.
func Encode(dst []byte, samples []float32) []byte {
	for _, x := range samples {
		dst = binary.LittleEndian.AppendUint32(dst, math.Float32bits(x))
	}
	return dst
}

// Stream is a running pw-cat fed through a short pipe.
type Stream struct {
	cmd  *exec.Cmd
	pipe *os.File
}

// Start launches a player reading raw PCM from a short pipe.
func Start(cmd *exec.Cmd) (*Stream, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	if err := shrink(w); err != nil {
		return nil, err
	}
	cmd.Stdin = r
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Stream{cmd: cmd, pipe: w}, r.Close()
}

// Write sends encoded PCM to the player, failing if it stalls past StallTimeout.
func (s *Stream) Write(p []byte) (int, error) {
	// Windows pipes take no deadlines; there a stalled player blocks the write.
	if err := s.pipe.SetWriteDeadline(time.Now().Add(StallTimeout)); err != nil && !errors.Is(err, os.ErrNoDeadline) {
		return 0, err
	}
	return s.pipe.Write(p)
}

// Close ends the stream and waits for the player to drain and exit.
func (s *Stream) Close() error { return errors.Join(s.pipe.Close(), s.cmd.Wait()) }
