package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/abdularis/kportfwd/internal/log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func ExecOnPod(ctx context.Context, cfg *ClientConfig, ns, pod, container string, stdout, stderr io.Writer, command []string) error {
	if stderr == nil || stdout == nil {
		return fmt.Errorf("stdout and stderr should not be nil")
	}

	url := cfg.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(ns).
		Name(pod).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec).
		URL()
	exec, err := remotecommand.NewSPDYExecutor(cfg.RestConfig, http.MethodPost, url)
	if err != nil {
		return err
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return err
	}

	return nil
}

func GetEnvVariablesFromPod(ctx context.Context, cfg *ClientConfig, ns, pod, container string) (map[string]string, error) {
	command := []string{"env"}
	output := &bytes.Buffer{}
	err := ExecOnPod(ctx, cfg, ns, pod, container, output, &bytes.Buffer{}, command)
	if err != nil {
		return nil, fmt.Errorf("unable to exec get env: %w", err)
	}
	envs := output.String()

	result := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(envs))
	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, "=")
		if len(splits) < 2 {
			log.Printf("GetEnvVariablesFromPod %s: %s can't be parsed", pod, line)
			continue
		}
		varName := strings.TrimSpace(splits[0])
		varValue := strings.TrimSpace(splits[1])
		result[varName] = varValue
	}

	return result, nil
}
