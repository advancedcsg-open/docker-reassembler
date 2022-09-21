// Copyright 2022 Advanced. All rights reserved.
// Package utils
// Original author pennywisdom (pennywisdom@users.noreply.github.com).
package utils

import (
	"fmt"

	"github.com/aws/smithy-go/logging"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
)

type PtermLogger struct{}

func (l *PtermLogger) Logf(classification logging.Classification, message string, args ...interface{}) {
	pterm.Debug.Printfln(message, args...)
	prefixPrinter := pterm.Info
	if classification == logging.Debug {
		prefixPrinter = pterm.Debug
	} else if classification == logging.Warn {
		prefixPrinter = pterm.Warning
	}
	prefixPrinter.Printfln("classification: %s, message: %s", classification, fmt.Sprintf(message, args...))
}

func (l *PtermLogger) Printfln(prefixPrinter pterm.PrefixPrinter, message string, args ...interface{}) {
	prefixPrinter.Printfln(message, args...)
}

func MarkFlagAsRequired(cmd *cobra.Command, flagName string, persistent bool) {
	var err error
	if persistent {
		err = cmd.MarkPersistentFlagRequired(flagName)
	} else {
		err = cmd.MarkFlagRequired(flagName)
	}
	if err != nil {
		pterm.Fatal.Println(fmt.Errorf("Error marking flag %s as required: %w", flagName, err))
	}
}

func MarkFlagsRequiredTogether(cmd *cobra.Command, flagNames ...string) {
	cmd.MarkFlagsRequiredTogether(flagNames...)
}

func Banner(version string) {
	s, err := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle(
			"Reassembler",
			pterm.NewStyle(pterm.FgDarkGray))).Srender()
	if err != nil {
		pterm.Fatal.Print(err)
	}
	pterm.DefaultCenter.Println(s) // Print BigLetters with the default CenterPrinter
	pterm.DefaultCenter.Printfln("Docker Reassembler (%s) tool build by Advanced.", version)
	pterm.DefaultCenter.Printfln("Powered by hopes and dreams...")
}
