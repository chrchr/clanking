package control_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chrchr/clanking/control"
	"github.com/chrchr/clanking/internal/approx"
	"github.com/chrchr/clanking/synth"
)

const tolerance = 1e-9

const settingsFileMode = 0o600

var errDisk = errors.New("disk full")

func TestParseAcceptsRequests(t *testing.T) {
	cases := map[string]control.Request{
		"mute":        {Command: control.Mute},
		"unmute\n":    {Command: control.Unmute},
		"status":      {Command: control.Status},
		"toggle":      {Command: control.Toggle},
		"volume -6":   {Command: control.Volume, VolumeDB: -6},
		"volume 24\n": {Command: control.Volume, VolumeDB: 24},
		"volume up":   {Command: control.VolumeUp},
		"volume down": {Command: control.VolumeDown},
		"tempo 90":    {Command: control.Tempo, TempoBPM: 90},
		"root 62":     {Command: control.Root, RootNote: 62},
		"root 86":     {Command: control.Root, RootNote: 86},
	}
	for line, want := range cases {
		t.Run(line, func(t *testing.T) {
			if got, _ := control.Parse(line); got != want {
				t.Errorf("Parse(%q) = %+v, want %+v", line, got, want)
			}
		})
	}
}

func TestParseRejectsInvalidRequests(t *testing.T) {
	for _, line := range []string{"", "louder", "volume", "volume loud", "volume up 3", "volume NaN", "volume -Inf", "volume 24.5", "mute now", "tempo 59", "tempo 241", "tempo fast", "tempo 90.5", "root 61", "root 87", "root D5"} {
		t.Run(line, func(t *testing.T) {
			if _, err := control.Parse(line); !errors.Is(err, control.ErrInvalid) {
				t.Errorf("Parse(%q) error = %v, want %v", line, err, control.ErrInvalid)
			}
		})
	}
}

func TestRequestStringParsesBack(t *testing.T) {
	want := control.Request{Command: control.Volume, VolumeDB: -7.5}
	if got, _ := control.Parse(want.String()); got != want {
		t.Errorf("Parse(String()) = %+v, want %+v", got, want)
	}
}

func TestVolumeStepRequestStringParsesBack(t *testing.T) {
	for _, want := range []control.Request{{Command: control.VolumeUp}, {Command: control.VolumeDown}} {
		if got, _ := control.Parse(want.String()); got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", want.String(), got, want)
		}
	}
}

func TestApplyVolumeStepsByVolumeStepDB(t *testing.T) {
	s := control.Settings{VolumeDB: -6}
	if got := s.Apply(control.Request{Command: control.VolumeUp}).VolumeDB; !approx.Equal(got, -6+control.VolumeStepDB, tolerance) {
		t.Errorf("volume up from -6 = %g, want %g", got, -6+control.VolumeStepDB)
	}
	if got := s.Apply(control.Request{Command: control.VolumeDown}).VolumeDB; !approx.Equal(got, -6-control.VolumeStepDB, tolerance) {
		t.Errorf("volume down from -6 = %g, want %g", got, -6-control.VolumeStepDB)
	}
}

func TestApplyVolumeUpStopsAtMax(t *testing.T) {
	s := control.Settings{VolumeDB: control.MaxVolumeDB - 1}
	if got := s.Apply(control.Request{Command: control.VolumeUp}).VolumeDB; got != control.MaxVolumeDB {
		t.Errorf("volume up near max = %g, want %g", got, control.MaxVolumeDB)
	}
}

func TestRootRequestStringParsesBack(t *testing.T) {
	want := control.Request{Command: control.Root, RootNote: 75}
	if got, _ := control.Parse(want.String()); got != want {
		t.Errorf("Parse(String()) = %+v, want %+v", got, want)
	}
}

func TestSettingsStringReportsStatus(t *testing.T) {
	if got := (control.Settings{VolumeDB: -6, Muted: true, TempoBPM: 90, RootNote: 75}).String(); got != "volume -6 dB, muted, root D♯5, 90 bpm" {
		t.Errorf("String = %q, want %q", got, "volume -6 dB, muted, root D♯5, 90 bpm")
	}
}

func TestGainIsSilentWhenMuted(t *testing.T) {
	if got := (control.Settings{Muted: true}).Gain(); got != 0 {
		t.Errorf("Gain = %v, want 0", got)
	}
}

