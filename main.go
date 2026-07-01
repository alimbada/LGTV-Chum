package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Configuration
const ControlScript = "{{BIN_DIR}}/lgtv-control.sh"
const TargetHDMIAppId = "com.webos.app.hdmi4"
const PollInterval = 1 * time.Second

var (
	lastState      string
	stateMutex     sync.Mutex
	ignoreDRMUntil time.Time
)

// LGTVAppResponse maps the JSON structure from getForegroundAppInfo
type LGTVAppResponse struct {
	Payload struct {
		AppId string `json:"appId"`
	} `json:"payload"`
}

// getTVPowerState queries the TV and returns "ON", "OFF", or "UNKNOWN"
func getTVPowerState() string {
	cmd := exec.Command(ControlScript, "getPowerState")
	outBytes, _ := cmd.Output() // We ignore the error because network drops/timeouts are expected here

	outStr := string(outBytes)

	// Isolate the JSON payload from the shell script logging
	startIndex := strings.Index(outStr, "{")
	if startIndex == -1 {
		return "OFF" // If we get no JSON, the TV is likely asleep/unreachable
	}

	jsonStr := outStr[startIndex:]

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return "UNKNOWN"
	}

	// If the response contains a "closing" block, the TV is off/standby
	if _, hasClosing := result["closing"]; hasClosing {
		return "OFF"
	}

	// If the response contains "payload" -> "state" == "Active", the TV is on
	if payloadRaw, hasPayload := result["payload"]; hasPayload {
		if payload, ok := payloadRaw.(map[string]interface{}); ok {
			if state, ok := payload["state"].(string); ok {
				if state == "Active" {
					return "ON"
				}
			}
		}
	}

	return "UNKNOWN"
}

// isTargetPortActive queries the TV and checks if the current app matches TargetHDMIAppId
func isTargetPortActive() bool {
	cmd := exec.Command(ControlScript, "getForegroundAppInfo")
	outBytes, err := cmd.Output()
	if err != nil {
		return false
	}

	outStr := string(outBytes)
	startIndex := strings.Index(outStr, "{")
	if startIndex == -1 {
		return false
	}

	jsonStr := outStr[startIndex:]

	var resp LGTVAppResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return false
	}

	return resp.Payload.AppId == TargetHDMIAppId
}

// triggerTV executes the control script using a robust verification and retry loop
func triggerTV(desiredState string) {
	const maxRetries = 4
	const retryDelay = 3 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		currentPower := getTVPowerState()

		// 1. Check if we are already in the desired state
		if currentPower == desiredState {
			fmt.Printf("[%s] TV power state verified as %s.\n", time.Now().Format("15:04:05"), desiredState)
			return
		}

		// 2. If turning OFF, strictly verify the port to avoid turning off someone else's console
		if desiredState == "OFF" {
			if !isTargetPortActive() {
				fmt.Printf("[%s] TV is not on target port (or unreachable). Aborting OFF sequence.\n", time.Now().Format("15:04:05"))
				return // Abort completely, do not retry.
			}
		}

		// 3. Determine action and execute
		action := "off"
		if desiredState == "ON" {
			action = "on"
		}

		fmt.Printf("[%s] Sending '%s' command (Attempt %d/%d)...\n", time.Now().Format("15:04:05"), action, attempt, maxRetries)
		cmd := exec.Command(ControlScript, action)
		cmd.Run()

		// Wait before the next iteration checks if the command succeeded
		time.Sleep(retryDelay)
	}

	// Final verification after the loop finishes
	if getTVPowerState() == desiredState {
		fmt.Printf("[%s] TV power state verified as %s on final check.\n", time.Now().Format("15:04:05"), desiredState)
	} else {
		fmt.Printf("[%s] Warning: Failed to reach %s state after %d attempts.\n", time.Now().Format("15:04:05"), desiredState, maxRetries)
	}
}

// updateState safely handles state transitions triggered by either DRM or D-Bus
func updateState(newState string, source string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// If DRM tries to turn the TV OFF immediately after a D-Bus wake, ignore it temporarily
	if source == "DRM Poller" && newState == "OFF" && time.Now().Before(ignoreDRMUntil) {
		return
	}

	forceUpdate := false

	// If D-Bus wakes the system, we FORCE an update regardless of the previous state.
	// This fixes desyncs if the TV was manually switched off while the PC was asleep.
	if source == "D-Bus System Wake" && newState == "ON" {
		ignoreDRMUntil = time.Now().Add(3 * time.Second)
		forceUpdate = true
	}

	// Trigger the TV if the state has changed, OR if it's a forced D-Bus wake event
	if newState != lastState || forceUpdate {
		fmt.Printf("[%s] %s triggered state change to: %s. Executing sequence...\n", time.Now().Format("15:04:05"), source, newState)
		lastState = newState
		triggerTV(newState)
	}
}

// getDisplayState checks the raw kernel DRM subsystem for monitor power states
func getDisplayState() (string, error) {
	files, err := filepath.Glob("/sys/class/drm/card*-*/status")
	if err != nil {
		return "UNKNOWN", err
	}

	for _, statusFile := range files {
		statusData, err := os.ReadFile(statusFile)
		if err != nil {
			continue
		}

		if strings.TrimSpace(string(statusData)) == "connected" {
			dpmsFile := strings.Replace(statusFile, "status", "dpms", 1)
			dpmsData, err := os.ReadFile(dpmsFile)
			if err != nil {
				continue
			}

			if strings.TrimSpace(string(dpmsData)) == "On" {
				return "ON", nil
			}
		}
	}
	return "OFF", nil
}

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
			updateState("ON", "D-Bus System Wake")
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("dbus-monitor exited: %v", err)
	}
}

func main() {
	log.Println("Starting Hybrid LGTV Daemon (Robust Verification + D-Bus Wake Listener)...")

	// 1. Initialize starting state
	initialState, err := getDisplayState()
	if err != nil {
		log.Fatalf("Critical error reading DRM sysfs: %v", err)
	}
	lastState = initialState
	log.Printf("Initial hardware state detected: %s.", lastState)

	// 2. Start the D-Bus wake listener in the background
	go listenForWake()

	// 3. Start the DRM polling loop in the foreground
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		currentState, err := getDisplayState()
		if err != nil {
			continue
		}
		updateState(currentState, "DRM Poller")
	}
}