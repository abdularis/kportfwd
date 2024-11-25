# Local Service Development For Kubernetes

kportfwd

Used in condition where we don't have deployed vpn inside cluster for local development or other
tools for helping local development for kubernetes services.

No setup on cluster and doesn't leave any residual.

## Install

### Build

You'll need `docker` to build the binary from source code.

Build for specific OS platform

```
make build-macos
# or
make build-linux
```

To install the binary on Go path directory

```
make install-macos
```

## Usage

Run help for more details

```bash
kportfwd --help
```

```

```

## Config YAML References



```yaml
target:
  pod:
    labelSelector: "app=backend,!job-name"
    container: service
    namespace: default
forwards:
  - name: ""
    localAddr: "localhost:8000"
    sourceAddr: ""
    targetAddr: "{{REDIS_HOST}}:{{REDIS_PORT}}"
```

## Relay Agent

Relay Agent: Is a program running inside target pod container to relay network traffic to any domain accessible from that container and exposed them into different ports so that is forwardable by `kubectl` outside cluster.

### Relay Agent Controller API Spec

This APIs is used to communicate with the relay agent on target pod.

API hosted on port: 8181

#### GET /ping

Should be called periodically to indicate there is a client using it, otherwise relay-agent will exit automatically.

Response

```text
pong
```

#### GET /relays

Response:

```json
{
    "relays": [
        {
            "sourceAddr": "",
            "targetAddr": "",
            "errorMsg": ""
        }
    ]
}
```