func TestGainFollowsVolume(t *testing.T) {
	if got := (control.Settings{VolumeDB: -20}).Gain(); !approx.Equal(0.1, got, tolerance) {
		t.Errorf("Gain = %v, want 0.1", got)
	}
}

func TestApplyVolumeKeepsMute(t *testing.T) {
	got := control.Settings{Muted: true}.Apply(control.Request{Command: control.Volume, VolumeDB: -3})
	if want := (control.Settings{VolumeDB: -3, Muted: true}); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}
}

func TestApplyUnmute(t *testing.T) {
	got := control.Settings{VolumeDB: -3, Muted: true}.Apply(control.Request{Command: control.Unmute})
	if want := (control.Settings{VolumeDB: -3}); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}
}

func TestApplyToggleMutes(t *testing.T) {
	got := control.Settings{VolumeDB: -3}.Apply(control.Request{Command: control.Toggle})
	if want := (control.Settings{VolumeDB: -3, Muted: true}); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}
}

func TestApplyToggleUnmutes(t *testing.T) {
	got := control.Settings{VolumeDB: -3, Muted: true}.Apply(control.Request{Command: control.Toggle})
	if want := (control.Settings{VolumeDB: -3}); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}
}

func TestHandleSavesChangedSettings(t *testing.T) {
	var saved control.Settings
	c := control.NewController(control.Settings{}, func(s control.Settings) error { saved = s; return nil }, func(control.Settings) {})
	_, _ = c.Handle(control.Request{Command: control.Mute})
	if want := (control.Settings{Muted: true}); saved != want {
		t.Errorf("saved = %+v, want %+v", saved, want)
	}
}

func TestHandleAppliesChangedSettings(t *testing.T) {
	var applied control.Settings
	c := control.NewController(control.Settings{}, func(control.Settings) error { return nil }, func(s control.Settings) { applied = s })
	_, _ = c.Handle(control.Request{Command: control.Volume, VolumeDB: -9})
	if want := (control.Settings{VolumeDB: -9}); applied != want {
		t.Errorf("applied = %+v, want %+v", applied, want)
	}
}

func TestHandleStatusSavesNothing(t *testing.T) {
	saves := 0
	c := control.NewController(control.Settings{}, func(control.Settings) error { saves++; return nil }, func(control.Settings) {})
	_, _ = c.Handle(control.Request{Command: control.Status})
	if saves != 0 {
		t.Errorf("saves = %d, want 0", saves)
	}
}

func TestHandleKeepsSettingsWhenSaveFails(t *testing.T) {
	c := control.NewController(control.Settings{}, func(control.Settings) error { return errDisk }, func(control.Settings) {})
	_, _ = c.Handle(control.Request{Command: control.Mute})
	if got, _ := c.Handle(control.Request{Command: control.Status}); got != (control.Settings{}) {
		t.Errorf("settings after failed save = %+v, want unchanged", got)
	}
}

func TestHandleReportsSaveFailure(t *testing.T) {
	c := control.NewController(control.Settings{}, func(control.Settings) error { return errDisk }, func(control.Settings) {})
	if _, err := c.Handle(control.Request{Command: control.Mute}); !errors.Is(err, errDisk) {
		t.Errorf("Handle error = %v, want %v", err, errDisk)
	}
}

