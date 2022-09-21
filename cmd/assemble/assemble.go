// Copyright 2022 Advanced. All rights reserved.
// Package assemble
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package assemble

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	builder "docker-reassembler/pkg/build"
	"docker-reassembler/pkg/download"
	"docker-reassembler/pkg/upload"
	"docker-reassembler/pkg/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/dustin/go-humanize"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

type fs struct{}

func (f *fs) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (f *fs) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

var (
	s3Prefix          string
	repositoryName    string
	localPath         string
	tag               string
	remove            bool
	downloadOnly      bool
	noDownload        bool
	putRoleToAssume   string
	putRoleExternalId string
	layersPath        string
	buildLocal        bool
	assembleCmd       = &cobra.Command{
		Use:     "assemble",
		Aliases: []string{"a"},
		Short:   "Assemble a Docker image from layers stored in S3 Bucket",
		RunE:    runAssembleCmd,
	}
)

func NewAssembleCmd() *cobra.Command {
	assembleCmd.Flags().StringVarP(&s3Prefix, "s3-prefix", "p", "", "S3 object key to migrate to")
	assembleCmd.Flags().StringVarP(&repositoryName, "repository-name", "r", "", "repository name")
	assembleCmd.Flags().StringVarP(&localPath, "local-path", "l", "/tmp/docker-reassembler", "local directory path to save the image layers")
	assembleCmd.Flags().StringVarP(&tag, "tag", "t", "", "tag to apply to the image")
	assembleCmd.Flags().StringVarP(&putRoleToAssume, "put-role-to-assume", "P", "", "The IAM role to assume for ECR image put")
	assembleCmd.Flags().StringVarP(&putRoleExternalId, "put-role-external-id", "", "", "External Id for the assumed role")
	assembleCmd.Flags().BoolVarP(&remove, "rm", "", false, "remove downloaded assets after put")
	assembleCmd.Flags().BoolVarP(&downloadOnly, "download-only", "", false, "download image layers from S3 only")
	assembleCmd.Flags().BoolVarP(&noDownload, "no-download", "", false, "do not download any layers from s3 before uploading - expects layers to be locally available")
	assembleCmd.Flags().StringVarP(&layersPath, "layers-path", "", "", "local path to image layer files")
	assembleCmd.Flags().BoolVarP(&buildLocal, "build-local", "", false, "build the image locally")
	assembleCmd.MarkFlagsMutuallyExclusive("s3-prefix", "no-download")
	assembleCmd.MarkFlagsMutuallyExclusive("repository-name", "download-only")
	assembleCmd.MarkFlagsMutuallyExclusive("download-only", "no-download")
	assembleCmd.MarkFlagsMutuallyExclusive("download-only", "repository-name")
	assembleCmd.MarkFlagsMutuallyExclusive("download-only", "tag")
	assembleCmd.MarkFlagsMutuallyExclusive("download-only", "rm")
	return assembleCmd
}

func runAssembleCmd(cmd *cobra.Command, args []string) error {
	bucket := cmd.Parent().PersistentFlags().Lookup("s3-bucket").Value.String()

	imgTag := tag
	if imgTag == "" {
		imgTag = filepath.Base(s3Prefix)
	}

	region := cmd.Parent().PersistentFlags().Lookup("region")

	pterm.Debug.Printfln("********************************************************")
	pterm.Debug.Printfln("Region: %s", region.Value.String())
	pterm.Debug.Printfln("S3 Bucket: %s", bucket)
	pterm.Debug.Printfln("S3 Prefix: %s", s3Prefix)
	pterm.Debug.Printfln("Repository Name: %s", repositoryName)
	pterm.Debug.Printfln("Local Path: %s", localPath)
	pterm.Debug.Printfln("Tag: %s", imgTag)
	pterm.Debug.Printfln("********************************************************")

	logger := &utils.PtermLogger{}
	var downloadRes []string
	if !noDownload {
		dloader := download.NewDownloader()

		cfg, err := config.LoadDefaultConfig(context.TODO(),
			config.WithDefaultRegion(region.Value.String()),
			config.WithLogConfigurationWarnings(true),
			config.WithLogger(&utils.PtermLogger{}))
		if err != nil {
			return fmt.Errorf("assemble error: %w", err)
		}

		client := s3.NewFromConfig(cfg)
		manager := manager.NewDownloader(client)
		manager.Logger = logger

		pager := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
			Prefix: aws.String(s3Prefix),
		})

		downloadRes, err = dloader.Download(context.TODO(), download.S3DownloaderInput{
			Pager:          pager,
			Downloader:     manager,
			Filesystem:     &fs{},
			Bucket:         bucket,
			LocalDirectory: localPath,
			Logger:         logger,
		})
		if err != nil {
			return fmt.Errorf("error download docker layers from s3: %w", err)
		}

		if len(downloadRes) == 0 {
			pterm.Error.WithFatal(false).Printfln("no layers downloaded")
			return nil
		}
	}

	if downloadOnly {
		return nil
	}

	pathToLayers := layersPath
	if downloadRes != nil {
		pathToLayers = filepath.Dir(downloadRes[0])
	}

	if buildLocal {
		img, err := builder.Build(pathToLayers, repositoryName, imgTag, "/tmp/", true, logger)
		if err != nil {
			return fmt.Errorf("error building container image locally: %w", err)
		}
		size, err := img.Size()
		if err != nil {
			return fmt.Errorf("error getting local container image size: %w", err)
		}

		pterm.Success.Printfln("Container image was built locally (%s)", humanize.Bytes(uint64(size)))
	}

	ecrCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithDefaultRegion(region.Value.String()),
		config.WithLogConfigurationWarnings(true),
		config.WithLogger(&utils.PtermLogger{}))
	if err != nil {
		return fmt.Errorf("assemble error: %w", err)
	}

	// We now need to assume the role where we are putting image
	// https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/credentials/stscreds#hdr-Assume_Role
	stsClient := sts.NewFromConfig(ecrCfg)
	assumeCreds := stscreds.NewAssumeRoleProvider(stsClient,
		putRoleToAssume, func(aro *stscreds.AssumeRoleOptions) {
			if putRoleExternalId != "" {
				aro.ExternalID = &putRoleExternalId
			}
			aro.Duration = time.Minute * 60
		})

	ecrCfg.Credentials = aws.NewCredentialsCache(assumeCreds)

	ecrClient := ecr.NewFromConfig(ecrCfg)

	idOut, err := stsClient.GetCallerIdentity(context.TODO(), nil, func(o *sts.Options) {
		o.Credentials = ecrCfg.Credentials
	})
	if err != nil {
		return fmt.Errorf("error getting called identity: %w", err)
	}

	img, err := upload.Upload(context.TODO(), &upload.UploadInput{
		RepositoryName:  repositoryName,
		RegistryId:      *idOut.Account,
		ImageLayersPath: pathToLayers,
		Logger:          logger,
		Tag:             imgTag,
		Client:          ecrClient,
	})
	if err != nil {
		return fmt.Errorf("error uploading docker image to ECR: %w", err)
	}

	if remove {
		err = os.RemoveAll(filepath.Join(localPath, bucket, s3Prefix))
		if err != nil {
			pterm.Warning.Printfln("error removing %q", localPath)
		} else {
			pterm.Info.Printfln("%s removed", localPath)
		}
	}

	pterm.Success.Printfln("image %v successfully put to %s in registry with id %s",
		*img.ImageId.ImageTag, *img.RepositoryName, *idOut.Account)

	return nil
}
