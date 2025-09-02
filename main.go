package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerInfo represents a server with its IP address
type ServerInfo struct {
	Name    string
	IP      string
	Comment string
}

// VServerInfo represents a virtual server configuration
type VServerInfo struct {
	Name     string
	Protocol string
	IP       string
	Port     string
}

// ServiceGroup represents a service group binding
type ServiceGroup struct {
	Name       string
	ServerName string
	Port       string
	Comment    string
}

// TraefikService represents a Traefik service configuration
type TraefikService struct {
	LoadBalancer TraefikLoadBalancer `yaml:"loadBalancer"`
	Comment      string              `yaml:"-"` // Service-level comment (not serialized)
}

// TraefikLoadBalancer represents the load balancer configuration
type TraefikLoadBalancer struct {
	Servers []TraefikServer `yaml:"servers"`
}

// TraefikServer represents a server in the load balancer
type TraefikServer struct {
	URL     string `yaml:"url"`
	Comment string `yaml:"-"` // Don't include in YAML output
}

// TraefikConfig represents the complete Traefik configuration
type TraefikConfig struct {
	HTTP TraefikHTTP `yaml:"http"`
}

// TraefikHTTP represents the HTTP section of Traefik config
type TraefikHTTP struct {
	Services map[string]TraefikService `yaml:"services"`
}

// MappingEntry represents a mapping entry with optional comment
type MappingEntry struct {
	Key     string
	Value   string
	Comment string
}

// MappingConfig represents the mapping configuration
type MappingConfig struct {
	Entries []MappingEntry
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <input-file>\n", os.Args[0])
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// Parse the input file
	servers, vservers, serviceGroups, err := parseL7Settings(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
		os.Exit(1)
	}

	// Generate timestamp folder
	timestamp := time.Now().Format("200601021504") // YYYYMMDDHHMM format
	outputDir := timestamp

	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Generate Traefik configuration
	traefik := generateTraefikConfig(servers, vservers, serviceGroups)

	// Generate mapping configuration
	mapping := generateMappingConfig(vservers, serviceGroups)

	// Write Traefik configuration to file
	traefiktFile := filepath.Join(outputDir, "traefik-services.yaml")
	err = writeYAMLFile(traefiktFile, traefik)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing Traefik config: %v\n", err)
		os.Exit(1)
	}

	// Write mapping configuration to file
	mappingFile := filepath.Join(outputDir, "mapping.yaml")
	err = writeYAMLFile(mappingFile, mapping)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing mapping config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated configurations in directory: %s\n", outputDir)
	fmt.Printf("  - %s\n", traefiktFile)
	fmt.Printf("  - %s\n", mappingFile)
}

// parseL7Settings parses the L7 configuration file
func parseL7Settings(filename string) ([]ServerInfo, []VServerInfo, []ServiceGroup, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, nil, err
	}
	defer file.Close()

	var servers []ServerInfo
	var vservers []VServerInfo
	var serviceGroups []ServiceGroup

	scanner := bufio.NewScanner(file)

	// Regular expressions for parsing different line types
	addServerRe := regexp.MustCompile(`^add server\s+(\S+)\s+(\S+)(?:\s+-comment\s+"([^"]+)")?`)
	addVServerRe := regexp.MustCompile(`^add lb vserver\s+(?:"([^"]+)"|(\S+))\s+(\S+)\s+(\S+)\s+(\S+)`)
	bindServiceGroupRe := regexp.MustCompile(`^bind serviceGroup\s+(?:"([^"]+)"|(\S+))\s+(\S+)\s+(\S+)(?:\s+-comment\s+"([^"]*)")?`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "add server" lines
		if matches := addServerRe.FindStringSubmatch(line); matches != nil {
			comment := ""
			if len(matches) > 3 && matches[3] != "" {
				comment = matches[3]
			}
			servers = append(servers, ServerInfo{
				Name:    matches[1],
				IP:      matches[2],
				Comment: comment,
			})
			continue
		}

		// Parse "add lb vserver" lines
		if matches := addVServerRe.FindStringSubmatch(line); matches != nil {
			// Handle quoted or unquoted vserver name
			vserverName := matches[1] // quoted name
			if vserverName == "" {
				vserverName = matches[2] // unquoted name
			}
			vservers = append(vservers, VServerInfo{
				Name:     vserverName,
				Protocol: matches[3],
				IP:       matches[4],
				Port:     matches[5],
			})
			continue
		}

		// Parse "bind serviceGroup" lines (excluding monitor bindings)
		if matches := bindServiceGroupRe.FindStringSubmatch(line); matches != nil {
			// Skip monitor bindings
			if strings.Contains(line, "-monitorName") {
				continue
			}
			// Handle quoted or unquoted service group name
			serviceName := matches[1] // quoted name
			if serviceName == "" {
				serviceName = matches[2] // unquoted name
			}
			comment := ""
			if len(matches) > 5 && matches[5] != "" {
				comment = matches[5]
			}
			serviceGroups = append(serviceGroups, ServiceGroup{
				Name:       serviceName,
				ServerName: matches[3],
				Port:       matches[4],
				Comment:    comment,
			})
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, err
	}

	return servers, vservers, serviceGroups, nil
}

