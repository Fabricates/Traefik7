package parser

import (
	"fmt"
	"io"
	"sort"
)

// WriteTraefikConfigWithComments writes the Traefik config to the writer with YAML comments
func WriteTraefikConfigWithComments(w io.Writer, config TraefikConfig) error {
	// Write the beginning of the YAML
	fmt.Fprintf(w, "http:\n")
	fmt.Fprintf(w, "  services:\n")

	// Get service names and sort them
	serviceNames := make([]string, 0, len(config.HTTP.Services))
	for serviceName := range config.HTTP.Services {
		serviceNames = append(serviceNames, serviceName)
	}
	sort.Strings(serviceNames)

	// Write each service with comments in sorted order
	for _, serviceName := range serviceNames {
		service := config.HTTP.Services[serviceName]
		// Write service-level comment on a new line before the service
		if service.Comment != "" {
			fmt.Fprintf(w, "    # %s\n", service.Comment)
		}

		fmt.Fprintf(w, "    %s:\n", serviceName)
		fmt.Fprintf(w, "      loadBalancer:\n")
		fmt.Fprintf(w, "        servers:\n")

		// Sort servers by URL
		servers := make([]TraefikServer, len(service.LoadBalancer.Servers))
		copy(servers, service.LoadBalancer.Servers)
		sort.Slice(servers, func(i, j int) bool {
			return servers[i].URL < servers[j].URL
		})

		for _, server := range servers {
			if server.Comment != "" {
				fmt.Fprintf(w, "          # %s\n", server.Comment)
				fmt.Fprintf(w, "          - url: %s\n", server.URL)
			} else {
				fmt.Fprintf(w, "          - url: %s\n", server.URL)
			}
		}
	}

	return nil
}

// WriteMappingConfigWithComments writes the mapping config to the writer with YAML comments
func WriteMappingConfigWithComments(w io.Writer, config MappingConfig) error {
	// Sort entries by key
	entries := make([]MappingEntry, len(config.Entries))
	copy(entries, config.Entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	for _, entry := range entries {
		if entry.Comment != "" {
			fmt.Fprintf(w, "# %s\n", entry.Comment)
			fmt.Fprintf(w, "\"%s\": \"%s\"\n", entry.Key, entry.Value)
		} else {
			fmt.Fprintf(w, "\"%s\": \"%s\"\n", entry.Key, entry.Value)
		}
	}
	return nil
}
