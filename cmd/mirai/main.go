package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mirai/internal/panel/cli"
	"mirai/web"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:], version, web.Dist()); err != nil {
		fmt.Fprintln(os.Stderr, "mirai:", err)
		os.Exit(1)
	}
}