// generateTraefikConfig generates the Traefik configuration
func generateTraefikConfig(servers []ServerInfo, vservers []VServerInfo, serviceGroups []ServiceGroup) TraefikConfig {
	// Create a map of server names to server info
	serverMap := make(map[string]ServerInfo)
	for _, server := range servers {
		serverMap[server.Name] = server
	}

	// Group service groups by service name
	serviceGroupMap := make(map[string][]ServiceGroup)
	for _, sg := range serviceGroups {
		serviceGroupMap[sg.Name] = append(serviceGroupMap[sg.Name], sg)
	}

	services := make(map[string]TraefikService)

	// For each service group, create a Traefik service
	for serviceName, groups := range serviceGroupMap {
		var traefiktServers []TraefikServer
		var serviceComment string

		for _, group := range groups {
			if serverInfo, exists := serverMap[group.ServerName]; exists {
				url := fmt.Sprintf("http://%s:%s", serverInfo.IP, group.Port)
				traefiktServer := TraefikServer{URL: url}

				// For server-level comments, only use server comment (not service group comment)
				if serverInfo.Comment != "" {
					traefiktServer.Comment = serverInfo.Comment
				}

				// For service-level comment, use the first non-empty service group comment
				if serviceComment == "" && group.Comment != "" {
					serviceComment = group.Comment
				}

				traefiktServers = append(traefiktServers, traefiktServer)
			}
		}

		if len(traefiktServers) > 0 {
			services[serviceName] = TraefikService{
				LoadBalancer: TraefikLoadBalancer{
					Servers: traefiktServers,
				},
				Comment: serviceComment,
			}
		}
	}

	return TraefikConfig{
		HTTP: TraefikHTTP{
			Services: services,
		},
	}
}

// generateMappingConfig generates the mapping configuration
func generateMappingConfig(vservers []VServerInfo, serviceGroups []ServiceGroup) MappingConfig {
	var entries []MappingEntry

	// Create a map to find service groups by vserver name
	serviceGroupsByVServer := make(map[string][]ServiceGroup)
	for _, sg := range serviceGroups {
		serviceGroupsByVServer[sg.Name] = append(serviceGroupsByVServer[sg.Name], sg)
	}

	for _, vserver := range vservers {
		key := fmt.Sprintf("%s:%s", vserver.IP, vserver.Port)
		value := fmt.Sprintf("%s@nacoscs", vserver.Name)

		// Check if there's a service group comment for this vserver
		comment := ""
		if groups, exists := serviceGroupsByVServer[vserver.Name]; exists {
			// Use the first non-empty comment found
			for _, group := range groups {
				if group.Comment != "" {
					comment = group.Comment
					break
				}
			}
		}

		entries = append(entries, MappingEntry{
			Key:     key,
			Value:   value,
			Comment: comment,
		})
	}

	return MappingConfig{Entries: entries}
}

