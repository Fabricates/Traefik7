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

Input file containing L7 settings:
```
add server highavailableapplicationap001 10.1.2.121
add server highavailableapplicationap002 10.1.2.122
add server highavailableapplicationap003 10.1.2.123
add lb vserver targetapplicationserver:8351 HTTP 10.0.28.130 8351 -persistenceType COOKIEINSERT -persistenceBackup SOURCEIP -cltTimeout 180
bind lb vserver targetapplicationserver:8351 targetapplicationserver:8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap001 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap002 8351
bind serviceGroup targetapplicationserver:8351 highavailableapplicationap003 8351
bind serviceGroup targetapplicationserver:8351 -monitorName tcp
```

Generated output:
- `traefik-services.yaml`: Traefik services configuration
- `mapping.yaml`: Service mapping rules
