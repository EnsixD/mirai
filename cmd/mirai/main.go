package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mirai/internal/panel/cli"
	"mirai/internal/panel/domain"
	"mirai/web"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Every command starts empty, the panel and "admin node add" alike: no inbounds and no
	// tariffs of their own (domain.StarterContent stays on for the tests' fixtures).
	domain.StarterContent = false
	if err := cli.Run(ctx, os.Args[1:], version, web.Dist()); err != nil {
		fmt.Fprintln(os.Stderr, "mirai:", err)
		os.Exit(1)
	}
}
