// Copyright 2022 Advanced. All rights reserved.
// Package docker-reassembler
// Original author pennywisdom (pennywisdom@users.noreply.github.com).
package docker

import (
	"fmt"

	lgr "docker-reassembler/pkg/logger"

	man "github.com/containers/image/v5/manifest"
	"github.com/pterm/pterm"
)

func FromBlob(manifestBlob []byte, logger lgr.ILogger) (man.Manifest, error) {
	mimeType := man.GuessMIMEType(manifestBlob)
	if mimeType == "" {
		return nil, fmt.Errorf("MIME type is unknown or unrecognised")
	}

	parsed, err := man.FromBlob(manifestBlob, mimeType)
	if err != nil {
		return nil, fmt.Errorf("error parsing image from blob: %w", err)
	}

	logger.Printfln(pterm.Debug, fmt.Sprintf("manifest MIMEType: %q", parsed.ConfigInfo().MediaType))

	return parsed, nil
}
