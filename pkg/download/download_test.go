// Copyright 2022 Advanced. All rights reserved.
// Package docker-reassembler
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package download_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"docker-reassembler/pkg/download"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3man "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
)

type mockDownloadManager struct{}

var downloadFunc func(downloader download.IDownloadManager, localDirectory, bucket, key string) (int64, error)

func (m *mockDownloadManager) Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3man.Downloader)) (n int64, err error) {
	return downloadFunc(m, *input.Bucket, *input.Key, *input.Bucket)
}

type mockListObjectsV2Pager struct {
	PageNum  int
	Pages    []*s3.ListObjectsV2Output
	Contents []s3types.Object
}

var (
	nextPageFunc     func(ctx context.Context, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	hasMorePagesFunc func() bool
)

func (m *mockListObjectsV2Pager) NextPage(ctx context.Context, opts ...func(*s3.Options)) (output *s3.ListObjectsV2Output, err error) {
	return nextPageFunc(ctx, opts...)
}

func (m *mockListObjectsV2Pager) HasMorePages() bool {
	return hasMorePagesFunc()
}

type mockFs struct{}

var (
	fsCreateFunc   func(name string) (file *os.File, err error)
	fsMkdirAllFunc func(name string, perm os.FileMode) error
)

func (m *mockFs) Create(name string) (file *os.File, err error) {
	return fsCreateFunc(name)
}

func (m *mockFs) MkdirAll(name string, perm os.FileMode) error {
	return fsMkdirAllFunc(name, perm)
}

func TestDownloaderDownloadPagerError(t *testing.T) {
	cases := []struct {
		pager          download.IListObjectsV2Pager
		hasMorePagesFn func() bool
		nextPageFn     func(ctx context.Context, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	}{
		{
			pager:          &mockListObjectsV2Pager{},
			hasMorePagesFn: func() bool { return true },
			nextPageFn: func(ctx context.Context, options ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
				return nil, fmt.Errorf("error getting next page")
			},
		},
	}

	for _, tt := range cases {
		nextPageFunc = tt.nextPageFn
		hasMorePagesFunc = tt.hasMorePagesFn

		dl := download.NewDownloader()
		results, err := dl.Download(context.Background(), download.S3DownloaderInput{
			Pager: tt.pager,
		})

		assert.Equal(t, []string(nil), results, "results should be empty")
		assert.NotNil(t, err, "error should not be nil")
	}
}

func TestErrorDownloadingObjects(t *testing.T) {
	dl := download.NewDownloader()

	pager := &mockListObjectsV2Pager{
		Pages: []*s3.ListObjectsV2Output{
			{
				KeyCount: 1,
				Contents: []s3types.Object{
					{
						Key: aws.String("test-key"),
					},
				},
			},
		},
	}
	nextPageFunc = func(ctx context.Context, options ...func(*s3.Options)) (output *s3.ListObjectsV2Output, err error) {
		if pager.PageNum >= len(pager.Pages) {
			return nil, fmt.Errorf("no more pages")
		}
		output = pager.Pages[pager.PageNum]
		pager.PageNum++
		return output, nil
	}
	hasMorePagesFunc = func() bool { return pager.PageNum < len(pager.Pages) }
	downloadFunc = func(downloader download.IDownloadManager, localDirectory, bucket, key string) (int64, error) {
		return 0, fmt.Errorf("object not found")
	}
	fsCreateFunc = func(name string) (file *os.File, err error) {
		return &os.File{}, nil
	}
	fsMkdirAllFunc = func(name string, perm os.FileMode) error {
		return nil
	}

	results, err := dl.Download(context.Background(), download.S3DownloaderInput{
		Pager:      pager,
		Downloader: &mockDownloadManager{},
		Filesystem: &mockFs{},
	})

	assert.Equal(t, []string(nil), results, "results should be empty")
	assert.NotNil(t, err, "error should not be nil")
}

func TestDownloadErrorMkdir(t *testing.T) {
	dl := download.NewDownloader()

	pager := &mockListObjectsV2Pager{
		Pages: []*s3.ListObjectsV2Output{
			{
				KeyCount: 1,
				Contents: []s3types.Object{
					{
						Key: aws.String("test-key"),
					},
				},
			},
		},
	}
	nextPageFunc = func(ctx context.Context, options ...func(*s3.Options)) (output *s3.ListObjectsV2Output, err error) {
		if pager.PageNum >= len(pager.Pages) {
			return nil, fmt.Errorf("no more pages")
		}
		output = pager.Pages[pager.PageNum]
		pager.PageNum++
		return output, nil
	}
	hasMorePagesFunc = func() bool { return pager.PageNum < len(pager.Pages) }
	fsMkdirAllFunc = func(name string, perm os.FileMode) error {
		return fmt.Errorf("error creating directory")
	}
	downloadFunc = func(downloader download.IDownloadManager, localDirectory, bucket, key string) (int64, error) {
		return 0, fmt.Errorf("error creating directory")
	}

	results, err := dl.Download(context.Background(), download.S3DownloaderInput{
		Pager:          pager,
		Downloader:     &mockDownloadManager{},
		Bucket:         "test-bucket",
		LocalDirectory: "./testdata",
		Filesystem:     &mockFs{},
	})

	assert.Equal(t, []string(nil), results)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "failed to download object: failed to create directories for file: error creating directory")
}

func TestDownloadErrorFsCreate(t *testing.T) {
	dl := download.NewDownloader()

	pager := &mockListObjectsV2Pager{
		Pages: []*s3.ListObjectsV2Output{
			{
				KeyCount: 1,
				Contents: []s3types.Object{
					{
						Key: aws.String("test-key"),
					},
				},
			},
		},
	}
	nextPageFunc = func(ctx context.Context, options ...func(*s3.Options)) (output *s3.ListObjectsV2Output, err error) {
		if pager.PageNum >= len(pager.Pages) {
			return nil, fmt.Errorf("no more pages")
		}
		output = pager.Pages[pager.PageNum]
		pager.PageNum++
		return output, nil
	}
	hasMorePagesFunc = func() bool { return pager.PageNum < len(pager.Pages) }
	fsMkdirAllFunc = func(name string, perm os.FileMode) error {
		return nil
	}
	fsCreateFunc = func(name string) (file *os.File, err error) {
		return nil, fmt.Errorf("permission error")
	}
	downloadFunc = func(downloader download.IDownloadManager, localDirectory, bucket, key string) (int64, error) {
		return 0, fmt.Errorf("error creating file")
	}

	results, err := dl.Download(context.Background(), download.S3DownloaderInput{
		Pager:          pager,
		Downloader:     &mockDownloadManager{},
		Bucket:         "test-bucket",
		LocalDirectory: "./testdata",
		Filesystem:     &mockFs{},
	})

	assert.Equal(t, []string(nil), results)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "failed to download object: failed to create file: permission error")
}

