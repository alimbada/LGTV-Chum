package main

import (
	"bufio"
	"log"
	"os/exec"
	"strings"

	"alimbada/LGTV-Chum/internal/tv"
)

// listenForWake runs as a background Goroutine listening for the system resume event
func listenForWake() {
	cmd := exec.Command("dbus-monitor", "--system", "type='signal',interface='org.freedesktop.login1.Manager',member='PrepareForSleep'")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to attach to dbus-monitor stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start dbus-monitor: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 'boolean false' indicates the system has just woken from sleep
		if strings.Contains(line, "boolean false") {
			updateState(tv.PowerOn, "D-Bus System Wake")
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("dbus-monitor exited: %v", err)
	}
}
