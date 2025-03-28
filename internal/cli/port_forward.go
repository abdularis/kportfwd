package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abdularis/kportfwd/internal/config"
	"github.com/abdularis/kportfwd/internal/etchosts"
	"github.com/abdularis/kportfwd/internal/ifconfig"
	"github.com/abdularis/kportfwd/internal/k8s"
	"github.com/abdularis/kportfwd/internal/log"
)

func PortForwardFromConfig(ctx context.Context, cfg *config.Config, k8sClient *k8s.ClientConfig, target config.AgentTarget) {
	// 1. Get target pod name
	// 2. Copy relay agent to target pod container
	// 3. Execute relay agent on target pod container
	// 4. Port forward relay agent api
	// 5. Ping relay agent periodically
	// 6. Port forward all relayed ports from target pod
	// 7. Run local process inside container

	// We'll have multiple port forwards commands running in parallel,
	// 	What happen when one of them exited?
	//  all components need to works properly, if one error then all process need to be stopped

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	onReadyCh := make(chan struct{}, 1)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer func() {
			cancelFn()
			wg.Done()
		}()
		err := runForwarderAgent(ctx, cfg, k8sClient, onReadyCh, target.Namespace, target.Pod, target.Container)
		if err != nil {
			log.Printf("run relay agent err: %s", err)
		}
	}()
	select {
	case <-onReadyCh:
	case <-ctx.Done():
		return
	}

	go func() {
		defer func() {
			cancelFn()
			wg.Done()
		}()
		portForwardAll(ctx, k8sClient, target.Namespace, target.Pod, cfg.Forwards)
	}()

	wg.Wait()
}

func portForwardAll(ctx context.Context, k8sClient *k8s.ClientConfig, ns, targetPod string, cfgs []config.ForwardConfig) {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	wg := sync.WaitGroup{}
	for _, rc := range cfgs {
		wg.Add(1)
		go func(cfg config.ForwardConfig) {
			defer func() {
				etchosts.RemoveHost(rc.TargetAddrParsed.Hostname())
				wg.Done()
				cancelFn()
			}()

			if cfg.LocalAddrParsed.Hostname() != "127.0.0.1" {
				// add the address to the hosts file
				if err := ifconfig.AddLoopbackAlias(cfg.LocalAddrParsed.Hostname()); err != nil {
					log.Errorf("unable to add loopback addr alias %s: %s", cfg.LocalAddrParsed.Hostname(), err)
					return
				}
				defer func() {
					if err := ifconfig.RemoveLoopbackAlias(cfg.LocalAddrParsed.Hostname()); err != nil {
						log.Errorf("unable to remove loopback addr alias %s: %s", cfg.LocalAddrParsed.Hostname(), err)
						return
					}
				}()
			}

			etchosts.AddHost(cfg.LocalAddrParsed.Hostname(), rc.TargetAddrParsed.Hostname())
			err := k8s.PortForward(ctx, k8sClient, nil, ns, targetPod, cfg.LocalAddrParsed.Hostname(), cfg.LocalAddrParsed.Port(), cfg.SourceAddrParsed.Port(), false)
			if err != nil {
				log.Printf("port forwarding %s: %s", cfg.Name, err)
			}
		}(rc)
	}

	log.Printf("forwarding all ports from relay configs...")
	wg.Wait()
}

