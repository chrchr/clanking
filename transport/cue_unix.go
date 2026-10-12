//go:build !windows

package transport

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/chrchr/clanking/synth"
)

const network = "unixgram"

// Listen binds the daemon's cue socket, replacing any stale socket file.
func Listen(path string) (*net.UnixConn, error) {
	if err := removeStale(path); err != nil {
		return nil, err
	}
	return net.ListenUnixgram(network, &net.UnixAddr{Name: path, Net: network})
}

// Serve hands each received cue and working directory to play until the connection is closed.
func Serve(conn *net.UnixConn, play func(synth.Cue, string)) error {
	buf := make([]byte, maxDatagram)
	for {
		n, err := conn.Read(buf)
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}
		cue, cwd, _ := strings.Cut(string(buf[:n]), fieldSeparator)
		play(synth.Cue(cue), cwd)
	}
}

// Send delivers one cue and the sender's working directory to the daemon socket.
func Send(path string, c synth.Cue, cwd string) error {
	conn, err := net.DialUnix(network, nil, &net.UnixAddr{Name: path, Net: network})
	if err != nil {
		return err
	}
	// macOS caps datagrams at the send buffer size, 2 KiB by default.
	if err := conn.SetWriteBuffer(maxDatagram); err != nil {
		return errors.Join(err, conn.Close())
	}
	if err := conn.SetWriteDeadline(time.Now().Add(sendTimeout)); err != nil {
		return errors.Join(err, conn.Close())
	}
	_, err = conn.Write([]byte(string(c) + fieldSeparator + cwd))
	return errors.Join(err, conn.Close())
}
