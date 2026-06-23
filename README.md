# lssh

List SSH `Host` names from the current user's SSH config.

`lssh` supports Linux and Windows. It reads:

- Linux: `~/.ssh/config`
- Windows: `%USERPROFILE%\.ssh\config`

## Features

- Lists concrete `Host` names, one per line
- Skips wildcard hosts such as `Host *` and `Host *.example.com`
- Follows `Include` entries, including globs like `Include config.d/*`
- Has no external dependencies

## Usage

```bash
lssh
```

Example output:

```text
gitlab
myserver
prod-web
```

If the SSH config file does not exist, `lssh` exits successfully without output.

## Building

Linux:

```bash
go build -o lssh .
```

Windows:

```powershell
go build -o lssh.exe .
```

Cross-compile for Windows from Linux:

```bash
GOOS=windows GOARCH=amd64 go build -o lssh.exe .
```

## Example SSH Config

```ssh-config
Host myserver
    HostName 192.168.1.50
    User admin

Host gitlab git
    HostName gitlab.example.com
    User git

Host *
    User root

Include config.d/*
```

The example above prints:

```text
git
gitlab
myserver
```

## License

MIT
