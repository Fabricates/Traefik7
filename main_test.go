package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestParseL7Settings(t *testing.T) {
	tests := []struct {
		name                  string
		content               string
		expectedServers       []ServerInfo
		expectedVServers      []VServerInfo
		expectedServiceGroups []ServiceGroup
		expectError           bool
	}{
		{
			name: "basic configuration",
			content: `add server web01 192.168.1.10
add server web02 192.168.1.11
add lb vserver webapp:80 HTTP 10.0.1.100 80
bind serviceGroup webapp:80 web01 80
bind serviceGroup webapp:80 web02 80`,
			expectedServers: []ServerInfo{
				{Name: "web01", IP: "192.168.1.10"},
				{Name: "web02", IP: "192.168.1.11"},
			},
			expectedVServers: []VServerInfo{
				{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
			},
			expectedServiceGroups: []ServiceGroup{
				{Name: "webapp:80", ServerName: "web01", Port: "80"},
				{Name: "webapp:80", ServerName: "web02", Port: "80"},
			},
			expectError: false,
		},
		{
			name: "configuration with comments and empty lines",
			content: `# This is a comment
add server app01 10.1.2.121

# Another comment
add lb vserver app:8080 HTTP 10.0.28.130 8080
bind serviceGroup app:8080 app01 8080`,
			expectedServers: []ServerInfo{
				{Name: "app01", IP: "10.1.2.121"},
			},
			expectedVServers: []VServerInfo{
				{Name: "app:8080", Protocol: "HTTP", IP: "10.0.28.130", Port: "8080"},
			},
			expectedServiceGroups: []ServiceGroup{
				{Name: "app:8080", ServerName: "app01", Port: "8080"},
			},
			expectError: false,
		},
		{
			name: "configuration with monitor bindings (should be skipped)",
			content: `add server web01 192.168.1.10
add lb vserver webapp:80 HTTP 10.0.1.100 80
bind serviceGroup webapp:80 web01 80
bind serviceGroup webapp:80 -monitorName tcp`,
			expectedServers: []ServerInfo{
				{Name: "web01", IP: "192.168.1.10"},
			},
			expectedVServers: []VServerInfo{
				{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
			},
			expectedServiceGroups: []ServiceGroup{
				{Name: "webapp:80", ServerName: "web01", Port: "80"},
			},
			expectError: false,
		},
		{
			name: "empty configuration",
			content: `# Only comments
# and empty lines

`,
			expectedServers:       []ServerInfo{},
			expectedVServers:      []VServerInfo{},
			expectedServiceGroups: []ServiceGroup{},
			expectError:           false,
		},
		{
			name: "complex configuration with multiple services",
			content: `add server highavailableapplicationap001 10.1.2.121
add server highavailableapplicationap002 10.1.2.122
add server highavailableapplicationap003 10.1.2.123
add server dbserver01 10.1.3.10
add lb vserver targetapplicationserver:8351 HTTP 10.0.28.130 8351
add lb vserver database:3306 TCP 10.0.28.131 3306
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap001 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap002 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap003 8351
bind serviceGroup database:3306 dbserver01 3306`,
			expectedServers: []ServerInfo{
				{Name: "highavailableapplicationap001", IP: "10.1.2.121"},
				{Name: "highavailableapplicationap002", IP: "10.1.2.122"},
				{Name: "highavailableapplicationap003", IP: "10.1.2.123"},
				{Name: "dbserver01", IP: "10.1.3.10"},
			},
			expectedVServers: []VServerInfo{
				{Name: "targetapplicationserver:8351", Protocol: "HTTP", IP: "10.0.28.130", Port: "8351"},
				{Name: "database:3306", Protocol: "TCP", IP: "10.0.28.131", Port: "3306"},
			},
			expectedServiceGroups: []ServiceGroup{
				{Name: "targetapplicationserver:8351", ServerName: "highavailableapplicationap001", Port: "8351"},
				{Name: "targetapplicationserver:8351", ServerName: "highavailableapplicationap002", Port: "8351"},
				{Name: "targetapplicationserver:8351", ServerName: "highavailableapplicationap003", Port: "8351"},
				{Name: "database:3306", ServerName: "dbserver01", Port: "3306"},
			},
			expectError: false,
		},
		{
			name: "configuration with malformed lines (should be ignored)",
			content: `add server web01 192.168.1.10
malformed line that should be ignored
add lb vserver webapp:80 HTTP 10.0.1.100 80
another malformed line
bind serviceGroup webapp:80 web01 80`,
			expectedServers: []ServerInfo{
				{Name: "web01", IP: "192.168.1.10"},
			},
			expectedVServers: []VServerInfo{
				{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
			},
			expectedServiceGroups: []ServiceGroup{
				{Name: "webapp:80", ServerName: "web01", Port: "80"},
			},
			expectError: false,
		},
		{
			name: "configuration with extra whitespace",
			content: `   add server web01 192.168.1.10   
	add lb vserver webapp:80 HTTP 10.0.1.100 80	
     bind serviceGroup webapp:80 web01 80     `,
			expectedServers: []ServerInfo{
				{Name: "web01", IP: "192.168.1.10"},
			},
			expectedVServers: []VServerInfo{
				{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
			},
			expectedServiceGroups: []ServiceGroup{
				{Name: "webapp:80", ServerName: "web01", Port: "80"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpfile, err := ioutil.TempFile("", "test-l7-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpfile.Name())

			// Write test content
			if _, err := tmpfile.Write([]byte(tt.content)); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpfile.Close()

			// Parse the file
			servers, vservers, serviceGroups, err := parseL7Settings(tmpfile.Name())

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Compare servers
			if len(servers) != len(tt.expectedServers) {
				t.Errorf("Servers length mismatch.\nExpected: %d\nGot: %d", len(tt.expectedServers), len(servers))
			} else if len(servers) > 0 && !reflect.DeepEqual(servers, tt.expectedServers) {
				t.Errorf("Servers mismatch.\nExpected: %+v\nGot: %+v", tt.expectedServers, servers)
			}

			// Compare vservers
			if len(vservers) != len(tt.expectedVServers) {
				t.Errorf("VServers length mismatch.\nExpected: %d\nGot: %d", len(tt.expectedVServers), len(vservers))
			} else if len(vservers) > 0 && !reflect.DeepEqual(vservers, tt.expectedVServers) {
				t.Errorf("VServers mismatch.\nExpected: %+v\nGot: %+v", tt.expectedVServers, vservers)
			}

			// Compare service groups
			if len(serviceGroups) != len(tt.expectedServiceGroups) {
				t.Errorf("ServiceGroups length mismatch.\nExpected: %d\nGot: %d", len(tt.expectedServiceGroups), len(serviceGroups))
			} else if len(serviceGroups) > 0 && !reflect.DeepEqual(serviceGroups, tt.expectedServiceGroups) {
				t.Errorf("ServiceGroups mismatch.\nExpected: %+v\nGot: %+v", tt.expectedServiceGroups, serviceGroups)
			}
		})
	}
}

func TestParseL7SettingsFileNotFound(t *testing.T) {
	_, _, _, err := parseL7Settings("nonexistent-file.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file, but got none")
	}
}

func TestGenerateTraefikConfig(t *testing.T) {
	servers := []ServerInfo{
		{Name: "web01", IP: "192.168.1.10"},
		{Name: "web02", IP: "192.168.1.11"},
		{Name: "api01", IP: "192.168.1.20"},
	}

	vservers := []VServerInfo{
		{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
		{Name: "api:8080", Protocol: "HTTP", IP: "10.0.1.101", Port: "8080"},
	}

	serviceGroups := []ServiceGroup{
		{Name: "webapp:80", ServerName: "web01", Port: "80"},
		{Name: "webapp:80", ServerName: "web02", Port: "80"},
		{Name: "api:8080", ServerName: "api01", Port: "8080"},
	}

	config := generateTraefikConfig(servers, vservers, serviceGroups)

	// Check if the config has the expected structure
	if config.HTTP.Services == nil {
		t.Error("Expected services to be initialized")
	}

	// Check webapp service
	webappService, exists := config.HTTP.Services["webapp:80"]
	if !exists {
		t.Error("Expected webapp:80 service to exist")
	}

	expectedWebappServers := []TraefikServer{
		{URL: "http://192.168.1.10:80"},
		{URL: "http://192.168.1.11:80"},
	}

	if !reflect.DeepEqual(webappService.LoadBalancer.Servers, expectedWebappServers) {
		t.Errorf("Webapp servers mismatch.\nExpected: %+v\nGot: %+v",
			expectedWebappServers, webappService.LoadBalancer.Servers)
	}

	// Check api service
	apiService, exists := config.HTTP.Services["api:8080"]
	if !exists {
		t.Error("Expected api:8080 service to exist")
	}

	expectedAPIServers := []TraefikServer{
		{URL: "http://192.168.1.20:8080"},
	}

	if !reflect.DeepEqual(apiService.LoadBalancer.Servers, expectedAPIServers) {
		t.Errorf("API servers mismatch.\nExpected: %+v\nGot: %+v",
			expectedAPIServers, apiService.LoadBalancer.Servers)
	}
}

func TestGenerateTraefikConfigWithUnknownServer(t *testing.T) {
	servers := []ServerInfo{
		{Name: "web01", IP: "192.168.1.10"},
	}

	vservers := []VServerInfo{
		{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
	}

	serviceGroups := []ServiceGroup{
		{Name: "webapp:80", ServerName: "web01", Port: "80"},
		{Name: "webapp:80", ServerName: "unknown-server", Port: "80"}, // Unknown server
	}

	config := generateTraefikConfig(servers, vservers, serviceGroups)

	webappService, exists := config.HTTP.Services["webapp:80"]
	if !exists {
		t.Error("Expected webapp:80 service to exist")
	}

	// Should only include the known server
	expectedServers := []TraefikServer{
		{URL: "http://192.168.1.10:80"},
	}

	if !reflect.DeepEqual(webappService.LoadBalancer.Servers, expectedServers) {
		t.Errorf("Expected only known servers.\nExpected: %+v\nGot: %+v",
			expectedServers, webappService.LoadBalancer.Servers)
	}
}

func TestGenerateMappingConfig(t *testing.T) {
	vservers := []VServerInfo{
		{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
		{Name: "api:8080", Protocol: "HTTP", IP: "10.0.1.101", Port: "8080"},
		{Name: "database:3306", Protocol: "TCP", IP: "10.0.1.102", Port: "3306"},
	}

	serviceGroups := []ServiceGroup{
		{Name: "webapp:80", ServerName: "web01", Port: "80", Comment: "Web cluster"},
		{Name: "api:8080", ServerName: "api01", Port: "8080", Comment: ""},
	}

	mapping := generateMappingConfig(vservers, serviceGroups)

	expected := MappingConfig{
		Entries: []MappingEntry{
			{Key: "10.0.1.100:80", Value: "webapp:80@nacoscs", Comment: "Web cluster"},
			{Key: "10.0.1.101:8080", Value: "api:8080@nacoscs", Comment: ""},
			{Key: "10.0.1.102:3306", Value: "database:3306@nacoscs", Comment: ""},
		},
	}

	if len(mapping.Entries) != len(expected.Entries) {
		t.Errorf("Expected %d mapping entries, got %d", len(expected.Entries), len(mapping.Entries))
	}

	for i, entry := range mapping.Entries {
		if entry.Key != expected.Entries[i].Key {
			t.Errorf("Expected key %s, got %s", expected.Entries[i].Key, entry.Key)
		}
		if entry.Value != expected.Entries[i].Value {
			t.Errorf("Expected value %s, got %s", expected.Entries[i].Value, entry.Value)
		}
		if entry.Comment != expected.Entries[i].Comment {
			t.Errorf("Expected comment %s, got %s", expected.Entries[i].Comment, entry.Comment)
		}
	}
}

func TestWriteYAMLFile(t *testing.T) {
	// Create temporary directory
	tmpdir, err := ioutil.TempDir("", "test-yaml-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Test data
	testData := map[string]interface{}{
		"http": map[string]interface{}{
			"services": map[string]interface{}{
				"test-service": map[string]interface{}{
					"loadBalancer": map[string]interface{}{
						"servers": []map[string]string{
							{"url": "http://192.168.1.10:80"},
						},
					},
				},
			},
		},
	}

	// Write YAML file
	filename := filepath.Join(tmpdir, "test.yaml")
	err = writeYAMLFile(filename, testData)
	if err != nil {
		t.Fatalf("Failed to write YAML file: %v", err)
	}

	// Read and verify the file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read YAML file: %v", err)
	}

	// Parse the YAML back
	var parsed map[string]interface{}
	err = yaml.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Verify the structure exists
	http, ok := parsed["http"].(map[string]interface{})
	if !ok {
		t.Error("Expected 'http' section in YAML")
	}

	services, ok := http["services"].(map[string]interface{})
	if !ok {
		t.Error("Expected 'services' section in YAML")
	}

	if _, exists := services["test-service"]; !exists {
		t.Error("Expected 'test-service' to exist in services")
	}
}

func TestWriteYAMLFileInvalidPath(t *testing.T) {
	// Try to write to an invalid path
	err := writeYAMLFile("/invalid/path/file.yaml", map[string]string{"test": "data"})
	if err == nil {
		t.Error("Expected error when writing to invalid path, but got none")
	}
}

func TestIntegration(t *testing.T) {
	// Create temporary directory for integration test
	tmpdir, err := ioutil.TempDir("", "test-integration-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Create input file
	inputContent := `add server web01 192.168.1.10
add server web02 192.168.1.11
add lb vserver webapp:80 HTTP 10.0.1.100 80
bind serviceGroup webapp:80 web01 80
bind serviceGroup webapp:80 web02 80`

	inputFile := filepath.Join(tmpdir, "input.txt")
	err = ioutil.WriteFile(inputFile, []byte(inputContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	// Parse input
	servers, vservers, serviceGroups, err := parseL7Settings(inputFile)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	// Generate configurations
	traefik := generateTraefikConfig(servers, vservers, serviceGroups)
	mapping := generateMappingConfig(vservers, serviceGroups)

	// Create output directory with timestamp
	timestamp := time.Now().Format("200601021504")
	outputDir := filepath.Join(tmpdir, timestamp)
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	// Write output files
	traefiktFile := filepath.Join(outputDir, "traefik-services.yaml")
	err = writeYAMLFile(traefiktFile, traefik)
	if err != nil {
		t.Fatalf("Failed to write Traefik config: %v", err)
	}

	mappingFile := filepath.Join(outputDir, "mapping.yaml")
	err = writeYAMLFile(mappingFile, mapping)
	if err != nil {
		t.Fatalf("Failed to write mapping config: %v", err)
	}

	// Verify files exist and are readable
	if _, err := os.Stat(traefiktFile); os.IsNotExist(err) {
		t.Error("Traefik config file was not created")
	}

	if _, err := os.Stat(mappingFile); os.IsNotExist(err) {
		t.Error("Mapping config file was not created")
	}

	// Read and verify Traefik config
	traefiktData, err := ioutil.ReadFile(traefiktFile)
	if err != nil {
		t.Fatalf("Failed to read Traefik config: %v", err)
	}

	var traefiktParsed TraefikConfig
	err = yaml.Unmarshal(traefiktData, &traefiktParsed)
	if err != nil {
		t.Fatalf("Failed to parse Traefik config: %v", err)
	}

	// Verify Traefik config structure
	if traefiktParsed.HTTP.Services == nil {
		t.Error("Expected services in Traefik config")
	}

	service, exists := traefiktParsed.HTTP.Services["webapp:80"]
	if !exists {
		t.Error("Expected webapp:80 service in Traefik config")
	}

	expectedURLs := []string{"http://192.168.1.10:80", "http://192.168.1.11:80"}
	actualURLs := make([]string, len(service.LoadBalancer.Servers))
	for i, server := range service.LoadBalancer.Servers {
		actualURLs[i] = server.URL
	}

	for _, expectedURL := range expectedURLs {
		found := false
		for _, actualURL := range actualURLs {
			if expectedURL == actualURL {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected URL %s not found in Traefik config", expectedURL)
		}
	}

	// Read and verify mapping config
	mappingData, err := ioutil.ReadFile(mappingFile)
	if err != nil {
		t.Fatalf("Failed to read mapping config: %v", err)
	}

	var mappingParsed map[string]string
	err = yaml.Unmarshal(mappingData, &mappingParsed)
	if err != nil {
		t.Fatalf("Failed to parse mapping config: %v", err)
	}

	expectedMapping := "webapp:80@nacoscs"
	actualMapping, exists := mappingParsed["10.0.1.100:80"]
	if !exists {
		t.Error("Expected mapping for 10.0.1.100:80 not found")
	}

	if actualMapping != expectedMapping {
		t.Errorf("Expected mapping %s, got %s", expectedMapping, actualMapping)
	}
}

// Benchmark tests
func BenchmarkParseL7Settings(b *testing.B) {
	content := `add server web01 192.168.1.10
add server web02 192.168.1.11
add server web03 192.168.1.12
add lb vserver webapp:80 HTTP 10.0.1.100 80
bind serviceGroup webapp:80 web01 80
bind serviceGroup webapp:80 web02 80
bind serviceGroup webapp:80 web03 80`

	// Create temporary file
	tmpfile, err := ioutil.TempFile("", "bench-l7-*.txt")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		b.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpfile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, err := parseL7Settings(tmpfile.Name())
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

func BenchmarkGenerateTraefikConfig(b *testing.B) {
	servers := []ServerInfo{
		{Name: "web01", IP: "192.168.1.10"},
		{Name: "web02", IP: "192.168.1.11"},
		{Name: "web03", IP: "192.168.1.12"},
	}

	vservers := []VServerInfo{
		{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
	}

	serviceGroups := []ServiceGroup{
		{Name: "webapp:80", ServerName: "web01", Port: "80"},
		{Name: "webapp:80", ServerName: "web02", Port: "80"},
		{Name: "webapp:80", ServerName: "web03", Port: "80"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateTraefikConfig(servers, vservers, serviceGroups)
	}
}

func BenchmarkGenerateMappingConfig(b *testing.B) {
	vservers := []VServerInfo{
		{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
		{Name: "api:8080", Protocol: "HTTP", IP: "10.0.1.101", Port: "8080"},
		{Name: "database:3306", Protocol: "TCP", IP: "10.0.1.102", Port: "3306"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateMappingConfig(vservers, []ServiceGroup{})
	}
}

func TestGenerateTraefikConfigEmptyInputs(t *testing.T) {
	// Test with empty inputs
	config := generateTraefikConfig([]ServerInfo{}, []VServerInfo{}, []ServiceGroup{})

	if config.HTTP.Services == nil {
		t.Error("Expected services to be initialized even with empty inputs")
	}

	if len(config.HTTP.Services) != 0 {
		t.Errorf("Expected empty services map, got %d services", len(config.HTTP.Services))
	}
}

func TestGenerateTraefikConfigNoMatchingServers(t *testing.T) {
	// Test when service groups reference servers that don't exist
	servers := []ServerInfo{} // No servers defined

	vservers := []VServerInfo{
		{Name: "webapp:80", Protocol: "HTTP", IP: "10.0.1.100", Port: "80"},
	}

	serviceGroups := []ServiceGroup{
		{Name: "webapp:80", ServerName: "nonexistent-server", Port: "80"},
	}

	config := generateTraefikConfig(servers, vservers, serviceGroups)

	// Should not create any services since no servers match
	if len(config.HTTP.Services) != 0 {
		t.Errorf("Expected no services when no servers match, got %d services", len(config.HTTP.Services))
	}
}

func TestGenerateMappingConfigEmpty(t *testing.T) {
	mapping := generateMappingConfig([]VServerInfo{}, []ServiceGroup{})

	if len(mapping.Entries) != 0 {
		t.Errorf("Expected empty mapping, got %d entries", len(mapping.Entries))
	}
}
