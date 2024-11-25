package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/abdularis/kportfwd/internal/log"
)

type tcpForwarder struct {
	healthCheckInterval time.Duration
	targetAddr          string
	sourceAddr          string
	bufferSize          int
	lastErr             error
}

func (r *tcpForwarder) Start(ctx context.Context, readyCh chan struct{}) error {
	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", r.sourceAddr)
	if err != nil {
		return fmt.Errorf("unable to listen tcp %s: %w", r.sourceAddr, err)
	}
	defer listener.Close()

	log.Infof("start forwarding %s -> %s", r.sourceAddr, r.targetAddr)
	defer func() {
		log.Infof("stop forwarding %s -> %s", r.sourceAddr, r.targetAddr)
	}()

	ctx, cancel := context.WithCancel(ctx)
	go r.healthCheckTarget(ctx, cancel)
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	if readyCh != nil {
		go func() { readyCh <- struct{}{} }()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			// NOTE: Don't print false-positive errors
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		log.Infof("connection established %s -> %s", conn.RemoteAddr(), conn.LocalAddr())
		go r.forward(ctx, conn)
	}
}

func (r *tcpForwarder) healthCheckTarget(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	timer := time.NewTimer(time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			// NOTE: Dial source to make sure it's alive
			conn, err := r.dialTarget(ctx)
			if err != nil {
				r.lastErr = fmt.Errorf("health check %s err: %s", r.targetAddr, err)
				log.Errorf(r.lastErr.Error())
				return
			}
			conn.Close()
			timer.Reset(r.healthCheckInterval)
		}
	}
}

func (r *tcpForwarder) dialTarget(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", r.targetAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial TCP address: %s: %w", r.targetAddr, err)
	}

	return conn, nil
}

func (r *tcpForwarder) forward(ctx context.Context, sourceConn net.Conn) {
	defer func() {
		_ = sourceConn.Close()
		log.Infof("connection closed %s", sourceConn.RemoteAddr())
	}()

	targetConn, err := r.dialTarget(ctx)
	if err != nil {
		log.Errorf(
			"could not read from source error: %s",
			err,
		)
		return
	}

	copyFn := func(wg *sync.WaitGroup, src net.Conn, dst net.Conn) {
		defer func() {
			_ = src.Close()
			_ = dst.Close()
			wg.Done()
		}()
		if _, err := io.Copy(dst, src); err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Errorf("forward: io copy %s -> %s err: %v", src.RemoteAddr(), dst.RemoteAddr(), err)
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go copyFn(&wg, sourceConn, targetConn)
	go copyFn(&wg, targetConn, sourceConn)

	wg.Wait()
}
