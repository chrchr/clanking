// Command clanking plays synthesized audio cues for Claude Code hook events.
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/chrchr/clanking/control"
	"github.com/chrchr/clanking/daemon"
	"github.com/chrchr/clanking/events"
	"github.com/chrchr/clanking/grid"
	"github.com/chrchr/clanking/hookevent"
	"github.com/chrchr/clanking/mixer"
	"github.com/chrchr/clanking/pan"
	"github.com/chrchr/clanking/playback"
	"github.com/chrchr/clanking/selftest"
	"github.com/chrchr/clanking/statedir"
	"github.com/chrchr/clanking/synth"
	"github.com/chrchr/clanking/transport"
)

const (
	lockName         = "clanking.lock"
	settingsName     = "mixer.json"
	noSpawnFlag      = "--no-spawn"
	helpFlag         = "--help"
	idleEnv          = "CLANKING_IDLE_TIMEOUT"
	limiterCeilingDB = -24.0
	soundQueue       = 64
	dirMode          = 0o755
	statusJoin       = " "
)

var (
	errUsage         = errors.New("usage: clanking hook [--no-spawn] | daemon | play <cue> | mute | unmute | toggle | volume <dB>|up|down | tempo <bpm> | root <midi> | status | selftest")
	errSpawnDisabled = errors.New("no daemon is running and spawning is disabled")
	errUnknownCue    = errors.New("unknown cue")
)

func main() {
	logFile, err := statedir.OpenLog()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, nil)))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("clanking failed", "args", os.Args[1:], "err", err)
		fmt.Fprintln(os.Stderr, "clanking:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	switch {
	case slices.Equal(args, []string{"hook"}):
		return runHook(spawnDaemon)
	case slices.Equal(args, []string{"hook", noSpawnFlag}):
		return runHook(refuseSpawn)
	case slices.Equal(args, []string{"daemon"}):
		return runDaemon()
	case slices.Equal(args, []string{"play", helpFlag}):
		_, err := fmt.Println(playUsage())
		return err
	case len(args) == 2 && args[0] == "play":
		return runPlay(synth.Cue(args[1]))
	case len(args) == 1 && slices.Contains([]string{control.Mute, control.Unmute, control.Toggle, control.Status}, args[0]):
		return runControl(args[0])
	case len(args) == 2 && slices.Contains([]string{control.Volume, control.Tempo, control.Root}, args[0]):
		return runControl(strings.Join(args, " "))
	case slices.Equal(args, []string{"selftest"}):
		return selftest.RunDaemon(spawnDaemon)
	}
	return errUsage
}

func playUsage() string {
	return "usage: clanking play <cue>\ncues: " + cueList(synth.Cues) + "\nui: " + cueList(synth.UICues) + "\ntheme: " + cueList(synth.ThemeCues) + "\naliases: " + cueList(synth.Aliases())
}

func cueList(cues []synth.Cue) string {
	names := make([]string, len(cues))
	for i, c := range cues {
		names[i] = string(c)
	}
	return strings.Join(names, " ")
}

func runHook(spawn func() error) error {
	ev, ok, err := hookevent.Parse(os.Stdin)
	if err != nil || !ok {
		return err
	}
	return sendCue(ev.Cue, ev.Cwd, spawn)
}

func sendCue(c synth.Cue, cwd string, spawn func() error) error {
	socket, err := statedir.RunPath(statedir.CueSocket)
	if err != nil {
		return err
	}
	return transport.Deliver(func() error { return transport.Send(socket, c, cwd) }, spawn, transport.SpawnTimeout, transport.RetryInterval)
}

func runControl(line string) error {
	r, err := control.Parse(line)
	if err != nil {
		return err
	}
	path, err := statedir.RunPath(statedir.ControlSocket)
	if err != nil {
		return err
	}
	settings, gridState, err := transport.Control(path, r, spawnDaemon)
	if err != nil {
		return err
	}
	_, err = fmt.Println(statusLine(settings, gridState))
	return err
}

func statusLine(s control.Settings, gridState string) string {
	return s.String() + statusJoin + gridState
}

func refuseSpawn() error { return errSpawnDisabled }

func spawnDaemon() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	return daemon.Spawn(self)
}

