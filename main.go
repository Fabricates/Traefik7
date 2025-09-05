package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fabricates/traefik7/pkg/parser"
)

// verify performs verification checks on the parsed configuration
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

func main() {
	// Define command line flags
	verifyMode := flag.Bool("y", false, "Verify mode - perform verification checks on the L7 settings file")
	outputMode := flag.Bool("o", false, "Output mode - print mappings to stdout instead of writing to files")
	flag.Parse()

	// Get remaining arguments after flags
	args := flag.Args()

	var filename string
	var useStdin bool

	// Check if we should read from stdin (pipeline or no file provided)
	if len(args) == 0 {
		// Check if there's data in stdin
		stat, err := os.Stdin.Stat()
		if err != nil {
			if *verifyMode {
				fmt.Println("Usage: traefik7 -y <l7_settings_file>")
				fmt.Println("       echo 'commands' | traefik7 -y")
			} else {
				fmt.Println("Usage: traefik7 <l7_settings_file>")
				fmt.Println("       traefik7 -y <l7_settings_file>  (verification mode)")
				fmt.Println("       traefik7 -o <l7_settings_file>  (output to stdout)")
				fmt.Println("       echo 'commands' | traefik7 [-y] [-o]")
			}
			os.Exit(1)
		}

		// If stdin has data (pipe or redirect), use it
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			useStdin = true
		} else {
			if *verifyMode {
				fmt.Println("Usage: traefik7 -y <l7_settings_file>")
				fmt.Println("       echo 'commands' | traefik7 -y")
			} else {
				fmt.Println("Usage: traefik7 <l7_settings_file>")
				fmt.Println("       traefik7 -y <l7_settings_file>  (verification mode)")
				fmt.Println("       traefik7 -o <l7_settings_file>  (output to stdout)")
				fmt.Println("       echo 'commands' | traefik7 [-y] [-o]")
			}
			os.Exit(1)
		}
	} else {
		filename = args[0]
		useStdin = false
	}

	// Parse the L7 settings
	var servers []parser.ServerInfo
	var vservers []parser.VServerInfo
	var serviceGroupDefs []parser.ServiceGroupDef
	var serviceGroups []parser.ServiceGroup
	var vserverBindings []parser.VServerBinding
	var err error

	if useStdin {
		servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, err = parser.ParseL7SettingsFromReader(os.Stdin)
	} else {
		servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, err = parser.ParseL7Settings(filename)
	}
	if err != nil {
		fmt.Printf("Error parsing L7 settings: %v\n", err)
		os.Exit(1)
	}

	// If in verification mode, perform verification and exit
	if *verifyMode {
		verified := verify(servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings)
		if !verified {
			fmt.Println("Verification failed")
			os.Exit(1)
		}
		fmt.Println("Verification passed")
		return
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
