package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"google.golang.org/protobuf/types/known/emptypb"
)

type menuService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedMenuServiceServer
}

func NewMenuService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *menuService {
	return &menuService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *menuService) Create(ctx context.Context, req *nb.CreateMenuRequest) (resp *nb.Menu, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.Create", req)
	defer dbSpan.Finish()

	f.log.Info("---CreateMenu--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateMenu--->>>", logger.Error(err))
		return &nb.Menu{}, err
	}

	return resp, nil
}

func (f *menuService) GetByID(ctx context.Context, req *nb.MenuPrimaryKey) (resp *nb.Menu, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetByID", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByIDMenu--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetById(ctx, req)
	if err != nil {
		f.log.Error("---GetByIDMenu--->>>", logger.Error(err))
		return &nb.Menu{}, err
	}

	return resp, nil
}

func (f *menuService) GetByLabel(ctx context.Context, req *nb.MenuPrimaryKey) (resp *nb.GetAllMenusResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetByLabel", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByLabelMenu--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetByLabel(ctx, req)
	if err != nil {
		f.log.Error("---GetByLabelMenu--->>>", logger.Error(err))
		return &nb.GetAllMenusResponse{}, err
	}

	return resp, nil
}

func (f *menuService) GetAll(ctx context.Context, req *nb.GetAllMenusRequest) (resp *nb.GetAllMenusResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetAll", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllMenu--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetAllMenusResponse--->>>", logger.Error(err))
		return &nb.GetAllMenusResponse{}, err
	}

	return resp, nil
}

func (f *menuService) Update(ctx context.Context, req *nb.Menu) (resp *nb.Menu, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.Update", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateMenu--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateMenu--->>>", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (f *menuService) UpdateMenuOrder(ctx context.Context, req *nb.UpdateMenuOrderRequest) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.UpdateMenuOrder", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateMenuOrder--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Menu().UpdateMenuOrder(ctx, req)
	if err != nil {
		f.log.Error("---UpdateMenuOrder--->>>", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (f *menuService) Delete(ctx context.Context, req *nb.MenuPrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.Delete", req)
	defer dbSpan.Finish()

	f.log.Info("---DeleteMenu--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Menu().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteMenu--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *menuService) GetMenuTree(ctx context.Context, req *nb.MenuPrimaryKey) (resp *nb.MenuTree, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetMenuTree", req)
	defer dbSpan.Finish()

	f.log.Info("---GetMenuTree--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetMenuTree(ctx, req)
	if err != nil {
		f.log.Error("---GetMenuTree--->>>", logger.Error(err))
		return &nb.MenuTree{}, err
	}

	return resp, nil
}

func (f *menuService) GetAllMenuSettings(ctx context.Context, req *nb.GetAllMenuSettingsRequest) (resp *nb.GetAllMenuSettingsResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetAllMenuSettings", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllMenuSettings--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetAllMenuSettings(ctx, req)
	if err != nil {
		f.log.Error("---GetAllMenuSettings--->>>", logger.Error(err))
		return &nb.GetAllMenuSettingsResponse{}, err
	}

	return resp, nil
}

func (f *menuService) GetByIDMenuSettings(ctx context.Context, req *nb.MenuSettingPrimaryKey) (resp *nb.MenuSettings, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetByIDMenuSettings", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByIDMenuSettings--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetByIDMenuSettings(ctx, req)
	if err != nil {
		f.log.Error("---GetByIDMenuSettings--->>>", logger.Error(err))
		return &nb.MenuSettings{}, err
	}
	return resp, nil
}

func (f *menuService) GetAllMenuTemplate(ctx context.Context, req *nb.GetAllMenuSettingsRequest) (resp *nb.GatAllMenuTemplateResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetAllMenuTemplate", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllMenuTemplate--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetAllMenuTemplate(ctx, req)
	if err != nil {
		f.log.Error("---GetAllMenuTemplate--->>>", logger.Error(err))
		return &nb.GatAllMenuTemplateResponse{}, err
	}
	return resp, nil
}

func (f *menuService) GetMenuTemplateWithEntities(ctx context.Context, req *nb.GetMenuTemplateRequest) (resp *nb.MenuTemplateWithEntities, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_menu.GetMenuTemplateWithEntities", req)
	defer dbSpan.Finish()

	f.log.Info("---GetMenuTemplateWithEntities--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Menu().GetMenuTemplateWithEntities(ctx, req)
	if err != nil {
		f.log.Error("---GetMenuTemplateWithEntities--->>>", logger.Error(err))
		return resp, err
	}
	return resp, nil
}
