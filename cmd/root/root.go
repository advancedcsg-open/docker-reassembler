// Copyright 2022 Advanced. All rights reserved.
// Package docker-reassembler
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package root

import (
	assembleCmd "docker-reassembler/cmd/assemble"
	utils "docker-reassembler/pkg/utils"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	s3Bucket               string
	region                 string
	debug, dryRun, verbose bool
	rootCmd                = &cobra.Command{
		Use:               "docker-reassembler",
		Aliases:           []string{"re"},
		Short:             "Reassembles a Docker images from S3 storage",
		PersistentPreRunE: rootPersistentPreRunE,
	}
)

func rootPersistentPreRunE(cmd *cobra.Command, args []string) error {
	if debug {
		pterm.EnableDebugMessages()
		pterm.Debug.Printfln("Debug mode enabled.")
	}
	if dryRun {
		pterm.Info.Printfln("Dry run mode enabled.")
	}

	return nil
}

func NewRootCmd() *cobra.Command {
	rootCmd.PersistentFlags().StringVarP(&s3Bucket, "s3-bucket", "b", "", "S3 bucket to migrate to")
	rootCmd.PersistentFlags().StringVarP(&region, "region", "", "eu-west-2", "AWS Region.")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug mode.")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "D", false, "Enable dry run mode.")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "", false, "Enable verbose mode.")
	utils.MarkFlagAsRequired(rootCmd, "s3-bucket", true)

	rootCmd.AddCommand(assembleCmd.NewAssembleCmd())

	return rootCmd
}
