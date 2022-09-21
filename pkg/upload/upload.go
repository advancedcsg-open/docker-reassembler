// Copyright 2022 Advanced. All rights reserved.
// Package upload
// Original author pennywisdom (pennywisdom@users.noreply.github.com).

package upload

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"

	dkr "docker-reassembler/pkg/docker"
	lgr "docker-reassembler/pkg/logger"

	man "github.com/containers/image/v5/manifest"
	"github.com/dustin/go-humanize"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	orderedmap "github.com/elliotchance/orderedmap/v2"
	"github.com/pterm/pterm"
)

type IClient interface {
	InitiateLayerUpload(ctx context.Context, params *ecr.InitiateLayerUploadInput,
		optFns ...func(*ecr.Options)) (*ecr.InitiateLayerUploadOutput, error)

	UploadLayerPart(ctx context.Context, params *ecr.UploadLayerPartInput,
		optFns ...func(*ecr.Options)) (*ecr.UploadLayerPartOutput, error)

	CompleteLayerUpload(ctx context.Context, params *ecr.CompleteLayerUploadInput,
		optFns ...func(*ecr.Options)) (*ecr.CompleteLayerUploadOutput, error)

	PutImage(ctx context.Context, params *ecr.PutImageInput,
		optFns ...func(*ecr.Options)) (*ecr.PutImageOutput, error)

	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput,
		optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)

	CreateRepository(ctx context.Context, params *ecr.CreateRepositoryInput,
		optFns ...func(*ecr.Options)) (*ecr.CreateRepositoryOutput, error)
}

type IUploadInput interface {
	Upload(ctx context.Context, input UploadInput) error
}

type UploadInput struct {
	Client          IClient
	RepositoryName  string
	RegistryId      string
	ImageLayersPath string
	Tag             string
	RoleToAssume    string
	Logger          lgr.ILogger
}

