# LGTV Chum for Linux

> ⚠️ **DISCLAIMER:** This is 99% vibe-coded by Google Gemini. Use at your own risk.

## What and why?

This repo consists of a pair of systemd services, a Go program and a wrapper script for an existing project (see [klattimer/LGWebOSRemote](https://github.com/klattimer/LGWebOSRemote)) which work together to provide functionality for automatically managing the power state of a network connected LG TV running webOS connected to a computer running on Linux.

I've enjoyed using the functionality provided by [LGTV Companion](https://github.com/JPersson77/LGTVCompanion) on Windows and [lgtv-control-macos](https://github.com/cmer/lg-tv-control-macos) on macOS. If anyone is looking for alternatives for those platforms, I highly recommend them. I'm not aware of any Linux alternatives as I started hacking my own solution together (and had some fun learning about some Linux internals on the way...) before I could get around to looking for one.

## Usage

### Caveats

- This has only been tested on a Fedora 44 machine running KDE 6.7
- I haven't yet been able to implement a solution to turn the TV off when the computer goes to sleep. Once sleep is triggered, the network connection drops too quickly to be able to send a command to the TV. There used to be a signal `aboutToTurnOff` in the [KWin scripting API](https://develop.kde.org/docs/plasma/kwin/api/) but it [has been removed as of KDE 6.6](https://discuss.kde.org/t/missing-signals-in-kwin-output-on-version-6-6-1/44782/2?u=alimbada)

### Installation

#### Pre-requisites

Install [klattimer/LGWebOSRemote](https://github.com/klattimer/LGWebOSRemote) and create a Python virtual environment for it at `$HOME/lgtv-venv`.

#### Install

Run the provided installation script. You can optionally specify a custom installation directory (defaults to `$HOME/bin`):

```shell
./install.sh [target_directory]
```

This will automatically:
- Replace placeholders in `main.go` and systemd services with the actual paths and user.
- Build the `lgtv-chum` binary.
- Copy the scripts and binary to your installation directory.
- Configure and enable the system-level boot/shutdown service (requires `sudo` privileges).
- Configure, start, and enable the user-level service.

#### Uninstall

To remove the installed binary, scripts, and disable/delete the systemd services:

```shell
./uninstall.sh [target_directory]
```

## To-do

- [x] Streamline configuration and installation
- [ ] Figure out TV off behaviour at system sleep.
