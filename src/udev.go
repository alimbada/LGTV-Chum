package main

import (
	"bufio"
	"log"
	"os/exec"
	"strings"
	"time"

	"alimbada/LGTV-Chum/internal/tv"
)

// targetVendorID is the USB vendor ID whose connection triggers an HDMI input switch
const targetVendorID = "045b" //TODO: config

var lastProcessedTime time.Time // tracks last time a matching device was processed

// listenForDeviceConnect monitors udev events and switches the TV to HDMI 4
// whenever a USB device with the configured vendor ID is connected.
func listenForDeviceConnect() {
	cmd := exec.Command("udevadm", "monitor", "--environment", "--udev")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Failed to attach to udevadm stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start udevadm monitor: %v", err)
	}

	// Each udev event is a block of KEY=VALUE lines separated by blank lines.
	// We accumulate lines per event then inspect them as a unit.
	var eventLines []string

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			// End of an event block — evaluate what we collected
			processUdevEvent(eventLines)
			eventLines = eventLines[:0]
			continue
		}

		// Skip the human-readable header lines emitted by udevadm
		// e.g. "UDEV  [12345.678] add      /devices/... (usb)"
		if !strings.Contains(line, "=") {
			continue
		}

		eventLines = append(eventLines, line)
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("udevadm monitor exited: %v", err)
	}
}

// processUdevEvent inspects the key=value pairs of a single udev event and
// switches the TV to HDMI 4 if a device with the target vendor ID is added.
func processUdevEvent(lines []string) {
	const debounceDuration = 3 * time.Second //TODO: config?

	props := make(map[string]string, len(lines))
	for _, line := range lines {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		props[key] = value
	}

	// We only care about "add" actions (device connected)
	if props["ACTION"] != "add" {
		return
	}

	// Match on the USB vendor ID property
	if !strings.EqualFold(props["ID_VENDOR_ID"], targetVendorID) {
		return
	}

	// Debounce: only process if enough time has passed since last matching event
	now := time.Now()
	if !lastProcessedTime.IsZero() && now.Sub(lastProcessedTime) < debounceDuration {
		log.Printf("Skipping duplicate device event (vendor ID %s) - within %ds window\n", targetVendorID, int(debounceDuration.Seconds()))
		return
	}

	lastProcessedTime = now

	if tv.GetPowerState() == tv.PowerOff {
		updateState(tv.PowerOn, UdevListener)
	}

	log.Printf("Device with vendor ID %s connected. Switching TV to HDMI 4...\n", targetVendorID)
	if !tv.IsTargetPortActive() {
		tv.SetInput()
	}
}
