// Clanking status bar control: shows the daemon's volume and mute state, and changes them through the clanking CLI.
// It listens on the daemon's events socket, so it never starts a daemon just to show the status.
const vscode = require('vscode');
const { execFile } = require('child_process');
const fs = require('fs');
const net = require('net');
const os = require('os');
const path = require('path');

const RETRY_MS = 5000;
const SETTINGS_PREFIX = 'settings ';

function stateDir() {
  const base = process.env.XDG_STATE_HOME || path.join(os.homedir(), '.local', 'state');
  return path.join(base, 'clanking');
}

function binary() {
  const configured = vscode.workspace.getConfiguration('clanking').get('path');
  if (configured) return configured;
  const local = path.join(os.homedir(), '.local', 'bin', 'clanking');
  return fs.existsSync(local) ? local : 'clanking';
}

// parseSettings reads "volume -6 dB, unmuted, root C5, 120 bpm" into { volume, muted }.
function parseSettings(text) {
  const m = /volume (\S+) dB, (muted|unmuted)/.exec(text);
  return m ? { volume: Number(m[1]), muted: m[2] === 'muted' } : null;
}

function activate(context) {
  const item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  item.command = 'clanking.menu';
  let current = null;

  function render() {
    if (!current) {
      item.text = '$(bell-slash) clanking';
      item.tooltip = 'Clanking daemon is not running. It starts on the next agent event.';
    } else if (current.muted) {
      item.text = '$(mute) muted';
      item.tooltip = `Clanking muted (volume ${current.volume} dB). Click to change.`;
    } else {
      item.text = `$(unmute) ${current.volume} dB`;
      item.tooltip = 'Clanking volume. Click to change.';
    }
    item.show();
  }

  let socket = null;
  let retry = null;
  function listen() {
    socket = net.createConnection(path.join(stateDir(), 'run', 'clanking.events'));
    let buffer = '';
    socket.setEncoding('utf8');
    socket.on('data', (chunk) => {
      buffer += chunk;
      let newline;
      while ((newline = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, newline);
        buffer = buffer.slice(newline + 1);
        if (line.startsWith(SETTINGS_PREFIX)) {
          current = parseSettings(line.slice(SETTINGS_PREFIX.length)) || current;
          render();
        }
      }
    });
    socket.on('error', () => {});
    socket.on('close', () => {
      current = null;
      render();
      retry = setTimeout(listen, RETRY_MS);
    });
  }

  function clanking(...args) {
    execFile(binary(), args, { timeout: 10000 }, (err, stdout) => {
      if (err) {
        vscode.window.showErrorMessage(`clanking ${args.join(' ')} failed: ${err.message}`);
        return;
      }
      current = parseSettings(stdout) || current;
      render();
    });
  }

  async function setVolume() {
    const value = await vscode.window.showInputBox({
      prompt: 'Clanking volume in dB (24 is the loudest)',
      value: current ? String(current.volume) : '-6',
      validateInput: (v) => (v.trim() !== '' && Number(v) <= 24 ? null : 'Enter a number of dB, at most 24'),
    });
    if (value !== undefined) clanking('volume', value.trim());
  }

  context.subscriptions.push(
    item,
    vscode.commands.registerCommand('clanking.volumeUp', () => clanking('volume', 'up')),
    vscode.commands.registerCommand('clanking.volumeDown', () => clanking('volume', 'down')),
    vscode.commands.registerCommand('clanking.toggleMute', () => clanking('toggle')),
    vscode.commands.registerCommand('clanking.setVolume', setVolume),
    vscode.commands.registerCommand('clanking.menu', async () => {
      const pick = await vscode.window.showQuickPick([
        { label: '$(arrow-up) Volume up', description: '+3 dB', run: () => clanking('volume', 'up') },
        { label: '$(arrow-down) Volume down', description: '−3 dB', run: () => clanking('volume', 'down') },
        { label: current && current.muted ? '$(unmute) Unmute' : '$(mute) Mute', run: () => clanking('toggle') },
        { label: '$(edit) Set volume…', run: setVolume },
      ], { placeHolder: current ? `Clanking at ${current.volume} dB${current.muted ? ', muted' : ''}` : 'Clanking' });
      if (pick) pick.run();
    }),
    { dispose: () => { clearTimeout(retry); if (socket) { socket.removeAllListeners('close'); socket.destroy(); } } },
  );

  render();
  listen();
}

function deactivate() {}

module.exports = { activate, deactivate, parseSettings };
