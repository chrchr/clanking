package transport_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/chrchr/clanking/control"
	"github.com/chrchr/clanking/transport"
)

const (
	controlName = "clanking.ctl"
	gridState   = "in 16ths, 0 queued, 0 dropped"
)

var errSave = errors.New("save failed")

func controlPath(t *testing.T) string { return filepath.Join(shortTempDir(t), controlName) }

func serveControl(t *testing.T, path string, handle func(control.Request) (control.Settings, error)) {
	t.Helper()
	l, err := transport.ListenStream(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() { _ = transport.ServeControl(l, handle, func() string { return gridState }) }()
}

func TestAskReturnsDaemonStatus(t *testing.T) {
	path := controlPath(t)
	serveControl(t, path, func(r control.Request) (control.Settings, error) {
		return control.Settings{RootNote: control.DefaultRootNote}.Apply(r), nil
	})
	got, _ := transport.Ask(path, control.Request{Command: control.Volume, VolumeDB: -6})
	if want := "volume -6 dB, unmuted, root C5, 0 bpm; " + gridState; got != want {
		t.Errorf("Ask = %q, want %q", got, want)
	}
}

func TestAskReportsDaemonError(t *testing.T) {
	path := controlPath(t)
	serveControl(t, path, func(control.Request) (control.Settings, error) { return control.Settings{}, errSave })
	if _, err := transport.Ask(path, control.Request{Command: control.Mute}); !errors.Is(err, transport.ErrDaemon) {
		t.Errorf("Ask error = %v, want %v", err, transport.ErrDaemon)
	}
}

func TestAskWithoutDaemonFails(t *testing.T) {
	if _, err := transport.Ask(controlPath(t), control.Request{Command: control.Status}); err == nil {
		t.Error("Ask error = nil, want an error")
	}
}

func TestServeControlReturnsNilWhenClosed(t *testing.T) {
	l, _ := transport.ListenStream(controlPath(t))
	_ = l.Close()
	if err := transport.ServeControl(l, nil, nil); err != nil {
		t.Errorf("ServeControl error = %v, want nil", err)
	}
}

func TestControlReturnsDaemonSettings(t *testing.T) {
	path := controlPath(t)
	serveControl(t, path, func(r control.Request) (control.Settings, error) { return control.Settings{}.Apply(r), nil })
	got, _, _ := transport.Control(path, control.Request{Command: control.Mute}, func() error { return nil })
	if want := (control.Settings{Muted: true}); got != want {
		t.Errorf("Control = %+v, want %+v", got, want)
	}
}

func TestControlSpawnsDaemonWhenNoneAnswers(t *testing.T) {
	path := controlPath(t)
	spawn := func() error {
		serveControl(t, path, func(r control.Request) (control.Settings, error) { return control.Settings{}.Apply(r), nil })
		return nil
	}
	got, _, _ := transport.Control(path, control.Request{Command: control.Volume, VolumeDB: -9}, spawn)
	if want := (control.Settings{VolumeDB: -9}); got != want {
		t.Errorf("Control = %+v, want %+v", got, want)
	}
}

func TestControlReportsDaemonError(t *testing.T) {
	path := controlPath(t)
	serveControl(t, path, func(control.Request) (control.Settings, error) { return control.Settings{}, errSave })
	if _, _, err := transport.Control(path, control.Request{Command: control.Mute}, func() error { return nil }); !errors.Is(err, transport.ErrDaemon) {
		t.Errorf("Control error = %v, want %v", err, transport.ErrDaemon)
	}
}

func TestControlReturnsGridState(t *testing.T) {
	path := controlPath(t)
	serveControl(t, path, func(r control.Request) (control.Settings, error) { return control.Settings{}.Apply(r), nil })
	if _, got, _ := transport.Control(path, control.Request{Command: control.Status}, func() error { return nil }); got != gridState {
		t.Errorf("grid = %q, want %q", got, gridState)
	}
}
