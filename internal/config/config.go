package config

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/abdularis/kportfwd/internal/ping"
	"gopkg.in/yaml.v3"
)

//go:embed build/*
var embeddedBinFiles embed.FS

func GetForwarderAgentBin(cfg *Config) (io.Reader, fs.FileInfo, string, error) {
	if cfg.ForwarderAgentPath != "" {
		// TODO not yet implemented, read forwarder agent binary executable file from provided path
	}

	data, err := embeddedBinFiles.Open("build/forwarder-agent-linux-amd64")
	if err != nil {
		return nil, nil, "", err
	}

	fsInfo, err := data.Stat()
	if err != nil {
		return nil, nil, "", err
	}

	md5sum, err := embeddedBinFiles.ReadFile("build/forwarder-agent-linux-amd64.md5sum")
	if err != nil {
		return nil, nil, "", err
	}

	return data, fsInfo, string(md5sum), nil
}

type Config struct {
	ForwarderAgentPath string `yaml:"forwarderAgentPath"`
	Target             struct {
		Pod *Pod `yaml:"pod"`
	}
	Forwards []ForwardConfig `yaml:"forwards"`
}

type AgentTarget struct {
	Namespace string
	Pod       string
	Container string
}

type Pod struct {
	Namespace     string `yaml:"namespace"`
	LabelSelector string `yaml:"labelSelector"`
	Container     string `yaml:"container"`
}

// ForwardConfig defines the configuration for a port forwarding rule.
//
// Port forwarding establishes a tunnel that routes network traffic through multiple hops:
//  1. Client connects to LocalAddr on the local machine
//  2. Traffic is forwarded to SourceAddr on the target pod (forwarder agent)
//  3. Forwarder agent forwards traffic to TargetAddr within the cluster
//
// Traffic flow: Local Machine (LocalAddr) → Target Pod (SourceAddr) → Cluster Service (TargetAddr)
//
// Example configuration:
//
//	name: "database"
//	localAddr: "127.0.0.1:5432"        # Local PostgreSQL port
//	sourceAddr: ":50001"               # Port on forwarder pod
//	targetAddr: "postgres.svc:5432"    # PostgreSQL service in cluster
type ForwardConfig struct {
	// Name is a human-readable identifier for this forwarding rule.
	// Used for logging and display purposes.
	Name string `yaml:"name"`

	// LocalAddr specifies the listener address on the local machine where clients connect.
	// Format: "host:port" or ":port" (e.g., "127.0.0.1:8080", ":3000")
	// If empty, defaults to the same value as SourceAddr.
	// The forwarder will bind to this address and forward incoming connections.
	LocalAddr string `yaml:"localAddr"`

	// SourceAddr specifies the listener address on the target pod (forwarder agent).
	// Format: "host:port" or ":port" (e.g., ":50001", "0.0.0.0:8080")
	// If empty, an available port starting from 50000 will be automatically assigned.
	// This is where the forwarder agent listens for incoming connections from LocalAddr.
	SourceAddr string `yaml:"sourceAddr"`

	// TargetAddr specifies the final destination address within the cluster.
	// Format: "host:port" (e.g., "postgres.svc.cluster.local:5432", "redis:6379")
	// Supports template variables that can be substituted with environment data from the target pod.
	// Example: "{{.SERVICE_NAME}}.{{.NAMESPACE}}.svc.cluster.local:{{.PORT}}"
	TargetAddr string `yaml:"targetAddr"`

	// Parsed URL representations of the address fields (computed at runtime)
	// These fields are populated during configuration processing and excluded from YAML serialization.
	LocalAddrParsed  *url.URL `yaml:"-"`
	SourceAddrParsed *url.URL `yaml:"-"`
	TargetAddrParsed *url.URL `yaml:"-"`
}

