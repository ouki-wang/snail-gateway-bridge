package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"snail-gateway-bridge/internal/backend"
	"snail-gateway-bridge/internal/config"
	"syscall"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func run(cmd *cobra.Command, args []string) error {

	tasks := []func() error{
		setLogLevel,
		printStartMessage,
		setupBackend,
		startBackend,
	}

	for _, t := range tasks {
		if err := t(); err != nil {
			log.Fatal(err)
		}
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	log.WithField("signal", <-sigChan).Info("signal received")
	log.Warning("shutting down server")

	return nil
}

func setLogLevel() error {
	log.SetLevel(log.Level(uint8(config.C.General.LogLevel)))
	fmt.Println("config.C.General.LogLevel=", config.C.General.LogLevel)
	return nil
}

func printStartMessage() error {
	log.WithFields(log.Fields{
		"version": version,
	}).Info("starting Snail Gateway Bridge")
	return nil
}

func setupBackend() error {
	if err := backend.Setup(config.C); err != nil {
		return errors.Wrap(err, "setup backend error")
	}
	return nil
}

func startBackend() error {
	fmt.Println("startBackend")
	if err := backend.GetBackend().Start(); err != nil {
		return errors.Wrap(err, "start backend error")
	}
	return nil
}
