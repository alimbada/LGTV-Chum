package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	TargetHDMIInputNumber string
	TVName                string
	TargetVendorID        string
	MonitorKVMConnection  bool
}

// loadConfig reads lgtv-chum.conf from the user's config directory (~/.config)
// and returns the parsed config. If any required values are missing, it returns an error.
func loadConfig() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to locate user config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "lgtv-chum.conf")
	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration file not found at %s. Please create it with hdmi_input and tv_name", configPath)
		}
		return nil, fmt.Errorf("failed to open config file at %s: %w", configPath, err)
	}
	defer file.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed config line %d in %s: %q", lineNum, configPath, line)
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		// Remove surrounding quotes if any
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "`") && strings.HasSuffix(val, "`")) {
			if unquoted, err := strconv.Unquote(val); err == nil {
				val = unquoted
			}
		} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
			if len(val) >= 2 {
				val = val[1 : len(val)-1]
			}
		}

		// Normalize key (remove underscores and dashes)
		normalizedKey := strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", "")

		switch normalizedKey {
		case "hdmiinput":
			cfg.TargetHDMIInputNumber = val
		case "tvname":
			cfg.TVName = val
		case "kvmvendorid":
			cfg.TargetVendorID = val
		case "monitorkvmconnection":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return nil, fmt.Errorf("invalid boolean value for monitor_kvm_connection on line %d: %q", lineNum, val)
			}
			cfg.MonitorKVMConnection = b
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Validate required variables
	var missing []string
	if cfg.TargetHDMIInputNumber == "" {
		missing = append(missing, "hdmi_input")
	}
	if cfg.TVName == "" {
		missing = append(missing, "tv_name")
	}
	if cfg.MonitorKVMConnection && cfg.TargetVendorID == "" {
		missing = append(missing, "kvm_vendor_id")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration values: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
