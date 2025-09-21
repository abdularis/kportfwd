package cli

import (
	"fmt"
	"strings"

	"github.com/abdularis/kportfwd/internal/config"
	"github.com/abdularis/kportfwd/internal/k8s"
	"github.com/abdularis/kportfwd/internal/log"

	"github.com/urfave/cli/v2"
)

const (
	flagNameConfigFile           = "config"
	flagNameForwarderAgentScript = "forwarder-agent"
	flagSaveTargetEnvarToFile    = "save-target-envar"
	flagNameTarget               = "t"
	flagNameNamespace            = "n"
	flagNameContainer            = "c"
	flagNameForwards             = "f"
)

const (
	resourceTypePod = "pod"
)

var (
	FlagConfigFile = &cli.StringFlag{
		Name:     flagNameConfigFile,
		Required: false,
		Usage:    "Path to YAML configuration file, otherwise use command line options to provide configuration",
	}

	FlagForwarderAgentScript = &cli.StringFlag{
		Name:  flagNameForwarderAgentScript,
		Value: "",
		Usage: "Custom forwarder agent binary (optional)",
	}

	FlagSaveTargetEnvar = &cli.BoolFlag{
		Name:  flagSaveTargetEnvarToFile,
		Value: false,
		Usage: "Save target pod environment variables to .envs/ directory",
	}

	FlagTarget = &cli.StringFlag{
		Name:  "t",
		Usage: "Target resource with label selector. Format: 'pod/labelSelector' or just 'labelSelector' (defaults to pod). (e.g., 'pod/app=backend', 'app=backend')",
	}

	FlagNamespace = &cli.StringFlag{
		Name:  "n",
		Usage: "Kubernetes namespace",
		Value: "default",
	}

	FlagContainer = &cli.StringFlag{
		Name:  "c",
		Usage: "Container name within the pod (e.g., 'service')",
	}

	FlagForwards = &cli.StringFlag{
		Name:  "f",
		Usage: "Comma-separated list of target addresses to forwards (e.g., 'postgres:5432,{{.REDIS_HOST}}:{{.REDIS_PORT}}')",
	}
)

func GetCLIApp() *cli.App {
	return &cli.App{
		Name:    "kportfwd",
		Version: "0.0.2",
		Usage:   "Port forward internal Kubernetes services to your local machine",
		Description: `Forward cluster-internal services and domains to your local machine without any cluster setup.

Requires sudo to modify /etc/hosts and create network aliases for transparent access.

EXAMPLES:

1. Using config file:
   kportfwd --config path/to/config.yaml

2. Using CLI options:
   kportfwd -t pod/app=backend -n default -c service -f "postgres:5432,redis:6379"
   kportfwd -t app=web -n production -c service -f "{{.DB_HOST}}:{{.DB_PORT}}"

3. Multiple forwards with CLI:
   kportfwd -t pod/app=api -n staging -c service -f "db.internal:5432,cache.internal:6379,queue.internal:5672"
`,
		Flags: []cli.Flag{
			FlagConfigFile,
			FlagForwarderAgentScript,
			FlagSaveTargetEnvar,
			FlagTarget,
			FlagNamespace,
			FlagContainer,
			FlagForwards,
		},
		Action: handleActionPortForward,
	}
}

// parseForwardsFlag parses comma-separated target addresses into ForwardConfig structs
func parseForwardsFlag(forwardsStr string) ([]config.ForwardConfig, error) {
	if forwardsStr == "" {
		return nil, fmt.Errorf("forwards flag cannot be empty")
	}

	addresses := strings.Split(forwardsStr, ",")
	forwards := make([]config.ForwardConfig, 0, len(addresses))

	for i, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}

		forward := config.ForwardConfig{
			Name:       fmt.Sprintf("forward-%d", i+1),
			TargetAddr: addr,
			// LocalAddr and SourceAddr will be auto-assigned if empty
		}
		forwards = append(forwards, forward)
	}

	return forwards, nil
}

