package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Manifest struct {
	Version    string      `json:"version"`
	Operations []Operation `json:"operations"`
}

type Operation struct {
	Type    string                       `json:"operation"`
	Entries map[string]map[string]string `json:"entries,omitempty"`
}

type OutputEntry struct {
	CurrentValue string `json:"current_value"`
	NewValue     string `json:"new_value"`
	Exists       bool   `json:"exists"`
}

type Output map[string]map[string]OutputEntry

// parseDefaultValues parses the .defaultvalues file into a map of sections and key-value pairs
func parseDefaultValues(filePath string) (map[string]map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sections := make(map[string]map[string]string)
	currentSection := "" // Default/unscoped section for KEY = VALUE entries

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fmt.Printf("Debug: Processing line: %q\n", line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			currentSection = "" // Reset to unscoped after blank line or comment
			fmt.Printf("Debug: Resetting to unscoped section\n")
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			fmt.Printf("Debug: Switching to section: %q\n", currentSection)
			if _, exists := sections[currentSection]; !exists {
				sections[currentSection] = make(map[string]string)
			}
			continue
		}

		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			fmt.Printf("Debug: Found key-value: %s = %s in section %q\n", key, value, currentSection)
			if _, exists := sections[currentSection]; !exists {
				sections[currentSection] = make(map[string]string)
			}
			sections[currentSection][key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return sections, nil
}

// updateDefaultValues updates the .defaultvalues file based on defaultvalues_comparison.json
// func updateDefaultValues(defaultValuesPath string, comparisonJSONPath string) error {
// 	// Read the comparison JSON
// 	outputData, err := os.ReadFile(comparisonJSONPath)
// 	if err != nil {
// 		return fmt.Errorf("error reading comparison JSON file: %v", err)
// 	}

// 	var output Output
// 	if err := json.Unmarshal(outputData, &output); err != nil {
// 		return fmt.Errorf("error parsing comparison JSON: %v", err)
// 	}

// 	// Read the current .defaultvalues content to preserve order and comments
// 	file, err := os.Open(defaultValuesPath)
// 	if err != nil {
// 		return fmt.Errorf("error opening .defaultvalues file: %v", err)
// 	}
// 	defer file.Close()

// 	lines := []string{}
// 	scanner := bufio.NewScanner(file)
// 	currentSection := ""
// 	sectionKeys := make(map[string]map[string]bool) // Track updated keys per section

// 	for scanner.Scan() {
// 		line := scanner.Text() // Preserve original formatting
// 		trimmedLine := strings.TrimSpace(line)

// 		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, ";") {
// 			lines = append(lines, line)
// 			currentSection = ""
// 			continue
// 		}

// 		if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
// 			currentSection = strings.TrimSpace(trimmedLine[1 : len(trimmedLine)-1])
// 			lines = append(lines, line)
// 			continue
// 		}

// 		if strings.Contains(trimmedLine, "=") {
// 			parts := strings.SplitN(trimmedLine, "=", 2)
// 			key := strings.TrimSpace(parts[0])
// 			section := currentSection
// 			if section == "" {
// 				section = "unscoped"
// 			}

// 			if sectionData, exists := output[section]; exists {
// 				if entry, keyExists := sectionData[key]; keyExists {
// 					// Update existing entry with new value
// 					lines = append(lines, fmt.Sprintf("%s = %s", key, entry.NewValue))
// 					if _, ok := sectionKeys[section]; !ok {
// 						sectionKeys[section] = make(map[string]bool)
// 					}
// 					sectionKeys[section][key] = true
// 					continue
// 				}
// 			}
// 			// Keep unchanged lines
// 			lines = append(lines, line)
// 		}
// 	}

// 	if err := scanner.Err(); err != nil {
// 		return fmt.Errorf("error reading .defaultvalues: %v", err)
// 	}

// 	// Add new unscoped entries first
// 	if unscopedData, exists := output["unscoped"]; exists {
// 		for key, entry := range unscopedData {
// 			if !entry.Exists {
// 				if _, ok := sectionKeys[""]; !ok {
// 					sectionKeys[""] = make(map[string]bool)
// 					lines = append(lines, "") // Add newline before unscoped entries
// 				}
// 				lines = append(lines, fmt.Sprintf("%s = %s", key, entry.NewValue))
// 				sectionKeys[""][key] = true
// 			}
// 		}
// 	}

// 	// Add new INI sections
// 	for section, keys := range output {
// 		if section == "unscoped" {
// 			continue // Already handled above
// 		}
// 		iniSection := section
// 		for key, entry := range keys {
// 			if !entry.Exists {
// 				if _, exists := sectionKeys[iniSection]; !exists {
// 					lines = append(lines, "", fmt.Sprintf("[%s]", iniSection))
// 					sectionKeys[iniSection] = make(map[string]bool)
// 				}
// 				lines = append(lines, fmt.Sprintf("%s = %s", key, entry.NewValue))
// 				sectionKeys[iniSection][key] = true
// 			}
// 		}
// 	}

//		// Write updated content back to .defaultvalues
//		return os.WriteFile(defaultValuesPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
//	}
func updateDefaultValues(defaultValuesPath string, comparisonJSONPath string) error {
	// Read the comparison JSON
	outputData, err := os.ReadFile(comparisonJSONPath)
	if err != nil {
		return fmt.Errorf("error reading comparison JSON file: %v", err)
	}

	var output Output
	if err := json.Unmarshal(outputData, &output); err != nil {
		return fmt.Errorf("error parsing comparison JSON: %v", err)
	}

	// Read the current .defaultvalues content
	file, err := os.Open(defaultValuesPath)
	if err != nil {
		return fmt.Errorf("error opening .defaultvalues file: %v", err)
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	currentSection := ""
	sectionKeys := make(map[string]map[string]bool)  // Track processed keys
	keysToRemove := make(map[string]map[string]bool) // Track keys to remove

	// Populate keysToRemove where exists: false and current_value: ""
	for section, keys := range output {
		iniSection := section
		if section == "unscoped" {
			iniSection = ""
		}
		for key, entry := range keys {
			if !entry.Exists && entry.CurrentValue == "" {
				if _, ok := keysToRemove[iniSection]; !ok {
					keysToRemove[iniSection] = make(map[string]bool)
				}
				keysToRemove[iniSection][key] = true
			}
		}
	}

	// Process existing .defaultvalues
	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") || strings.HasPrefix(trimmedLine, ";") {
			lines = append(lines, line)
			currentSection = ""
			continue
		}

		if strings.HasPrefix(trimmedLine, "[") && strings.HasSuffix(trimmedLine, "]") {
			currentSection = strings.TrimSpace(trimmedLine[1 : len(trimmedLine)-1])
			lines = append(lines, line)
			continue
		}

		if strings.Contains(trimmedLine, "=") {
			parts := strings.SplitN(trimmedLine, "=", 2)
			key := strings.TrimSpace(parts[0])
			section := currentSection
			if section == "" {
				section = "unscoped"
			}

			// Check if this key should be removed
			if removeSection, exists := keysToRemove[currentSection]; exists && removeSection[key] {
				continue // Skip this line to remove the key
			}

			// Update key with current_value if exists: true
			if sectionData, exists := output[section]; exists {
				if entry, keyExists := sectionData[key]; keyExists && entry.Exists {
					lines = append(lines, fmt.Sprintf("%s = %s", key, entry.CurrentValue))
					if _, ok := sectionKeys[section]; !ok {
						sectionKeys[section] = make(map[string]bool)
					}
					sectionKeys[section][key] = true
					continue
				}
			}
			// Keep unchanged lines
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading .defaultvalues: %v", err)
	}

	// No new keys are added (only updates or removals)
	return os.WriteFile(defaultValuesPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}
func main() {
	inputFile := flag.String("input", "", "Path to the input JSON manifest file")
	restore := flag.Bool("restore", false, "Update .defaultvalues using defaultvalues_comparison.json")
	restorefileManifest := flag.String("manifest", "defaultvalues_comparison.json", "Path to the defaultvalues_comparison.json file (used with --restore)")

	flag.Parse()

	if *inputFile == "" && !*restore {
		fmt.Println("Error: Please provide an input JSON file using --input or use --restore")
		fmt.Println("Usage: generate_defaultvalues_comparison --input <path_to_json> [--restore] [--manifest <path_to_comparison_json>]")
		os.Exit(1)
	}

	// Step 1: Generate the comparison JSON if --input is provided
	if *inputFile != "" {
		manifestData, err := os.ReadFile(*inputFile)
		if err != nil {
			fmt.Printf("Error reading input file: %v\n", err)
			os.Exit(1)
		}

		var manifest Manifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			os.Exit(1)
		}

		var modifyDefaultsEntries map[string]map[string]string
		for _, op := range manifest.Operations {
			if op.Type == "modify_defaults" {
				modifyDefaultsEntries = op.Entries
				break
			}
		}

		if modifyDefaultsEntries == nil {
			fmt.Println("No 'modify_defaults' operation found in the manifest")
			os.Exit(0)
		}

		defaultValues, err := parseDefaultValues("/sda1/data/.defaultvalues")
		if err != nil {
			fmt.Printf("Error parsing .defaultvalues file: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Debug: Parsed .defaultvalues:")
		for section, keys := range defaultValues {
			fmt.Printf("Section: %q\n", section)
			for key, value := range keys {
				fmt.Printf("  %s = %s\n", key, value)
			}
		}

		output := make(Output)

		for sectionName, keys := range modifyDefaultsEntries {
			outputSectionName := sectionName
			iniSectionName := sectionName

			if sectionName == "global" {
				iniSectionName = ""
				outputSectionName = "unscoped"
			}

			fmt.Printf("Debug: Processing section %q (mapped to %q in .defaultvalues)\n", outputSectionName, iniSectionName)

			if _, exists := output[outputSectionName]; !exists {
				output[outputSectionName] = make(map[string]OutputEntry)
			}

			for key, newValue := range keys {
				var currentValue string
				exists := false

				if sectionData, sectionExists := defaultValues[iniSectionName]; sectionExists {
					if val, keyExists := sectionData[key]; keyExists {
						currentValue = val
						exists = true
					}
				}
				fmt.Printf("Debug: Key %q - Current: %q, New: %q, Exists: %v\n", key, currentValue, newValue, exists)

				output[outputSectionName][key] = OutputEntry{
					CurrentValue: currentValue,
					NewValue:     newValue,
					Exists:       exists,
				}
			}
		}

		outputJSON, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling output JSON: %v\n", err)
			os.Exit(1)
		}

		defaultOutputFile := "/tmp/defaultvalues_comparison.json"
		if err := os.WriteFile(defaultOutputFile, outputJSON, 0644); err != nil {
			fmt.Printf("Error writing output file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Comparison JSON file created: %s\n", defaultOutputFile)
	}

	// Step 2: Update .defaultvalues if --restore is provided
	if *restore {
		if _, err := os.Stat(*restorefileManifest); os.IsNotExist(err) {
			fmt.Printf("Error: %s does not exist. Run with --input first to generate it or provide a valid path with --manifest.\n", *restorefileManifest)
			os.Exit(1)
		}

		if err := updateDefaultValues("/sda1/data/.defaultvalues", *restorefileManifest); err != nil {
			fmt.Printf("Error updating .defaultvalues: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Updated /sda1/data/.defaultvalues based on", *restorefileManifest)
	}
}
