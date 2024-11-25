package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/abdularis/kportfwd/internal/log"
)

const processTimeoutDuration = time.Second * 30

func main() {
	log.SetComponentName("forwarder-agent")

	var addresses addressList
	flag.Var(&addresses, "address", "TCP address pair to forward, example: 'sourcehost:port->targethost:port', forwarder will create listener for sourcehost:port and forward any network traffic to targethost:port")
	flag.Parse()

	forwarderConfigList, err := parseForwarderConfigList(addresses)
	if err != nil {
		log.Fatalf("unable to parse forwarder config: %s", err)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	forwarderList := []*tcpForwarder{}

	wg := sync.WaitGroup{}

	forwarderReadyCh := make(chan struct{}, 1)
	for _, cfg := range forwarderConfigList {
		f := &tcpForwarder{
			healthCheckInterval: time.Second * 15,
			targetAddr:          cfg.TargetAddr,
			sourceAddr:          cfg.SourceAddr,
			bufferSize:          4096,
		}

		forwarderList = append(forwarderList, f)

		wg.Add(1)
		go func(fwd *tcpForwarder) {
			defer wg.Done()

			err := fwd.Start(ctx, forwarderReadyCh)
			if err != nil {
				log.Errorf("error starting port forward: %s", err)
			}
		}(f)

		select {
		case <-forwarderReadyCh:
		case <-time.NewTimer(time.Second * 10).C:
			log.Errorf("timeout waiting for forwarder %s to be ready", f.targetAddr)
			return
		}
	}

	log.Infof("FORWARDERS READY. count: %d", len(forwarderConfigList))

	processTimer := time.NewTimer(processTimeoutDuration)
	httpApi := api{
		forwarderList: forwarderList,
		processTimer:  processTimer,
	}
	apiServer := httpApi.start(cancelFunc)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		apiServer.Shutdown(context.Background())
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				_ = apiServer.Shutdown(context.Background())
				return
			case <-processTimer.C:
				log.Infof("process timeout, exit.")
				cancelFunc()
				return
			}
		}
	}()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		cancelFunc()
	}()

	wg.Wait()
}

type api struct {
	forwarderList []*tcpForwarder
	processTimer  *time.Timer
}

func (a *api) start(cancelFn context.CancelFunc) *http.Server {
	listenAddr := ":8181"
	if portStr := os.Getenv("FORWARDER_API_PORT"); portStr != "" {
		_, err := strconv.ParseUint(portStr, 10, 16)
		if err == nil {
			listenAddr = ":" + portStr
		}
	}

	http.HandleFunc("/ping", a.pingHandler)
	http.HandleFunc("/forwarders", a.getForwardersHandler)

	srv := &http.Server{Addr: listenAddr}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Errorf("http api listener error: %s", err)
			cancelFn()
		} else {
			log.Infof("http api listener exit.")
		}
	}()

	return srv
}

func (a *api) pingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.processTimer.Reset(processTimeoutDuration)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "pong")
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Method not allowed")
	}
}

func (a *api) getForwardersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Method not allowed")
	}

	response := []map[string]interface{}{}
	for _, fwd := range a.forwarderList {
		item := map[string]interface{}{
			"targetAddr": fwd.targetAddr,
			"sourceAddr": fwd.sourceAddr,
		}
		if fwd.lastErr != nil {
			item["error"] = fwd.lastErr.Error()
		}
		response = append(response, item)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

type addressList []string

func (s *addressList) String() string {
	return strings.Join(*s, ",")
}

func (s *addressList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
