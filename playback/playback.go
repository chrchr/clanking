// Package playback streams float32 PCM to an audio player: pw-cat on Linux, sox on macOS.
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

// ErrPlayerStuck reports a player that did not exit within StallTimeout after its input closed, so Close killed it.
var ErrPlayerStuck = errors.New("player did not exit after its input closed")

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

// Close ends the stream and waits for the player to drain and exit.
// A player still running after StallTimeout is killed, so a wedged audio device cannot hang the daemon's shutdown.
func (s *Stream) Close() error {
	closeErr := s.pipe.Close()
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case err := <-done:
		return errors.Join(closeErr, err)
	case <-time.After(StallTimeout):
		return errors.Join(closeErr, ErrPlayerStuck, s.cmd.Process.Kill(), <-done)
	}
}
