// Copyright 2022 Advanced. All rights reserved.
// Package build
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package build

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	lgr "docker-reassembler/pkg/logger"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pterm/pterm"
)

func Build(path, repository, tag, destinationPath string, createTar bool, logger lgr.ILogger) (v1.Image, error) {
	err := os.Chdir(path)
	if err != nil {
		return nil, fmt.Errorf("error changing directory: %w", err)
	}
	logger.Printfln(pterm.Info, "building local image with files found here: %s", path)

	tmpDir, err := os.MkdirTemp("/tmp", "built-docker-reassembler")
	if err != nil {
		return nil, fmt.Errorf("erroring creating temp dir in /tmp")
	}

	// defer os.RemoveAll(tmpDir)

	tarballPath := filepath.Join(tmpDir, strings.Join([]string{tag, "tar"}, "."))

	if createTar {
		filesToInclude := []string{}
		err := filepath.WalkDir("./", func(p string, d os.DirEntry, err error) error {
			if _, err := os.Stat(p); err == nil {
				if !d.IsDir() {
					filesToInclude = append(filesToInclude, p)
				}
			} else if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("%q does *not* exist: %w", p, err)
			} else {
				// Schrodinger: file may or may not exist. See err for details.

				return fmt.Errorf("Schrodinger: file may or may not exist. See err for details : %w", err)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error getting files to include: %w", err)
		}

		logger.Printfln(pterm.Info, "creating %q", tarballPath)
		logger.Printfln(pterm.Debug, "with files: %v", filesToInclude)
		err = createTarball(tarballPath, filesToInclude)
		if err != nil {
			return nil, err
		}
	}

	img, err := tarball.ImageFromPath(tarballPath, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating image from path: %w", err)
	}

	return img, err
}

func createTarball(path string, filepaths []string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating tarball %q: %w", path, err)
	}
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()

	for _, fp := range filepaths {
		err := addFileToTarWriter(fp, tarWriter)
		if err != nil {
			return fmt.Errorf("error adding file to tar writer: %w", err)
		}
	}

	return nil
}

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.New(fmt.Sprintf("could not open file '%s', got error '%s'", filePath, err.Error()))
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return errors.New(fmt.Sprintf("could not get stat for file '%s', got error '%s'", filePath, err.Error()))
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return errors.New(fmt.Sprintf("could not write header for file '%s', got error '%s'", filePath, err.Error()))
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return errors.New(fmt.Sprintf("could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error()))
	}

	return nil
}
