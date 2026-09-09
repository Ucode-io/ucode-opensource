package service

import (
	"context"
	"encoding/base64"
	"encoding/json"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	vaultclient "github.com/Ucode-io/ucode-opensource/services/company/pkg/vault"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
)

// AdminService ...
type AirbyteService struct {
	storage storage.StorageI
	logger  l.Logger
	vault   vaultclient.VaultClient
	pb.UnimplementedAirbyteServiceServer
}

func NewAirbyteService(strg storage.StorageI, log l.Logger, vault vaultclient.VaultClient) *AirbyteService {
	return &AirbyteService{
		storage: strg,
		logger:  log,
		vault:   vault,
	}
}

func (a *AirbyteService) GetById(ctx context.Context, req *pb.GetAirbyteByIdRequest) (*pb.Airbyte, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_airbyte.GetById", req)
	defer dbSpan.Finish()

	a.logger.Info("--AirbyteServiceGetById--", l.Any("req", req))

	var clickHouseCreds pb.ClickhouseCredentials

	resp, err := a.storage.Airbyte().GetById(ctx, req)
	if err != nil {
		a.logger.Error("--AirbyteServiceGetById--", l.Error(err))
		return nil, err
	}

	secret, err := a.vault.Get(ctx, resp.Password)
	if err != nil {
		a.logger.Error("--AirbyteServiceGetByIdVaultGet--", l.Error(err))
		return nil, err
	}

	bytes, err := json.Marshal(secret)
	if err != nil {
		a.logger.Error("--AirbyteServiceGetByIdVaultGet--", l.Error(err))
		return nil, err
	}

	if err = json.Unmarshal(bytes, &clickHouseCreds); err != nil {
		a.logger.Error("--AirbyteServiceGetByIdVaultGet--", l.Error(err))
		return nil, err
	}

	var password = base64.StdEncoding.EncodeToString([]byte(clickHouseCreds.GetPassword()))

	resp.Password = password

	return resp, err
}

func (a *AirbyteService) GetList(ctx context.Context, req *pb.GetListAirbyteRequest) (*pb.GetListAirbyteResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_airbyte.GetList", req)
	defer dbSpan.Finish()

	a.logger.Info("--AirbyteServiceGetList--", l.Any("req", req))

	resp, err := a.storage.Airbyte().GetList(ctx, req)
	if err != nil {
		a.logger.Error("--AirbyteServiceGetList--", l.Error(err))
		return nil, err
	}

	return resp, err
}
