package vaultclient

import (
	"context"
	"errors"
)

func (v vaultClient) Get(ctx context.Context, secretPath string) (map[string]any, error) {
	secret, err := v.client.KVv2("secret").Get(ctx, secretPath)
	if err != nil {
		return nil, err
	}

	return secret.Data, nil
}

func (v vaultClient) Put(ctx context.Context, secretPath string, data map[string]any) error {
	_, err := v.client.KVv2("secret").Put(ctx, secretPath, data)
	if err != nil {
		return err
	}

	return nil
}

func (v vaultClient) GetSecretPath(ctx context.Context) (string, error) {
	if len(v.secretPath) == 0 {
		return "", errors.New("no secret path provided")
	}
	return v.secretPath, nil
}