// writeYAMLFile writes data to a YAML file
func writeYAMLFile(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if this is a TraefikConfig that needs special comment handling
	if traefik, ok := data.(TraefikConfig); ok {
		return writeTraefikConfigWithComments(file, traefik)
	}

	// Check if this is a MappingConfig that needs special comment handling
	if mapping, ok := data.(MappingConfig); ok {
		return writeMappingConfigWithComments(file, mapping)
	}

	// For other types, use standard YAML encoding
	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	return encoder.Encode(data)
}

// writeTraefikConfigWithComments writes TraefikConfig with inline comments
func writeTraefikConfigWithComments(file *os.File, config TraefikConfig) error {
	// First, create a copy without comments for standard YAML marshaling
	configCopy := TraefikConfig{
		HTTP: TraefikHTTP{
			Services: make(map[string]TraefikService),
		},
	}

	// Store comments by URL for easier lookup
	urlComments := make(map[string]string)
	// Store service-level comments
	serviceComments := make(map[string]string)

	for serviceName, service := range config.HTTP.Services {
		newService := TraefikService{
			LoadBalancer: TraefikLoadBalancer{
				Servers: make([]TraefikServer, len(service.LoadBalancer.Servers)),
			},
		}

		for i, server := range service.LoadBalancer.Servers {
			newService.LoadBalancer.Servers[i] = TraefikServer{
				URL: server.URL,
			}

			// Store comment by URL if it exists
			if server.Comment != "" {
				urlComments[server.URL] = server.Comment
			}
		}

		// Store service-level comment
		if service.Comment != "" {
			serviceComments[serviceName] = service.Comment
		}

		configCopy.HTTP.Services[serviceName] = newService
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(configCopy)
	if err != nil {
		return err
	}

	// Convert to string and add comments
	yamlStr := string(yamlBytes)
	lines := strings.Split(yamlStr, "\n")

	// Process lines to add comments
	for i, line := range lines {
		// Look for service names and add service-level comments before them
		trimmedLine := strings.TrimSpace(line)
		for serviceName, comment := range serviceComments {
			if strings.HasPrefix(trimmedLine, serviceName+":") {
				// Add service comment on the line before the service definition
				indentation := strings.Repeat(" ", len(line)-len(trimmedLine))
				commentLine := indentation + "# " + comment
				lines[i] = commentLine + "\n" + line
				break
			}
		}

		// Look for URL lines and add server comments
		if strings.Contains(line, "url: http://") {
			// Extract the URL from the line
			urlStart := strings.Index(line, "http://")
			if urlStart != -1 {
				url := strings.TrimSpace(line[urlStart:])
				if comment, exists := urlComments[url]; exists {
					lines[i] = line + " # " + comment
				}
			}
		}
	}

	// Write the modified YAML
	_, err = file.WriteString(strings.Join(lines, "\n"))
	return err
}

// writeMappingConfigWithComments writes MappingConfig with inline comments
func writeMappingConfigWithComments(file *os.File, config MappingConfig) error {
	// Create a simple map for YAML marshaling
	mappingMap := make(map[string]string)
	entryComments := make(map[string]string)

	for _, entry := range config.Entries {
		mappingMap[entry.Key] = entry.Value
		if entry.Comment != "" {
			entryComments[entry.Key] = entry.Comment
		}
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(mappingMap)
	if err != nil {
		return err
	}

	// Convert to string and add comments
	yamlStr := string(yamlBytes)
	lines := strings.Split(yamlStr, "\n")

	// Process lines to add comments
	for i, line := range lines {
		// Look for mapping lines (key: value format)
		if strings.Contains(line, ": ") {
			// Extract the key from the line
			colonIndex := strings.Index(line, ": ")
			if colonIndex != -1 {
				key := strings.TrimSpace(line[:colonIndex])
				if comment, exists := entryComments[key]; exists {
					lines[i] = line + " # " + comment
				}
			}
		}
	}

	// Write the modified YAML
	_, err = file.WriteString(strings.Join(lines, "\n"))
	return err
}
