// Copyright 2022 Advanced. All rights reserved.
// Package docker-reassembler
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package docker

const (
	// Even though the documentation says 20Mb or 20971520 bytes
	// we have erratic results, so reducing to 10Mb or 10485760
	// to give some leeway
	LAYER_PART_MAX_SIZE     int64 = 10485760
	IMAGE_MANIFEST_MAX_SIZE int64 = 4194304
)
