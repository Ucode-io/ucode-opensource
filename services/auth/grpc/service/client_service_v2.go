package service

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *clientService) V2CreateClientType(ctx context.Context, req *pb.V2CreateClientTypeRequest) (*pb.CommonMessage, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "grpc_client.V2CreateClientType")
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2CreateClientType", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2CreateClientType", memoryUsed))
		}
	}()

	s.log.Info("---CreateClientType--->", logger.Any("req", req))
	var result *pbObject.CommonMessage

	requestToObjBuilderService := &pb.CreateClientTypeRequestToObjService{
		Name:         req.Name,
		ConfirmBy:    req.ConfirmBy.String(),
		SelfRegister: req.SelfRegister,
		SelfRecover:  req.SelfRecover,
		ProjectId:    req.DbProjectId,
		TableSlug:    req.GetTableSlug(),
		SessionLimit: req.SessionLimit,
		Columns:      req.Columns,
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		if len(req.GetTableSlug()) == 0 {
			tableAttributes, err := helper.ConvertMapToStruct(map[string]any{
				"auth_info": map[string]any{"login_strategy": []string{"phone", "email", "login"}},
				"label":     "",
				"label_en":  fmt.Sprintf("%s Users", req.Name),
			})
			if err != nil {
				s.log.Error("!!!CreateClientType.ObjectBuilderService.MapToStruct--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			var tableSlug = fmt.Sprintf("%s_users", strings.ToLower(strings.ReplaceAll(req.GetName(), " ", "_")))

			_, err = services.GetTableServiceByType(req.NodeType).Create(ctx, &pbObject.CreateTableRequest{
				Label:        fmt.Sprintf("%s Users", req.Name),
				Slug:         tableSlug,
				Description:  fmt.Sprintf("This is created login table by client_type %s", req.Name),
				ShowInMenu:   true,
				Icon:         "",
				IncrementId:  &pbObject.IncrementID{},
				Fields:       []*pbObject.CreateFieldsRequest{},
				Layouts:      []*pbObject.LayoutRequest{},
				CommitType:   config.COMMIT_TYPE_TABLE,
				Name:         fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123)),
				ViewId:       uuid.NewString(),
				LayoutId:     uuid.NewString(),
				ProjectId:    req.ResourceEnvrironmentId,
				Attributes:   tableAttributes,
				IsLoginTable: true,
			})
			if err != nil {
				s.log.Error("!!!CreateClientType.ObjectBuilderService.Table.Create--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			requestToObjBuilderService.TableSlug = fmt.Sprintf("%s_users", strings.ToLower(req.Name))
		}

		structData, err := helper.ConvertRequestToSturct(requestToObjBuilderService)
		if err != nil {
			s.log.Error("!!!CreateClientType--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		result, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(
			ctx, &pbObject.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.ResourceEnvrironmentId,
			})

		if err != nil {
			s.log.Error("!!!CreateClientType.ObjectBuilderService.Create--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case 3:
		if len(req.GetTableSlug()) == 0 {
			tableAttributes, err := helper.ConvertMapToStruct(map[string]any{
				"auth_info": map[string]any{"login_strategy": []string{"phone", "email", "login"}},
				"label":     "",
				"label_en":  fmt.Sprintf("%s Users", req.Name),
			})
			if err != nil {
				s.log.Error("!!!CreateClientType.ObjectBuilderService.MapToStruct--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			var tableSlug = fmt.Sprintf("%s_users", strings.ToLower(strings.ReplaceAll(req.GetName(), " ", "_")))

			_, err = services.GoTableService().Create(ctx, &nobs.CreateTableRequest{
				Label:        fmt.Sprintf("%s Users", req.Name),
				Slug:         tableSlug,
				Description:  fmt.Sprintf("This is created login table by client_type %s", req.Name),
				ShowInMenu:   true,
				Icon:         "",
				IncrementId:  &nobs.IncrementID{},
				Fields:       []*nobs.CreateFieldsRequest{},
				Layouts:      []*nobs.LayoutRequest{},
				CommitType:   config.COMMIT_TYPE_TABLE,
				Name:         fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123)),
				ViewId:       uuid.NewString(),
				LayoutId:     uuid.NewString(),
				ProjectId:    req.ResourceEnvrironmentId,
				Attributes:   tableAttributes,
				IsLoginTable: true,
			})
			if err != nil {
				s.log.Error("!!!CreateClientType.ObjectBuilderService.Table.Create--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}

			requestToObjBuilderService.TableSlug = fmt.Sprintf("%s_users", strings.ToLower(req.Name))
		}

		structData, err := helper.ConvertRequestToSturct(requestToObjBuilderService)
		if err != nil {
			s.log.Error("!!!CreateClientType--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		result, err := services.GoItemService().Create(
			ctx, &nobs.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			},
		)
		if err != nil {
			s.log.Error("!!!CreateClientType.PostgresObjectBuilderService.Create--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return &pb.CommonMessage{
			TableSlug: result.TableSlug,
			Data:      result.Data,
		}, nil
	}

	return &pb.CommonMessage{
		TableSlug: result.TableSlug,
		Data:      result.Data,
	}, nil
}

func (s *clientService) V2GetClientTypeByID(ctx context.Context, req *pb.V2ClientTypePrimaryKey) (*pb.CommonMessage, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "grpc_client.V2GetClientTypeByID")
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetClientTypeByID", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetClientTypeByID", memoryUsed))
		}
	}()

	s.log.Info("---GetClientTypeByID--->", logger.Any("req", req))

	var (
		result *pbObject.CommonMessage
	)
	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!GetClientTypeByID--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(
		req.ProjectId,
		req.NodeType,
	)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx,
			&pbObject.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})

		if err != nil {
			s.log.Error("!!!GetClientTypeByID.ObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case 3:
		result, err := services.GoItemService().GetSingle(ctx,
			&nobs.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})

		if err != nil {
			s.log.Error("!!!GetClientTypeByID.PostgresObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return &pb.CommonMessage{
			TableSlug: result.TableSlug,
			Data:      result.Data,
		}, nil
	}

	return &pb.CommonMessage{
		TableSlug: result.TableSlug,
		Data:      result.Data,
	}, nil
}

