package k8s

import (
	"context"
	"fmt"
	"io"

	"github.com/abdularis/kportfwd/internal/log"

	"net/http"
	"os"

	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func PortForward(ctx context.Context, cfg *ClientConfig, readyCh chan struct{}, ns, pod string, localAddr string, localPort, podPort string, silent bool) error {
	restClient := cfg.Clientset.CoreV1().RESTClient()
	url := restClient.Post().
		Resource("pods").
		Namespace(ns).
		Name(pod).
		SubResource("portforward").URL()

	roundTripper, upgrader, err := spdy.RoundTripperFor(cfg.RestConfig)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, "POST", url)
	ports := []string{
		fmt.Sprintf("%s:%s", localPort, podPort),
	}

	stopChan := make(chan struct{}, 1)
	var stdout io.Writer
	if !silent {
		stdout = log.NewTaggedWriter(os.Stdout, "k8sclient")
	}

	if localAddr == "" {
		localAddr = "localhost"
	}
	pf, err := portforward.NewOnAddresses(dialer, []string{localAddr}, ports, stopChan, make(chan struct{}), stdout, os.Stderr)
	if err != nil {
		return err
	}

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	go func() {
		<-ctx.Done()
		close(stopChan)
		log.Printf("port forward %s -> %s closed", localPort, podPort)
	}()

	if readyCh != nil {
		go func() {
			<-pf.Ready
			readyCh <- struct{}{}
		}()
	}

	if err := pf.ForwardPorts(); err != nil {
		return fmt.Errorf("error forwarding ports: %w", err)
	}

	return nil
}
