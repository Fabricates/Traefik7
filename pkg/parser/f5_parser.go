package parser

import (
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// F5 configuration structures for simple parser
type F5NodeSimple struct {
	Name    string
	Address string
}

type F5PoolSimple struct {
	Name        string
	Description string
	Members     []F5PoolMemberSimple
	Monitor     string
}

type F5PoolMemberSimple struct {
	Address string
	Port    int
}

type F5VirtualSimple struct {
	Name        string
	Description string
	Destination string
	Pool        string
	Profiles    []string
}

// ParseF5SettingsFromFileSimple parses F5 configuration from a file using simple approach
func ParseF5SettingsFromFileSimple(filename string) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return ParseF5ConfigSimple(string(content))
}

// ParseF5SettingsFromReaderSimple parses F5 configuration from an io.Reader using simple approach
func ParseF5SettingsFromReaderSimple(reader io.Reader) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return ParseF5ConfigSimple(string(content))
}

// ParseF5ConfigSimple parses F5 configuration using simple regex approach
func ParseF5ConfigSimple(content string) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	// Parse nodes, pools, and virtuals using simple regex approach
	nodes := parseF5NodesSimple(content)
	pools := parseF5PoolsSimple(content)
	virtuals := parseF5VirtualsSimple(content)

	// Convert to Citrix-compatible format
	servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings := convertF5ToTraefikFormat(nodes, pools, virtuals)

	return servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings, nil
}

// Simple regex-based parsers that extract key information line by line
func parseF5NodesSimple(content string) []F5NodeSimple {
	var nodes []F5NodeSimple

	// Find all ltm node blocks
	nodePattern := regexp.MustCompile(`(?s)ltm node (/Common/[^\s]+)\s*\{([^}]*)\}`)
	matches := nodePattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		nodeName := match[1]
		nodeBlock := match[2]

		// Extract address from the block
		addressPattern := regexp.MustCompile(`address\s+([^\s\n]+)`)
		if addrMatch := addressPattern.FindStringSubmatch(nodeBlock); addrMatch != nil {
			nodes = append(nodes, F5NodeSimple{
				Name:    nodeName,
				Address: addrMatch[1],
			})
		}
	}

	return nodes
}

func parseF5PoolsSimple(content string) []F5PoolSimple {
	var pools []F5PoolSimple

	// Find all ltm pool blocks - handle nested braces carefully
	lines := strings.Split(content, "\n")
	var currentPool *F5PoolSimple
	var braceLevel int
	var inPool bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for pool start
		poolPattern := regexp.MustCompile(`^ltm pool (/Common/[^\s]+)\s*\{`)
		if match := poolPattern.FindStringSubmatch(trimmed); match != nil {
			currentPool = &F5PoolSimple{Name: match[1]}
			inPool = true
			braceLevel = 1
			continue
		}

		if !inPool || currentPool == nil {
			continue
		}

		// Count braces to track nesting
		braceLevel += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

		// Extract information from within the pool block
		if braceLevel > 0 {
			// Description
			if descMatch := regexp.MustCompile(`description\s+(.+)`).FindStringSubmatch(trimmed); descMatch != nil {
				currentPool.Description = descMatch[1]
			}

			// Pool member
			if memberMatch := regexp.MustCompile(`(/Common/\d{1,3}(?:\.\d{1,3}){3}):(\d+)\s*\{`).FindStringSubmatch(trimmed); memberMatch != nil {
				port, _ := strconv.Atoi(memberMatch[2])
				currentPool.Members = append(currentPool.Members, F5PoolMemberSimple{
					Address: strings.TrimPrefix(memberMatch[1], "/Common/"),
					Port:    port,
				})
			}

			// Monitor
			if monMatch := regexp.MustCompile(`monitor\s+(.+)`).FindStringSubmatch(trimmed); monMatch != nil {
				currentPool.Monitor = monMatch[1]
			}
		}

		// Pool block ended
		if braceLevel == 0 {
			pools = append(pools, *currentPool)
			currentPool = nil
			inPool = false
		}
	}

	return pools
}

func parseF5VirtualsSimple(content string) []F5VirtualSimple {
	var virtuals []F5VirtualSimple

	// Find all ltm virtual blocks
	lines := strings.Split(content, "\n")
	var currentVirtual *F5VirtualSimple
	var braceLevel int
	var inVirtual bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for virtual start
		virtualPattern := regexp.MustCompile(`^ltm virtual (/Common/[^\s]+)\s*\{`)
		if match := virtualPattern.FindStringSubmatch(trimmed); match != nil {
			currentVirtual = &F5VirtualSimple{Name: match[1]}
			inVirtual = true
			braceLevel = 1
			continue
		}

		if !inVirtual || currentVirtual == nil {
			continue
		}

		// Count braces to track nesting
		braceLevel += strings.Count(trimmed, "{") - strings.Count(trimmed, "}")

		// Extract information from within the virtual block
		if braceLevel > 0 {
			// Description
			if descMatch := regexp.MustCompile(`description\s+(.+)`).FindStringSubmatch(trimmed); descMatch != nil {
				currentVirtual.Description = descMatch[1]
			}

			// Destination (VIP:port)
			if destMatch := regexp.MustCompile(`destination\s+(/Common/[^:]+):(\d+)`).FindStringSubmatch(trimmed); destMatch != nil {
				currentVirtual.Destination = strings.TrimPrefix(destMatch[1], "/Common/") + ":" + destMatch[2]
			}

			// Pool
			if poolMatch := regexp.MustCompile(`pool\s+(.+)`).FindStringSubmatch(trimmed); poolMatch != nil {
				currentVirtual.Pool = poolMatch[1]
			}
		}

		// Virtual block ended
		if braceLevel == 0 {
			virtuals = append(virtuals, *currentVirtual)
			currentVirtual = nil
			inVirtual = false
		}
	}

	return virtuals
}

