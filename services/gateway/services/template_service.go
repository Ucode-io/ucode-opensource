package services

// import (
// 	"context"
// 	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
// 	tmp "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/template_service"

// 	"google.golang.org/grpc"
// 	"google.golang.org/grpc/credentials/insecure"
// )

// type TemplateServiceI interface {
// 	Template() tmp.TemplateFolderServiceClient
// }

// type templateServiceClient struct {
// 	templateService tmp.TemplateFolderServiceClient
// }

// func NewTemplateServiceClient(ctx context.Context, cfg config.Config) (TemplateServiceI, error) {

// 	connTemplateService, err := grpc.DialContext(
// 		ctx,
// 		cfg.TemplateServiceHost+cfg.TemplateGRPCPort,
// 		grpc.WithTransportCredentials(insecure.NewCredentials()),
// 	)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &templateServiceClient{
// 		templateService: tmp.NewTemplateFolderServiceClient(connTemplateService),
// 	}, nil
// }

// func (g *templateServiceClient) Template() tmp.TemplateFolderServiceClient {
// 	return g.templateService
// }
