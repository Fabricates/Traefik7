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
	Name string
	IP   string
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
}

// TraefikService represents a Traefik service configuration
type TraefikService struct {
	LoadBalancer TraefikLoadBalancer `yaml:"loadBalancer"`
}

// TraefikLoadBalancer represents the load balancer configuration
type TraefikLoadBalancer struct {
	Servers []TraefikServer `yaml:"servers"`
}

// TraefikServer represents a server in the load balancer
type TraefikServer struct {
	URL string `yaml:"url"`
}

// TraefikConfig represents the complete Traefik configuration
type TraefikConfig struct {
	HTTP TraefikHTTP `yaml:"http"`
}

// TraefikHTTP represents the HTTP section of Traefik config
type TraefikHTTP struct {
	Services map[string]TraefikService `yaml:"services"`
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
	mapping := generateMappingConfig(vservers)

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
	addServerRe := regexp.MustCompile(`^add server\s+(\S+)\s+(\S+)`)
	addVServerRe := regexp.MustCompile(`^add lb vserver\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)`)
	bindServiceGroupRe := regexp.MustCompile(`^bind serviceGroup\s+(\S+)\s+(\S+)\s+(\S+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse "add server" lines
		if matches := addServerRe.FindStringSubmatch(line); matches != nil {
			servers = append(servers, ServerInfo{
				Name: matches[1],
				IP:   matches[2],
			})
			continue
		}

		// Parse "add lb vserver" lines
		if matches := addVServerRe.FindStringSubmatch(line); matches != nil {
			vservers = append(vservers, VServerInfo{
				Name:     matches[1],
				Protocol: matches[2],
				IP:       matches[3],
				Port:     matches[4],
			})
			continue
		}

		// Parse "bind serviceGroup" lines (excluding monitor bindings)
		if matches := bindServiceGroupRe.FindStringSubmatch(line); matches != nil {
			// Skip monitor bindings
			if strings.Contains(line, "-monitorName") {
				continue
			}
			serviceGroups = append(serviceGroups, ServiceGroup{
				Name:       matches[1],
				ServerName: matches[2],
				Port:       matches[3],
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
	// Create a map of server names to IPs
	serverMap := make(map[string]string)
	for _, server := range servers {
		serverMap[server.Name] = server.IP
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
		
		for _, group := range groups {
			if ip, exists := serverMap[group.ServerName]; exists {
				url := fmt.Sprintf("http://%s:%s", ip, group.Port)
				traefiktServers = append(traefiktServers, TraefikServer{URL: url})
			}
		}

		if len(traefiktServers) > 0 {
			services[serviceName] = TraefikService{
				LoadBalancer: TraefikLoadBalancer{
					Servers: traefiktServers,
				},
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
func generateMappingConfig(vservers []VServerInfo) map[string]string {
	mapping := make(map[string]string)
	
	for _, vserver := range vservers {
		key := fmt.Sprintf("%s:%s", vserver.IP, vserver.Port)
		value := fmt.Sprintf("%s@nacoscs", vserver.Name)
		mapping[key] = value
	}
	
	return mapping
}

// writeYAMLFile writes data to a YAML file
func writeYAMLFile(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	defer encoder.Close()
	
	return encoder.Encode(data)
}