func Upload(ctx context.Context, input *UploadInput) (*ecrTypes.Image, error) {
	manBuffer, err := ioutil.ReadFile(filepath.Join(input.ImageLayersPath, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("error reading manifest file: %w", err)
	}

	manifest, err := dkr.FromBlob(manBuffer, input.Logger)
	if err != nil {
		return nil, fmt.Errorf("error parsing manifest from blob: %w", err)
	}

	if len(manifest.LayerInfos())+1 > 100 {
		return nil, fmt.Errorf("too many layers (100 max): %d - https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_PutImage.html",
			len(manifest.LayerInfos())+1)
	}

	descOut, createOut, err := checkRepo(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("error checking Repository: %w", err)
	}
	if descOut != nil {
		// Only 1 repo will be returned
		repo := descOut.Repositories[0]
		input.Logger.Printfln(pterm.Info, "existing repository found: %s", *repo.RepositoryName)
		input.Logger.Printfln(pterm.Debug, "repository arn: %s", *repo.RepositoryArn)
		input.Logger.Printfln(pterm.Debug, "repository uri: %s", *repo.RepositoryUri)
	}
	if createOut != nil {
		input.Logger.Printfln(pterm.Success, "new repository created: %s", *createOut.Repository.RepositoryName)
		input.Logger.Printfln(pterm.Debug, "repository arn: %s", *createOut.Repository.RepositoryArn)
		input.Logger.Printfln(pterm.Debug, "repository uri: %s", *createOut.Repository.RepositoryUri)
	}

	// Upload layer parts after reading manifest
	err = uploadLayerParts(ctx, input, manifest)
	if err != nil {
		return nil, fmt.Errorf("error uploading layer parts: %w", err)
	}

	// Put image (manifest)
	image, err := putEcrImage(ctx, input, manifest)
	if err != nil {
		return nil, fmt.Errorf("error putting image: %w", err)
	}

	return image, nil
}

func checkRepo(ctx context.Context, input *UploadInput) (
	*ecr.DescribeRepositoriesOutput, *ecr.CreateRepositoryOutput, error,
) {
	descOut, err := input.Client.DescribeRepositories(context.TODO(), &ecr.DescribeRepositoriesInput{
		RegistryId:      aws.String(input.RegistryId),
		RepositoryNames: []string{input.RepositoryName},
	})
	if err != nil {
		var notFoundEx *ecrTypes.RepositoryNotFoundException
		if errors.As(err, &notFoundEx) {
			// Create new repository
			createOut, cErr := input.Client.CreateRepository(ctx, &ecr.CreateRepositoryInput{
				RepositoryName: aws.String(input.RepositoryName),
				EncryptionConfiguration: &ecrTypes.EncryptionConfiguration{
					EncryptionType: ecrTypes.EncryptionTypeKms,
					// Use default AWS KMS key
					// KmsKey:         new(string),
				},
				ImageScanningConfiguration: &ecrTypes.ImageScanningConfiguration{
					ScanOnPush: true,
				},
				ImageTagMutability: ecrTypes.ImageTagMutabilityImmutable,
				RegistryId:         aws.String(input.RegistryId),
				Tags: []ecrTypes.Tag{
					{
						Key:   aws.String("bu"),
						Value: aws.String("corporate"),
					},
					{
						Key:   aws.String("div"),
						Value: aws.String("coe"),
					},
					{
						Key:   aws.String("proj"),
						Value: aws.String("adc"),
					},
				},
			})

			if cErr != nil {
				return nil, nil, fmt.Errorf("error creating repository: %w", err)
			}

			return nil, createOut, nil
		} else {
			return nil, nil, fmt.Errorf("error describing repositories: %w", err)
		}
	}

	if len(descOut.Repositories) != 1 {
		return nil, nil, fmt.Errorf(
			"invalid number of repositories found (%d), expected 1",
			len(descOut.Repositories))
	}
	return descOut, nil, nil
}

func initLayerUpload(ctx context.Context, input *UploadInput) (*ecr.InitiateLayerUploadOutput, error) {
	return input.Client.InitiateLayerUpload(ctx, &ecr.InitiateLayerUploadInput{
		RepositoryName: aws.String(input.RepositoryName),
		RegistryId:     aws.String(input.RegistryId),
	}, func(ops *ecr.Options) {
		ops.Logger = input.Logger
	})
}

func uploadLayerParts(ctx context.Context, input *UploadInput, manifest man.Manifest) error {
	input.Logger.Printfln(pterm.Info, "uploading layer parts, depending on your connect, this might take some time")
	input.Logger.Printfln(pterm.Info, "uploading config layer with digest: %s", manifest.ConfigInfo().Digest.String())
	err := doUploadLayerParts(ctx, input.Client, input, manifest.ConfigInfo().Digest.String())
	if err != nil {
		return fmt.Errorf("error uploading config layer: %w", err)
	}
	for _, layer := range manifest.LayerInfos() {
		input.Logger.Printfln(pterm.Info, "uploading layer with digest: %s", layer.Digest.String())
		err = doUploadLayerParts(ctx, input.Client, input, layer.Digest.String())
		if err != nil {
			return fmt.Errorf("error uploading layer: %w", err)
		}
	}

	return nil
}

func doUploadLayerParts(ctx context.Context, client IClient, input *UploadInput, digest string) error {
	initOut, err := initLayerUpload(ctx, input)
	if err != nil {
		return fmt.Errorf("error initiating layer upload: %w", err)
	}

	layerName := strings.Replace(digest, ":", "__", 1)
	blobPath := filepath.Join(input.ImageLayersPath, strings.Replace(digest, ":", "__", 1))
	fi, err := os.Stat(blobPath)
	if err != nil {
		return fmt.Errorf("error reading file info for %s", blobPath)
	}

	input.Logger.Printfln(pterm.Debug, "*********************************************")
	input.Logger.Printfln(pterm.Debug, "uploadId: %q", *initOut.UploadId)
	input.Logger.Printfln(pterm.Debug, "layerName: %q", layerName)
	input.Logger.Printfln(pterm.Debug, "layer digest: %s", digest)
	input.Logger.Printfln(pterm.Debug, "blobPath: %s", blobPath)
	input.Logger.Printfln(pterm.Debug, "layer size: %d bytes", fi.Size())

	blobs, err := layerBlobParts(
		ctx, digest, blobPath, input.Logger)
	if err != nil {
		return fmt.Errorf("error getting layer parts: %w", err)
	}

	firstPart := 0

	input.Logger.Printfln(pterm.Info, "Uploading %d layer parts", len(blobs.Keys()))
	for _, key := range blobs.Keys() {
		v, ok := blobs.Get(key)
		if !ok {
			return fmt.Errorf("error getting blob")
		}
		spinnerInfo, err := pterm.DefaultSpinner.Start(fmt.Sprintf("uploading blob %s (%s)", key, humanize.Bytes(uint64(len(v)))))
		if err != nil {
			return fmt.Errorf("error starting spinner: %w", err)
		}

		input.Logger.Printfln(pterm.Debug, "blobPart size: %s (%d bytes)",
			humanize.IBytes(uint64(len(v))), len(v))
		input.Logger.Printfln(pterm.Debug, "blobPart firstPart: %d", firstPart)
		input.Logger.Printfln(pterm.Debug, "blobPart lastPart: %d", (firstPart + (len(v) - 1)))
		input.Logger.Printfln(pterm.Debug, "uploadId: %s", *initOut.UploadId)

		output, err := client.UploadLayerPart(ctx, &ecr.UploadLayerPartInput{
			LayerPartBlob:  v,
			PartFirstByte:  aws.Int64(int64(firstPart)),
			PartLastByte:   aws.Int64(int64(firstPart + (len(v) - 1))),
			RepositoryName: aws.String(input.RepositoryName),
			UploadId:       initOut.UploadId,
			RegistryId:     aws.String(input.RegistryId),
		})
		if err != nil {
			spinnerInfo.Fail()
			return fmt.Errorf("upload layer part error: %w", err)
		}

		// Update firstpart for next iteration
		firstPart += len(v)

		input.Logger.Printfln(pterm.Debug, "last layer part byte received %d", output.LastByteReceived)
		input.Logger.Printfln(pterm.Debug, "*********************************************")
		spinnerInfo.Success()
	}

	_, err = completeLayerUpload(ctx, input,
		initOut.UploadId, []string{digest})

	var existsEx *ecrTypes.LayerAlreadyExistsException
	if errors.As(err, &existsEx) {
		input.Logger.Printfln(pterm.Warning, "complete layer part upload: %s", existsEx)
	} else {
		return err
	}

	return nil
}

func completeLayerUpload(ctx context.Context, input *UploadInput,
	uploadId *string, layerDigest []string,
) (string, error) {
	output, err := input.Client.CompleteLayerUpload(ctx, &ecr.CompleteLayerUploadInput{
		LayerDigests:   layerDigest,
		RepositoryName: aws.String(input.RepositoryName),
		UploadId:       uploadId,
		RegistryId:     aws.String(input.RegistryId),
	})
	if err != nil {
		var digestEx *ecrTypes.InvalidLayerException
		if errors.As(err, &digestEx) {
			return "", fmt.Errorf("completeLayerUpload invalid layer: %w\nlayer digest: %v", digestEx, layerDigest)
		} else {
			return "", fmt.Errorf("completeLayerUpload: %w\nlayer digest: %v", err, layerDigest)
		}
	}

	return *output.LayerDigest, nil
}

func putEcrImage(ctx context.Context, input *UploadInput, manifest man.Manifest,
) (*ecrTypes.Image, error) {
	manifestPath := filepath.Join(input.ImageLayersPath, "manifest.json")
	manBuffer, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %q", manifestPath)
	}

	if int64(len(manBuffer)) > dkr.IMAGE_MANIFEST_MAX_SIZE {
		return nil, fmt.Errorf("image manifest too large, %d is greated than %d", len(manBuffer), dkr.IMAGE_MANIFEST_MAX_SIZE)
	}

	output, err := input.Client.PutImage(ctx, &ecr.PutImageInput{
		ImageManifest:  aws.String(string(manBuffer)),
		RepositoryName: aws.String(input.RepositoryName),
		// ImageDigest only for existing images (I think)
		// ImageDigest:            aws.String(manifest.ConfigInfo().Digest.String()),
		// ImageManifestMediaType: aws.String(manifest.ConfigInfo().MediaType),
		ImageTag:   aws.String(input.Tag),
		RegistryId: aws.String(input.RegistryId),
	})
	if err != nil {
		return nil, fmt.Errorf("put image error: %w", err)
	}

	return output.Image, nil
}

