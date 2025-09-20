package cli

import (
	"fmt"

	"github.com/abdularis/kportfwd/internal/config"
	"github.com/abdularis/kportfwd/internal/k8s"
	"github.com/abdularis/kportfwd/internal/log"

	"github.com/urfave/cli/v2"
)

const (
	flagNameConfigFile           = "config"
	flagNameForwarderAgentScript = "forwarder-agent"
	flagSaveTargetEnvarToFile    = "save-target-envar"
)

var (
	FlagConfigFile = &cli.StringFlag{
		Name:     flagNameConfigFile,
		Required: true,
		Usage:    "Path to YAML configuration file",
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
)

func GetCLIApp() *cli.App {
	return &cli.App{
		Name:        "kportfwd",
		Version:     "0.0.2",
		Usage:       "Port forward internal Kubernetes services to your local machine",
		Description: "Forward cluster-internal services and domains to your local machine without any cluster setup.\n\nRequires sudo to modify /etc/hosts and create network aliases for transparent access.",
		Flags: []cli.Flag{
			FlagConfigFile,
			FlagForwarderAgentScript,
			FlagSaveTargetEnvar,
		},
		Action: handleActionPortForward,
	}
}

func handleActionPortForward(c *cli.Context) error {
	configFileName := c.String(flagNameConfigFile)
	cfg, err := config.GetConfig(configFileName)
	if err != nil {
		log.Fatalf("unable to read config: %s", err)
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
