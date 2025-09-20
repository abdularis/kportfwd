# kportfwd - Kubernetes Port Forwarding Tool

A simple and powerful tool for port forwarding to internal services and domain names accessible within a Kubernetes cluster, **without requiring any setup or modifications on the cluster itself**.

## 🎯 Use Cases

**Perfect for accessing cluster-internal resources locally:**

- **Database Access**: Connect to RDS instances or databases that are only accessible within your EKS/GKE cluster
- **Internal Services**: Access microservices that don't have public endpoints
- **Development & Debugging**: Test and debug applications that depend on cluster-internal resources

**When you DON'T need this tool:**
- If you already have a VPN solution that makes internal domain names available on your local machine

## 📦 Installation

### Option 1: Download Pre-built Binary
Download the latest binary from the [Release Page](https://github.com/abdularis/kportfwd/releases)

### Option 2: Build from Source
See the [Build from Source](#-build-from-source) section below.

## 🚀 Usage

### Prerequisites
- **kubectl CLI**: Must be installed and configured to access your target Kubernetes cluster
- **Cluster Access**: The tool uses your current kubectl context/cluster configuration

### Basic Commands

**Get help and see all available options:**
```bash
kportfwd --help
```

**Run with a configuration file:**
```bash
kportfwd --config /path/to/config.yaml
```

**Run with elevated permissions (recommended):**
```bash
sudo kportfwd --config /path/to/config.yaml
```

> **💡 Why sudo?** 
> - Updates `/etc/hosts` so you can access internal services by name
> - Adds virtual IP addresses to your network interface. This lets you run multiple services on the same port (like having two databases both on port 5432, but using different IPs representing different host names)

### 🔐 One-Time Setup: Passwordless Sudo (Optional)

To avoid typing your password every time, you can configure sudo to not require a password for `kportfwd`:

**Setup (one-time only):**
```bash
# Edit sudoers file safely
sudo visudo -f /etc/sudoers.d/kportfwd
```

**Add one of these lines to the file:**
```bash
# For your specific user (replace 'username' with your actual username):
username ALL=(root) NOPASSWD: /usr/local/bin/kportfwd

# Or for all admin users:
%admin ALL=(root) NOPASSWD: /usr/local/bin/kportfwd
```

**After setup, run without password prompts:**
```bash
sudo kportfwd --config config.yaml  # No password required!
```

> **🔒 Security Note:** This is safe because it only applies to the specific `kportfwd` binary path. Make sure the path matches where you installed kportfwd (use `which kportfwd` to check).

## ⚙️ Configuration

### Configuration File Structure

Create a YAML configuration file that defines your port forwarding rules:

```yaml
# Target pod configuration - where the forwarder agent will be deployed
target:
  pod:
    labelSelector: "app=backend,!job-name"  # Kubernetes label selector to find target pod
    container: service                      # Container name within the pod
    namespace: default                      # Kubernetes namespace

# Port forwarding rules
forwards:
  - name: "redis-cache"                            # Optional: Human-readable name
    localAddr: "localhost:6379"                   # Optional: Local binding address
    sourceAddr: ":50001"                          # Optional: Port on forwarder agent (auto-assigned if empty)
    targetAddr: "{{REDIS_HOST}}:{{REDIS_PORT}}"   # Target address (supports Go templates)
  
  - name: "postgres-db"
    localAddr: "192.168.0.126:5432"              # Creates network alias (requires sudo)
    targetAddr: "internal.postgresql:5432"        # Direct service address
```

### Configuration Options Explained

| Field | Required | Description |
|-------|----------|-------------|
| `target.pod.labelSelector` | ✅ | Kubernetes label selector to identify the target pod |
| `target.pod.container` | ✅ | Container name where the forwarder agent will run |
| `target.pod.namespace` | ✅ | Kubernetes namespace to search for the pod |
| `forwards[].name` | ❌ | Human-readable identifier for the forwarding rule |
| `forwards[].localAddr` | ❌ | Local address to bind to (defaults to sourceAddr if empty) |
| `forwards[].sourceAddr` | ❌ | Address on forwarder agent (auto-assigned port ≥50000 if empty) |
| `forwards[].targetAddr` | ✅ | Final destination address within the cluster |

### Template Variables in `targetAddr`

You can use Go template syntax to extract values from environment variables in the target pod:

```yaml
targetAddr: "{{.SERVICE_NAME}}.{{.NAMESPACE}}.svc.cluster.local:{{.PORT}}"
```

**Available template functions:**
- `{{.ENV_VAR_NAME}}` - Access environment variable
- `{{splitAt "string" "separator" index}}` - Split string and get element at index

### Minimal Configuration Example

For simple use cases, you can use a minimal configuration:

```yaml
target:
  pod:
    labelSelector: "app=backend"
    container: service
    namespace: default
forwards:
  - targetAddr: "{{REDIS_HOST}}:{{REDIS_PORT}}"    # Uses env vars from target pod
  - targetAddr: "internal.postgresql:5432"         # Direct service address
```

> **🔍 How it works:** When `localAddr` and `sourceAddr` are omitted, the tool automatically assigns available local IP addresses and ports.

## 🔄 How It Works

The tool creates a multi-hop forwarding tunnel:

```
Local Machine → Target Pod (Forwarder Agent) → Cluster Service
     :8080            :50001                    internal.db:5432
```

![Port Forwarding Process](port-forwarding-process.jpg)

1. **Agent Deployment**: A lightweight forwarder agent is deployed to your target pod
2. **Tunnel Creation**: Local traffic is forwarded to the agent via Kubernetes port-forwarding
3. **Internal Routing**: The agent forwards traffic to internal cluster services/domains
4. **Seamless Access**: Internal services become accessible on your local machine


## 🔨 Build from Source

### Prerequisites
- `make` - Build automation tool
- `docker` - Container runtime for building cross-platform binaries
- `go` - Go programming language (for local installation)

### Build Commands

**Build for specific platforms:**
```bash
# Build for macOS
make build-macos

# Build for Linux
make build-linux
```

**Install locally (macOS):**
```bash
make install-macos
```

> **📝 Note:** Make sure your Go binary directory (`$GOPATH/bin` or `$HOME/go/bin`) is in your `PATH` environment variable.

### Manual Build
```bash
# Clone the repository
git clone https://github.com/abdularis/kportfwd.git
cd kportfwd

# Build for your current platform
go build -o kportfwd ./cmd/portfwd

# Make it executable and move to PATH
chmod +x kportfwd
sudo mv kportfwd /usr/local/bin/
```