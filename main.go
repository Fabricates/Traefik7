package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fabricates/traefik7/pkg/parser"
)

// verify performs basic verification checks on the parsed configuration
func verify(servers []parser.ServerInfo, vservers []parser.VServerInfo, serviceGroupDefs []parser.ServiceGroupDef, serviceGroups []parser.ServiceGroup, vserverBindings []parser.VServerBinding) bool {
	success := true

	// Check if all referenced servers exist
	serverMap := make(map[string]bool)
	for _, server := range servers {
		serverMap[server.Name] = true
	}

	for _, sg := range serviceGroups {
		if !serverMap[sg.ServerName] {
			fmt.Printf("Error: Service group '%s' references non-existent server '%s'\n", sg.Name, sg.ServerName)
			success = false
		}
	}

	// Check if all service groups have at least one server binding
	serviceGroupMap := make(map[string]bool)
	for _, sg := range serviceGroups {
		serviceGroupMap[sg.Name] = true
	}

	for _, sgDef := range serviceGroupDefs {
		if !serviceGroupMap[sgDef.Name] {
			fmt.Printf("Warning: Service group '%s' is defined but has no server bindings\n", sgDef.Name)
		}
	}

	// Check for duplicate server names
	seenServers := make(map[string]bool)
	for _, server := range servers {
		if seenServers[server.Name] {
			fmt.Printf("Error: Duplicate server name '%s'\n", server.Name)
			success = false
		}
		seenServers[server.Name] = true
	}

	// Check for duplicate vserver names
	seenVServers := make(map[string]bool)
	for _, vserver := range vservers {
		if seenVServers[vserver.Name] {
			fmt.Printf("Error: Duplicate vserver name '%s'\n", vserver.Name)
			success = false
		}
		seenVServers[vserver.Name] = true
	}

	// Check vserver bindings
	vserverMap := make(map[string]bool)
	for _, vserver := range vservers {
		vserverMap[vserver.Name] = true
	}

	for _, binding := range vserverBindings {
		if !vserverMap[binding.VServerName] {
			fmt.Printf("Error: VServer binding references non-existent vserver '%s'\n", binding.VServerName)
			success = false
		}
		// Only check service group if there's actually a service name (not policy-only bindings)
		if binding.ServiceName != "" && !serviceGroupMap[binding.ServiceName] {
			fmt.Printf("Warning: VServer binding '%s' references service '%s' that has no group definition\n",
				binding.VServerName, binding.ServiceName)
		}
	}

	// Report summary
	fmt.Printf("Found %d servers, %d vservers, %d service group definitions, %d service group bindings, %d vserver bindings\n",
		len(servers), len(vservers), len(serviceGroupDefs), len(serviceGroups), len(vserverBindings))

	return success
}

