package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Signal test running. Press Ctrl+C.")

	sig := <-signals

	fmt.Println("Received signal:", sig)
	fmt.Println("Process exiting normally.")
}
