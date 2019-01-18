package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	// TODO: What is this package for?
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// Basic structure for a simple cobra cli application
// For more info on the structure of the code in package main,
// see: https://github.com/spf13/cobra
func main() {

	var rootCmd = &cobra.Command{}

	rootCmd.AddCommand(
		newOperatorCmd(),
		newSidecarCmd(),
	)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Root command: a fatal error occured: %+v", err)
	}
}
