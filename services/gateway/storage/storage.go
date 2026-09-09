package storage

import (
	"context"
	"errors"
	"time"
)

var ErrorTheSameId = errors.New("cannot use the same uuid for 'id' and 'parent_id' fields")
var ErrorProjectId = errors.New("not valid 'project_id'")

type StorageI interface {
	CloseDB()
}

type RedisStorageI interface {
	SetX(ctx context.Context, key string, value string, duration time.Duration, projectId string, nodeType string) error
	Get(ctx context.Context, key string, projectId string, nodeType string) (string, error)
	Del(ctx context.Context, key string, projectId string, nodeType string) error
	Set(ctx context.Context, key string, value any, duration time.Duration, projectId string, nodeType string) error
	DelMany(ctx context.Context, keys []string, projectId string, nodeType string) error
}
