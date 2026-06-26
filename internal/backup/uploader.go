package backup

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/odit-services/cnpg-plugin-pgdump/internal/config"
)

type Object struct {
	Key  string
	Size int64
}

type Uploader interface {
	Upload(ctx context.Context, localPath, key string) (int64, error)
	List(ctx context.Context, prefix string) ([]Object, error)
	Delete(ctx context.Context, key string) error
}

type S3Uploader struct {
	bucket string
	client *s3.Client
}

func NewS3Uploader(ctx context.Context, backupConfig config.BackupConfig) (*S3Uploader, error) {
	options := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(backupConfig.Region),
	}
	if backupConfig.AccessKeyID != "" || backupConfig.SecretAccessKey != "" {
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backupConfig.AccessKeyID,
			backupConfig.SecretAccessKey,
			"",
		)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if backupConfig.EndpointURL != "" {
			o.BaseEndpoint = aws.String(backupConfig.EndpointURL)
			o.UsePathStyle = true
		}
	})

	return &S3Uploader{bucket: backupConfig.Bucket, client: client}, nil
}

func (u *S3Uploader) Upload(ctx context.Context, localPath, key string) (int64, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	_, err = u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(cleanKey(key)),
		Body:   file,
	})
	if err != nil {
		return 0, fmt.Errorf("upload %s to s3://%s/%s: %w", localPath, u.bucket, key, err)
	}

	return stat.Size(), nil
}

func (u *S3Uploader) List(ctx context.Context, prefix string) ([]Object, error) {
	var objects []Object
	paginator := s3.NewListObjectsV2Paginator(u.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(u.bucket),
		Prefix: aws.String(cleanPrefix(prefix)),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, object := range page.Contents {
			if object.Key == nil {
				continue
			}
			objects = append(objects, Object{Key: *object.Key, Size: *object.Size})
		}
	}

	return objects, nil
}

func (u *S3Uploader) Delete(ctx context.Context, key string) error {
	_, err := u.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(cleanKey(key)),
	})
	return err
}

func ObjectKey(prefix, namespace, cluster, database, backupID string) string {
	return cleanKey(path.Join(prefix, namespace, cluster, database, backupID+".dump"))
}

func DatabasePrefix(prefix, namespace, cluster, database string) string {
	return cleanPrefix(path.Join(prefix, namespace, cluster, database))
}

func cleanKey(key string) string {
	return strings.TrimLeft(path.Clean("/"+strings.TrimSpace(key)), "/")
}

func cleanPrefix(prefix string) string {
	prefix = cleanKey(prefix)
	if prefix == "." || prefix == "" {
		return ""
	}
	return strings.TrimRight(prefix, "/") + "/"
}
