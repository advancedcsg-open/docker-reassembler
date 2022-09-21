// Copyright 2022 Advanced. All rights reserved.
// Package upload_test
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package upload_test

import (
	"context"
	"testing"

	"docker-reassembler/pkg/upload"

	"github.com/stretchr/testify/assert"
)

func TestUpload(t *testing.T) {
	img, err := upload.Upload(context.TODO(), &upload.UploadInput{})
	assert.NotNil(t, img)
	assert.Nil(t, err)
}
