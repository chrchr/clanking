# Clanking for VS Code

A status bar item that shows the Clanking daemon's volume and mute state.
Click it to step the volume up or down by 3 dB, mute or unmute, or set an
exact level. It uses the same `clanking` CLI commands as the terminal
(`clanking volume up`, `clanking toggle`, `clanking volume -6`).

The item listens on the daemon's events socket, so it updates when the
volume changes from anywhere (CLI, GTK UI, or VS Code) and never starts a
daemon on its own.

## Install

```sh
cd editors/vscode
npx @vscode/vsce package
code --install-extension clanking-0.1.0.vsix
```

If `clanking` isn't on VS Code's `PATH` and isn't in `~/.local/bin`, set
`clanking.path` in your settings.