// parseTargetFlag parses and validates the target flag value
// Expected format: "pod/labelSelector" or just "labelSelector" (defaults to pod)
// Returns the label selector part after validation
func parseTargetFlag(target string) (string, string, error) {
	if target == "" {
		return "", "", fmt.Errorf("target cannot be empty")
	}

	// Check if target contains a resource type prefix
	if strings.Contains(target, "/") {
		parts := strings.SplitN(target, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid target format: %s (expected 'resourceType/labelSelector')", target)
		}

		resourceType := parts[0]
		labelSelector := parts[1]

		// Only support "pod" resource type for now
		if resourceType != resourceTypePod {
			return "", "", fmt.Errorf("unknown resource type: %s (only '%s' is supported)", resourceType, resourceTypePod)
		}

		if labelSelector == "" {
			return "", "", fmt.Errorf("label selector cannot be empty after '%s/'", resourceTypePod)
		}

		return resourceType, labelSelector, nil
	}

	// No prefix provided, default to pod
	return resourceTypePod, target, nil
}

// createConfigFromFlags creates a Config struct from CLI flags instead of YAML file
func createConfigFromFlags(c *cli.Context) (*config.Config, error) {
	target := c.String(flagNameTarget)
	namespace := c.String(flagNameNamespace)
	container := c.String(flagNameContainer)
	forwardsStr := c.String(flagNameForwards)

	// Validate required flags
	if target == "" {
		return nil, fmt.Errorf("target flag (-t) is required when not using config file")
	}
	if container == "" {
		return nil, fmt.Errorf("container flag (-c) is required when not using config file")
	}
	if forwardsStr == "" {
		return nil, fmt.Errorf("forwards flag (-f) is required when not using config file")
	}

	// Parse and validate target flag
	resType, labelSelector, err := parseTargetFlag(target)
	if err != nil {
		return nil, fmt.Errorf("error parsing target: %w", err)
	}

	// Parse forwards
	forwards, err := parseForwardsFlag(forwardsStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing forwards: %w", err)
	}

	// Create config
	cfg := &config.Config{
		Forwards: forwards,
	}
	if resType == resourceTypePod {
		cfg.Target.Pod = &config.Pod{
			LabelSelector: labelSelector,
			Container:     container,
			Namespace:     namespace,
		}
	}

	return cfg, nil
}

func handleActionPortForward(c *cli.Context) error {
	configFileName := c.String(flagNameConfigFile)

	var cfg *config.Config
	var err error

	if configFileName != "" {
		// Use YAML config file
		cfg, err = config.GetConfig(configFileName)
		if err != nil {
			log.Fatalf("unable to read config: %s", err)
		}
	} else {
		// Create config from CLI flags
		cfg, err = createConfigFromFlags(c)
		if err != nil {
			log.Fatalf("unable to create config from flags: %s", err)
		}
	}

	k8sClient, err := k8s.NewKubeClientConfig()
	if err != nil {
		return err
	}

	cfg.ForwarderAgentPath = c.String(flagNameForwarderAgentScript)

	log.Printf("find target pod on cluster: %s", k8sClient.Context)

	saveTargetEnvarToFile := c.Bool(flagSaveTargetEnvarToFile)
	target, err := FindTargetPod(c.Context, cfg, k8sClient)
	if err != nil {
		return err
	}

	log.Printf("found target pod: %s", target.Pod)

	envvars, err := GetTargetPodEnvars(c.Context, k8sClient, target.Namespace, target.Pod, target.Container, saveTargetEnvarToFile)
	if err != nil {
		return fmt.Errorf("unable to get environment variables from target pod: %w", err)
	}

	if err := config.ParseConfigAddresses(cfg, envvars); err != nil {
		return fmt.Errorf("unable to render environment variables to config: %w", err)
	}

	PortForwardFromConfig(c.Context, cfg, k8sClient, target)

	return nil
}
