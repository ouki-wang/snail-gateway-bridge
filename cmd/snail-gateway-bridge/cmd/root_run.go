package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func run(cmd *cobra.Command, args []string) error {
	log.Info("run....", args)
	return nil
}