func GetConfig(filepath string) (*Config, error) {
	configData, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	cfg := Config{}
	err = yaml.Unmarshal([]byte(configData), &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func ParseConfigAddresses(cfg *Config, data map[string]string) error {
	currentLocalIP := "10.0.0.10" // starting IP address to assign for localAddr if not specified
	takenLocalIPs := map[string]struct{}{}
	usedLocalPorts := map[string]struct{}{}

	sourceAddrPort := 50000

	for idx, fwdConfig := range cfg.Forwards {
		// 1. Process from TargetAddr, give access to env data from target pod to render the address template given from config
		if fwdConfig.TargetAddr == "" {
			continue
		}

		// process config template, parses config string and substitute with env data from target pod
		tmplt, err := template.New(fmt.Sprintf("%s_%d", fwdConfig.Name, idx)).
			Funcs(template.FuncMap{
				"splitAt": func(s string, sep string, index int) string {
					parts := strings.Split(s, sep)
					if index >= len(parts) || index < 0 {
						return ""
					}
					return parts[index]
				},
			}).
			Parse(fwdConfig.TargetAddr)
		if err != nil {
			return err
		}
		tmplt.Option("missingkey=error")

		output := bytes.NewBufferString("")
		if err := tmplt.Execute(output, data); err != nil {
			return err
		}

		addr := output.String()
		ur, err := parseURL(addr)
		if err != nil {
			return fmt.Errorf("failed to parse addr %s: %s", addr, err)
		}

		if ur.Port() == "" {
			if port, ok := knownPortByScheme[ur.Scheme]; ok {
				ur.Host += ":" + port
			} else {
				return fmt.Errorf("no port specified for addr: %s", addr)
			}
		}

		cfg.Forwards[idx].TargetAddr = ur.Host
		cfg.Forwards[idx].TargetAddrParsed = ur

		// 2. If LocalAddr is empty, set it to the same as SourceAddr
		localAddrListener := fwdConfig.LocalAddr
		if localAddrListener == "" {
			for {
				// prefer to use localhost if the port is available
				if _, used := usedLocalPorts[ur.Port()]; !used {
					localAddrListener = "127.0.0.1:" + ur.Port()
					break
				}

				// otherwise try to use 10.0.0.x address range, that will be aliased to loopback interface
				if _, taken := takenLocalIPs[currentLocalIP]; !taken {
					if err := ping.Ping(currentLocalIP); err != nil {
						// not reachable, can be used
						localAddrListener = ur.Scheme + "://" + net.JoinHostPort(currentLocalIP, ur.Port())
						takenLocalIPs[currentLocalIP] = struct{}{}
						break
					}
				}

				// increment to next ip address
				currentLocalIP, err = nextIPAddress(currentLocalIP)
				if err != nil {
					return fmt.Errorf("failed to get next ip address: %w", err)
				}
			}
		}

		localAddrParsed, err := parseURL(localAddrListener)
		if err != nil {
			return fmt.Errorf("failed to parse local addr %s: %s", localAddrListener, err)
		}
		usedLocalPorts[localAddrParsed.Port()] = struct{}{}
		cfg.Forwards[idx].LocalAddrParsed = localAddrParsed
		cfg.Forwards[idx].LocalAddr = localAddrListener

		// 3. If SourceAddr is empty, assign an available port starting from 50000
		sourceAddr := fwdConfig.SourceAddr
		if sourceAddr == "" {
			sourceAddr = fmt.Sprintf(":%d", sourceAddrPort)
			sourceAddrPort++
		}

		sourceAddrParsed, err := parseURL(sourceAddr)
		if err != nil {
			return fmt.Errorf("failed to parse source addr %s: %s", sourceAddr, err)
		}

		cfg.Forwards[idx].SourceAddr = sourceAddr
		cfg.Forwards[idx].SourceAddrParsed = sourceAddrParsed
	}

	return nil
}

var knownPortByScheme = map[string]string{
	"https": "443",
	"http":  "80",
}

var networkSchemeRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+\-.]*:\/\/`)

func parseURL(rawURL string) (*url.URL, error) {
	if !networkSchemeRegex.MatchString(rawURL) {
		// no url scheme, add default tcp, so that parsing url will be correct
		rawURL = "tcp://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func nextIPAddress(ipAddr string) (string, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", ipAddr)
	}

	// Increment the IP address
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}

	// Check if the resulting IP is a network identifier or broadcast address
	// For common subnet masks (/24, /16, /8), avoid these addresses
	if isNetworkOrBroadcastAddress(ip) {
		// Recursively get the next IP address if current one is network/broadcast
		return nextIPAddress(ip.String())
	}

	return ip.String(), nil
}

// isNetworkOrBroadcastAddress checks if an IP address is likely a network identifier
// or broadcast address for common subnet configurations
func isNetworkOrBroadcastAddress(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Convert to 4-byte representation for IPv4
	if ipv4 := ip.To4(); ipv4 != nil {
		lastOctet := ipv4[3]

		// Check for common network identifiers and broadcast addresses:
		// - .0 (network identifier for /24 subnets)
		// - .255 (broadcast address for /24 subnets)
		// - .0.0 (network identifier for /16 subnets)
		// - .255.255 (broadcast address for /16 subnets)
		if lastOctet == 0 || lastOctet == 255 {
			return true
		}

		// For /16 subnets, check if last two octets are .0.0 or .255.255
		if ipv4[2] == 0 && lastOctet == 0 {
			return true // network identifier for /16
		}
		if ipv4[2] == 255 && lastOctet == 255 {
			return true // broadcast address for /16
		}
	}

	return false
}
