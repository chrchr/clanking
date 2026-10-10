package transport

import (
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/chrchr/clanking/synth"
)

// Windows has no Unix datagram sockets, so each cue travels over its own stream connection.
const network = "unix"

// Listen binds the daemon's cue socket, replacing any stale socket file.
func Listen(path string) (*net.UnixListener, error) {
	if err := removeStale(path); err != nil {
		return nil, err
	}
	return net.ListenUnix(network, &net.UnixAddr{Name: path, Net: network})
}

// Serve hands each received cue and working directory to play until the listener is closed.
func Serve(l *net.UnixListener, play func(synth.Cue, string)) error {
	for {
		conn, err := l.AcceptUnix()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}
		go receive(conn, play)
	}
}

func receive(conn *net.UnixConn, play func(synth.Cue, string)) {
	defer func() { _ = conn.Close() }()
	if err := conn.SetReadDeadline(time.Now().Add(sendTimeout)); err != nil {
		return
	}
	msg, err := io.ReadAll(io.LimitReader(conn, maxDatagram))
	if err != nil {
		return
	}
	cue, cwd, _ := strings.Cut(string(msg), fieldSeparator)
	play(synth.Cue(cue), cwd)
}

// Send delivers one cue and the sender's working directory to the daemon socket.
func Send(path string, c synth.Cue, cwd string) error {
	conn, err := net.DialUnix(network, nil, &net.UnixAddr{Name: path, Net: network})
	if err != nil {
		return err
	}
	if err := conn.SetWriteDeadline(time.Now().Add(sendTimeout)); err != nil {
		return errors.Join(err, conn.Close())
	}
	_, err = conn.Write([]byte(string(c) + fieldSeparator + cwd))
	return errors.Join(err, conn.Close())
}
