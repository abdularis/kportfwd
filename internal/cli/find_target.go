package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/abdularis/kportfwd/internal/config"
	"github.com/abdularis/kportfwd/internal/k8s"

	v1 "k8s.io/api/core/v1"
)

func FindTargetPod(ctx context.Context, cfg *config.Config, k8sClient *k8s.ClientConfig, parseConfigWithTargetEnvar bool) (config.AgentTarget, error) {
	pods, err := k8s.FindPod(ctx, k8sClient, cfg.Target.Pod.Namespace, cfg.Target.Pod.LabelSelector, "")
	if err != nil {
		return config.AgentTarget{}, fmt.Errorf("unable to find target pod: %w", err)
	}

	if len(pods) <= 0 {
		return config.AgentTarget{}, fmt.Errorf("target pod not found")
	}

	podName := ""

	for _, pod := range pods {
		podReady := true
		for _, condition := range pod.Status.Conditions {
			if condition.Status != v1.ConditionTrue {
				podReady = false
				break
			}
		}

		if podReady {
			podName = pod.ObjectMeta.Name
			break
		}
	}

	if podName == "" {
		return config.AgentTarget{}, fmt.Errorf("ready target pod not found")
	}

	if parseConfigWithTargetEnvar {
		envvars, err := k8s.GetEnvVariablesFromPod(ctx, k8sClient, cfg.Target.Pod.Namespace, podName, cfg.Target.Pod.Container)
		if err != nil {
			return config.AgentTarget{}, fmt.Errorf("unable get get environment variables from %s: %w", podName, err)
		}

		if err := config.ProcessConfigTemplateString(cfg, envvars); err != nil {
			return config.AgentTarget{}, fmt.Errorf("unable to render environment variables to config: %w", err)
		}

		if err := saveEnvar(podName, envvars); err != nil {
			return config.AgentTarget{}, fmt.Errorf("unable to save envar: %w", err)
		}
	}

	return config.AgentTarget{
		Namespace: cfg.Target.Pod.Namespace,
		Container: cfg.Target.Pod.Container,
		Pod:       podName,
	}, nil
}

func saveEnvar(podName string, envars map[string]string) error {
	data := ""
	for name, value := range envars {
		data += fmt.Sprintf("%s=%s\n", name, value)
	}

	if err := os.MkdirAll("./.envs/", 0777); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create envs dir err: %w", err)
		}
	}

	fileName := fmt.Sprintf("./.envs/%s", podName)
	log.Printf("save .env into %s", fileName)
	err := os.WriteFile(fileName, []byte(data), 0777)
	if err != nil {
		return fmt.Errorf("write file err: %w", err)
	}
	return nil
}
