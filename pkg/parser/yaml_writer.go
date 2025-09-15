package parser

import (
	"fmt"
	"io"
)

// WriteTraefikConfigWithComments writes the Traefik config to the writer with YAML comments
func WriteTraefikConfigWithComments(w io.Writer, config TraefikConfig) error {
	// Write the beginning of the YAML
	fmt.Fprintf(w, "http:\n")
	fmt.Fprintf(w, "  services:\n")

	// Write each service with comments
	for serviceName, service := range config.HTTP.Services {
		// Write service-level comment on a new line before the service
		if service.Comment != "" {
			fmt.Fprintf(w, "    # %s\n", service.Comment)
		}

		fmt.Fprintf(w, "    %s:\n", serviceName)
		fmt.Fprintf(w, "      loadBalancer:\n")
		fmt.Fprintf(w, "        servers:\n")

		for _, server := range service.LoadBalancer.Servers {
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
	for _, entry := range config.Entries {
		if entry.Comment != "" {
			fmt.Fprintf(w, "# %s\n", entry.Comment)
			fmt.Fprintf(w, "\"%s\": \"%s\"\n", entry.Key, entry.Value)
		} else {
			fmt.Fprintf(w, "\"%s\": \"%s\"\n", entry.Key, entry.Value)
		}
	}
	return nil
}
