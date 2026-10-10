# What's that *Clanking* sound?

For machines called "clankers", LLM agents sure don't make much noise.
Are they *too* quiet? Yes! But not anymore.

Clanking turns your LLM agents' activity into a subtle soundscape. It
signals when an agent is working, waiting for input, or in another state.
Clanking plays cues from many agents at once and harmonizes them into a
happy chorus of chattering clankers.

## Demo

https://github.com/user-attachments/assets/437b30b0-3852-4670-b634-daad4949b651

## Design

Coding agent hooks invoke the Clanking CLI, which reads each hook
event and plays a sound cue. For tool calls, it plays a short tick on
one of several notes to signal that the agent is active. Other sounds
indicate that the agent is outputting a response, waiting for input or
permission, has finished, or has encountered an error.

Clanking runs as a daemon that starts on its first invocation, and the
CLI sends messages to the background task on subsequent invocations.
Sounds are quantized to sixteenth notes at the configured tempo, which
defaults to 120 BPM. During high activity, Clanking switches to
thirty-second notes. The program synthesizes the sounds.

Clanking uses stereo position to help you distinguish agent sessions.
It positions each session according to the agent's working directory,
so clanks from different sessions seem to come from different places.

You can configure the tempo and root note from the command line or
Clanking's GTK 4 UI.

## Installation

Clanking requires a Unix-like system with PipeWire and the `pw-cat`
utility. It has an optional GTK 4 user interface for configuration.
Clanking is vibe-coded in Go and requires a Go toolchain and GTK 4
development libraries to build.

On a Unix-like system with the required dependencies and an XDG-style
desktop (such as GNOME or KDE), `make install` installs `clanking`,
`clanking-ui`, a desktop file, and an icon in your home directory.

The included `Containerfile` lets you build Clanking without installing
all the development dependencies on your system. The `podman-make`
script builds and runs the container. Adapting it for Docker is left as
an exercise for the reader or the reader's coding agent.

### macOS

On macOS, Clanking plays sound through [SoX](https://sourceforge.net/projects/sox/)
instead of PipeWire. The GTK 4 user interface is not supported, so configure
Clanking from the command line (`clanking tempo 100`, `clanking volume -6`,
`clanking mute`).

```sh
brew install go sox
CGO_ENABLED=0 go build -o ~/.local/bin/clanking ./cmd/clanking
```

Make sure `~/.local/bin` is in your `$PATH`, then run `clanking selftest`.

## Configure your Coding Agents

With `clanking` and `pw-cat` (or `play` from SoX on macOS) in your `$PATH`,
you can configure your coding agent to invoke Clanking.

For Claude, install the Clanking plugin from this repository's marketplace:

```text
/plugin install clanking --marketplace chrchr/clanking
```

For Codex, add this repository as a plugin marketplace:

```sh
codex plugin marketplace add chrchr/clanking
codex plugin add clanking@clanking
```

Restart Codex, then run `/hooks`. Trust the Clanking hooks.

## Inspo

* [Peep (The Network Auralizer): Monitoring Your Network With Sound][peep]
  by Michael Gilfix and Prof. Alva Couch, Tufts University.

* [Star Trek: The Next Generation](https://youtu.be/Drr6_zikuZQ?si=BIBxn--yIwatbIYS).
  The voice-interactive computer in the "Star Trek" franchise uses
  nonverbal sounds to signal that it is doing something.

* [Earcons (Wikipedia)](https://en.wikipedia.org/wiki/Earcon). "An earcon is a
  brief, distinctive sound that represents a specific event or conveys other
  information."

## Copyright

Copyright © 2026 by Robert Church

[peep]: https://www.usenix.org/legacy/publications/library/proceedings/lisa2000/full_papers/gilfix/gilfix_html/index.html
