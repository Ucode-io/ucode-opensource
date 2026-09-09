package service

import (
	"context"
	"runtime"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type permissionService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedPermissionServiceServer
}

func NewPermissionService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *permissionService {
	return &permissionService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (s *permissionService) V2AddRole(ctx context.Context, req *pb.V2AddRoleRequest) (*pb.CommonMessage, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_permission_v2.V2AddRole", req)
	defer dbSpan.Finish()

	s.log.Info("---AddRole--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2AddRole", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2AddRole", memoryUsed))
		}
	}()

	var result *pbObject.CommonMessage

	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!AddRole--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		if !req.Status {
			roleGetList, err := services.GetObjectBuilderServiceByType(req.NodeType).GetList(ctx, &pbObject.CommonMessage{
				TableSlug: "role",
				Data: &structpb.Struct{Fields: map[string]*structpb.Value{
					"status":         structpb.NewBoolValue(req.Status),
					"client_type_id": structpb.NewStringValue(req.ClientTypeId),
				}},
				ProjectId: req.GetProjectId(),
			})
			if err != nil {
				s.log.Error("!!!AddRole.ObjectBuilderService.GetList--->", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}

			roleResponse := roleGetList.Data.AsMap()["response"].([]any)

			if len(roleResponse) >= 1 {
				return nil, status.Error(codes.InvalidArgument, "invalid role")
			}
		}

		result, err = services.GetObjectBuilderServiceByType(req.NodeType).Create(ctx, &pbObject.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetProjectId(),
		})
		if err != nil {
			s.log.Error("!!!AddRole.ObjectBuilderService.Create--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		roleData, err := helper.ConvertStructToResponse(result.Data)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		var roleDataData = cast.ToStringMap(roleData["data"])

		_, err = services.BuilderPermissionService().CreateDefaultPermission(
			ctx, &pbObject.CreateDefaultPermissionRequest{
				ProjectId: req.GetProjectId(),
				RoleId:    cast.ToString(roleDataData["guid"]),
			},
		)
		if err != nil {
			s.log.Error("!!!AddRole.ObjectBuilderService.CreateDefaultPermission--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

	case 3:
		if !req.Status {
			roleGetList, err := services.GoObjectBuilderService().GetList2(ctx, &nobs.CommonMessage{
				TableSlug: "role",
				Data: &structpb.Struct{Fields: map[string]*structpb.Value{
					"status":         structpb.NewBoolValue(req.Status),
					"client_type_id": structpb.NewStringValue(req.ClientTypeId),
				}},
				ProjectId: req.GetProjectId(),
			})
			if err != nil {
				s.log.Error("!!!AddRole.ObjectBuilderService.GetList--->", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}

			roleResponse := cast.ToSlice(roleGetList.Data.AsMap()["response"])
			structData.Fields["status"] = structpb.NewBoolValue(false)
			if len(roleResponse) >= 1 {
				return nil, status.Error(codes.InvalidArgument, "invalid role")
			}
		}

		result, err := services.GoItemService().Create(ctx, &nobs.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetProjectId(),
		})
		if err != nil {
			s.log.Error("!!!AddRole.PostgresObjectBuilderService.Create--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		roleData, err := helper.ConvertStructToResponse(result.Data)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		_, err = services.GoObjectBuilderPermissionService().CreateDefaultPermission(
			ctx, &nobs.CreateDefaultPermissionRequest{
				ProjectId: req.GetProjectId(),
				RoleId:    cast.ToString(roleData["guid"]),
			},
		)
		if err != nil {
			s.log.Error("!!!AddRole.ObjectBuilderService.CreateDefaultPermission--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
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

func (s *permissionService) V2GetRoleById(ctx context.Context, req *pb.V2RolePrimaryKey) (*pb.CommonMessage, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_permission_v2.V2GetRoleById", req)
	defer dbSpan.Finish()

	s.log.Info("---GetRoleById--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetRoleById", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetRoleById", memoryUsed))
		}
	}()

	var result *pbObject.CommonMessage

	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!GetRoleById--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRoleById.ObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &pb.CommonMessage{
		TableSlug: result.TableSlug,
		Data:      result.Data,
	}, nil
}

func (s *permissionService) V2GetRolesList(ctx context.Context, req *pb.V2GetRolesListRequest) (*pb.CommonMessage, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_permission_v2.V2GetRolesList", req)
	defer dbSpan.Finish()

	s.log.Info("---GetRolesList--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2GetRolesList", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2GetRolesList", memoryUsed))
		}
	}()

	var (
		result *pbObject.CommonMessage
		err    error
	)

	structData, err := helper.ConvertRequestToSturct(map[string]any{})
	if err != nil {
		s.log.Error("!!!GetRolesList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if req.FromPermission {
		if len(req.GetClientTypeId()) != 0 {
			structData, err = helper.ConvertRequestToSturct(map[string]any{"client_type_id": req.GetClientTypeId()})
			if err != nil {
				s.log.Error("!!!GetRolesList--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}
	} else {
		structData, err = helper.ConvertRequestToSturct(map[string]any{
			"status":         req.GetStatus(),
			"client_type_id": req.GetClientTypeId(),
		})
		if err != nil {
			s.log.Error("!!!GetRolesList--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).GetList(ctx, &pbObject.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRolesList.ObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		result, err := services.GoObjectBuilderService().GetList2(ctx, &nobs.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRolesList.GoObjectBuilderService.GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
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

func (s *permissionService) V2UpdateRole(ctx context.Context, req *pb.V2UpdateRoleRequest) (*pb.CommonMessage, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_permission_v2.V2UpdateRole", req)
	defer dbSpan.Finish()

	s.log.Info("---UpdateRole--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2UpdateRole", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2UpdateRole", memoryUsed))
		}
	}()

	var result *pbObject.CommonMessage

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	req.ProjectId = req.GetDbProjectId()

	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!UpdateRole--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).Update(ctx, &pbObject.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!UpdateRole.ObjectBuilderService.Update--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &pb.CommonMessage{
		TableSlug: result.TableSlug,
		Data:      result.Data,
	}, nil
}

func (s *permissionService) V2RemoveRole(ctx context.Context, req *pb.V2RolePrimaryKey) (*pb.CommonMessage, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_permission_v2.V2RemoveRole", req)
	defer dbSpan.Finish()

	s.log.Info("---RemoveRole--->", logger.Any("req", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2RemoveRole", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2RemoveRole", memoryUsed))
		}
	}()

	structData, err := helper.ConvertRequestToSturct(req)
	if err != nil {
		s.log.Error("!!!GetRoleById--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	var result *pbObject.CommonMessage

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		result, err = services.GetObjectBuilderServiceByType(req.NodeType).GetSingle(ctx, &pbObject.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRoleById.ObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		_, err = services.GetObjectBuilderServiceByType(req.NodeType).Delete(ctx, &pbObject.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRoleById.ObjectBuilderService.Delete--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		result, err := services.GoItemService().GetSingle(ctx, &nobs.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRoleById.PostgresObjectBuilderService.GetSingle--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
		_, err = services.GoItemService().Delete(ctx, &nobs.CommonMessage{
			TableSlug: "role",
			Data:      structData,
			ProjectId: req.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!GetRoleById.PostgresObjectBuilderService.Delete--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
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
