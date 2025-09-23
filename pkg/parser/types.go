package parser

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
	Name              string
	ServerName        string
	Port              string
	Comment           string
	Disabled          bool
	Ratio             int
	LoadBalancingMode string
}

// ServiceGroupDef represents a service group definition from add command
type ServiceGroupDef struct {
	Name     string
	Protocol string
	Comment  string
}

// VServerBinding represents a bind lb vserver command that binds a service to a vserver
type VServerBinding struct {
	VServerName    string
	ServiceName    string // Can be empty for policy-only bindings
	PolicyName     string
	Priority       string
	GotoExpression string
	Type           string
	Comment        string
}

// TraefikService represents a Traefik service configuration
type TraefikService struct {
	LoadBalancer      TraefikLoadBalancer `yaml:"loadBalancer"`
	Comment           string              `yaml:"-"` // Service-level comment (not serialized)
	LoadBalancingMode string              `yaml:"-"` // Load balancing mode comment
}

// TraefikLoadBalancer represents the load balancer configuration
type TraefikLoadBalancer struct {
	Servers []TraefikServer `yaml:"servers"`
}

// TraefikServer represents a server in the load balancer
type TraefikServer struct {
	URL      string `yaml:"url"`
	Comment  string `yaml:"-"` // Don't include in YAML output
	Disabled bool   `yaml:"-"` // Don't include in YAML output
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