func TestLoadWithoutFileGivesDefaults(t *testing.T) {
	if got, _ := control.Load(filepath.Join(t.TempDir(), "mixer.json")); got != (control.Settings{TempoBPM: control.DefaultTempoBPM, RootNote: control.DefaultRootNote}) {
		t.Errorf("Load = %+v, want defaults", got)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixer.json")
	want := control.Settings{VolumeDB: -4.5, Muted: true, TempoBPM: control.DefaultTempoBPM, RootNote: 70}
	_ = control.Save(path, want)
	if got, _ := control.Load(path); got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestParseSettingsReadsStatusLine(t *testing.T) {
	want := control.Settings{VolumeDB: -7.5, Muted: true, TempoBPM: 90, RootNote: 75}
	if got, _ := control.ParseSettings(want.String()); got != want {
		t.Errorf("ParseSettings(String()) = %+v, want %+v", got, want)
	}
}

func TestParseSettingsReadsUnmuted(t *testing.T) {
	want := control.Settings{VolumeDB: 24, TempoBPM: 240, RootNote: control.MinRootNote}
	if got, _ := control.ParseSettings(want.String()); got != want {
		t.Errorf("ParseSettings(String()) = %+v, want %+v", got, want)
	}
}

func TestParseSettingsRejectsMalformedLines(t *testing.T) {
	for _, line := range []string{"", "volume -6 dB, muted, 90 bpm", "volume loud dB, muted, root D5, 90 bpm", "volume -6 dB, quiet, root D5, 90 bpm", "volume -6 dB, muted, root D5, fast bpm", "volume -6 dB, muted, root H5, 90 bpm", "volume -6 dB, muted, D5, 90 bpm", "volume -6 dB, muted, root D5, 90 bpm, extra"} {
		t.Run(line, func(t *testing.T) {
			if _, err := control.ParseSettings(line); !errors.Is(err, control.ErrStatus) {
				t.Errorf("ParseSettings(%q) error = %v, want %v", line, err, control.ErrStatus)
			}
		})
	}
}

func TestApplyTempo(t *testing.T) {
	got := control.Settings{TempoBPM: control.DefaultTempoBPM}.Apply(control.Request{Command: control.Tempo, TempoBPM: 90})
	if want := (control.Settings{TempoBPM: 90}); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}
}

func TestLoadWithoutTempoFieldGivesDefaultTempo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixer.json")
	if err := os.WriteFile(path, []byte(`{"volume_db":-3,"muted":false}`), settingsFileMode); err != nil {
		t.Fatal(err)
	}
	if got, _ := control.Load(path); got.TempoBPM != control.DefaultTempoBPM {
		t.Errorf("TempoBPM = %d, want %d", got.TempoBPM, control.DefaultTempoBPM)
	}
}

func loadJSON(t *testing.T, saved string) control.Settings {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mixer.json")
	if err := os.WriteFile(path, []byte(saved), settingsFileMode); err != nil {
		t.Fatal(err)
	}
	s, _ := control.Load(path)
	return s
}

func loadTempo(t *testing.T, tempo string) int {
	t.Helper()
	return loadJSON(t, `{"tempo_bpm":`+tempo+`}`).TempoBPM
}

func loadRoot(t *testing.T, root string) int {
	t.Helper()
	return loadJSON(t, `{"root_note":`+root+`}`).RootNote
}

func TestLoadClampsSlowTempo(t *testing.T) {
	if got := loadTempo(t, "0"); got != control.MinTempoBPM {
		t.Errorf("TempoBPM = %d, want %d", got, control.MinTempoBPM)
	}
}

func TestLoadClampsFastTempo(t *testing.T) {
	if got := loadTempo(t, "999"); got != control.MaxTempoBPM {
		t.Errorf("TempoBPM = %d, want %d", got, control.MaxTempoBPM)
	}
}

func TestLoadKeepsValidTempo(t *testing.T) {
	if got := loadTempo(t, "90"); got != 90 {
		t.Errorf("TempoBPM = %d, want 90", got)
	}
}

func TestApplyRoot(t *testing.T) {
	got := control.Settings{RootNote: control.DefaultRootNote}.Apply(control.Request{Command: control.Root, RootNote: 70})
	if want := (control.Settings{RootNote: 70}); got != want {
		t.Errorf("Apply = %+v, want %+v", got, want)
	}
}

func TestLoadWithoutRootFieldGivesDefaultRoot(t *testing.T) {
	if got := loadJSON(t, `{"volume_db":-3}`).RootNote; got != control.DefaultRootNote {
		t.Errorf("RootNote = %d, want %d", got, control.DefaultRootNote)
	}
}

func TestLoadClampsLowRoot(t *testing.T) {
	if got := loadRoot(t, "0"); got != control.MinRootNote {
		t.Errorf("RootNote = %d, want %d", got, control.MinRootNote)
	}
}

func TestLoadClampsHighRoot(t *testing.T) {
	if got := loadRoot(t, "127"); got != control.MaxRootNote {
		t.Errorf("RootNote = %d, want %d", got, control.MaxRootNote)
	}
}

func TestLoadKeepsValidRoot(t *testing.T) {
	if got := loadRoot(t, "70"); got != 70 {
		t.Errorf("RootNote = %d, want 70", got)
	}
}

func TestDefaultRootIsC5(t *testing.T) {
	if got := synth.NoteName(control.DefaultRootNote); got != "C5" {
		t.Errorf("default root = %s, want C5", got)
	}
}
