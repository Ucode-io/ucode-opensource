package repo

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
)

type AirbyteStorageI interface {
	Create(ctx context.Context, airbyte *pb.CreateAirbyteRequest) (*pb.CreateAirbyteResponse, error)
	GetById(ctx context.Context, req *pb.GetAirbyteByIdRequest) (*pb.Airbyte, error)
	GetList(ctx context.Context, queryParam *pb.GetListAirbyteRequest) (*pb.GetListAirbyteResponse, error)
}
