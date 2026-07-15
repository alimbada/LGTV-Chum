package main

import (
	"log"
	"sync"
	"time"

	"alimbada/LGTV-Chum/internal/tv"
)

// Configuration
const PollInterval = 1 * time.Second //TODO: config?

var (
	lastState      tv.PowerState
	stateMutex     sync.Mutex
	ignoreDRMUntil time.Time
)

type TriggerSource string

const (
	UdevListener     TriggerSource = "UDEV_LISTENER"
	DbusWakeListener TriggerSource = "DBUS_WAKE_LISTENER"
	DrmPoller        TriggerSource = "DRM_POLLER"
)

// updateState safely handles state transitions triggered by either DRM or D-Bus
func updateState(newState tv.PowerState, source TriggerSource) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	// If DRM tries to turn the TV OFF immediately after a D-Bus wake, ignore it temporarily
	if source == DrmPoller && newState == tv.PowerOff && time.Now().Before(ignoreDRMUntil) {
		return
	}

	forceUpdate := false

	// If D-Bus wakes the system, we FORCE an update regardless of the previous state.
	// This fixes desyncs if the TV was manually switched off while the PC was asleep.
	if (source == DbusWakeListener || source == UdevListener) && newState == tv.PowerOn {
		ignoreDRMUntil = time.Now().Add(3 * time.Second)
		forceUpdate = true
	}

	// Trigger the TV if the state has changed, OR if it's a forced D-Bus wake event
	if newState != lastState || forceUpdate {
		log.Printf("%s triggered state change to: %s. Executing sequence...\n", source, newState)
		lastState = newState
		tv.TriggerTV(newState)
	}
}

func main() {
	log.Println("Starting Hybrid LGTV Daemon (Robust Verification + D-Bus Wake Listener)...")

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Initialize package variables from config
	tv.Initialize(cfg.TargetHDMIInputNumber, cfg.TVName)
	targetVendorID = cfg.TargetVendorID

	// Initialize starting state
	initialState, err := getDisplayState()
	if err != nil {
		log.Fatalf("Critical error reading DRM sysfs: %v", err)
	}
	lastState = initialState
	log.Printf("Initial hardware state detected: %s.", lastState)

	// Start the D-Bus wake listener in the background
	go listenForWake()

	// Start the udev device-connect listener in the background if enabled
	if cfg.MonitorKVMConnection {
		go listenForDeviceConnect()
	}

	// Start the DRM polling loop in the foreground
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for range ticker.C {
		currentState, err := getDisplayState()
		if err != nil {
			continue
		}
		updateState(currentState, DrmPoller)
	}
}
