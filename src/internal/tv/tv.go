package tv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const TargetHDMIAppId = "com.webos.app.hdmi4"

type PowerState string

const (
	PowerOn      PowerState = "ON"
	PowerOff     PowerState = "OFF"
	PowerUnknown PowerState = "UNKNOWN"
)

type LGTVAppResponse struct {
	Payload struct {
		AppId string `json:"appId"`
	} `json:"payload"`
}

// waitForNetwork waits for the default gateway to be reachable
func waitForNetwork() bool {
	maxAttempts := 15
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Get default gateway
		cmd := exec.Command("ip", "route")
		out, err := cmd.Output()
		if err == nil {
			var gateway string
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "default") {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						gateway = fields[2]
						break
					}
				}
			}

			if gateway != "" {
				// Ping the gateway
				pingCmd := exec.Command("ping", "-c", "1", "-W", "1", gateway)
				if err := pingCmd.Run(); err == nil {
					log.Printf("Network gateway found (%s). Proceeding.\n", gateway)
					return true
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	log.Printf("Error: Network timed out after %d seconds.\n", maxAttempts)
	return false
}

// execLGTV runs the python lgtv command from the virtual environment
func execLGTV(args ...string) ([]byte, error) {
	if !waitForNetwork() {
		return nil, fmt.Errorf("aborting LGTV command due to missing network context")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home dir: %w", err)
	}
	lgtvPath := filepath.Join(homeDir, "lgtv-venv", "bin", "lgtv")

	cmdArgs := append([]string{"--name", "MyTV", "--ssl"}, args...)
	cmd := exec.Command(lgtvPath, cmdArgs...)

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("lgtv command failed: %v, stderr: %s", err, errBuf.String())
	}

	// Read first line
	lines := strings.Split(outBuf.String(), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, fmt.Errorf("empty response from lgtv")
	}
	response := lines[0]

	// Parse JSON to check for error
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err == nil {
		if typ, ok := result["type"].(string); ok && typ == "error" {
			if errMsg, ok := result["error"].(string); ok {
				return nil, fmt.Errorf("lgtv error: %s", errMsg)
			}
			return nil, fmt.Errorf("lgtv returned an error response")
		}
	}

	return []byte(response), nil
}

// GetPowerState queries the TV and returns the power state
func GetPowerState() PowerState {
	outBytes, err := execLGTV("getPowerState")
	if err != nil {
		return PowerOff // If we get an error, the TV is likely asleep/unreachable
	}

	jsonStr := string(outBytes)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return PowerUnknown
	}

	// If the response contains a "closing" block, the TV is off/standby
	if _, hasClosing := result["closing"]; hasClosing {
		return PowerOff
	}

	// If the response contains "payload" -> "state" == "Active", the TV is on
	if payloadRaw, hasPayload := result["payload"]; hasPayload {
		if payload, ok := payloadRaw.(map[string]interface{}); ok {
			if state, ok := payload["state"].(string); ok {
				if state == "Active" {
					return PowerOn
				}
			}
		}
	}

	return PowerUnknown
}

// IsTargetPortActive queries the TV and checks if the current app matches TargetHDMIAppId
func IsTargetPortActive() bool {
	outBytes, err := execLGTV("getForegroundAppInfo")
	if err != nil {
		return false
	}

	jsonStr := string(outBytes)

	var resp LGTVAppResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return false
	}

	return resp.Payload.AppId == TargetHDMIAppId
}

// TriggerTV executes the control script using a verification and retry loop
func TriggerTV(desiredState PowerState) {
	const maxRetries = 4
	const retryDelay = 3 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		currentPower := GetPowerState()

		// Check if we are already in the desired state
		if currentPower == desiredState {
			log.Printf("TV power state verified as %s.\n", desiredState)
			return
		}

		// If turning OFF, strictly verify the port to avoid turning off someone else's console
		if desiredState == PowerOff {
			if !IsTargetPortActive() {
				log.Printf("TV is not on target port (or unreachable). Aborting OFF sequence.\n")
				return // Abort completely, do not retry.
			}
		}

		// Determine action and execute
		action := "off"
		if desiredState == PowerOn {
			action = "on"
		}

		log.Printf("Sending '%s' command (Attempt %d/%d)...\n", action, attempt, maxRetries)
		execLGTV(action)

		// Wait before the next iteration checks if the command succeeded
		time.Sleep(retryDelay)
	}

	// Final verification after the loop finishes
	if GetPowerState() == desiredState {
		log.Printf("TV power state verified as %s on final check.\n", desiredState)
	} else {
		log.Printf("Warning: Failed to reach %s state after %d attempts.\n", desiredState, maxRetries)
	}
}
