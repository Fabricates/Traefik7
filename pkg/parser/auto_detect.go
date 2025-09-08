package parser

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// ConfigType represents the type of load balancer configuration
type ConfigType int

const (
	ConfigTypeUnknown ConfigType = iota
	ConfigTypeCitrix
	ConfigTypeF5
)

// DetectConfigType detects whether a configuration file is Citrix or F5 format
func DetectConfigType(filename string) (ConfigType, error) {
	file, err := os.Open(filename)
	if err != nil {
		return ConfigTypeUnknown, err
	}
	defer file.Close()

	return DetectConfigTypeFromReader(file)
}

// DetectConfigTypeFromReader detects configuration type from an io.Reader
func DetectConfigTypeFromReader(reader io.Reader) (ConfigType, error) {
	scanner := bufio.NewScanner(reader)
	lineCount := 0
	maxLinesToCheck := 100 // Check first 100 lines

	citrixIndicators := 0
	f5Indicators := 0

	for scanner.Scan() && lineCount < maxLinesToCheck {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		// Skip empty lines
		if line == "" {
			continue
		}

		// F5 indicators
		if strings.HasPrefix(line, "#TMSH-VERSION") {
			f5Indicators += 10 // Strong indicator
		}
		if strings.HasPrefix(line, "ltm ") {
			f5Indicators += 5 // Strong indicator
		}
		if strings.Contains(line, "/Common/") {
			f5Indicators += 1 // Weak indicator
		}
		if strings.HasPrefix(line, "apm ") || strings.HasPrefix(line, "sys ") {
			f5Indicators += 2 // Medium indicator
		}

		// Citrix indicators
		if strings.HasPrefix(line, "add server ") {
			citrixIndicators += 5 // Strong indicator
		}
		if strings.HasPrefix(line, "add lb vserver ") {
			citrixIndicators += 5 // Strong indicator
		}
		if strings.HasPrefix(line, "add serviceGroup ") {
			citrixIndicators += 5 // Strong indicator
		}
		if strings.HasPrefix(line, "bind serviceGroup ") {
			citrixIndicators += 5 // Strong indicator
		}
		if strings.HasPrefix(line, "bind lb vserver ") {
			citrixIndicators += 5 // Strong indicator
		}
		if strings.HasPrefix(line, "set ") && (strings.Contains(line, " server ") || strings.Contains(line, " vserver ")) {
			citrixIndicators += 3 // Medium indicator
		}
	}

	if err := scanner.Err(); err != nil {
		return ConfigTypeUnknown, err
	}

	// Determine configuration type based on indicators
	if f5Indicators > citrixIndicators && f5Indicators > 0 {
		return ConfigTypeF5, nil
	} else if citrixIndicators > 0 {
		return ConfigTypeCitrix, nil
	}

	return ConfigTypeUnknown, nil
}

// ParseL7SettingsAuto automatically detects configuration type and parses accordingly
func ParseL7SettingsAuto(filename string) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	configType, err := DetectConfigType(filename)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	switch configType {
	case ConfigTypeF5:
		return ParseF5SettingsFromFileSimple(filename)
	case ConfigTypeCitrix:
		return ParseL7Settings(filename)
	default:
		// Default to Citrix parser for backward compatibility
		return ParseL7Settings(filename)
	}
}

// ParseL7SettingsFromReaderAuto automatically detects configuration type and parses accordingly from a reader
func ParseL7SettingsFromReaderAuto(reader io.Reader) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	// For readers, we need to buffer the content to detect type and then parse
	// Read all content into memory
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// Create a new reader from the buffered content for type detection
	content := strings.Join(lines, "\n")
	typeReader := strings.NewReader(content)

	configType, err := DetectConfigTypeFromReader(typeReader)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// Create another reader for actual parsing
	parseReader := strings.NewReader(content)

	switch configType {
	case ConfigTypeF5:
		return ParseF5SettingsFromReaderSimple(parseReader)
	case ConfigTypeCitrix:
		return ParseL7SettingsFromReader(parseReader)
	default:
		// Default to Citrix parser for backward compatibility
		return ParseL7SettingsFromReader(parseReader)
	}
}
