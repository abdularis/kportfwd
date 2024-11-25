package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/abdularis/kportfwd/internal/etchosts"
	"github.com/abdularis/kportfwd/internal/log"

	"github.com/abdularis/kportfwd/internal/cli"
)

func main() {
	log.SetComponentName("kportfwd")
	etchosts.Init()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Printf("got interrupt/terminate, exiting")
		cancelFn()
	}()

	app := cli.GetCLIApp()
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err.Error())
	}
}