func runDaemon() error {
	lockPath, err := statedir.RunPath(lockName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), dirMode); err != nil {
		return err
	}
	lock, held, err := daemon.Lock(lockPath)
	if err != nil || !held {
		return err
	}
	defer func() { _ = lock.Close() }()
	idle, err := parseIdle(os.Getenv(idleEnv))
	if err != nil {
		return err
	}
	settingsPath, err := statedir.Path(settingsName)
	if err != nil {
		return err
	}
	settings, err := control.Load(settingsPath)
	if err != nil {
		return err
	}
	socket, err := statedir.RunPath(statedir.CueSocket)
	if err != nil {
		return err
	}
	controlSocket, err := statedir.RunPath(statedir.ControlSocket)
	if err != nil {
		return err
	}
	eventsSocket, err := statedir.RunPath(statedir.EventsSocket)
	if err != nil {
		return err
	}
	conn, err := transport.Listen(socket)
	if err != nil {
		return err
	}
	listener, err := transport.ListenStream(controlSocket)
	if err != nil {
		return errors.Join(err, conn.Close(), os.Remove(socket))
	}
	eventsListener, err := transport.ListenStream(eventsSocket)
	if err != nil {
		return errors.Join(err, conn.Close(), os.Remove(socket), listener.Close())
	}
	stream, err := playback.Start(playback.Command())
	if err != nil {
		return errors.Join(err, conn.Close(), os.Remove(socket), listener.Close(), eventsListener.Close())
	}
	sched := grid.New(settings.TempoBPM)
	sounds := make(chan mixer.Sound, soundQueue)
	changes := make(chan control.Settings, soundQueue)
	hub := events.NewHub()
	controller := control.NewController(settings,
		func(s control.Settings) error { return control.Save(settingsPath, s) },
		func(s control.Settings) { changes <- s; hub.Publish(events.SettingsLine(s)) })
	current := func() control.Settings {
		s, _ := controller.Handle(control.Request{Command: control.Status})
		return s
	}
	go func() {
		if err := transport.Serve(conn, enqueue(sounds, newRNG(), func() int { return current().RootNote })); err != nil {
			slog.Error("cue socket failed", "err", err)
		}
	}()
	go func() {
		if err := transport.ServeControl(listener, controller.Handle, func() string { return sched.State().String() }); err != nil {
			slog.Error("control socket failed", "err", err)
		}
	}()
	go func() {
		snapshot := func() string { return events.SettingsLine(current()) }
		if err := transport.ServeEvents(eventsListener, hub.Subscribe, snapshot); err != nil {
			slog.Error("events socket failed", "err", err)
		}
	}()
	slog.Info("daemon started", "socket", socket, "settings", settings.String(), "idle", idle)
	playErr := daemon.Play(mixer.New(settings.Gain(), synth.Amplitude(limiterCeilingDB)), sched, sounds, changes, stream, idle, time.Now, announce(hub.Publish))
	slog.Info("daemon stopping")
	return errors.Join(playErr, conn.Close(), os.Remove(socket), listener.Close(), eventsListener.Close(), stream.Close())
}

func enqueue(sounds chan<- mixer.Sound, rng *rand.Rand, root func() int) func(synth.Cue, string) {
	return func(name synth.Cue, cwd string) {
		c := synth.Canonical(name)
		samples, ok := synth.Sound(c, root(), rng)
		if !ok {
			slog.Warn("unknown cue", "cue", c)
			return
		}
		select {
		case sounds <- mixer.Sound{Samples: samples, Cue: c, Pan: pan.Of(cwd)}:
		default:
			slog.Warn("sound queue full, cue dropped", "cue", c)
		}
	}
}

func announce(publish func(string)) func(mixer.Sound) {
	return func(s mixer.Sound) { publish(events.CueLine(s.Cue)) }
}

func runPlay(c synth.Cue) error {
	if !synth.Known(c) {
		return fmt.Errorf("%w %q", errUnknownCue, c)
	}
	return sendCue(c, "", spawnDaemon)
}

func parseIdle(timeout string) (time.Duration, error) {
	if timeout == "" {
		return daemon.NeverIdle, nil
	}
	return time.ParseDuration(timeout)
}

func newRNG() *rand.Rand { return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) }