// verifyWithMappingsAndSource performs enhanced verification by comparing Citrix commands with generated mappings
// Supports both file input and stdin input
func verifyWithMappingsAndSource(inputSource, mappingFolder string, useStdin bool) bool {
	if useStdin {
		fmt.Printf("Enhanced verification: comparing Citrix commands from stdin with mappings in '%s'\n", mappingFolder)
	} else {
		fmt.Printf("Enhanced verification: comparing Citrix commands in '%s' with mappings in '%s'\n", inputSource, mappingFolder)
	}

	// Parse the Citrix settings
	var servers []parser.ServerInfo
	var vservers []parser.VServerInfo
	var serviceGroupDefs []parser.ServiceGroupDef
	var serviceGroups []parser.ServiceGroup
	var vserverBindings []parser.VServerBinding
	var err error

	if useStdin {
		servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, err = parser.ParseL7SettingsFromReader(os.Stdin)
	} else {
		servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, err = parser.ParseL7Settings(inputSource)
	}

	if err != nil {
		fmt.Printf("Error parsing Citrix settings: %v\n", err)
		return false
	}

	// Perform basic verification first
	if !verify(servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings) {
		fmt.Println("Basic verification failed, skipping mapping verification")
		return false
	}

	// Read the generated mapping files
	traefikPath := filepath.Join(mappingFolder, "traefik-services.yaml")
	mappingPath := filepath.Join(mappingFolder, "mapping.yaml")

	// Check if mapping files exist
	if _, err := os.Stat(traefikPath); os.IsNotExist(err) {
		fmt.Printf("Error: Traefik services file not found: %s\n", traefikPath)
		return false
	}
	if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
		fmt.Printf("Error: Mapping file not found: %s\n", mappingPath)
		return false
	}

	// Generate expected configurations to compare
	expectedTraefikConfig := parser.GenerateTraefikConfig(servers, vservers, serviceGroupDefs, serviceGroups)
	expectedMappingConfig := parser.GenerateMappingConfig(vservers, serviceGroupDefs, serviceGroups)

	success := true

	// Verify Traefik services mapping
	fmt.Println("\n=== Verifying Traefik Services ===")
	actualTraefikConfig, err := parser.ReadTraefikConfig(traefikPath)
	if err != nil {
		fmt.Printf("Error reading Traefik config: %v\n", err)
		success = false
	} else {
		success = verifyTraefikServices(expectedTraefikConfig, actualTraefikConfig) && success
	}

	// Verify IP:Port mappings
	fmt.Println("\n=== Verifying IP:Port Mappings ===")
	actualMappingConfig, err := parser.ReadMappingConfig(mappingPath)
	if err != nil {
		fmt.Printf("Error reading mapping config: %v\n", err)
		success = false
	} else {
		success = verifyMappings(expectedMappingConfig, actualMappingConfig) && success
	}

	// Verify that all Citrix services have corresponding Traefik services
	fmt.Println("\n=== Verifying Service Coverage ===")
	success = verifyServiceCoverage(serviceGroups, expectedTraefikConfig) && success

	// Verify that all virtual servers have corresponding mappings
	fmt.Println("\n=== Verifying Virtual Server Coverage ===")
	success = verifyVServerCoverage(vservers, expectedMappingConfig) && success

	if success {
		fmt.Println("\n✅ Enhanced verification passed - all Citrix commands correctly mapped!")
	} else {
		fmt.Println("\n❌ Enhanced verification failed - discrepancies found!")
	}

	return success
}

// verifyTraefikServices compares expected and actual Traefik service configurations
func verifyTraefikServices(expected, actual parser.TraefikConfig) bool {
	success := true

	// Check if all expected services are present
	for serviceName, expectedService := range expected.HTTP.Services {
		actualService, exists := actual.HTTP.Services[serviceName]
		if !exists {
			fmt.Printf("❌ Missing Traefik service: %s\n", serviceName)
			success = false
			continue
		}

		// Check if server counts match
		expectedCount := len(expectedService.LoadBalancer.Servers)
		actualCount := len(actualService.LoadBalancer.Servers)
		if expectedCount != actualCount {
			fmt.Printf("❌ Service '%s': expected %d servers, found %d\n", serviceName, expectedCount, actualCount)
			success = false
		}

		// Check if all expected server URLs are present
		expectedURLs := make(map[string]bool)
		for _, server := range expectedService.LoadBalancer.Servers {
			expectedURLs[server.URL] = true
		}

		for _, server := range actualService.LoadBalancer.Servers {
			if !expectedURLs[server.URL] {
				fmt.Printf("❌ Service '%s': unexpected server URL: %s\n", serviceName, server.URL)
				success = false
			} else {
				delete(expectedURLs, server.URL)
			}
		}

		// Check for missing URLs
		for missingURL := range expectedURLs {
			fmt.Printf("❌ Service '%s': missing server URL: %s\n", serviceName, missingURL)
			success = false
		}

		if expectedCount == actualCount && len(expectedURLs) == 0 {
			fmt.Printf("✅ Service '%s': %d servers correctly mapped\n", serviceName, expectedCount)
		}
	}

	// Check for unexpected services
	for serviceName := range actual.HTTP.Services {
		if _, exists := expected.HTTP.Services[serviceName]; !exists {
			fmt.Printf("⚠️  Unexpected Traefik service found: %s\n", serviceName)
		}
	}

	return success
}

// verifyMappings compares expected and actual mapping configurations
func verifyMappings(expected, actual parser.MappingConfig) bool {
	success := true

	expectedMappings := make(map[string]string)
	for _, entry := range expected.Entries {
		expectedMappings[entry.Key] = entry.Value
	}

	actualMappings := make(map[string]string)
	for _, entry := range actual.Entries {
		actualMappings[entry.Key] = entry.Value
	}

	// Check if all expected mappings are present
	for key, expectedValue := range expectedMappings {
		actualValue, exists := actualMappings[key]
		if !exists {
			fmt.Printf("❌ Missing mapping: %s -> %s\n", key, expectedValue)
			success = false
		} else if actualValue != expectedValue {
			fmt.Printf("❌ Incorrect mapping: %s -> expected '%s', found '%s'\n", key, expectedValue, actualValue)
			success = false
		} else {
			fmt.Printf("✅ Mapping verified: %s -> %s\n", key, expectedValue)
		}
	}

	// Check for unexpected mappings
	for key, value := range actualMappings {
		if _, exists := expectedMappings[key]; !exists {
			fmt.Printf("⚠️  Unexpected mapping found: %s -> %s\n", key, value)
		}
	}

	return success
}

