//go:build !baremetal

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func initExitHandler() context.Context {
	return contextWithSignal(context.Background())
}

// ContextWithSignal creates a context canceled when SIGINT or SIGTERM are notified
func contextWithSignal(ctx context.Context) context.Context {
	newCtx, cancel := context.WithCancel(ctx)
	signals := make(chan os.Signal)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-signals:
			cancel()
		}
	}()
	return newCtx
}

func connectAddresses() ([]string, error) {
	if len(os.Args) < 2 {
		println("usage: multiples [address],[address]")
		os.Exit(1)
	}

	addrs := strings.Split(os.Args[1], ",")
	if len(addrs) == 0 {
		return nil, errors.New("no devices specified")
	}

	return addrs, nil
}

func failMessage(msg string) {
	println(msg)
	exitCtx.Done()
}
