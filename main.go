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
const ControlScript = "/home/ammaar/bin/lgtv-control.sh"
const TargetHDMIAppId = "com.webos.app.hdmi4"
const PollInterval = 1 * time.Second

var (
	lastState      string
	stateMutex     sync.Mutex
	ignoreDRMUntil time.Time
)

// LGTVResponse maps the expected JSON structure from getForegroundAppInfo
type LGTVResponse struct {
	Payload struct {
		AppId string `json:"appId"`
	} `json:"payload"`
}

// isTargetPortActive queries the TV and checks if the current app matches TargetHDMIAppId
func isTargetPortActive() bool {
	cmd := exec.Command(ControlScript, "getForegroundAppInfo")
	outBytes, err := cmd.Output()
	if err != nil {
		log.Printf("Warning: Failed to execute TV query: %v", err)
		return false
	}

	outStr := string(outBytes)

	// The script prepends standard text logs before the JSON.
	// We find the first '{' to isolate the JSON payload.
	startIndex := strings.Index(outStr, "{")
	if startIndex == -1 {
		log.Printf("Warning: No JSON payload found in TV response: %s", outStr)
		return false
	}

	jsonStr := outStr[startIndex:]

	var resp LGTVResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		log.Printf("Warning: Failed to parse TV JSON response: %v", err)
		return false
	}

	if resp.Payload.AppId != TargetHDMIAppId {
		log.Printf("Port check failed: TV is currently on '%s', not '%s'.", resp.Payload.AppId, TargetHDMIAppId)
		return false
	}

	return true
}

// triggerTV executes the external control script
func triggerTV(state string) {
	// Only check the HDMI port if we are trying to turn the TV OFF.
	// If the TV is asleep, it cannot report its port, so we must allow ON commands through.
	if state == "OFF" {
		if !isTargetPortActive() {
			fmt.Printf("[%s] TV is not on target port (or unreachable). Ignoring OFF command.\n", time.Now().Format("15:04:05"))
			return
		}
	}

	action := "off"
	if state == "ON" {
		action = "on"
	}

	cmd := exec.Command(ControlScript, action)
	if err := cmd.Run(); err != nil {
		log.Printf("Error executing TV script: %v", err)
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

	// If D-Bus wakes the system, set a 3-second grace period where DRM "OFF" states are ignored
	if source == "D-Bus System Wake" && newState == "ON" {
		ignoreDRMUntil = time.Now().Add(3 * time.Second)
	}

	// Trigger the TV if the state has actually changed
	if newState != lastState {
		fmt.Printf("[%s] %s triggered state change to: %s. Executing script...\n", time.Now().Format("15:04:05"), source, newState)
		triggerTV(newState)
		lastState = newState
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
	log.Println("Starting Hybrid LGTV Daemon (DRM Poller + D-Bus Wake Listener)...")

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