func runForwarderAgent(ctx context.Context, cfg *config.Config, k8sClient *k8s.ClientConfig, onReadyCh chan struct{}, ns, targetPod, container string) error {
	// What run relay agent do?
	// - Copy relay agent script to target pod
	// - Execute relay agent on target pod
	// - Port forward relay agent api
	// - Ping relay agent periodically

	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	relayFileBin, relayFileInfo, md5sumResult, err := config.GetForwarderAgentBin(cfg)
	if err != nil {
		return err
	}

	targetBaseDir := "/tmp"
	targetForwarderFilePath := fmt.Sprintf("%s/%s", targetBaseDir, relayFileInfo.Name())

	result, err := checkMD5SumOnTargetPod(ctx, k8sClient, ns, targetPod, container, md5sumResult, targetBaseDir)
	if err != nil {
		log.Printf("%s", err)
		log.Printf("copying %s to target pod...", relayFileInfo.Name())
		err = k8s.CopyFileToPod(ctx, k8sClient, ns, targetPod, container, targetForwarderFilePath, os.FileMode(0555), relayFileBin)
		if err != nil {
			return fmt.Errorf("unable to copy agent to target container: %w", err)
		}
	} else {
		log.Printf("forwarder agent already exist: %s", result)
	}

	readyCh := make(chan struct{})

	go func() {
		defer cancelFn()
		err := execRelayAgentOnPod(ctx, k8sClient, readyCh, targetForwarderFilePath, ns, targetPod, container, cfg.Forwards)
		if err != nil {
			log.Printf("error on relay agent: %s", err)
		}
	}()
	if err := waitReady(readyCh); err != nil {
		return fmt.Errorf("executing forwarder agent err: %w", err)
	}

	go func() {
		defer cancelFn()
		err := k8s.PortForward(ctx, k8sClient, readyCh, ns, targetPod, "127.0.0.1", "8181", "8181", true)
		if err != nil {
			log.Printf("port forwarding relay-agent api: %s", err)
		}
	}()
	if err := waitReady(readyCh); err != nil {
		return fmt.Errorf("forwarder agent api port err: %w", err)
	}

	onReadyCh <- struct{}{}

	interval := time.Second * 20
	timer := time.NewTimer(0)
	for {
		select {
		case <-ctx.Done():
			log.Printf("relay agent stopped, ping exited.")
			return nil
		case <-timer.C:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8181/ping", nil)
		if err != nil {
			log.Printf("unable to create ping request: %s", err)
			timer.Reset(time.Second * 5)
			continue
		}

		client := http.Client{
			Timeout: time.Second * 10,
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("unable to call ping: %s", err)
			timer.Reset(time.Second * 5)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("ping got status code: %d, expect %d", resp.StatusCode, http.StatusOK)
			timer.Reset(time.Second * 5)
			continue
		}

		timer.Reset(interval)
	}
}

func execRelayAgentOnPod(ctx context.Context, k8sClient *k8s.ClientConfig, readyCh chan struct{}, targetForwarderFilePath, ns string, pod string, container string, configs []config.ForwardConfig) error {
	remoteCommand := fmt.Sprintf("FORWARDER_API_PORT=8181 %s", targetForwarderFilePath)
	for _, rc := range configs {
		remoteCommand += fmt.Sprintf(" -address '%s->%s'", rc.SourceAddr, rc.TargetAddr)
	}

	isReady := false
	output := &customIOWriter{
		WriterFn: func(p []byte) (n int, err error) {
			if !isReady && strings.Contains(string(p), "FORWARDERS READY") {
				isReady = true
				readyCh <- struct{}{}
			}
			return os.Stdout.Write(p)
		},
	}

	cmd := []string{"sh", "-c", remoteCommand}
	return k8s.ExecOnPod(ctx, k8sClient, ns, pod, container, output, os.Stderr, cmd)
}

func checkMD5SumOnTargetPod(ctx context.Context, k8sClient *k8s.ClientConfig, ns, targetPod, container, md5sumResult, targetDir string) (string, error) {
	output := &bytes.Buffer{}
	command := fmt.Sprintf("cd %s && echo '%s' | md5sum -c -", targetDir, strings.TrimSpace(md5sumResult))
	err := k8s.ExecOnPod(ctx, k8sClient, ns, targetPod, container, output, &bytes.Buffer{}, []string{"sh", "-c", command})
	if err != nil {
		return "", fmt.Errorf("check md5sum on target pod err: %w (stderr: %s)", err, output.String())
	}
	return output.String(), nil
}

func md5sumLocalFile(forwarderAgentPath string) (string, error) {
	basePath := filepath.Dir(forwarderAgentPath)
	fileName := filepath.Base(forwarderAgentPath)
	out, err := exec.CommandContext(
		context.Background(), "sh", "-c",
		fmt.Sprintf("cd %s && md5sum %s", basePath, fileName)).
		CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get md5sum path: %s name: %s err: %w", basePath, fileName, err)
	}
	return string(out), nil
}

func waitReady(readyCh chan struct{}) error {
	select {
	case <-readyCh:
	case <-time.NewTimer(time.Second * 10).C:
		return fmt.Errorf("timeout waiting to be ready")
	}
	return nil
}

type customIOWriter struct {
	WriterFn func(p []byte) (n int, err error)
}

func (cw *customIOWriter) Write(p []byte) (n int, err error) {
	return cw.WriterFn(p)
}