func convertF5ToTraefikFormat(nodes []F5NodeSimple, pools []F5PoolSimple, virtuals []F5VirtualSimple) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding) {
	var servers []ServerInfo
	var vservers []VServerInfo
	var serviceGroupDefs []ServiceGroupDef
	var serviceGroups []ServiceGroup
	var vserverBindings []VServerBinding

	// Create a map to track unique server IP addresses and their names
	serverMap := make(map[string]bool)
	ipToServerName := make(map[string]string)

	// Convert F5 nodes to ServerInfo
	for _, node := range nodes {
		cleanName := strings.TrimPrefix(node.Name, "/Common/")
		servers = append(servers, ServerInfo{
			Name:    cleanName,
			IP:      node.Address,
			Comment: "F5 Node",
		})
		serverMap[node.Address] = true
		ipToServerName[node.Address] = cleanName
	}

	// Create a map of pools for quick lookup
	poolMap := make(map[string]F5PoolSimple)
	for _, pool := range pools {
		poolMap[pool.Name] = pool
	}

	// Convert F5 virtual servers to VServerInfo and create service groups using virtual server names
	for _, virtual := range virtuals {
		cleanVirtualName := strings.TrimPrefix(virtual.Name, "/Common/")

		if virtual.Destination != "" {
			// Split destination IP:port
			parts := strings.Split(virtual.Destination, ":")
			if len(parts) == 2 {
				vservers = append(vservers, VServerInfo{
					Name:     cleanVirtualName,
					Protocol: "HTTP", // Default to HTTP for F5 virtuals
					IP:       parts[0],
					Port:     parts[1],
				})

				// If this virtual server has a pool, create service group using virtual server name
				if virtual.Pool != "" {
					if pool, exists := poolMap[virtual.Pool]; exists {
						// Create service group definition using virtual server name
						serviceGroupDefs = append(serviceGroupDefs, ServiceGroupDef{
							Name:     cleanVirtualName, // Use virtual server name instead of pool name
							Protocol: "HTTP",
							Comment:  pool.Description,
						})

						// Create service group bindings for each pool member
						for _, member := range pool.Members {
							// Determine the server name to use
							var serverName string
							if existingName, exists := ipToServerName[member.Address]; exists {
								// Use the existing node name
								serverName = existingName
							} else {
								// Ensure we have a server entry for this IP
								if !serverMap[member.Address] && member.Address != "" {
									servers = append(servers, ServerInfo{
										Name:    member.Address, // Use IP as name if no node definition exists
										IP:      member.Address,
										Comment: "Auto-generated from F5 pool member",
									})
									serverMap[member.Address] = true
									ipToServerName[member.Address] = member.Address
								}
								serverName = member.Address
							}

							serviceGroups = append(serviceGroups, ServiceGroup{
								Name:       cleanVirtualName, // Use virtual server name
								ServerName: serverName,
								Port:       strconv.Itoa(member.Port),
								Comment:    pool.Description,
							})
						}

						// Create vserver binding using virtual server name
						vserverBindings = append(vserverBindings, VServerBinding{
							VServerName: cleanVirtualName,
							ServiceName: cleanVirtualName, // Service group also uses virtual server name
							Comment:     virtual.Description,
						})
					}
				} else {
					// Virtual server without pool - create empty service group
					serviceGroupDefs = append(serviceGroupDefs, ServiceGroupDef{
						Name:     cleanVirtualName,
						Protocol: "HTTP",
						Comment:  "F5 Virtual Server without pool",
					})
				}
			}
		}
	}

	return servers, vservers, serviceGroupDefs, serviceGroups, vserverBindings
}

// Legacy functions for backward compatibility (if needed)
// Keep the old complex parser functions as backup, but use simple parser by default

// ParseF5Settings parses F5 configuration file (backward compatibility - now uses simple parser)
func ParseF5Settings(filename string) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	return ParseF5SettingsFromFileSimple(filename)
}

// ParseF5SettingsFromReader parses F5 configuration from reader (backward compatibility - now uses simple parser)
func ParseF5SettingsFromReader(reader io.Reader) ([]ServerInfo, []VServerInfo, []ServiceGroupDef, []ServiceGroup, []VServerBinding, error) {
	return ParseF5SettingsFromReaderSimple(reader)
}
