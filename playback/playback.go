// Package playback streams float32 PCM to PipeWire through pw-cat.
package playback

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"os/exec"
	"time"
)

// PipeBytes bounds the pipe to pw-cat so that queued audio stays short.
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
	if err := s.pipe.SetWriteDeadline(time.Now().Add(StallTimeout)); err != nil {
		return 0, err
	}
	return s.pipe.Write(p)
}

// Close ends the stream and waits for pw-cat to drain and exit.
func (s *Stream) Close() error { return errors.Join(s.pipe.Close(), s.cmd.Wait()) }
