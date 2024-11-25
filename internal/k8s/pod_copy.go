package k8s

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

func CopyFileToPod(ctx context.Context, cfg *ClientConfig, ns, pod, container string, targetPath string, targetFileMode os.FileMode, data io.Reader) error {
	url := cfg.Clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(ns).
		Name(pod).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: container,
			Command:   []string{"sh", "-c", fmt.Sprintf("tee %s && chmod %o %s", targetPath, targetFileMode, targetPath)},
			Stdin:     true,
			Stdout:    false,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec).
		URL()
	exec, err := remotecommand.NewSPDYExecutor(cfg.RestConfig, http.MethodPost, url)
	if err != nil {
		return err
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  data,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}

	return nil
}
