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
)

var (
	FlagConfigFile = &cli.StringFlag{
		Name:     flagNameConfigFile,
		Required: true,
		Usage:    "Path to config yaml file",
	}

	FlagForwarderAgentScript = &cli.StringFlag{
		Name:  flagNameForwarderAgentScript,
		Value: "",
		Usage: "Path to forwarder agent script or binary",
	}
)

func GetCLIApp() *cli.App {
	return &cli.App{
		Name:        "portfwd",
		Version:     "0.0.1-beta1",
		Usage:       "Helper to port forwards any domain names in kubernetes cluster",
		Description: "Port forwards services, pods, or any accessible domain names inside kubernetes cluster",
		Commands: []*cli.Command{
			getPortForwardsCmd(),
		},
	}
}

func getPortForwardsCmd() *cli.Command {
	return &cli.Command{
		Name:  "port-forward",
		Usage: "Port forward all target addresses specified in config yaml file and made available for use locally",
		Flags: []cli.Flag{
			FlagConfigFile,
			FlagForwarderAgentScript,
		},
		Action: func(c *cli.Context) error {
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

			target, err := FindTargetPod(c.Context, cfg, k8sClient, true)
			if err != nil {
				return err
			}

			log.Printf("found target pod: %s", target.Pod)

			PortForwardFromConfig(c.Context, cfg, k8sClient, target)

			return nil
		},
	}
}
