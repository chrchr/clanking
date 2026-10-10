package statedir_test

import (
	"path/filepath"
	"testing"

	"github.com/chrchr/clanking/statedir"
)

func TestPathLivesUnderStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, _ := statedir.Path("clanking.log"); got != filepath.FromSlash("/state/clanking/clanking.log") {
		t.Errorf("Path = %q, want /state/clanking/clanking.log", got)
	}
}

func TestPathFallsBackToHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/u")
	t.Setenv("USERPROFILE", "/home/u")
	if got, _ := statedir.Path("clanking.log"); got != filepath.FromSlash("/home/u/.local/state/clanking/clanking.log") {
		t.Errorf("Path = %q, want /home/u/.local/state/clanking/clanking.log", got)
	}
}

func TestRunPathLivesInSharedRunDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, _ := statedir.RunPath(statedir.ControlSocket); got != filepath.FromSlash("/state/clanking/run/clanking.ctl") {
		t.Errorf("RunPath = %q, want /state/clanking/run/clanking.ctl", got)
	}
}

func TestOpenLogCreatesStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	f, err := statedir.OpenLog()
	if err == nil {
		_ = f.Close()
	}
	if err != nil {
		t.Errorf("OpenLog error = %v, want nil", err)
	}
}

func TestCueSocketLivesInSharedRunDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, _ := statedir.RunPath(statedir.CueSocket); got != filepath.FromSlash("/state/clanking/run/clanking.sock") {
		t.Errorf("RunPath = %q, want /state/clanking/run/clanking.sock", got)
	}
}

func TestEventsSocketLivesInSharedRunDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got, _ := statedir.RunPath(statedir.EventsSocket); got != filepath.FromSlash("/state/clanking/run/clanking.events") {
		t.Errorf("RunPath = %q, want /state/clanking/run/clanking.events", got)
	}
}
