package daemon_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/chrchr/clanking/control"
	"github.com/chrchr/clanking/daemon"
	"github.com/chrchr/clanking/grid"
	"github.com/chrchr/clanking/internal/approx"
	"github.com/chrchr/clanking/mixer"
	"github.com/chrchr/clanking/pan"
	"github.com/chrchr/clanking/synth"
)

const (
	lockName       = "clanking.lock"
	idle           = 10 * time.Millisecond
	clockStep      = time.Millisecond
	unity          = 1.0
	wideCeiling    = 100.0
	sample         = 0.1
	tolerance      = 1e-4
	bytesPerSample = 4
	hardLeft       = -1.0
)

var errBroken = errors.New("pipe broken")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errBroken }

func steppingClock() func() time.Time {
	t := time.Unix(0, 0)
	return func() time.Time {
		t = t.Add(clockStep)
		return t
	}
}

func play(sounds <-chan mixer.Sound) *bytes.Buffer {
	var out bytes.Buffer
	_ = daemon.Play(mixer.New(unity, wideCeiling), grid.New(control.DefaultTempoBPM), sounds, make(chan control.Settings), &out, idle, steppingClock(), func(mixer.Sound) {})
	return &out
}

func TestLockExcludesSecondHolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	first, _, _ := daemon.Lock(path)
	t.Cleanup(func() { _ = first.Close() })
	if _, held, _ := daemon.Lock(path); held {
		t.Error("second Lock held = true, want false")
	}
}

func TestLockIsFreeAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	first, _, _ := daemon.Lock(path)
	_ = first.Close()
	second, held, _ := daemon.Lock(path)
	t.Cleanup(func() { _ = second.Close() })
	if !held {
		t.Error("Lock after release held = false, want true")
	}
}

func TestPlayStopsWhenIdle(t *testing.T) {
	err := daemon.Play(mixer.New(unity, wideCeiling), grid.New(control.DefaultTempoBPM), make(chan mixer.Sound), make(chan control.Settings), &bytes.Buffer{}, idle, steppingClock(), func(mixer.Sound) {})
	if err != nil {
		t.Errorf("Play error = %v, want nil", err)
	}
}

func TestPlayWritesWholeBlocks(t *testing.T) {
	if got := play(make(chan mixer.Sound)).Len() % (pan.Channels * daemon.BlockFrames * bytesPerSample); got != 0 {
		t.Errorf("output length remainder = %d, want 0", got)
	}
}

func TestPlayWritesQueuedSound(t *testing.T) {
	sounds := make(chan mixer.Sound, 1)
	sounds <- mixer.Sound{Samples: []float32{sample}, Pan: hardLeft}
	got := float64(math.Float32frombits(binary.LittleEndian.Uint32(play(sounds).Bytes())))
	if !approx.Equal(sample, got, tolerance) {
		t.Errorf("first sample = %v, want %v", got, sample)
	}
}

func TestPlayReportsWriteFailure(t *testing.T) {
	err := daemon.Play(mixer.New(unity, wideCeiling), grid.New(control.DefaultTempoBPM), make(chan mixer.Sound), make(chan control.Settings), failingWriter{}, idle, steppingClock(), func(mixer.Sound) {})
	if !errors.Is(err, errBroken) {
		t.Errorf("Play error = %v, want %v", err, errBroken)
	}
}

type brokenAfter struct{ writes int }

func (w *brokenAfter) Write(p []byte) (int, error) {
	if w.writes == 0 {
		return 0, errBroken
	}
	w.writes--
	return len(p), nil
}

func TestPlayWithNeverIdleKeepsPlaying(t *testing.T) {
	out := &brokenAfter{writes: 100}
	err := daemon.Play(mixer.New(unity, wideCeiling), grid.New(control.DefaultTempoBPM), make(chan mixer.Sound), make(chan control.Settings), out, daemon.NeverIdle, steppingClock(), func(mixer.Sound) {})
	if !errors.Is(err, errBroken) {
		t.Errorf("Play error = %v, want %v", err, errBroken)
	}
}

func TestLockRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, lockName)
	if err := os.Symlink(filepath.Join(dir, "target"), path); err != nil {
		t.Skip("cannot create symlinks here:", err)
	}
	if _, _, err := daemon.Lock(path); err == nil {
		t.Error("Lock error = nil, want an error for a symlinked lock")
	}
}

func TestPlayAppliesQueuedGain(t *testing.T) {
	sounds := make(chan mixer.Sound, 1)
	sounds <- mixer.Sound{Samples: []float32{sample}, Pan: hardLeft}
	settings := make(chan control.Settings, 1)
	settings <- control.Settings{Muted: true, TempoBPM: control.DefaultTempoBPM}
	var out bytes.Buffer
	_ = daemon.Play(mixer.New(unity, wideCeiling), grid.New(control.DefaultTempoBPM), sounds, settings, &out, idle, steppingClock(), func(mixer.Sound) {})
	if got := math.Float32frombits(binary.LittleEndian.Uint32(out.Bytes())); got != 0 {
		t.Errorf("first sample = %v, want 0", got)
	}
}

func TestPlayReportsStartedSound(t *testing.T) {
	sounds := make(chan mixer.Sound, 1)
	sounds <- mixer.Sound{Samples: []float32{sample}, Cue: synth.Done}
	var started []synth.Cue
	_ = daemon.Play(mixer.New(unity, wideCeiling), grid.New(control.DefaultTempoBPM), sounds, make(chan control.Settings), &bytes.Buffer{}, idle, steppingClock(),
		func(s mixer.Sound) { started = append(started, s.Cue) })
	if want := []synth.Cue{synth.Done}; !slices.Equal(started, want) {
		t.Errorf("started = %v, want %v", started, want)
	}
}

const slowSixteenth = 8000

func TestPlayAppliesQueuedTempo(t *testing.T) {
	s := grid.New(control.DefaultTempoBPM)
	settings := make(chan control.Settings, 1)
	settings <- control.Settings{TempoBPM: 90}
	_ = daemon.Play(mixer.New(unity, wideCeiling), s, make(chan mixer.Sound), settings, &bytes.Buffer{}, idle, steppingClock(), func(mixer.Sound) {})
	s.Offer(mixer.Sound{Cue: synth.Done})
	s.Offer(mixer.Sound{Cue: synth.Done})
	var at []int
	for clock := 0; clock < 2*slowSixteenth; clock += daemon.BlockFrames {
		for _, st := range s.Due(daemon.BlockFrames) {
			at = append(at, clock+st.Offset)
		}
	}
	if len(at) != 2 || at[1]-at[0] != slowSixteenth {
		t.Errorf("starts = %v, want %d apart", at, slowSixteenth)
	}
}
