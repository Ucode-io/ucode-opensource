package vault

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	vaultapi "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/approle"
)

type VaultClient interface {
	Get(ctx context.Context, secretPath string) (map[string]any, error)
	Put(ctx context.Context, secretPath string, data map[string]any) error
	Delete(ctx context.Context, secretPath string) error
}

type Config struct {
	Address   string
	RoleID    string
	SecretID  string
	MountPath string
}

type vaultClient struct {
	client    *vaultapi.Client
	roleID    string
	secretID  string
	mountPath string
}

func New(ctx context.Context, cfg Config) (VaultClient, error) {
	if cfg.Address == "" {
		return nil, errors.New("vault address is required")
	}

	apiCfg := vaultapi.DefaultConfig()
	apiCfg.Address = strings.TrimRight(cfg.Address, "/")

	client, err := vaultapi.NewClient(apiCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize vault client: %w", err)
	}

	vc := &vaultClient{
		client:    client,
		roleID:    cfg.RoleID,
		secretID:  cfg.SecretID,
		mountPath: cfg.MountPath,
	}

	if _, err := vc.login(ctx); err != nil {
		return nil, fmt.Errorf("vault login failed: %w", err)
	}

	go func() {
		for {
			loginResp, err := vc.login(ctx)
			if err != nil {
				log.Printf("vault: re-login failed: %v, retrying...", err)
				continue
			}
			if err := vc.manageTokenLifecycle(loginResp); err != nil {
				log.Printf("vault: token lifecycle error: %v", err)
			}
		}
	}()

	return vc, nil
}

func (v *vaultClient) login(ctx context.Context) (*vaultapi.Secret, error) {
	appRoleAuth, err := auth.NewAppRoleAuth(
		v.roleID,
		&auth.SecretID{FromString: v.secretID},
		auth.WithMountPath(v.mountPath),
	)
	if err != nil {
		return nil, err
	}

	authInfo, err := v.client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return nil, err
	}
	if authInfo == nil {
		return nil, errors.New("no auth info returned from vault")
	}

	return authInfo, nil
}

// manageTokenLifecycle uses Vault's LifetimeWatcher to renew the token automatically.
// Returns nil when token can no longer be renewed (caller should re-login).
func (v *vaultClient) manageTokenLifecycle(token *vaultapi.Secret) error {
	if !token.Auth.Renewable {
		return nil
	}

	watcher, err := v.client.NewLifetimeWatcher(&vaultapi.LifetimeWatcherInput{
		Secret:    token,
		Increment: 3600,
	})
	if err != nil {
		return fmt.Errorf("unable to initialize lifetime watcher: %w", err)
	}

	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case err := <-watcher.DoneCh():
			if err != nil {
				log.Printf("vault: failed to renew token: %v. Re-attempting login.", err)
			} else {
				log.Printf("vault: token reached max TTL. Re-attempting login.")
			}
			return nil
		case renewal := <-watcher.RenewCh():
			log.Printf("vault: token successfully renewed: %v", renewal.Secret.Auth.ClientToken[:8]+"...")
		}
	}
}

func (v *vaultClient) Get(ctx context.Context, secretPath string) (map[string]any, error) {
	secret, err := v.client.KVv2("secret").Get(ctx, secretPath)
	if err != nil {
		return nil, err
	}
	return secret.Data, nil
}

func (v *vaultClient) Put(ctx context.Context, secretPath string, data map[string]any) error {
	_, err := v.client.KVv2("secret").Put(ctx, secretPath, data)
	return err
}

func (v *vaultClient) Delete(ctx context.Context, secretPath string) error {
	return v.client.KVv2("secret").Delete(ctx, secretPath)
}
