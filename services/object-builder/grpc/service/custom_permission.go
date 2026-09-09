package service

import (
	"context"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type CustomPermissionsService struct {
	nb.UnimplementedCustomPermissionsServiceServer
	storage storage.StorageI
}

func NewCustomPermissionsService(strg storage.StorageI) *CustomPermissionsService {
	return &CustomPermissionsService{
		storage: strg,
	}
}

// ==================== Definition ====================

func (s *CustomPermissionsService) CreateCustomPermission(ctx context.Context, req *nb.CreateCustomPermissionRequest) (*nb.CustomPermission, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.CreateCustomPermission")
	defer span.Finish()

	return s.storage.CustomPermissions().Create(ctx, req)
}

func (s *CustomPermissionsService) UpdateCustomPermission(ctx context.Context, req *nb.UpdateCustomPermissionRequest) (*nb.CustomPermission, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.UpdateCustomPermission")
	defer span.Finish()

	return s.storage.CustomPermissions().Update(ctx, req)
}

func (s *CustomPermissionsService) DeleteCustomPermission(ctx context.Context, req *nb.DeleteCustomPermissionRequest) (*emptypb.Empty, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.DeleteCustomPermission")
	defer span.Finish()

	err := s.storage.CustomPermissions().Delete(ctx, req)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *CustomPermissionsService) GetAllCustomPermissions(ctx context.Context, req *nb.GetAllCustomPermissionsRequest) (*nb.GetAllCustomPermissionsResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.GetAllCustomPermissions")
	defer span.Finish()

	return s.storage.CustomPermissions().GetAll(ctx, req)
}

// ==================== Access ====================

func (s *CustomPermissionsService) GetCustomPermissionAccesses(ctx context.Context, req *nb.GetCustomPermissionAccessesRequest) (*nb.GetCustomPermissionAccessesResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.GetCustomPermissionAccesses")
	defer span.Finish()

	return s.storage.CustomPermissions().GetAccesses(ctx, req)
}

func (s *CustomPermissionsService) GetAllCustomPermissionAccesses(ctx context.Context, req *nb.GetAllCustomPermissionAccessesRequest) (*nb.GetCustomPermissionAccessesResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.GetAllCustomPermissionAccesses")
	defer span.Finish()

	return s.storage.CustomPermissions().GetAllAccesses(ctx, req)
}

func (s *CustomPermissionsService) UpdateCustomPermissionAccess(ctx context.Context, req *nb.UpdateCustomPermissionAccessRequest) (*emptypb.Empty, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "CustomPermissionsService.UpdateCustomPermissionAccess")
	defer span.Finish()

	err := s.storage.CustomPermissions().UpdateAccess(ctx, req)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}
