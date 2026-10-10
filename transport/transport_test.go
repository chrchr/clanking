package transport_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chrchr/clanking/synth"
	"github.com/chrchr/clanking/transport"
)

const (
	socketName    = "clanking.sock"
	receiveWait   = time.Second
	shortTimeout  = 20 * time.Millisecond
	retryInterval = 5 * time.Millisecond
	stallWait     = 2 * time.Second
)

var errSpawn = errors.New("spawn failed")

func socketPath(t *testing.T) string { return filepath.Join(shortTempDir(t), socketName) }

// shortTempDir keeps socket paths under macOS's 104-byte limit, which t.TempDir can exceed.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "clanking")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

type delivery struct {
	cue synth.Cue
	cwd string
}

func serve(t *testing.T, path string) <-chan delivery {
	t.Helper()
	conn, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	got := make(chan delivery, 1)
	go func() { _ = transport.Serve(conn, func(c synth.Cue, cwd string) { got <- delivery{c, cwd} }) }()
	return got
}

func receive(got <-chan delivery) delivery {
	select {
	case d := <-got:
		return d
	case <-time.After(receiveWait):
		return delivery{}
	}
}

func noSpawn() error { return nil }

func sendCue(path string, c synth.Cue) func() error {
	return func() error { return transport.Send(path, c, "") }
}

func TestSendReachesServer(t *testing.T) {
	path := socketPath(t)
	cues := serve(t, path)
	_ = transport.Send(path, synth.Done, "")
	if got := receive(cues).cue; got != synth.Done {
		t.Errorf("received %q, want %q", got, synth.Done)
	}
}

func TestSendWithoutDaemonFails(t *testing.T) {
	if err := transport.Send(socketPath(t), synth.Done, ""); err == nil {
		t.Error("Send error = nil, want an error")
	}
}

func TestListenReplacesStaleSocket(t *testing.T) {
	path := socketPath(t)
	_ = os.WriteFile(path, nil, 0o600)
	cues := serve(t, path)
	_ = transport.Send(path, synth.Tick, "")
	if got := receive(cues).cue; got != synth.Tick {
		t.Errorf("received %q, want %q", got, synth.Tick)
	}
}

func TestServeReturnsNilWhenClosed(t *testing.T) {
	conn, _ := transport.Listen(socketPath(t))
	_ = conn.Close()
	if err := transport.Serve(conn, func(synth.Cue, string) {}); err != nil {
		t.Errorf("Serve error = %v, want nil", err)
	}
}

func TestDeliverSkipsSpawnWhenDaemonRuns(t *testing.T) {
	path := socketPath(t)
	serve(t, path)
	spawned := false
	_ = transport.Deliver(sendCue(path, synth.Tick), func() error { spawned = true; return nil }, shortTimeout, retryInterval)
	if spawned {
		t.Error("spawned = true, want false")
	}
}

func TestDeliverSpawnsDaemonAndSends(t *testing.T) {
	path := socketPath(t)
	var cues <-chan delivery
	_ = transport.Deliver(sendCue(path, synth.Done), func() error { cues = serve(t, path); return nil }, shortTimeout, retryInterval)
	if got := receive(cues).cue; got != synth.Done {
		t.Errorf("received %q, want %q", got, synth.Done)
	}
}

func TestDeliverReportsSpawnFailure(t *testing.T) {
	err := transport.Deliver(sendCue(socketPath(t), synth.Done), func() error { return errSpawn }, shortTimeout, retryInterval)
	if !errors.Is(err, errSpawn) {
		t.Errorf("Deliver error = %v, want %v", err, errSpawn)
	}
}

func TestDeliverTimesOutWhenDaemonNeverAppears(t *testing.T) {
	if err := transport.Deliver(sendCue(socketPath(t), synth.Done), noSpawn, shortTimeout, retryInterval); err == nil {
		t.Error("Deliver error = nil, want an error")
	}
}

func TestSendGivesUpWhenDaemonStopsReading(t *testing.T) {
	path := socketPath(t)
	conn, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	failed := make(chan struct{})
	go func() {
		var err error
		for err == nil {
			err = transport.Send(path, synth.Tick, "")
		}
		close(failed)
	}()
	select {
	case <-failed:
	case <-time.After(stallWait):
		t.Error("Send still blocked after the daemon stopped reading")
	}
}

const (
	spacedCwd      = "/home/rc/My Projects/clanking"
	longPathLength = 4000
)

func TestSendCarriesCwdWithSpaces(t *testing.T) {
	path := socketPath(t)
	got := serve(t, path)
	_ = transport.Send(path, synth.Done, spacedCwd)
	if d := receive(got); d.cwd != spacedCwd {
		t.Errorf("cwd = %q, want %q", d.cwd, spacedCwd)
	}
}

func TestSendCarriesCueBesideCwd(t *testing.T) {
	path := socketPath(t)
	got := serve(t, path)
	_ = transport.Send(path, synth.Done, spacedCwd)
	if d := receive(got); d.cue != synth.Done {
		t.Errorf("cue = %q, want %q", d.cue, synth.Done)
	}
}

func TestSendCarriesLongCwd(t *testing.T) {
	path := socketPath(t)
	got := serve(t, path)
	long := "/" + strings.Repeat("d", longPathLength)
	_ = transport.Send(path, synth.Tick, long)
	if d := receive(got); d.cwd != long {
		t.Errorf("cwd length = %d, want %d", len(d.cwd), len(long))
	}
}