func layerBlobParts(ctx context.Context, digest string, blobPath string,
	logger lgr.ILogger,
) (*orderedmap.OrderedMap[string, []byte], error) {
	file, err := os.Open(blobPath)
	if err != nil {
		return nil, fmt.Errorf("error reading %q: %w", blobPath, err)
	}

	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error with file.Stat(): %w", err)
	}

	var fileSize int64 = fileInfo.Size()

	const fileChunk = dkr.LAYER_PART_MAX_SIZE

	// Calculate total number of parts the file will be chunked into
	totalPartsNum := uint64(math.Ceil(float64(fileSize) / float64(fileChunk)))

	logger.Printfln(pterm.Debug, fmt.Sprintf("splitting %q into %d chunks.", blobPath, totalPartsNum))
	blobsOrdered := orderedmap.NewOrderedMap[string, []byte]()

	for i := uint64(0); i < totalPartsNum; i++ {

		partSize := int(math.Min(float64(fileChunk), float64(fileSize-int64(int64(i)*fileChunk))))
		partBuffer := make([]byte, partSize)

		_, err := file.Read(partBuffer)
		if err != nil {
			return nil, fmt.Errorf("error reading partBuffer: %w", err)
		}

		blobsOrdered.Set(fmt.Sprintf("part_%d", i), partBuffer)

	}

	return blobsOrdered, nil
}
