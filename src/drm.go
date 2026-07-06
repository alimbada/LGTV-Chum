package main

import (
	"os"
	"path/filepath"
	"strings"

	"alimbada/LGTV-Chum/internal/tv"
)

// getDisplayState checks the raw kernel DRM subsystem for monitor power states
func getDisplayState() (tv.PowerState, error) {
	files, err := filepath.Glob("/sys/class/drm/card*-*/status")
	if err != nil {
		return tv.PowerUnknown, err
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
				return tv.PowerOn, nil
			}
		}
	}
	return tv.PowerOff, nil
}
