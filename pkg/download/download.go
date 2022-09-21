// Copyright 2022 Advanced. All rights reserved.
// Package download
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package download

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	lgr "docker-reassembler/pkg/logger"

	s3man "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	humanize "github.com/dustin/go-humanize"
	"github.com/pterm/pterm"
)

type IS3Downloader interface {
	Download(ctx context.Context, input S3DownloaderInput) (results []string, err error)
}

type IListObjectsV2Pager interface {
	HasMorePages() bool
	NextPage(context.Context, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type IDownloadManager interface {
	Download(ctx context.Context, w io.WriterAt, input *s3.GetObjectInput,
		options ...func(*s3man.Downloader)) (n int64, err error)
}

type IFileSystem interface {
	Create(name string) (file *os.File, err error)
	MkdirAll(name string, perm os.FileMode) error
}

type (
	S3Downloader      struct{}
	S3DownloaderInput struct {
		Pager          IListObjectsV2Pager
		Downloader     IDownloadManager
		Filesystem     IFileSystem
		Bucket         string
		LocalDirectory string
		Logger         lgr.ILogger
	}
	osFS struct{}
)

func (osFS) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (osFS) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

func (d *S3Downloader) Download(ctx context.Context, input S3DownloaderInput) (
	results []string, err error,
) {
	downloaded := []string{}
	for input.Pager.HasMorePages() {
		page, err := input.Pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}
		for _, object := range page.Contents {
			if _, err := downloadToFile(input.Downloader, input.LocalDirectory, input.Bucket,
				*object.Key, input.Filesystem, input.Logger); err != nil {
				return nil, fmt.Errorf("failed to download object: %w", err)
			}
			downloaded = append(downloaded, filepath.Join(input.LocalDirectory, input.Bucket, *object.Key))
		}

	}

	return downloaded, nil
}

func NewDownloader() S3Downloader {
	return S3Downloader{}
}

func downloadToFile(downloader IDownloadManager, localDirectory, bucket, key string, fs IFileSystem, logger lgr.ILogger) (int64, error) {
	// Create the directories in the path
	file := filepath.Join(localDirectory, bucket, key)
	if err := fs.MkdirAll(filepath.Dir(file), 0o775); err != nil {
		return 0, fmt.Errorf("failed to create directories for file: %w", err)
	}

	// Set up the local file
	fd, err := fs.Create(file)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer fd.Close()

	size, err := downloader.Download(context.TODO(),
		fd,
		&s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return 0, fmt.Errorf("failed to download file: %w", err)
	}

	if logger != nil {
		logger.Printfln(pterm.Info, "Downloaded %s (%s)", fd.Name(), humanize.Bytes(uint64(size)))
	}

	return size, nil
}
