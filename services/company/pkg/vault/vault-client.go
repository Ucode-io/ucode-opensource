package vaultclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

const (
	ProviderVault = "vault"
	ProviderRedis = "redis"
)

type VaultClient interface {
	Provider() string
	RenewToken(ctx context.Context) error
	Get(ctx context.Context, secretPath string) (map[string]any, error)
	Put(ctx context.Context, secretPath string, data map[string]any) error
	GetSecretPath(ctx context.Context) (string, error)
}

type secretClient struct {
	requestedProvider string
	activeProvider    string
	active            VaultClient
	vault             VaultClient
	redis             VaultClient
}

func (s *secretClient) Provider() string {
	return s.activeProvider
}

func (s *secretClient) RenewToken(ctx context.Context) error {
	if s.active == nil {
		return errors.New("no active secrets provider configured")
	}

	if s.activeProvider != ProviderVault {
		return nil
	}

	return s.active.RenewToken(ctx)
}

func (s *secretClient) Get(ctx context.Context, secretPath string) (map[string]any, error) {
	if s.active == nil {
		return nil, errors.New("no active secrets provider configured")
	}

	return s.active.Get(ctx, secretPath)
}

func (s *secretClient) Put(ctx context.Context, secretPath string, data map[string]any) error {
	if s.active == nil {
		return errors.New("no active secrets provider configured")
	}

	return s.active.Put(ctx, secretPath, data)
}

func (s *secretClient) GetSecretPath(ctx context.Context) (string, error) {
	if s.active == nil {
		return "", errors.New("no active secrets provider configured")
	}

	return s.active.GetSecretPath(ctx)
}

func (s *secretClient) RequestedProvider() string {
	return s.requestedProvider
}

func (s *secretClient) setActive(store VaultClient) {
	if store == nil {
		return
	}

	s.active = store
	s.activeProvider = store.Provider()
}

type vaultClient struct {
	client     *vault.Client
	roleID     string
	secretID   string
	mountPath  string
	secretPath string
}

func (v vaultClient) Provider() string {
	return ProviderVault
}

type NewClientArgs struct {
	Provider   string
	Address    string
	RoleID     string
	SecretID   string
	MountPath  string
	SecretPath string

	RedisAddress  string
	RedisPassword string
	RedisDB       int
}

func NewClient(ctx context.Context, args NewClientArgs) (VaultClient, error) {
	requested := strings.ToLower(strings.TrimSpace(args.Provider))
	if requested == "" {
		requested = ProviderVault
	}

	client := &secretClient{
		requestedProvider: requested,
	}

	var vaultErr error
	if args.Address != "" {
		client.vault, vaultErr = newVaultClient(args)
	} else if requested == ProviderVault {
		vaultErr = errors.New("vault address is empty")
	}

	var redisErr error
	if args.RedisAddress != "" {
		client.redis, redisErr = newRedisClient(ctx, args)
	} else if requested == ProviderRedis {
		redisErr = errors.New("redis address is empty")
	}

	switch requested {
	case ProviderVault:
		if client.vault != nil {
			client.setActive(client.vault)
		} else if client.redis != nil {
			client.setActive(client.redis)
		} else {
			if vaultErr != nil {
				return nil, fmt.Errorf("vault provider init failed: %w", vaultErr)
			}
			if redisErr != nil {
				return nil, fmt.Errorf("redis fallback init failed: %w", redisErr)
			}
			return nil, errors.New("no secrets provider configured")
		}
	case ProviderRedis:
		if client.redis != nil {
			client.setActive(client.redis)
		} else {
			if redisErr != nil {
				return nil, fmt.Errorf("redis provider init failed: %w", redisErr)
			}
			return nil, errors.New("redis provider requested but not configured")
		}
	default:
		return nil, fmt.Errorf("unsupported secrets provider %q", requested)
	}

	return client, nil
}

func newVaultClient(args NewClientArgs) (VaultClient, error) {
	config := vault.DefaultConfig()
	config.Address = args.Address

	client, err := vault.NewClient(config)
	if err != nil {
		return nil, err
	}

	return vaultClient{
		client:     client,
		roleID:     args.RoleID,
		secretID:   args.SecretID,
		mountPath:  args.MountPath,
		secretPath: args.SecretPath,
	}, nil
}
