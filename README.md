# Traefik7

An parser to extract L7 settings to Traefik services🚀🚀🚀

## Build

```bash
go build -o traefik7 .
```

## Usage

```bash
./traefik7 <input-file>
```

The tool parses L7 load balancer configuration files and generates:
- Traefik loadBalancer services configuration in YAML format
- Mapping rules with @nacoscs suffix in YAML format

Both files are generated in a timestamp-named directory (format: YYYYMMDDHHMM) in the current working directory.

## Example

# L7traefik

A complete Go-based command line tool that parses L7 load balancer configuration files and generates Traefik-compatible YAML configurations.

## Citrix Load Balancer Concept Hierarchy

This tool parses Citrix/NetScaler load balancer configurations and converts them to Traefik format. Understanding the Citrix concept hierarchy is essential for proper configuration migration.

### Citrix Concept Hierarchy

```mermaid
flowchart TB
    subgraph Citrix_Infrastructure["Citrix Load Balancer Infrastructure"]
        VServer1[Virtual Server<br/>webapp-vs<br/>HTTP 192.168.1.100:80]
        VServer2[Virtual Server<br/>api-vs<br/>HTTPS 192.168.1.101:443]
        
        ServiceGroup1[Service Group<br/>webapp<br/>HTTP]
        ServiceGroup2[Service Group<br/>api-backend<br/>HTTPS]
        
        subgraph Physical_Servers["Physical Server Pool"]
            Server1[Server<br/>web01<br/>10.1.2.121]
            Server2[Server<br/>web02<br/>10.1.2.122]
            Server3[Server<br/>api01<br/>10.1.2.131]
        end
        
        subgraph Server_Ports["Server:Port Bindings"]
            Port1[web01:80]
            Port2[web01:8080]
            Port3[web02:80]
            Port4[api01:443]
            Port5[api01:8443]
        end
    end
    
    %% Virtual Server to Service Group bindings
    VServer1 -.->|"bind lb vserver<br/>webapp-vs webapp"| ServiceGroup1
    VServer2 -.->|"bind lb vserver<br/>api-vs api-backend"| ServiceGroup2
    
    %% Service Group to Server:Port bindings
    ServiceGroup1 -.->|"bind serviceGroup<br/>webapp web01 80"| Port1
    ServiceGroup1 -.->|"bind serviceGroup<br/>webapp web02 80"| Port3
    ServiceGroup2 -.->|"bind serviceGroup<br/>api-backend api01 443"| Port4
    
    %% Server to Port relationships (one server can have multiple ports)
    Server1 --> Port1
    Server1 --> Port2
    Server2 --> Port3
    Server3 --> Port4
    Server3 --> Port5
    
    %% Styling
    classDef vserver fill:#e3f2fd,stroke:#1976d2,stroke-width:2px
    classDef servicegroup fill:#f3e5f5,stroke:#7b1fa2,stroke-width:2px
    classDef server fill:#e8f5e8,stroke:#388e3c,stroke-width:2px
    classDef port fill:#fff3e0,stroke:#f57c00,stroke-width:2px
    
    class VServer1,VServer2 vserver
    class ServiceGroup1,ServiceGroup2 servicegroup
    class Server1,Server2,Server3 server
    class Port1,Port2,Port3,Port4,Port5 port
```

### Citrix Component Hierarchy

1. **Server** - Physical backend servers that handle actual requests
   - Defined with: `add server <name> <ip>`
   - Example: `add server web01 10.1.2.121`
   - **Key Point**: One server can listen on multiple ports

2. **Service Group** - Logical grouping of servers providing the same service
   - Defined with: `add serviceGroup <name> <protocol>`
   - Bound with: `bind serviceGroup <name> <server> <port>`
   - **Key Point**: Groups server:port combinations, not just servers

3. **Virtual Server (VServer)** - External-facing load balancer endpoint
   - Defined with: `add lb vserver <name> <protocol> <ip> <port>`
   - Example: `add lb vserver webapp-vs HTTP 192.168.1.100 80`
   - **Key Point**: Can be bound to zero or more service groups

4. **VServer Binding** - Connection between virtual servers and service groups
   - Defined with: `bind lb vserver <vserver> <servicegroup>`
   - **Key Point**: Creates the routing from external endpoint to server pool

### Relationships Summary

- **1 Virtual Server** → **0 or more Service Groups** (via bind lb vserver)
- **1 Service Group** → **0 or more Server:Port combinations** (via bind serviceGroup)
- **1 Server** → **Multiple Ports** (each server can listen on different ports)
- **Traffic Flow**: Client → Virtual Server → Service Group → Server:Port

### Conversion Process

The tool transforms Citrix configurations into Traefik-compatible formats:

- **Service Groups + Servers** → **Traefik Services with LoadBalancer**
- **Virtual Servers** → **Mapping entries (IP:Port → Service@nacoscs)**

## Features

The tool accepts L7 setting files containing commands like:

- `add server <name> <ip>` - Define server mappings
- `add lb vserver <name> <protocol> <ip> <port>` - Define virtual servers
- `bind serviceGroup <name> <server> <port>` - Bind servers to service groups

And generates two output files in a timestamp-named directory:

- `traefik-services.yaml` - Traefik HTTP services configuration with loadBalancer settings
- `mapping.yaml` - IP:port to service@nacoscs mappings

## Build

```bash
go build -o traefik7 .
```

## Usage

```bash
./traefik7 <input-file>
```

## Example

Given this input:

```
add server highavailableapplicationap001 10.1.2.121
add server highavailableapplicationap002 10.1.2.122
add server highavailableapplicationap003 10.1.2.123
add lb vserver targetapplicationserver:8351 HTTP 10.0.28.130 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap001 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap002 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap003 8351
```

The tool generates:

**traefik-services.yaml:**
```yaml
http:
  services:
    targetapplicationserver:8351:
      loadBalancer:
        servers:
          - url: http://10.1.2.121:8351
          - url: http://10.1.2.122:8351
          - url: http://10.1.2.123:8351
```

**mapping.yaml:**
```yaml
10.0.28.130:8351: targetapplicationserver:8351@nacoscs
```

Generated output:
- `traefik-services.yaml`: Traefik services configuration
- `mapping.yaml`: Service mapping rules
