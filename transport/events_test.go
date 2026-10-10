package transport_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chrchr/clanking/events"
	"github.com/chrchr/clanking/transport"
)

const (
	eventsName  = "clanking.events"
	snapshot    = "settings volume 0 dB, unmuted, 120 bpm"
	eventWait   = time.Second
	watchBuffer = 8
)

func serveEvents(t *testing.T, hub *events.Hub) string {
	t.Helper()
	path := filepath.Join(shortTempDir(t), eventsName)
	l, err := transport.ListenStream(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() { _ = transport.ServeEvents(l, hub.Subscribe, func() string { return snapshot }) }()
	return path
}

func watch(path string) <-chan string {
	lines := make(chan string, watchBuffer)
	go func() { _ = transport.Watch(path, func(line string) { lines <- line }) }()
	return lines
}

func next(t *testing.T, lines <-chan string) string {
	t.Helper()
	select {
	case line := <-lines:
		return line
	case <-time.After(eventWait):
		return ""
	}
}

func TestWatchFirstReceivesSnapshot(t *testing.T) {
	path := serveEvents(t, events.NewHub())
	if got := next(t, watch(path)); got != snapshot {
		t.Errorf("first line = %q, want %q", got, snapshot)
	}
}

func TestWatchReceivesPublishedLine(t *testing.T) {
	hub := events.NewHub()
	path := serveEvents(t, hub)
	lines := watch(path)
	next(t, lines)
	hub.Publish("cue spawn")
	if got := next(t, lines); got != "cue spawn" {
		t.Errorf("line = %q, want %q", got, "cue spawn")
	}
}

func TestWatchReturnsWhenServerCloses(t *testing.T) {
	path := filepath.Join(shortTempDir(t), eventsName)
	l, err := transport.ListenStream(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		conn, _ := l.Accept()
		_ = conn.Close()
	}()
	done := make(chan error, 1)
	go func() { done <- transport.Watch(path, func(string) {}) }()
	select {
	case <-done:
	case <-time.After(eventWait):
		t.Error("Watch did not return after the server closed")
	}
}
