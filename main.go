package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Configuration
const ControlScript = "/home/ammaar/bin/lgtv-control.sh"
const PollInterval = 1 * time.Second

// triggerTV executes the external control script
func triggerTV(state string) {
	fmt.Printf("[%s] Hardware DPMS state changed to: %s. Triggering TV...\n", time.Now().Format("15:04:05"), state)

	action := "off"
	if state == "ON" {
		action = "on"
	}

	cmd := exec.Command(ControlScript, action)
	if err := cmd.Run(); err != nil {
		log.Printf("Error executing TV script: %v", err)
	}
}

// getDisplayState checks the raw kernel DRM subsystem for monitor power states
func getDisplayState() (string, error) {
	// Find all graphics card connector status files
	files, err := filepath.Glob("/sys/class/drm/card*-*/status")
	if err != nil {
		return "UNKNOWN", err
	}

	for _, statusFile := range files {
		// Read connector status (e.g., connected vs disconnected)
		statusData, err := os.ReadFile(statusFile)
		if err != nil {
			continue
		}

		// We only care about physically connected displays
		if strings.TrimSpace(string(statusData)) == "connected" {
			// The DPMS file sits next to the status file and contains the power state
			dpmsFile := strings.Replace(statusFile, "status", "dpms", 1)
			dpmsData, err := os.ReadFile(dpmsFile)
			if err != nil {
				continue
			}

			// If ANY connected display is "On", we consider the system display ON
			if strings.TrimSpace(string(dpmsData)) == "On" {
				return "ON", nil
			}
		}
	}

	// If no connected displays are "On" (e.g., they are Off, Standby, or Suspend), the display is OFF
	return "OFF", nil
}

func main() {
	log.Println("Starting LGTV Kernel DRM Polling Daemon...")

	lastState, err := getDisplayState()
	if err != nil {
		log.Fatalf("Critical error reading DRM sysfs: %v", err)
	}

	log.Printf("Initial hardware state detected: %s. Monitoring for kernel state changes...", lastState)

	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		currentState, err := getDisplayState()
		if err != nil {
			// Suppress spammy logs if the DRM system is temporarily busy
			continue
		}

		// Check for state transition
		if currentState != lastState {
			triggerTV(currentState)
			lastState = currentState
		}
	}
}