func TestDownload(t *testing.T) {
	dl := download.NewDownloader()

	pager := &mockListObjectsV2Pager{
		Pages: []*s3.ListObjectsV2Output{
			{
				KeyCount: 1,
				Contents: []s3types.Object{
					{
						Key: aws.String("test-key"),
					},
				},
			},
		},
	}
	nextPageFunc = func(ctx context.Context, options ...func(*s3.Options)) (output *s3.ListObjectsV2Output, err error) {
		if pager.PageNum >= len(pager.Pages) {
			return nil, fmt.Errorf("no more pages")
		}
		output = pager.Pages[pager.PageNum]
		pager.PageNum++
		return output, nil
	}
	hasMorePagesFunc = func() bool { return pager.PageNum < len(pager.Pages) }
	downloadFunc = func(downloader download.IDownloadManager, localDirectory, bucket, key string) (int64, error) {
		return 123, nil
	}
	fsCreateFunc = func(name string) (file *os.File, err error) {
		return &os.File{}, nil
	}
	fsMkdirAllFunc = func(name string, perm os.FileMode) error {
		return nil
	}

	results, err := dl.Download(context.Background(), download.S3DownloaderInput{
		Pager:      pager,
		Downloader: &mockDownloadManager{},
		Bucket:     "test-bucket",
		Filesystem: &mockFs{},
	})

	assert.Equal(t, []string{"test-bucket/test-key"}, results)
	assert.Nil(t, err)
}
