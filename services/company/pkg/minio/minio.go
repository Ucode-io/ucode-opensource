package minio

import (
	"bytes"
	"context"
	"net/url"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const presignedURLExpiry = 7 * 24 * time.Hour

type Client struct {
	mc     *minio.Client
	bucket string
}

// NewClient returns a nil client when MinIO is not configured (empty endpoint),
// so the service can start without object storage and surface a clear error
// only when an export is actually requested.
func NewClient(cfg config.BaseConfig) (*Client, error) {
	if cfg.Minio.Endpoint == "" {
		return nil, nil
	}

	mc, err := minio.New(cfg.Minio.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
		Secure: cfg.Minio.UseSSL,
	})
	if err != nil {
		return nil, err
	}

	return &Client{mc: mc, bucket: cfg.Minio.Bucket}, nil
}

func (c *Client) ensureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
}

func (c *Client) Upload(ctx context.Context, objectName, contentType string, data []byte) (string, error) {
	if err := c.ensureBucket(ctx); err != nil {
		return "", err
	}

	_, err := c.mc.PutObject(ctx, c.bucket, objectName, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}

	presigned, err := c.mc.PresignedGetObject(ctx, c.bucket, objectName, presignedURLExpiry, make(url.Values))
	if err != nil {
		return "", err
	}

	return presigned.String(), nil
}
