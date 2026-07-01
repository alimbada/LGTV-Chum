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

0. Install [klattimer/LGWebOSRemote](https://github.com/klattimer/LGWebOSRemote) and create a Python virtual environment for it at `$HOME/lgtv-venv`
1. Copy `lgtv-control.sh` to somewhere in your path, e.g. `$HOME/bin`
1. In `main.go` change the path in the `ControlScript` const to point to where you put script in step 1.
1. Compile the Go file and copy/move it to the same location as the script:
   ```shell
    go build -o lgtv-chum main.go && mv lgtv-chum ~/bin
   ```

#### TV on/off at boot/shutdown

1. In `lgtv-boot.service` change the `User` and paths in the values for `ExecStart` and `ExecStop` to point to `lgtv-control.sh` script.
1. Copy `lgtv-boot.service` to `/etc/systemd/system`
1. Enable it:
   ```shell
   sudo systemctl enable lgtv-boot.service
   ```

#### TV on/off at display on/off and at system wake (after sleep)

1. In `lgtv-chum.service` change the paths in the values for `Environment` and `ExecStart` to point to your user directory.
1. Copy `lgtv-chum.service` to `$HOME/.config/systemd/user/`.
1. Enable it:
   ```shell
   systemctl --user enable --now lgtv-chum.service
   ```

## To-do

[ ] Streamline configuration and installation
[ ] Figure out TV off behaviour at system sleep.