func (s *clientService) V2GetClientTypeList(ctx context.Context, req *pb.V2GetClientTypeListRequest) (*pb.CommonMessage, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "grpc_client.V2GetClientTypeList")
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetClientTypeList", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetClientTypeList", memoryUsed))
		}
	}()

	s.log.Info("---GetClientTypeList--->", logger.Any("req", req))
	result := &pbObject.CommonMessage{}

	if req.Limit == 0 {
		req.Limit = 1000
	}

	structReq := map[string]any{
		"limit":  req.GetLimit(),
		"offset": req.GetOffset(),
	}

	if req.Guids != nil {
		structReq["guid"] = req.Guids
	}

	structData, err := helper.ConvertRequestToSturct(structReq)

	if err != nil {
		s.log.Error("!!!GetClientTypeList--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).GetListSlim(ctx,
			&pbObject.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})
		if err != nil {
			s.log.Error("!!!GetClientTypeList.ObjectBuilderService.GetList--->", logger.Error(err))
			return &pb.CommonMessage{}, nil
		}
	case 3:
		result2, err := services.GoObjectBuilderService().GetList2(ctx,
			&nobs.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})

		if err != nil {
			s.log.Error("!!!GetClientTypeList.PostgresObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		result.Data = result2.Data
	}

	return &pb.CommonMessage{
		TableSlug: result.TableSlug,
		Data:      result.Data,
	}, nil
}

func (s *clientService) V2UpdateClientType(ctx context.Context, req *pb.V2UpdateClientTypeRequest) (*pb.CommonMessage, error) {
	s.log.Info("---UpdateClientType--->", logger.Any("req", req))

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "grpc_client.V2UpdateClientType")
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2UpdateClientType", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2UpdateClientType", memoryUsed))
		}
	}()

	var (
		result *pbObject.CommonMessage
	)
	requestToObjBuilderService := map[string]any{
		"name":            req.Name,
		"confirm_by":      req.ConfirmBy.String(),
		"self_register":   req.SelfRegister,
		"self_recorder":   req.SelfRecover,
		"guid":            req.Guid,
		"client_type_ids": req.ClientPlatformIds,
		"table_slug":      req.TableSlug,
		"id":              req.Guid,
		"default_page":    req.DefaultPage,
		"session_limit":   req.SessionLimit,
		"columns":         req.Columns,
	}

	structData, err := helper.ConvertRequestToSturct(requestToObjBuilderService)
	if err != nil {
		s.log.Error("!!!GetClientTypeList--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).Update(ctx,
			&pbObject.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})
		if err != nil {
			s.log.Error("!!!UpdateClientType.ObjectBuilderService.Update--->", logger.Error(err))
			return nil, status.Error(codes.NotFound, err.Error())
		}
	case 3:
		result, err := services.GoItemService().Update(ctx,
			&nobs.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})
		if err != nil {
			s.log.Error("!!!UpdateClientType.PostgresObjectBuilderService.Update--->", logger.Error(err))
			return nil, status.Error(codes.NotFound, err.Error())
		}

		return &pb.CommonMessage{
			TableSlug: result.TableSlug,
			Data:      result.Data,
		}, nil
	}

	return &pb.CommonMessage{
		TableSlug: result.TableSlug,
		Data:      result.Data,
	}, nil
}

func (s *clientService) V2DeleteClientType(ctx context.Context, req *pb.V2ClientTypePrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "grpc_client.V2DeleteClientType")
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2DeleteClientType", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2DeleteClientType", memoryUsed))
		}
	}()

	s.log.Info("---DeleteClientType--->", logger.Any("req", req))

	res := &emptypb.Empty{}

	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!DeleteClientType--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(
		req.ProjectId,
		req.NodeType,
	)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		_, err = services.GetObjectBuilderServiceByType(req.NodeType).Delete(ctx,
			&pbObject.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})

		if err != nil {
			s.log.Error("!!!DeleteClientType.ObjectBuilderService.Delete--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		_, err = services.GoItemService().Delete(ctx,
			&nobs.CommonMessage{
				TableSlug: "client_type",
				Data:      structData,
				ProjectId: req.GetResourceEnvrironmentId(),
			})

		if err != nil {
			s.log.Error("!!!DeleteClientType.PostgresObjectBuilderService.Delete--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return res, nil
}
