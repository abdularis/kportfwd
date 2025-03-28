package cli

import (
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
		Usage:    "Path to config yaml file (see README.md for more info)",
	}

	FlagForwarderAgentScript = &cli.StringFlag{
		Name:  flagNameForwarderAgentScript,
		Value: "",
		Usage: "Path to forwarder agent binary file",
	}

	FlagSaveTargetEnvar = &cli.BoolFlag{
		Name:  flagSaveTargetEnvarToFile,
		Value: false,
		Usage: "If specified will save target pod environment variables into file stored in .envs directory",
	}
)

func GetCLIApp() *cli.App {
	return &cli.App{
		Name:        "kportfwd",
		Version:     "0.0.1",
		Usage:       "CLI tool to port forwards any domain names inside kubernetes cluster",
		Description: "Port forwards services, pods, or any accessible domain names inside kubernetes cluster and make them available on local machine.\nRun this tool with sudo permission to allow modifying /etc/hosts file that make internal cluster domain name accessible in local host, this permission is also used to add IP address alias for localhost if you're using custom IP local address in configuration",
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
	target, err := FindTargetPod(c.Context, cfg, k8sClient, true, saveTargetEnvarToFile)
	if err != nil {
		return err
	}

	log.Printf("found target pod: %s", target.Pod)

	PortForwardFromConfig(c.Context, cfg, k8sClient, target)

	return nil
}
