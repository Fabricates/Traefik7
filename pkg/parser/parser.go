package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// CommandProcessor handles processing of parsed F5 commands
type CommandProcessor struct{}

// NewCommandProcessor creates a new command processor
func NewCommandProcessor() *CommandProcessor {
	return &CommandProcessor{}
}

// handleAddCommand processes add commands
func (p *CommandProcessor) handleAddCommand(command *F5Command, servers *[]ServerInfo, vservers *[]VServerInfo, serviceGroupDefs *[]ServiceGroupDef) error {
	objectType := strings.ToLower(strings.ReplaceAll(command.ObjectType, " ", ""))
	switch objectType {
	case "server":
		return p.handleAddServer(command, servers)
	case "lbvserver":
		return p.handleAddLBVServer(command, vservers)
	case "servicegroup":
		return p.handleAddServiceGroup(command, serviceGroupDefs)
	default:
		// Ignore unknown object types for now
		return nil
	}
}

// handleAddServer processes "add server" commands
func (p *CommandProcessor) handleAddServer(command *F5Command, servers *[]ServerInfo) error {
	if len(command.Arguments) < 1 {
		return fmt.Errorf("add server command requires IP address argument")
	}

	comment := command.Parameters["-comment"]

	*servers = append(*servers, ServerInfo{
		Name:    command.Name,
		IP:      command.Arguments[0],
		Comment: comment,
	})

	return nil
}

// handleAddLBVServer processes "add lb vserver" commands
func (p *CommandProcessor) handleAddLBVServer(command *F5Command, vservers *[]VServerInfo) error {
	if len(command.Arguments) < 3 {
		return fmt.Errorf("add lb vserver command requires protocol, IP, and port arguments")
	}

	*vservers = append(*vservers, VServerInfo{
		Name:     command.Name,
		Protocol: command.Arguments[0],
		IP:       command.Arguments[1],
		Port:     command.Arguments[2],
	})

	return nil
}

// handleAddServiceGroup processes "add serviceGroup" commands
func (p *CommandProcessor) handleAddServiceGroup(command *F5Command, serviceGroupDefs *[]ServiceGroupDef) error {
	comment := command.Parameters["-comment"]
	protocol := ""
	if len(command.Arguments) > 0 {
		protocol = command.Arguments[0]
	}

	*serviceGroupDefs = append(*serviceGroupDefs, ServiceGroupDef{
		Name:     command.Name,
		Protocol: protocol,
		Comment:  comment,
	})

	return nil
}

// handleBindCommand processes bind commands
func (p *CommandProcessor) handleBindCommand(command *F5Command, serviceGroups *[]ServiceGroup, vserverBindings *[]VServerBinding) error {
	objectType := strings.ToLower(strings.ReplaceAll(command.ObjectType, " ", ""))
	switch objectType {
	case "servicegroup":
		return p.handleBindServiceGroup(command, serviceGroups)
	case "lbvserver":
		return p.handleBindLBVServer(command, vserverBindings)
	default:
		// Ignore unknown object types for now
		return nil
	}
}

// handleBindServiceGroup processes "bind serviceGroup" commands
func (p *CommandProcessor) handleBindServiceGroup(command *F5Command, serviceGroups *[]ServiceGroup) error {
	// Skip monitor bindings (they don't have server/port arguments)
	if command.Parameters["-monitorName"] != "" {
		return nil
	}

	if len(command.Arguments) < 2 {
		return fmt.Errorf("bind serviceGroup command requires server name and port arguments")
	}

	comment := command.Parameters["-comment"]

	*serviceGroups = append(*serviceGroups, ServiceGroup{
		Name:       command.Name,
		ServerName: command.Arguments[0],
		Port:       command.Arguments[1],
		Comment:    comment,
	})

	return nil
}

// handleBindLBVServer processes "bind lb vserver" commands
func (p *CommandProcessor) handleBindLBVServer(command *F5Command, vserverBindings *[]VServerBinding) error {
	var serviceName string

	// Check if there are arguments and if the first one doesn't start with '-'
	// If so, it's likely a service name
	if len(command.Arguments) > 0 && !strings.HasPrefix(command.Arguments[0], "-") {
		serviceName = command.Arguments[0]
	}

	// Extract policy-related parameters
	policyName := command.Parameters["-policyName"]
	priority := command.Parameters["-priority"]
	gotoExpression := command.Parameters["-gotoPriorityExpression"]
	bindType := command.Parameters["-type"]
	comment := command.Parameters["-comment"]

	*vserverBindings = append(*vserverBindings, VServerBinding{
		VServerName:    command.Name,
		ServiceName:    serviceName,
		PolicyName:     policyName,
		Priority:       priority,
		GotoExpression: gotoExpression,
		Type:           bindType,
		Comment:        comment,
	})

	return nil
}

