# sshls

Simple Windows SSH host picker written in Go.

Scans your `~/.ssh/config`, lists the defined `Host` aliases, lets you pick one (by number or partial name), then opens your default terminal (Windows Terminal preferred) and runs `ssh <alias>`.

## Features

- Zero external dependencies — single static `.exe`
- Parses `Host` lines + follows `Include` (supports globs like `Include config.d/*`)
- Skips wildcard hosts (`*`, `*.foo`)
- Interactive selector (number or substring match) or direct CLI use
- Prefers `wt` (Windows Terminal) when available, falls back to `cmd`
- Cross-compile friendly (build on Linux/macOS for Windows)

## Requirements (on target Windows machine)

- Windows 10 / 11
- OpenSSH Client installed (built-in on most Win10/11):
  - Run `ssh -V` in cmd/PowerShell — if it works you're good.
  - If not: Settings → Apps → Optional features → Add a feature → "OpenSSH Client"
- (Recommended) Windows Terminal (`wt`)

## Usage

```cmd
# Interactive picker
sshls

# Direct connect (bypasses picker)
sshls myserver

# List hosts only (useful for scripts / fzf etc.)
sshls --list
```

In the interactive picker:
- Type a **number** and press Enter
- Or type part of the name (case-insensitive substring)
- `q` or empty input to cancel

Example session:

```
sshls - SSH Host Picker
Found 3 hosts in C:\Users\you\.ssh\config

   1) gitlab
   2) myserver
   3) prod-web

Enter number or partial name (q to quit): 2
Launching terminal for: ssh myserver
```

A new terminal window/tab will open and execute `ssh myserver` using all settings from your config (HostName, User, Port, IdentityFile, ProxyJump, etc.).

## Building

### On Windows (simplest)

```powershell
go mod init sshls   # only first time
go build -o sshls.exe -ldflags "-s -w" .
```

### Cross-compile from Linux / macOS

```bash
GOOS=windows GOARCH=amd64 go build -o sshls.exe -ldflags "-s -w" .
```

Place `sshls.exe` somewhere in your PATH (e.g. `C:\tools\` or next to other utilities).

## Example ~/.ssh/config

```ssh-config
Host myserver
    HostName 192.168.1.50
    User admin
    Port 2222
    IdentityFile ~/.ssh/id_ed25519

Host gitlab
    HostName gitlab.example.com
    User git
    IdentityFile ~/.ssh/id_gitlab

Host prod-web
    HostName web01.prod.internal
    User deploy

# Split your config (optional but popular)
Include config.d/*
```

## Notes

- Only concrete host aliases are shown (wildcards like `Host *` or `Host *.lan` are ignored).
- The program does **not** implement SSH itself — it launches the real `ssh.exe` so all your existing keys, agents, and config tricks continue to work.
- After the SSH session ends, the terminal window behavior depends on the shell:
  - Using `wt`: tab usually closes
  - Using classic `cmd /k`: window stays open

## License

MIT (or public domain — do what you want).