// verifyServiceCoverage ensures all Citrix service groups have corresponding Traefik services
func verifyServiceCoverage(serviceGroups []parser.ServiceGroup, traefikConfig parser.TraefikConfig) bool {
	success := true

	// Collect unique service group names
	serviceGroupNames := make(map[string]bool)
	for _, sg := range serviceGroups {
		serviceGroupNames[sg.Name] = true
	}

	// Check if each service group has a corresponding Traefik service
	for serviceName := range serviceGroupNames {
		if _, exists := traefikConfig.HTTP.Services[serviceName]; !exists {
			fmt.Printf("❌ Citrix service group '%s' not found in Traefik services\n", serviceName)
			success = false
		} else {
			fmt.Printf("✅ Citrix service group '%s' mapped to Traefik service\n", serviceName)
		}
	}

	return success
}

// verifyVServerCoverage ensures all Citrix virtual servers have corresponding mappings
func verifyVServerCoverage(vservers []parser.VServerInfo, mappingConfig parser.MappingConfig) bool {
	success := true

	// Create a map of existing mappings by virtual server name
	mappingsByVServer := make(map[string]bool)
	for _, entry := range mappingConfig.Entries {
		// Extract virtual server name from the mapping value (remove @nacoscs suffix)
		vserverName := entry.Value
		if idx := strings.Index(vserverName, "@"); idx != -1 {
			vserverName = vserverName[:idx]
		}
		mappingsByVServer[vserverName] = true
	}

	// Check if each virtual server has a corresponding mapping
	for _, vserver := range vservers {
		if !mappingsByVServer[vserver.Name] {
			fmt.Printf("❌ Citrix virtual server '%s' (%s:%s) not found in mappings\n", vserver.Name, vserver.IP, vserver.Port)
			success = false
		} else {
			fmt.Printf("✅ Citrix virtual server '%s' (%s:%s) mapped correctly\n", vserver.Name, vserver.IP, vserver.Port)
		}
	}

	return success
}

