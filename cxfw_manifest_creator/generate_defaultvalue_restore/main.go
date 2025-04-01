package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Manifest structure
type Manifest struct {
	Version    string      `json:"version"`
	Operations []Operation `json:"operations"`
}

// Operation structure
type Operation struct {
	Operation string                 `json:"operation"`
	Entries   map[string]interface{} `json:"entries,omitempty"`
}

func main() {
	// Define CLI arguments
	manifestPath := flag.String("manifest", "", "Path to patch_manifest.json")
	flag.Parse()

	if *manifestPath == "" {
		fmt.Println("Error: Please provide the path to patch_manifest.json using --manifest")
		os.Exit(1)
	}

	restoreManifestPath := "patch_defaultvalue_restore_manifest.json"
	defaultValuesPath := "/sda1/data/.defaultvalues"

	// Read patch_manifest.json
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		fmt.Println("Error reading", *manifestPath, ":", err)
		os.Exit(1)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Println("Error parsing JSON:", err)
		os.Exit(1)
	}

	// Find modify_defaults operation
	modifiedEntries := make(map[string]string)
	foundModifyDefaults := false

	for _, op := range manifest.Operations {
		if op.Operation == "modify_defaults" {
			foundModifyDefaults = true
			modifiedEntries, err = flattenEntries(op.Entries)
			if err != nil {
				fmt.Println("Error flattening entries:", err)
				os.Exit(1)
			}
			break
		}
	}

	if !foundModifyDefaults {
		fmt.Println("No modify_defaults operation found. No restore file created.")
		return
	}

	// Read original values from .defaultvalues
	originalDefaults := readDefaultValues(defaultValuesPath, modifiedEntries)

	// Create restore manifest
	restoreManifest := Manifest{
		Version:    "1.0",
		Operations: []Operation{},
	}

	// Ensure only `modify_defaults` is used for restoring values
	if len(originalDefaults) > 0 {
		restoreManifest.Operations = append(restoreManifest.Operations, Operation{
			Operation: "modify_defaults",
			Entries:   toInterfaceMap(originalDefaults),
		})
	}

	// Save patch_defaultvalue_restore_manifest.json
	restoreData, err := json.MarshalIndent(restoreManifest, "", "  ")
	if err != nil {
		fmt.Println("Error creating restore JSON:", err)
		os.Exit(1)
	}

	if err := os.WriteFile(restoreManifestPath, restoreData, 0600); err != nil {
		fmt.Println("Error writing restore JSON:", err)
		os.Exit(1)
	}

	fmt.Println("Restore manifest created:", restoreManifestPath)
}

// Flattens nested map while preserving section headers like "[Auto Login]"
func flattenEntries(entries map[string]interface{}) (map[string]string, error) {
	result := make(map[string]string)
	for key, value := range entries {
		if nested, ok := value.(map[string]interface{}); ok {
			for subKey, subValue := range nested {
				if sv, ok := subValue.(string); ok {
					result[key+"."+subKey] = sv
				} else {
					return nil, fmt.Errorf("unsupported nested value type for %s.%s: %T", key, subKey, subValue)
				}
			}
		} else if sv, ok := value.(string); ok {
			result[key] = sv
		} else {
			return nil, fmt.Errorf("unsupported value type for %s: %T", key, value)
		}
	}
	return result, nil
}

// Converts string map to interface{} map
func toInterfaceMap(m map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Reads .defaultvalues and restores correct keys
func readDefaultValues(filePath string, modifiedEntries map[string]string) map[string]string {
	originalDefaults := make(map[string]string)

	// Read .defaultvalues
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Warning: Unable to read .defaultvalues file:", err)
		return originalDefaults
	}

	// Track the current section header
	var currentSection string

	// Parse line by line
	for _, line := range splitLines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if line is a section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = line
			continue
		}

		// Parse key=value
		if key, value, found := parseKeyValue(line); found {
			fullKey := key
			if currentSection != "" {
				fullKey = currentSection + "." + key
			}

			// Restore only if it was modified
			if _, exists := modifiedEntries[fullKey]; exists {
				originalDefaults[fullKey] = value
			} else if currentSection == "" { // Handling global entries
				originalDefaults[key] = value
			}
		}
	}

	return originalDefaults
}

// Splits string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// Parses "key=value" format
func parseKeyValue(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	return key, value, key != ""
}
