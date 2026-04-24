# Installation

filehub ships two binaries:

- `filehub` — the CLI used to manage roots, ACLs, and the daemon lifecycle.
- `filehubd` — the long-running HTTP server that stores and serves images.

The one-line installer downloads both, puts them on your `PATH`, and wires
`filehubd` into your platform's user-level service manager so it starts on
login.

## One-line install

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/1996fanrui/filehub/main/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr https://raw.githubusercontent.com/1996fanrui/filehub/main/install.ps1 -UseBasicParsing | iex
```

The PowerShell installer downloads `filehub.exe` and `filehubd.exe` into
`%LOCALAPPDATA%\Programs\filehub\` and appends that directory to your user
`PATH`. It does not register a Windows service — see
[Running filehubd on Windows](#running-filehubd-on-windows) below.

To select a channel or pin a version, download the script and invoke it with
parameters:

```powershell
# latest stable (default)
iwr https://raw.githubusercontent.com/1996fanrui/filehub/main/install.ps1 -UseBasicParsing -OutFile install.ps1
./install.ps1

# latest release, including pre-releases (alpha)
./install.ps1 -Pre

# exact version
./install.ps1 -Version v0.1.0
./install.ps1 -Version v0.1.0-alpha.1
```

If you run the `bash` command from a Windows-like shell (MSYS, MinGW, Cygwin),
the script refuses to proceed and directs you to `install.ps1`. Use PowerShell
instead.

#### Running filehubd on Windows

`install.ps1` intentionally leaves daemon supervision to the user. Two common
options:

1. Run `filehubd.exe` manually from any new PowerShell session (the new session
   picks up the updated `PATH`).
2. Register a Windows Task Scheduler task that runs `filehubd.exe` at logon.
   A minimal recipe:

   ```powershell
   $action  = New-ScheduledTaskAction  -Execute "$env:LOCALAPPDATA\Programs\filehub\filehubd.exe"
   $trigger = New-ScheduledTaskTrigger -AtLogOn
   Register-ScheduledTask -TaskName 'filehubd' -Action $action -Trigger $trigger
   ```

   Adjust the trigger/settings as needed. The installer does not create this
   task for you.

## Channel selection

The installer accepts positional arguments:

```bash
# latest stable (default)
bash install.sh

# latest release, including pre-releases (alpha)
bash install.sh --pre

# exact version
bash install.sh v0.1.0
bash install.sh v0.1.0-alpha.1

# resolve version and print target — no filesystem changes
bash install.sh --dry-run
```

`--dry-run` is useful in CI to verify the installer can reach the GitHub
Releases API and selects the expected version without touching the system.

## What install.sh does

1. Rejects Windows-like shells (`$OSTYPE` matching `msys*`/`mingw*`/`cygwin*`).
2. Resolves the target version via the GitHub Releases API (or uses the tag
   you passed explicitly).
3. If `~/.local/bin/filehub` already reports the target version, skips the
   download. Otherwise, downloads `filehub_<os>_<arch>` and
   `filehubd_<os>_<arch>` into a tempdir and installs them with mode `0755`
   to `~/.local/bin/`.
4. Ensures `~/.local/bin` is on your `PATH` by appending a marker block to
   your shell profile (zsh → `~/.zshrc`; bash on Linux → `~/.bashrc`;
   bash on macOS → `~/.bash_profile`; otherwise `~/.profile`). The block is
   delimited by `# >>> filehub installer >>>` / `# <<< filehub installer <<<`
   and is appended only once.
5. **Linux**: writes `~/.config/systemd/user/filehubd.service`, runs
   `loginctl enable-linger "$USER"` so the daemon survives logout, then
   `systemctl --user daemon-reload` and `systemctl --user enable --now filehubd`.
6. **macOS**: writes `~/Library/LaunchAgents/com.filehub.filehubd.plist` and
   bootstraps it into the `gui/$UID` launchd domain using the modern
   `launchctl bootstrap` / `launchctl bootout` APIs (the same calls
   `filehub service start|stop` uses).

Re-running the installer is equivalent to an upgrade: the version check is
idempotent, the `systemd` unit / launchd plist are regenerated, and the
service is reloaded.

## Post-install validation

```bash
# CLI is on PATH
filehub version

# daemon is running
# Linux:
systemctl --user status filehubd
journalctl --user -u filehubd -f

# macOS:
launchctl list | grep filehubd
tail -f ~/Library/Logs/filehubd.log
```

Open the swagger UI to confirm the daemon is serving:

```
http://localhost:34286/swagger/index.html
```

The daemon will auto-inject a `_default` root pointing at the platform's XDG
data directory if you have not configured any `[[root]]` in your
`config.toml`, so a fresh install is immediately usable.

## Environment variables

| Name                      | Purpose                                                |
|---------------------------|--------------------------------------------------------|
| `FILEHUB_NO_MODIFY_PATH`  | Set to `1` to skip writing the PATH marker block.      |

## Troubleshooting

### "this script does not support Windows-like shells"

You ran `install.sh` from Git Bash, MSYS, MinGW, or Cygwin. Open PowerShell
and run the `install.ps1` one-liner in the [Windows section](#windows-powershell)
instead.

### `loginctl enable-linger` failed

Some distributions require polkit authorization to enable user lingering. The
installer treats this as a warning, not a fatal error — the daemon will still
start for the current session, but will stop when you log out.

Retry manually:

```bash
sudo loginctl enable-linger "$USER"
```

### Port 34286 already in use

The daemon listens on `127.0.0.1:34286` by default. If another process holds
the port, `systemctl --user status filehubd` / `launchctl list` will show the
daemon failing to start. Identify the conflicting process with
`ss -ltnp | grep 34286` (Linux) or `lsof -iTCP:34286 -sTCP:LISTEN` (macOS),
stop it, then restart filehubd:

```bash
systemctl --user restart filehubd           # Linux
launchctl kickstart -k gui/$(id -u)/com.filehub.filehubd   # macOS
```

### macOS log divergence

The launchd plist routes stdout and stderr to
`~/Library/Logs/filehubd.log`. The `filehub service logs` CLI subcommand
currently queries macOS unified logging (`log show --predicate
"subsystem == com.filehub.filehubd"`), but `filehubd` does not yet emit via
`os_log` — so on macOS, read the log file directly until the daemon is
migrated to unified logging. On Linux, `filehub service logs` maps to
`journalctl --user -u filehubd` and works as expected.