func main() {
	// Define command line flags
	verifyMode := flag.Bool("y", false, "Verify mode - perform verification checks on the L7 settings file and mapping folder")
	outputMode := flag.Bool("o", false, "Output mode - print mappings to stdout instead of writing to files")
	inputFile := flag.String("i", "", "Input Citrix settings file (use '-' or omit for stdin)")
	mappingFolder := flag.String("m", "", "Mapping folder containing traefik-services.yaml and mapping.yaml (required for verification mode)")
	flag.Parse()

	// Handle verification mode
	if *verifyMode {
		// Check if mapping folder is provided
		if *mappingFolder == "" {
			fmt.Println("Error: Mapping folder (-m) is required for verification mode")
			fmt.Println()
			fmt.Println("Usage: traefik7 -y -i <l7_settings_file> -m <mapping_folder>")
			fmt.Println("       traefik7 -y -m <mapping_folder> (read from stdin)")
			fmt.Println("       echo 'commands' | traefik7 -y -m <mapping_folder>")
			os.Exit(1)
		}

		// Determine input source
		var inputSource string
		var useStdin bool

		if *inputFile == "" || *inputFile == "-" {
			// Check if there's data in stdin
			stat, err := os.Stdin.Stat()
			if err != nil {
				fmt.Printf("Error checking stdin: %v\n", err)
				os.Exit(1)
			}

			if (stat.Mode() & os.ModeCharDevice) == 0 {
				useStdin = true
				inputSource = "stdin"
			} else {
				fmt.Println("Error: No input provided (stdin is empty and no input file specified)")
				fmt.Println()
				fmt.Println("Usage: traefik7 -y -i <l7_settings_file> -m <mapping_folder>")
				fmt.Println("       traefik7 -y -m <mapping_folder> (read from stdin)")
				fmt.Println("       echo 'commands' | traefik7 -y -m <mapping_folder>")
				os.Exit(1)
			}
		} else {
			inputSource = *inputFile
			useStdin = false
		}

		verified := verifyWithMappingsAndSource(inputSource, *mappingFolder, useStdin)
		if !verified {
			fmt.Println("Enhanced verification failed")
			os.Exit(1)
		}
		fmt.Println("Enhanced verification passed")
		return
	}

	// Handle regular processing mode
	var filename string
	var useStdin bool

	// Determine input source
	if *inputFile == "" || *inputFile == "-" {
		// Check remaining args for backward compatibility
		args := flag.Args()
		if len(args) > 0 {
			filename = args[0]
			useStdin = false
		} else {
			// Check if there's data in stdin
			stat, err := os.Stdin.Stat()
			if err != nil {
				fmt.Println("Usage: traefik7 [-i] <l7_settings_file>")
				fmt.Println("       traefik7 -y -i <l7_settings_file> -m <mapping_folder>  (verification mode)")
				fmt.Println("       traefik7 -o [-i] <l7_settings_file>  (output to stdout)")
				fmt.Println("       traefik7 [-o] (read from stdin)")
				fmt.Println("       echo 'commands' | traefik7 [-o]")
				os.Exit(1)
			}

			// If stdin has data (pipe or redirect), use it
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				useStdin = true
			} else {
				fmt.Println("Usage: traefik7 [-i] <l7_settings_file>")
				fmt.Println("       traefik7 -y -i <l7_settings_file> -m <mapping_folder>  (verification mode)")
				fmt.Println("       traefik7 -o [-i] <l7_settings_file>  (output to stdout)")
				fmt.Println("       traefik7 [-o] (read from stdin)")
				fmt.Println("       echo 'commands' | traefik7 [-o]")
				os.Exit(1)
			}
		}
	} else {
		filename = *inputFile
		useStdin = false
	}

	// Parse the L7 settings
	var servers []parser.ServerInfo
	var vservers []parser.VServerInfo
	var serviceGroupDefs []parser.ServiceGroupDef
	var serviceGroups []parser.ServiceGroup
	var err error

	if useStdin {
		servers, vservers, serviceGroupDefs, serviceGroups, _, err = parser.ParseL7SettingsFromReader(os.Stdin)
	} else {
		servers, vservers, serviceGroupDefs, serviceGroups, _, err = parser.ParseL7Settings(filename)
	}
	if err != nil {
		fmt.Printf("Error parsing L7 settings: %v\n", err)
		os.Exit(1)
	}

	// Generate Traefik configuration
	traefikConfig := parser.GenerateTraefikConfig(servers, vservers, serviceGroupDefs, serviceGroups)

	// Generate mapping configuration
	mappingConfig := parser.GenerateMappingConfig(vservers, serviceGroupDefs, serviceGroups)

	// If output mode is enabled, print to stdout
	if *outputMode {
		fmt.Println("# Traefik Services Configuration")
		err = parser.WriteTraefikConfigWithComments(os.Stdout, traefikConfig)
		if err != nil {
			fmt.Printf("Error writing Traefik config to stdout: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("# Mapping Configuration")
		err = parser.WriteMappingConfigWithComments(os.Stdout, mappingConfig)
		if err != nil {
			fmt.Printf("Error writing mapping config to stdout: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Create timestamped output directory
	timestamp := time.Now().Format("200601021504") // yyyymmddhhMM format
	outputDir := filepath.Join(".", timestamp)
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory %s: %v\n", outputDir, err)
		os.Exit(1)
	}

	// Write Traefik configuration to file
	traefikPath := filepath.Join(outputDir, "traefik-services.yaml")
	outputFile, err := os.Create(traefikPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", traefikPath, err)
		os.Exit(1)
	}
	defer outputFile.Close()

	err = parser.WriteTraefikConfigWithComments(outputFile, traefikConfig)
	if err != nil {
		fmt.Printf("Error writing Traefik config: %v\n", err)
		os.Exit(1)
	}

	// Write mapping configuration to file
	mappingPath := filepath.Join(outputDir, "mapping.yaml")
	mappingFile, err := os.Create(mappingPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", mappingPath, err)
		os.Exit(1)
	}
	defer mappingFile.Close()

	err = parser.WriteMappingConfigWithComments(mappingFile, mappingConfig)
	if err != nil {
		fmt.Printf("Error writing mapping config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated files in directory: %s\n", outputDir)
	fmt.Printf("  - %s\n", traefikPath)
	fmt.Printf("  - %s\n", mappingPath)
}
