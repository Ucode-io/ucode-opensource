package vaultclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type redisClient struct {
	client     *redis.Client
	secretPath string
}

func newRedisClient(ctx context.Context, args NewClientArgs) (VaultClient, error) {
	if args.RedisAddress == "" {
		return nil, errors.New("redis address is required when using redis secrets provider")
	}

	opts := &redis.Options{
		Addr:     args.RedisAddress,
		Password: args.RedisPassword,
		DB:       args.RedisDB,
	}

	client := redis.NewClient(opts)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	secretPath := strings.Trim(args.SecretPath, "/")
	if secretPath == "" {
		return nil, errors.New("no secret path provided")
	}

	return redisClient{
		client:     client,
		secretPath: secretPath,
	}, nil
}

func (r redisClient) Provider() string {
	return ProviderRedis
}

func (r redisClient) RenewToken(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (r redisClient) Get(ctx context.Context, secretPath string) (map[string]any, error) {
	raw, err := r.client.Get(ctx, secretPath).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("secret not found at path %q", secretPath)
		}
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("failed to decode secret at %q: %w", secretPath, err)
	}

	return data, nil
}

func (r redisClient) Put(ctx context.Context, secretPath string, data map[string]any) error {
	if data == nil {
		return errors.New("secret data cannot be nil")
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal secret data: %w", err)
	}

	return r.client.Set(ctx, secretPath, payload, 0).Err()
}

func (r redisClient) GetSecretPath(context.Context) (string, error) {
	if r.secretPath == "" {
		return "", errors.New("no secret path provided")
	}

	return r.secretPath, nil
}
