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

// ForwardConfig is a configuration for port forwarding
// The traffic flow:
// Local machine --> Target Pod (Forwarder) --> Target Address
type ForwardConfig struct {
	Name string `yaml:"name"`
	// LocalAddr is a listener address in local machine where the traffic will be forwarded to target pod (forwarder)
	// if empty, the address for local will use the same address as SourceAddr
	LocalAddr string `yaml:"localAddr"`
	// SourceAddr is a listener address in target pod (forwarder) where incoming traffic will be forwarded to target address
	SourceAddr string `yaml:"sourceAddr"`
	// TargetAddr is an address in cluster where the traffic will be forwarded to e.g.: "postgres.svc.internal:8080"
	TargetAddr string `yaml:"targetAddr"`

	//
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

	sourceAddrPort := 50000
	// sanitize config for forwarders
	for idx, fwdConfig := range cfg.Forwards {
		if fwdConfig.SourceAddr == "" {
			fwdConfig.SourceAddr = fmt.Sprintf(":%d", sourceAddrPort)
			sourceAddrPort++
		}

		if fwdConfig.LocalAddr == "" {
			fwdConfig.LocalAddr = fwdConfig.SourceAddr
		}

		localAddrParsed, err := parseURL(fwdConfig.LocalAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse local addr %s: %s", fwdConfig.LocalAddr, err)
		}
		if net.ParseIP(localAddrParsed.Hostname()) == nil {
			return nil, fmt.Errorf("local addr %s is not an IP address", localAddrParsed.Hostname())
		}

		sourceAddrParsed, err := parseURL(fwdConfig.SourceAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source addr %s: %s", fwdConfig.SourceAddr, err)
		}

		fwdConfig.LocalAddrParsed = localAddrParsed
		fwdConfig.SourceAddrParsed = sourceAddrParsed

		// update config on list
		cfg.Forwards[idx] = fwdConfig
	}

	return &cfg, nil
}

func ProcessConfigTemplateString(cfg *Config, data map[string]string) error {
	for idx, fwdConfig := range cfg.Forwards {
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
