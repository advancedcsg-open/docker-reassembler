// Copyright 2022 Advanced. All rights reserved.
// Package logger
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package Logger

import (
	"github.com/aws/smithy-go/logging"
	"github.com/pterm/pterm"
)

type ILogger interface {
	Printfln(prefixPrinter pterm.PrefixPrinter, format string, args ...interface{})
	Logf(classification logging.Classification, format string, args ...interface{})
}
