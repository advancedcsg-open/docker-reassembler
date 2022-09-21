// Copyright 2022 Advanced. All rights reserved.
// Package docker-reassembler
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package main

import (
	"fmt"

	rootCmd "docker-reassembler/cmd/root"
	utils "docker-reassembler/pkg/utils"

	"github.com/pterm/pterm"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

func main() {
	pterm.DisableColor()

	rCmd := rootCmd.NewRootCmd()
	rCmd.Version = fmt.Sprintf(
		`docker-reassembler %s, commit %s, built at %s by %s`,
		version, commit, date, builtBy)

	utils.Banner(version)
	if err := rCmd.Execute(); err != nil {
		pterm.Fatal.WithShowLineNumber().Printfln("Error running docker-reassembler: %v", err)
	}

	// dl := download.DownloadProps{
	// 	Context:          nil,
	// 	S3ClientAPI:      nil,
	// 	S3ManagerAPI:     nil,
	// 	FileCreator:      nil,
	// 	MkDirCreator:     nil,
	// 	S3Bucket:         "",
	// 	S3Prefix:         "",
	// 	Region:           "",
	// 	LocalDestination: "",
	// }
}