// handleSetCommand processes set commands
func (p *CommandProcessor) handleSetCommand(command *F5Command) error {
	// For now, we ignore set commands as they typically modify existing objects
	// rather than define new ones
	return nil
}

// ParseL7Settings parses the L7 configuration file using proper F5 command parsing
func ParseL7Settings(filename string) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	defer file.Close()

	var servers []ServerInfo
	var vservers []VServerInfo
	var serviceGroupDefs []ServiceGroupDef
	var serviceGroups []ServiceGroup
	var vserverBindings []VServerBinding

	processor := NewCommandProcessor()
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the F5 command
		command, err := ParseF5Command(line)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("line %d: %v", lineNumber, err)
		}

		// Skip if command is nil (empty line or comment)
		if command == nil {
			continue
		}

		// Process the command based on action and object type
		switch command.Action {
		case "add":
			err = processor.handleAddCommand(command, &servers, &vservers, &serviceGroupDefs)
		case "bind":
			err = processor.handleBindCommand(command, &serviceGroups, &vserverBindings)
		case "set":
			err = processor.handleSetCommand(command)
		default:
			// Ignore unknown commands for now
			continue
		}

		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("line %d: %v", lineNumber, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, nil
}

// ParseL7SettingsFromReader parses F5 L7 settings from an io.Reader (stdin, pipe, etc.)
func ParseL7SettingsFromReader(reader io.Reader) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	var servers []ServerInfo
	var vservers []VServerInfo
	var serviceGroupDefs []ServiceGroupDef
	var serviceGroups []ServiceGroup
	var vserverBindings []VServerBinding

	processor := NewCommandProcessor()
	scanner := bufio.NewScanner(reader)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the F5 command
		command, err := ParseF5Command(line)
		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("line %d: %v", lineNumber, err)
		}

		// Skip if command is nil (empty line or comment)
		if command == nil {
			continue
		}

		// Process the command based on action and object type
		switch command.Action {
		case "add":
			err = processor.handleAddCommand(command, &servers, &vservers, &serviceGroupDefs)
		case "bind":
			err = processor.handleBindCommand(command, &serviceGroups, &vserverBindings)
		case "set":
			err = processor.handleSetCommand(command)
		default:
			// Ignore unknown commands for now
			continue
		}

		if err != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("line %d: %v", lineNumber, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, nil
}

// GenerateTraefikConfig generates the Traefik configuration
func GenerateTraefikConfig(servers []ServerInfo, vservers []VServerInfo, serviceGroupDefs []ServiceGroupDef, serviceGroups []ServiceGroup) TraefikConfig {
	// Create a map of server names to server info
	serverMap := make(map[string]ServerInfo)
	for _, server := range servers {
		serverMap[server.Name] = server
	}

	// Create a map of service group definitions for comment lookup
	serviceGroupDefMap := make(map[string]ServiceGroupDef)
	for _, sgDef := range serviceGroupDefs {
		serviceGroupDefMap[sgDef.Name] = sgDef
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

		// Check if there's a service group definition with a comment (priority)
		if sgDef, exists := serviceGroupDefMap[serviceName]; exists && sgDef.Comment != "" {
			serviceComment = sgDef.Comment
		}

		for _, group := range groups {
			if serverInfo, exists := serverMap[group.ServerName]; exists {
				url := fmt.Sprintf("http://%s:%s", serverInfo.IP, group.Port)
				traefiktServer := TraefikServer{URL: url}

				// For server-level comments, only use server comment (not service group comment)
				if serverInfo.Comment != "" {
					traefiktServer.Comment = serverInfo.Comment
				}

				// For service-level comment, use add serviceGroup comment first, then bind serviceGroup comment
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

// GenerateMappingConfig generates the mapping configuration
func GenerateMappingConfig(vservers []VServerInfo, serviceGroupDefs []ServiceGroupDef, serviceGroups []ServiceGroup) MappingConfig {
	var entries []MappingEntry

	// Create a map of service group definitions for comment lookup
	serviceGroupDefMap := make(map[string]ServiceGroupDef)
	for _, sgDef := range serviceGroupDefs {
		serviceGroupDefMap[sgDef.Name] = sgDef
	}

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

		// Priority 1: Check for add serviceGroup comment
		if sgDef, exists := serviceGroupDefMap[vserver.Name]; exists && sgDef.Comment != "" {
			comment = sgDef.Comment
		}

		// Priority 2: Check for bind serviceGroup comment (if no add comment found)
		if comment == "" {
			if groups, exists := serviceGroupsByVServer[vserver.Name]; exists {
				// Use the first non-empty comment found
				for _, group := range groups {
					if group.Comment != "" {
						comment = group.Comment
						break
					}
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
