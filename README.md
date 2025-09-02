# L7traefik

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
