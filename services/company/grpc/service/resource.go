package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/new_function_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/company/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/object_builder_service"
	tb "github.com/Ucode-io/ucode-opensource/services/company/genproto/transcoder_service"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/airbyte"
	ch "github.com/Ucode-io/ucode-opensource/services/company/pkg/clickhouse"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/metabase"
	mongodb "github.com/Ucode-io/ucode-opensource/services/company/pkg/mongo"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/postgres_client"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/util"
	vaultclient "github.com/Ucode-io/ucode-opensource/services/company/pkg/vault"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ResourceService AdminService ...
type ResourceService struct {
	storage        storage.StorageI
	services       ServiceNodesI
	logger         l.Logger
	cfg            config.Config
	vault          vaultclient.VaultClient
	serviceManager client.ServiceManagerI
	pb.UnimplementedResourceServiceServer
}

// NewResourceService NewAdminService ...
func NewResourceService(strg storage.StorageI, log l.Logger, vault vaultclient.VaultClient, services ServiceNodesI, cfg config.Config, serviceManager client.ServiceManagerI) *ResourceService {
	return &ResourceService{
		storage:        strg,
		services:       services,
		logger:         log,
		vault:          vault,
		cfg:            cfg,
		serviceManager: serviceManager,
	}
}

func (s *ResourceService) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.AddResourceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.AddResource", req)
	defer dbSpan.Finish()

	s.logger.Info("--AddResource-- requested", l.Any("req", req))

	secretPath, err := s.vault.GetSecretPath(ctx)
	if err != nil {
		s.logger.Error("--AddResourceGetSecretPath--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	var (
		credentialData any
		resourceType   = req.ResourceType
		serviceType    = req.ServiceType
	)

	switch req.GetResourceType() {
	case pb.ResourceType_CLICKHOUSE:
		credentialData = pb.ClickhouseCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: req.Credentials.GetPassword(),
			Database: req.Credentials.GetDatabase(),
		}
	case pb.ResourceType_MONGODB:
		credentialData = pb.MongodbCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: req.Credentials.GetPassword(),
			Database: req.Credentials.GetDatabase(),
		}
	case pb.ResourceType_POSTGRESQL:
		credentialData = pb.PostgresSqlCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: req.Credentials.GetPassword(),
			Database: req.Credentials.GetDatabase(),
		}
	default:
		s.logger.Error("--AddResourceDefault--", l.Any("msg", "resource type not specified"))
		return nil, status.Error(codes.InvalidArgument, "resource type not specified")
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	var (
		path = fmt.Sprintf("%s/%s/%s/%s/%s", strings.Trim(secretPath, "/"), req.CompanyId, req.ProjectId, id.String(), req.ResourceType.String())
		data = map[string]any{}
	)

	bytes, err := json.Marshal(credentialData)
	if err != nil {
		s.logger.Error("--AddResourceMarshalCredentialData--", l.Error(err))
		return nil, err
	}

	if err = json.Unmarshal(bytes, &data); err != nil {
		s.logger.Error("--AddResourceUnmarshalData--", l.Error(err))
		return nil, err
	}

	if err = s.vault.Put(ctx, path, data); err != nil {
		s.logger.Error("--AddResourceVaultPut--", l.Error(err))
		return nil, err
	}

	service, err := s.services.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		s.logger.Error("-- GetFromServiceNodeGetByNodeType--", l.Error(err))
		return nil, err
	}

	resourceCreate, err := s.storage.Resource().AddResource(ctx, &pb.Resource{
		Id:           id.String(),
		ProjectId:    req.GetProjectId(),
		ResourceType: req.GetResourceType(),
		Credentials:  req.GetCredentials(),
		IsConfigured: req.GetIsConfigured(),
		VaultPath:    path,
		Title:        req.GetTitle(),
		NodeType:     req.GetNodeType(),
	})
	if err != nil {
		s.logger.Error("--AddResourceStorage--", l.Error(err))
		return nil, err
	}

	if req.NodeType == "" {
		req.NodeType = config.LowNodeType
	}

	_, err = s.storage.Resource().InsertResourceEnvironment(ctx, &pb.ResourceEnvironment{
		ProjectId:     req.ProjectId,
		ResourceId:    resourceCreate.GetId(),
		EnvironmentId: req.EnvironmentId,
		IsConfigured:  req.IsConfigured,
		VaultPath:     path,
		ServiceType:   int32(serviceType),
		ResourceType:  int32(resourceType),
		Host:          req.Credentials.Host,
		Port:          req.Credentials.Port,
		Database:      req.Credentials.Database,
		Username:      req.Credentials.Username,
		Default:       req.IsDefault,
		NodeType:      req.NodeType,
	})
	if err != nil {
		s.logger.Error("--AddResourceEnvironment--", l.Error(err))
		return nil, err
	}

	switch req.ServiceType {
	case pb.ServiceType_BUILDER_SERVICE:
		switch req.ResourceType {
		case pb.ResourceType_MONGODB:
			_, err := service.GetBuilderProjectServiceByType(req.NodeType).Register(ctx,
				&object_builder_service.RegisterProjectRequest{
					K8SNamespace: "cp-region-type-id",
					ProjectId:    req.ProjectId,
					SecretPath:   path,
					Credentials: &object_builder_service.RegisterProjectRequest_Credentials{
						Host:     req.Credentials.GetHost(),
						Port:     req.Credentials.GetPort(),
						Username: req.Credentials.GetUsername(),
						Password: req.Credentials.GetPassword(),
						Database: req.Credentials.GetDatabase(),
					},
					UserId:       req.UserId,
					ResourceId:   id.String(),
					ClientTypeId: req.GetClientTypeId(),
					RoleId:       req.GetRoleId(),
				},
			)
			if err != nil {
				s.logger.Error("--AddResource--", l.Error(err))
				return nil, err
			}
		case pb.ResourceType_POSTGRESQL:
			_, err := service.GoBuilderProjectService().Register(ctx,
				&nobs.RegisterProjectRequest{
					K8SNamespace: "cp-region-type-id",
					ProjectId:    req.ProjectId,
					SecretPath:   path,
					Credentials: &nobs.RegisterProjectRequest_Credentials{
						Host:     req.Credentials.GetHost(),
						Port:     req.Credentials.GetPort(),
						Username: req.Credentials.GetUsername(),
						Password: req.Credentials.GetPassword(),
						Database: req.Credentials.GetDatabase(),
					},
					UserId:       req.UserId,
					ResourceId:   id.String(),
					ClientTypeId: req.GetClientTypeId(),
					RoleId:       req.GetRoleId(),
				},
			)
			if err != nil {
				s.logger.Error("--AddResourceGoObjectBuilderServiceRegister--", l.Error(err))
				return nil, err
			}
		case pb.ResourceType_CLICKHOUSE:
			randomPassword, _ := helper.GenerateRandomPassword(9)
			if randomPassword == "" {
				randomPassword = req.GetCredentials().GetPassword()
			}

			resource, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{
				EnvironmentId: req.GetEnvironmentId(),
				ProjectId:     req.GetProjectId(),
				Default:       true,
			})
			if err != nil {
				s.logger.Error("--AddResourceGetSingleResourceEnvironment--", l.Error(err))
				return nil, err
			}

			project, err := s.storage.Project().GetById(ctx, req.GetProjectId())
			if err != nil {
				s.logger.Error("--AddResourceGetSingleProject--", l.Error(err))
				return nil, err
			}

			switch resource.GetResourceType() {
			case int32(pb.ResourceType_MONGODB):
				var (
					air             = airbyte.NewAirbyte(s.cfg)
					mongoCredential pb.MongodbCredentials
					chCredentials   = pb.ClickhouseCredentials{
						Host:     req.GetCredentials().GetHost(),
						Database: req.GetCredentials().GetDatabase(),
						Username: req.GetCredentials().GetUsername(),
						Password: req.GetCredentials().GetPassword(),
					}
				)

				workspaceId, err := air.GetWorkSpace()
				if err != nil {
					s.logger.Error("--AddResourceGetWorkSpaceId--", l.Error(err))
					return nil, err
				}

				secret, err := s.vault.Get(ctx, resource.GetVaultPath())
				if err != nil {
					s.logger.Error("--AddResourceVaultGet--", l.Error(err))
					return nil, err
				}

				bytes, err := json.Marshal(secret)
				if err != nil {
					s.logger.Error("--AddResourceVaultMarshal--", l.Error(err))
					return nil, err
				}

				if err = json.Unmarshal(bytes, &mongoCredential); err != nil {
					s.logger.Error("--AddResourceVaultUnmarshl--", l.Error(err))
					return nil, err
				}

				mongoSourceDefinitionId, err := air.GetSourceDefinitionId("MongoDb")
				if err != nil {
					s.logger.Error("--AddResourceGetSourceMongoId--", l.Error(err))
					return nil, err
				}

				mongoSource, err := air.CreateSourceMongoDb(airbyte.CreateMongoSource{
					Name:               project.GetTitle() + "-mongo",
					SourceDefinitionId: mongoSourceDefinitionId,
					WorkspaceId:        workspaceId.WorkspaceID,
					Configuration: airbyte.MongoConfiguration{
						InstanceType: airbyte.MongoInstanceType{
							Instance: "standalone",
							Host:     mongoCredential.GetHost(),
							Port:     cast.ToInt64(mongoCredential.GetPort()),
							TLS:      false,
						},
						AuthSource: mongoCredential.GetUsername(),
						Password:   mongoCredential.GetPassword(),
						Database:   mongoCredential.GetDatabase(),
						User:       mongoCredential.GetUsername(),
					},
				})
				if err != nil {
					s.logger.Error("--AddResourceCreateSourceMongo--", l.Error(err))
					return nil, err
				}

				mongoSyncCatalog, err := air.GetSyncCatalog(mongoSource.SourceId)
				if err != nil {
					s.logger.Error("--AddResourceGetCatalog--", l.Error(err))
					return nil, err
				}

				clickHouseDestinationId, err := air.GetDestinationId("Clickhouse")
				if err != nil {
					s.logger.Error("--AddResourceDestinationClickhouseId--", l.Error(err))
					return nil, err
				}

				var clickHouseConf = airbyte.CreateClickhouseDestination{
					Name:                    project.GetTitle() + "-clickhouse",
					DestinationDefinitionId: clickHouseDestinationId,
					WorkspaceId:             workspaceId.WorkspaceID,
					Configuration: airbyte.ClickHouseConfiguration{
						Host:     chCredentials.Host,
						Port:     30123,
						UserName: chCredentials.Username,
						Password: chCredentials.Password,
						Database: chCredentials.Database,
						SSL:      false,
						TunnelMethod: map[string]string{
							"tunnel_method": "NO_TUNNEL",
						},
					},
				}

				clickHouseDestination, err := air.CreateDestinationClickHouse(clickHouseConf)
				if err != nil {
					s.logger.Error("--AddResourceCreateDestinationClickHouse--", l.Error(err))
					return nil, err
				}

				connectionId, err := air.CreateConnection(workspaceId.WorkspaceID, airbyte.CreateConnection{
					SourceId:      mongoSource.SourceId,
					DestinitionId: clickHouseDestination,
					SyncCatalog:   mongoSyncCatalog.Catalog,
					Schedule: map[string]any{
						"units":    1,
						"timeUnit": "hours",
					},
					Name:                         project.GetTitle() + "-connection",
					Status:                       "active",
					NamespaceDefinition:          "destination",
					NamespaceFormat:              "${SOURCE_NAMESPACE}",
					NonBreakingChangesPreference: "ignore",
					Geography:                    "auto",
				})
				if err != nil {
					s.logger.Error("--AddResourceCreateConnection--", l.Error(err))
					return nil, err
				}

				_, err = s.storage.Airbyte().Create(ctx, &pb.CreateAirbyteRequest{
					ProjectId:      req.GetProjectId(),
					EnvironmentId:  req.GetEnvironmentId(),
					ResourceId:     resourceCreate.GetId(),
					ScheduleTime:   1,
					WorkspaceId:    workspaceId.WorkspaceID,
					SourceId:       mongoSource.SourceId,
					DestinationId:  clickHouseConf.DestinationDefinitionId,
					ConnectionId:   connectionId,
					ConnectionName: project.GetTitle() + "-connection",
				})
				if err != nil {
					s.logger.Error("--AddResourceCreateConnection--", l.Error(err))
					return nil, err
				}
			case int32(pb.ResourceType_POSTGRESQL):
				var (
					air           = airbyte.NewAirbyte(s.cfg)
					pgCredential  pb.PostgresSqlCredentials
					chCredentials = pb.ClickhouseCredentials{
						Host:     req.GetCredentials().GetHost(),
						Database: req.GetCredentials().GetDatabase(),
						Username: req.GetCredentials().GetUsername(),
						Password: req.GetCredentials().GetPassword(),
					}
				)

				workspaceId, err := air.GetWorkSpace()
				if err != nil {
					s.logger.Error("--AddResourceGetWorkSpaceId--", l.Error(err))
					return nil, err
				}

				secret, err := s.vault.Get(ctx, resource.GetVaultPath())
				if err != nil {
					s.logger.Error("--AddResourceVaultGetPostgres--", l.Error(err))
					return nil, err
				}

				bytes, err := json.Marshal(secret)
				if err != nil {
					s.logger.Error("--AddResourceVaultMarshal--", l.Error(err))
					return nil, err
				}

				if err = json.Unmarshal(bytes, &pgCredential); err != nil {
					s.logger.Error("--AddResourceVaultUnmarshal--", l.Error(err))
					return nil, err
				}

				pgSourceDefinitionId, err := air.GetSourceDefinitionId("Postgres")
				if err != nil {
					s.logger.Error("--AddResourceGetSourcePostgresId--", l.Error(err))
					return nil, err
				}

				pgSource, err := air.CreateSourcePostgres(airbyte.CreatePostgresSource{
					Name:               project.GetTitle() + "-postgres",
					SourceDefinitionId: pgSourceDefinitionId,
					WorkSpaceId:        workspaceId.WorkspaceID,
					Configuration: airbyte.PostgresConfiguration{
						Host:     pgCredential.GetHost(),
						Port:     cast.ToInt64(pgCredential.GetPort()),
						Username: pgCredential.GetUsername(),
						Database: pgCredential.GetDatabase(),
						Password: pgCredential.GetPassword(),
						Schemas:  []string{"public"},
						ReplicationMethod: map[string]any{
							"method": "Standard",
						},
						SslMode: map[string]any{
							"mode": "disable",
						},
						TunnelMethod: map[string]any{
							"tunnel_method": "NO_TUNNEL",
						},
					},
					SyncMode:            "incremental",
					DestinationSyncMode: "append_dedup",
				})
				if err != nil {
					s.logger.Error("--AddResourceCreateSourcePostgres--", l.Error(err))
					return nil, err
				}

				pgSyncCatalog, err := air.GetSyncCatalog(pgSource.SourceId)
				if err != nil {
					s.logger.Error("--AddResourceGetCatalog--", l.Error(err))
					return nil, err
				}

				clickHouseDestinationId, err := air.GetDestinationId("Clickhouse")
				if err != nil {
					s.logger.Error("--AddResourceDestinationClickhouseId--", l.Error(err))
					return nil, err
				}

				var clickHouseConf = airbyte.CreateClickhouseDestination{
					Name:                    project.GetTitle() + "-clickhouse",
					DestinationDefinitionId: clickHouseDestinationId,
					WorkspaceId:             workspaceId.WorkspaceID,
					Configuration: airbyte.ClickHouseConfiguration{
						Host:     chCredentials.Host,
						Port:     30123,
						UserName: chCredentials.Username,
						Password: chCredentials.Password,
						Database: chCredentials.Database,
						SSL:      false,
						TunnelMethod: map[string]string{
							"tunnel_method": "NO_TUNNEL",
						},
					},
				}

				clickHouseDestination, err := air.CreateDestinationClickHouse(clickHouseConf)
				if err != nil {
					s.logger.Error("--AddResourceCreateDestinationClickHouse--", l.Error(err))
					return nil, err
				}

				connectionId, err := air.CreateConnection(workspaceId.WorkspaceID, airbyte.CreateConnection{
					SourceId:      pgSource.SourceId,
					DestinitionId: clickHouseDestination,
					SyncCatalog:   pgSyncCatalog.Catalog,
					Schedule: map[string]any{
						"units":    1,
						"timeUnit": "hours",
					},
					Name:                         project.GetTitle() + "-connection",
					Status:                       "active",
					NamespaceDefinition:          "destination",
					NamespaceFormat:              "${SOURCE_NAMESPACE}",
					NonBreakingChangesPreference: "ignore",
					Geography:                    "auto",
					SyncMode:                     "incremental",
					DestinationSyncMode:          "append_dedup",
				})
				if err != nil {
					s.logger.Error("--AddResourceCreateConnection--", l.Error(err))
					return nil, err
				}

				_, err = s.storage.Airbyte().Create(ctx, &pb.CreateAirbyteRequest{
					ProjectId:      req.GetProjectId(),
					EnvironmentId:  req.GetEnvironmentId(),
					ResourceId:     resourceCreate.GetId(),
					ScheduleTime:   1,
					WorkspaceId:    workspaceId.WorkspaceID,
					SourceId:       pgSource.SourceId,
					DestinationId:  clickHouseConf.DestinationDefinitionId,
					ConnectionId:   connectionId,
					ConnectionName: project.GetTitle() + "-connection",
				})
				if err != nil {
					s.logger.Error("--AddResourceCreateConnection--", l.Error(err))
					return nil, err
				}
			}

			if req.IsSuperset {
				supersetName, _ := helper.GenerateRandomWord(4)
				err = airbyte.CreateSupersetConnection(airbyte.CreateSupersetConnectionRequest{
					DatabaseName:     req.GetTitle(),
					Host:             req.GetCredentials().GetHost(),
					Port:             s.cfg.UcodeClickhouseHTTPPort,
					Database:         req.GetCredentials().GetDatabase(),
					Username:         req.GetCredentials().GetUsername(),
					Password:         req.GetCredentials().GetPassword(),
					SupersetPassword: randomPassword,
					SupersetName:     fmt.Sprintf("%v_%v", req.GetTitle(), supersetName),
					Cfg:              s.cfg,
				})
				if err != nil {
					s.logger.Error("--CreateSupersetConnection--", l.Error(err))
					return nil, status.Error(codes.Internal, err.Error())
				}

				_, err = s.storage.Resource().AddResourceToProject(ctx, &pb.AddResourceToProjectRequest{
					ProjectId:     req.GetProjectId(),
					EnvironmentId: req.GetEnvironmentId(),
					Type:          pb.ResourceType_SUPERSET,
					Settings: &pb.Settings{
						Superset: &pb.SupersetCredentials{
							Username: fmt.Sprintf("%v_%v", req.GetTitle(), supersetName),
							Password: randomPassword,
							Url:      s.cfg.SupersetBaseUrl,
						},
					},
					Name: "Superset",
				})
				if err != nil {
					s.logger.Error("--AddResourceToProject--", l.Error(err))
					return nil, status.Error(codes.Internal, err.Error())
				}
			}
		default:
			return nil, errors.New("wrong resource type")
		}
	default:
		return nil, errors.New("wrong service type")
	}

	return &pb.AddResourceResponse{
		Id: id.String(),
	}, nil
}

func (s *ResourceService) CreateResource(ctx context.Context, req *pb.CreateResourceReq) (*pb.CreateResourceRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.CreateResource", req)
	defer dbSpan.Finish()

	s.logger.Info("--CreateResource-- requested", l.Any("req", req))

	var (
		err, errorUserCreate error
		errAddResource       error
		cfg                  = s.services.GetConfigByNamespace(req.ProjectId, req.NodeType)
		isDefault            = true
	)

	_, err = s.storage.Company().GetById(ctx, req.GetCompanyId())
	if err != nil {
		errGetCompanyInfo := errors.New("invalid or missing company_id")
		s.logger.Error("--CreateResourceCompanyGetByID--", l.Error(err))
		return nil, status.Error(codes.InvalidArgument, errGetCompanyInfo.Error())
	}

	project, err := s.storage.Project().GetById(ctx, req.GetProjectId())
	if err != nil {
		errGetProjectInfo := errors.New("invalid or missing project_id")
		s.logger.Error("--CreateResourceProjectGetById--", l.Error(err))
		return nil, status.Error(codes.InvalidArgument, errGetProjectInfo.Error())
	}

	environment, err := s.storage.Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{Id: req.GetEnvironmentId()})
	if err != nil {
		s.logger.Error("--CreateResourceEnvironmentGetById--", l.Error(err))
		return nil, err
	}

	var (
		projectName = strings.Split(project.GetTitle(), " ")
		envName     = strings.Split(environment.GetName(), " ")

		databaseName = fmt.Sprintf("%s_%s_%s_%s",
			strings.ToLower(projectName[0][:helper.Min(len(projectName[0]), 10)]),
			strings.ReplaceAll(project.GetProjectId(), "-", ""),
			strings.ToLower(envName[0])[:helper.Min(len(envName), 4)],
			"obj_build_svcs",
		)
		databaseNameForPostgres = fmt.Sprintf("%s_%s_%s_%s",
			strings.ToLower(projectName[0][:helper.Min(len(projectName[0]), 10)]),
			strings.ReplaceAll(project.GetProjectId(), "-", ""),
			strings.ToLower(envName[0])[:helper.Min(len(envName), 4)],
			"postgres_svcs",
		)
		databaseNameForClickHouse = fmt.Sprintf("%s_%s_%s_%s",
			strings.ToLower(projectName[0][:helper.Min(len(projectName[0]), 10)]),
			strings.ReplaceAll(project.GetProjectId(), "-", ""),
			strings.ToLower(envName[0])[:helper.Min(len(envName), 4)],
			"clickhouse_svcs",
		)

		userName                  = databaseName
		pass                      = helper.GeneratePassword(10, true, true, true, false)
		serviceType, resourceType int32
	)

	switch req.GetResource().GetResourceType() {
	case pb.ResourceType_CLICKHOUSE:
		resEnv, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{
			ProjectId:     req.GetProjectId(),
			EnvironmentId: req.GetEnvironmentId(),
			ResourceType:  int32(*pb.ResourceType_CLICKHOUSE.Enum()),
			IsConfigured:  true,
		})

		if err == nil {
			secret, err := s.vault.Get(ctx, resEnv.GetVaultPath())
			if err != nil {
				s.logger.Error("--AirbyteServiceGetByIdVaultGet--", l.Error(err))
				return nil, err
			}

			if req.IsSuperset {
				supersetName, _ := helper.GenerateRandomWord(4)
				randomPassword, _ := helper.GenerateRandomPassword(9)
				err = airbyte.CreateSupersetConnection(airbyte.CreateSupersetConnectionRequest{
					DatabaseName:     projectName[0],
					Host:             resEnv.GetHost(),
					Port:             s.cfg.UcodeClickhouseHTTPPort,
					Database:         resEnv.GetDatabase(),
					Username:         resEnv.GetUsername(),
					Password:         cast.ToString(secret["password"]),
					SupersetPassword: randomPassword,
					SupersetName:     fmt.Sprintf("%v_%v", projectName[0], supersetName),
					Cfg:              s.cfg,
				})
				if err != nil {
					return nil, status.Error(codes.Internal, err.Error())
				}

				_, err = s.storage.Resource().AddResourceToProject(ctx, &pb.AddResourceToProjectRequest{
					ProjectId:     req.GetProjectId(),
					EnvironmentId: req.GetEnvironmentId(),
					Type:          pb.ResourceType_SUPERSET,
					Settings: &pb.Settings{
						Superset: &pb.SupersetCredentials{
							Username: fmt.Sprintf("%v_%v", projectName[0], supersetName),
							Password: randomPassword,
							Url:      s.cfg.SupersetBaseUrl,
						},
					},
					Name: "Superset",
				})
				if err != nil {
					s.logger.Error("--AddResourceToProject--", l.Error(err))
					return nil, status.Error(codes.Internal, err.Error())
				}
			}

			return &pb.CreateResourceRes{
				Id:           resEnv.GetResourceId(),
				ProjectId:    req.GetProjectId(),
				CompanyId:    req.GetCompanyId(),
				ResourceType: pb.ResourceType_CLICKHOUSE,
			}, nil

		}

		resourceType = int32(*pb.ResourceType_CLICKHOUSE.Enum())
		serviceType = int32(*pb.ServiceType_BUILDER_SERVICE.Enum())

		var credential = pb.Resource_Credentials{
			Host:     cfg.UcodeClickhouseHost,
			Port:     cfg.UcodeClickhousePort,
			Username: databaseNameForClickHouse,
			Password: pass,
			Database: databaseNameForClickHouse,
		}

		chClientConn, errConnect := ch.NewClickHouse(cfg)
		if errConnect != nil {
			errConnClickHouse := errors.New("error unable to connect clickhouse db")
			s.logger.Error("--CreateResourceClickHouseConnection--", l.Error(errConnect))
			return nil, status.Error(codes.Internal, errConnClickHouse.Error())
		}

		errorClickHouseUserCreate := chClientConn.CreateUserAndDatabase(ctx, ch.CreateUser{
			UserName: databaseNameForClickHouse,
			Pass:     pass,
			DbName:   databaseNameForClickHouse,
		})
		if errorClickHouseUserCreate != nil {
			errCreateUser := errors.New("Error while creating clickhouse user ->" + errorClickHouseUserCreate.Error())
			s.logger.Error("--CreateResourceCreateUserAndDatabase--", l.Error(errorClickHouseUserCreate))
			return nil, status.Error(codes.Internal, errCreateUser.Error())
		}
		defer chClientConn.CloseDb()
		defer func(data ch.DeleteUser) {
			if errAddResource != nil {
				errDelete := chClientConn.DeleteUserAndDatabase(context.Background(), ch.DeleteUser{
					UserName: databaseNameForClickHouse,
					DbName:   databaseNameForClickHouse,
				})
				if errDelete != nil {
					s.logger.Error("--CreateResourceDeleteUserAndDatabase--", l.Error(errDelete))
				}
			}
		}(ch.DeleteUser{
			DbName:   databaseName,
			UserName: userName,
		})

		req.Resource.Credentials = &credential
		isDefault = false
	case pb.ResourceType_MONGODB:
		var credential = pb.Resource_Credentials{
			Host:     cfg.UcodeMongoHost,
			Port:     cfg.UcodeMongoPort,
			Username: userName,
			Password: pass,
			Database: databaseName,
		}
		resourceType = int32(*pb.ResourceType_MONGODB.Enum())
		serviceType = int32(*pb.ServiceType_BUILDER_SERVICE.Enum())

		clientConn, errConnect := mongodb.Connection(ctx, cfg, req.NodeType)
		if errConnect != nil {
			errConnMongo := errors.New("error unable to connect mongo cluster")
			s.logger.Error("--CreateResource--", l.Error(errConnect))
			return nil, status.Error(codes.Internal, errConnMongo.Error())
		}
		defer clientConn.Disconnect(ctx)
		errorUserCreate = clientConn.CreateUser(ctx, mongodb.CreateUser{
			Db:       databaseName,
			UserName: userName,
			Pass:     pass,
		})

		if errorUserCreate != nil {
			errCreateUser := errors.New("Error while creating user ->" + errorUserCreate.Error())
			s.logger.Error("--CreateResource--", l.Error(errorUserCreate))
			return nil, status.Error(codes.Internal, errCreateUser.Error())
		}
		defer func(data mongodb.DeleteUser) {
			if errAddResource != nil {
				errDelete := clientConn.DeleteUser(context.Background(), data)
				s.logger.Error("--DeletingUser--", l.Error(errDelete))
			}
		}(mongodb.DeleteUser{
			Db:       databaseName,
			UserName: userName,
		})
		req.Resource.Credentials = &credential
	case pb.ResourceType_POSTGRESQL:
		resourceType = int32(*pb.ResourceType_POSTGRESQL.Enum())
		serviceType = int32(*pb.ServiceType_BUILDER_SERVICE.Enum())

		var credential = pb.Resource_Credentials{
			Host:     cfg.NodePostgresHost,
			Port:     strconv.Itoa(cfg.NodePostgresPort),
			Username: databaseNameForPostgres,
			Password: pass,
			Database: databaseNameForPostgres,
		}

		if cfg.NodePostgresHost == "" {
			s.logger.Error("--CreateResourcePostgresConnection--", l.String("error", "NodePostgresHost is not configured"))
			return nil, status.Error(codes.Internal, "postgres node host is not configured")
		}
		pgClientConn, errConnect := postgres_client.NewPostgres(ctx, cfg)
		if errConnect != nil {
			s.logger.Error("--CreateResourcePostgresConnection--", l.Error(errConnect))
			return nil, status.Error(codes.Internal, fmt.Sprintf("error unable to connect postgres db: %v", errConnect))
		}
		errorPostgresUserCreate := pgClientConn.CreateUserAndDatabase(ctx, postgres_client.CreatePostgresUser{
			UserName: databaseNameForPostgres,
			Pass:     pass,
			Db:       databaseNameForPostgres,
		})
		if errorPostgresUserCreate != nil {
			errCreateUser := errors.New("Error while creating postgres user ->" + errorPostgresUserCreate.Error())
			s.logger.Error("--CreateResourceCreateUserAndDatabase--", l.Error(errorPostgresUserCreate))
			return nil, status.Error(codes.Internal, errCreateUser.Error())
		}
		defer pgClientConn.CloseDB()
		defer func() {
			if errAddResource != nil {
				errDelete := pgClientConn.DeleteUserAndDatabase(context.Background(), postgres_client.DeletePostgresUser{
					UserName: databaseNameForPostgres,
					Db:       databaseNameForPostgres,
				})
				if errDelete != nil {
					s.logger.Error("--CreateResourceDeleteUserAndDatabase--", l.Error(errDelete))
				}
			}
		}()
		req.Resource.Credentials = &credential
	default:
		s.logger.Error("--CreateResourceDefault--", l.Any("msg", "resource type not specified"))
		return nil, status.Error(codes.InvalidArgument, "resource type not specified")
	}

	addResResp, errAddResource := s.AddResource(ctx, &pb.AddResourceRequest{
		CompanyId:     req.GetCompanyId(),
		ProjectId:     req.GetProjectId(),
		UserId:        req.GetUserId(),
		EnvironmentId: environment.GetId(),
		ResourceType:  pb.ResourceType(resourceType),
		ServiceType:   pb.ServiceType(serviceType),
		Title:         req.GetName(),
		IsConfigured:  true,
		IsDefault:     isDefault,
		Credentials:   req.Resource.Credentials,
		NodeType:      req.GetNodeType(),
		ClientTypeId:  req.GetClientTypeId(),
		RoleId:        req.GetRoleId(),
		IsSuperset:    req.GetIsSuperset(),
	})

	if errAddResource != nil {
		s.logger.Error("--CreateResourceAddResource--", l.Error(errAddResource))
		return nil, errAddResource
	}

	return &pb.CreateResourceRes{
		ProjectId:    req.GetProjectId(),
		CompanyId:    req.GetCompanyId(),
		ResourceType: pb.ResourceType(resourceType),
		ServiceType:  pb.ServiceType(serviceType),
		NodeType:     req.GetNodeType(),
		Id:           addResResp.GetId(),
	}, nil
}

func (s *ResourceService) RemoveResource(ctx context.Context, req *pb.RemoveResourceRequest) (*pb.EmptyProto, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.RemoveResource", req)
	defer dbSpan.Finish()

	s.logger.Info("--RemoveResource--", l.Any("req", req))

	cfg := s.cfg

	resource, err := s.storage.Resource().GetResource(ctx,
		&pb.GetResourceRequest{
			Id: req.Id,
		},
	)
	if err != nil {
		s.logger.Error("--RemoveResource--", l.Error(err))
		return nil, err
	}

	// @TODO fix logic with service type
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		clientConn, err := mongodb.Connection(ctx, cfg, resource.NodeType)
		if err != nil {
			s.logger.Error("--RemoveResource--", l.Error(err))
			return nil, err
		}
		defer clientConn.Disconnect(ctx)

		err = clientConn.DeleteUser(ctx, mongodb.DeleteUser{
			Db:       resource.Credentials.Database,
			UserName: resource.Credentials.Username,
		})

		if err != nil {
			s.logger.Error("--RemoveResource--", l.Error(err))
		}
	case pb.ResourceType_POSTGRESQL:
		clientConn, err := postgres_client.NewPostgres(ctx, cfg)
		if err != nil {
			s.logger.Error("--RemoveResource--", l.Error(err))
			return nil, err
		}
		defer clientConn.CloseDB()
		err = clientConn.DeleteUserAndDatabase(ctx, postgres_client.DeletePostgresUser{
			Db:       resource.Credentials.Database,
			UserName: resource.Credentials.Username,
		})

		if err != nil {
			s.logger.Error("--RemoveResource--", l.Error(err))
		}
	}

	_, err = s.storage.Resource().RemoveResourceEnvironmentByResourceID(ctx,
		&pb.ResourceEnvironment{
			ResourceId: req.Id,
		},
	)
	if err != nil {
		s.logger.Error("--RemoveResource--", l.Error(err))
		return nil, err
	}

	_, err = s.storage.Resource().RemoveResource(ctx, req)
	if err != nil {
		s.logger.Error("--RemoveResource--", l.Error(err))
		return nil, err
	}

	return &pb.EmptyProto{}, nil
}

func (s *ResourceService) GetResource(ctx context.Context, in *pb.GetResourceRequest) (*pb.Resource, error) {
	s.logger.Info("--GetResource-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResource", in)
	defer dbSpan.Finish()

	var (
		respEnvironments                         []*pb.Environment
		host, port, database, username, password string
	)

	resource, err := s.storage.Resource().GetResource(ctx, in)
	if err != nil {
		s.logger.Error("--GetResource--", l.Error(err))
		return nil, err
	}

	secret, err := s.vault.Get(ctx, resource.VaultPath)
	if err != nil {
		s.logger.Error("-- GetResource: Get from serviceNode --", l.Error(err))
		return nil, err
	}

	bytes, err := json.Marshal(secret)
	if err != nil {
		s.logger.Error("-- GetResource: Marshal--", l.Error(err))
		return nil, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		mongoSecret := pb.MongodbCredentials{}
		err = json.Unmarshal(bytes, &mongoSecret)
		if err != nil {
			s.logger.Error("--GetResource: Mongodb Unmarshal--", l.Error(err))
			return nil, err
		}
		host = mongoSecret.Host
		port = mongoSecret.Port
		database = mongoSecret.Database
		username = mongoSecret.Username
		password = mongoSecret.Password
	case pb.ResourceType_CLICKHOUSE:
		clihouseSecret := pb.ClickhouseCredentials{}
		err = json.Unmarshal(bytes, &clihouseSecret)
		if err != nil {
			s.logger.Error("--GetResource: Clickhouse Unmarshal--", l.Error(err))
			return nil, err
		}
		host = clihouseSecret.Host
		port = clihouseSecret.Port
		database = clihouseSecret.Database
		username = clihouseSecret.Username
		password = clihouseSecret.Password
	}

	resourceEnvironments, err := s.storage.Resource().GetListResourceEnvironment(ctx,
		&pb.GetListResourceEnvironmentReq{
			ResourceId: resource.Id,
			ProjectId:  resource.ProjectId,
		},
	)
	if err != nil {
		s.logger.Error("--GetResource--", l.Error(err))
		return nil, err
	}

	envM := make(map[string]*pb.ResourceEnvironment)

	for _, v := range resourceEnvironments {
		envM[v.GetEnvironmentId()] = v
	}

	projectEnvs, err := s.storage.Environment().GetList(ctx, &pb.GetEnvironmentListRequest{
		ProjectId: resource.ProjectId,
	})
	if err != nil {
		s.logger.Error("--GetResource--", l.Error(err))
		return nil, err
	}

	for _, environ := range projectEnvs.Environments {
		if _, ok := envM[environ.GetId()]; ok {
			respEnvironments = append(respEnvironments, &pb.Environment{
				ResourceEnvironmentId: envM[environ.Id].GetId(),
				Name:                  environ.Name,
				DisplayColor:          environ.DisplayColor,
				Description:           environ.Description,
				IsConfigured:          envM[environ.Id].IsConfigured,
				Id:                    environ.Id,
				NodeType:              envM[environ.Id].GetNodeType(),
			})
		} else {
			respEnvironments = append(respEnvironments, &pb.Environment{
				ResourceEnvironmentId: "",
				Name:                  environ.Name,
				DisplayColor:          environ.DisplayColor,
				Description:           environ.Description,
				IsConfigured:          false,
				Id:                    environ.Id,
				NodeType:              envM[environ.Id].GetNodeType(),
			})
		}
	}

	return &pb.Resource{
		ResourceType: resource.ResourceType,
		Credentials: &pb.Resource_Credentials{
			Host:     host,
			Port:     port,
			Database: database,
			Username: username,
			Password: password,
		},
		Id:           resource.GetId(),
		ProjectId:    resource.GetProjectId(),
		Title:        resource.GetTitle(),
		Environments: respEnvironments,
	}, nil
}

func (s *ResourceService) GetResourceList(ctx context.Context, in *pb.GetResourceListRequest) (*pb.GetResourceListResponse, error) {
	s.logger.Info("--GetResourceList-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResourceList", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().GetResourceList(ctx, in)
	if err != nil {
		s.logger.Error("--GetReourceList--", l.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *ResourceService) ReconnectResource(ctx context.Context, in *pb.ReconnectResourceRequest) (*pb.ReconnectResourceRes, error) {
	s.logger.Info("--ReconnectResource-- requested\n", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.ReconnectResource", in)
	defer dbSpan.Finish()

	var (
		res pb.ReconnectResourceRes
		wg  sync.WaitGroup
	)
	// @TODO added other services
	resource, err := s.storage.Resource().GetResource(ctx,
		&pb.GetResourceRequest{
			Id: in.Id,
		},
	)
	if err != nil {
		s.logger.Error("--ReconnectResource--", l.Error(err))
		return nil, err
	}

	serviceResources, err := s.storage.ServiceResource().GetList(
		ctx,
		&pb.GetListServiceResourceReq{
			ProjectId:  in.ProjectId,
			ResourceId: in.Id,
		},
	)
	if err != nil {
		s.logger.Error("--ReconnectResource--", l.Error(err))
		return nil, err
	}

	servicePool, err := s.services.GetByNodeType(in.ProjectId, resource.NodeType)
	if err != nil {
		s.logger.Error("-- Get from serviceNode --", l.Error(err))
		return nil, err
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		secret, err := s.vault.Get(ctx, resource.VaultPath)
		if err != nil {
			s.logger.Error("--ReconnectResource--", l.Error(err))
			return nil, err
		}

		bytes, err := json.Marshal(secret)
		if err != nil {
			s.logger.Error("--ReconnectResource--", l.Error(err))
			return nil, err
		}

		mongoSecret := pb.MongodbCredentials{}
		err = json.Unmarshal(bytes, &mongoSecret)
		if err != nil {
			s.logger.Error("--ReconnectResource--", l.Error(err))
			return nil, err
		}

		for _, service := range serviceResources.GetServiceResources() {
			wg.Add(1)
			go func(arg *pb.ServiceResourceModel) {
				defer wg.Done()
				switch arg.ServiceType {
				case pb.ServiceType_BUILDER_SERVICE:
					_, err = servicePool.GetBuilderProjectServiceByType(resource.NodeType).Reconnect(
						ctx,
						&object_builder_service.RegisterProjectRequest{
							K8SNamespace: "cp-region-type-id",
							ProjectId:    arg.ResourceEnvironmentId,
							SecretPath:   resource.VaultPath,
							Credentials: &object_builder_service.RegisterProjectRequest_Credentials{
								Host:     mongoSecret.Host,
								Port:     mongoSecret.Port,
								Username: mongoSecret.Username,
								Password: mongoSecret.Password,
								Database: mongoSecret.Database,
							},
						},
					)
					if err != nil {
						s.logger.Error("--ReconnectResource-- [BuilderProjectService]", l.Error(err))
						res.ReconnectStatus = append(res.ReconnectStatus, &pb.ReconnectResourceRes_ReconnectStatus{
							ServiceType: arg.ServiceType,
							Status:      false,
						})
					} else {
						s.logger.Error("--ReconnectResource-- success")
						res.ReconnectStatus = append(res.ReconnectStatus, &pb.ReconnectResourceRes_ReconnectStatus{
							ServiceType: arg.ServiceType,
							Status:      true,
						})
					}
				case pb.ServiceType_FUNCTION_SERVICE:
					_, err = servicePool.FunctionProjectService().Reconnect(
						ctx,
						&new_function_service.RegisterProjectRequest{
							K8SNamespace: "cp-region-type-id",
							ProjectId:    arg.ResourceEnvironmentId,
							SecretPath:   resource.VaultPath,
							Credentials: &new_function_service.RegisterProjectRequest_Credentials{
								Host:     mongoSecret.Host,
								Port:     mongoSecret.Port,
								Username: mongoSecret.Username,
								Password: mongoSecret.Password,
								Database: mongoSecret.Database,
							},
						},
					)
					if err != nil {
						s.logger.Error("--ReconnectResource-- [FunctionProjectService]", l.Error(err))
						res.ReconnectStatus = append(res.ReconnectStatus, &pb.ReconnectResourceRes_ReconnectStatus{
							ServiceType: arg.ServiceType,
							Status:      false,
						})
						//return nil, err
					} else {
						res.ReconnectStatus = append(res.ReconnectStatus, &pb.ReconnectResourceRes_ReconnectStatus{
							ServiceType: arg.ServiceType,
							Status:      true,
						})
					}
				}
			}(service)
		}
		wg.Wait()
	case pb.ResourceType_POSTGRESQL:
		secret, err := s.vault.Get(ctx, resource.VaultPath)
		if err != nil {
			s.logger.Error("--ReconnectResource--", l.Error(err))
			return nil, err
		}

		bytes, err := json.Marshal(secret)
		if err != nil {
			s.logger.Error("--ReconnectResource--", l.Error(err))
			return nil, err
		}

		postgresSecret := pb.PostgresSqlCredentials{}
		err = json.Unmarshal(bytes, &postgresSecret)
		if err != nil {
			s.logger.Error("--ReconnectResource--", l.Error(err))
			return nil, err
		}

		for _, service := range serviceResources.GetServiceResources() {
			wg.Add(1)
			go func(arg *pb.ServiceResourceModel) {
				defer wg.Done()
				switch arg.ServiceType {
				case pb.ServiceType_BUILDER_SERVICE:
					ctxPostgres, finish := context.WithTimeout(ctx, 30*time.Second)
					defer finish()
					_, err = servicePool.GoBuilderProjectService().Reconnect(
						ctxPostgres,
						&nobs.RegisterProjectRequest{
							K8SNamespace: "cp-region-type-id",
							ProjectId:    arg.ResourceEnvironmentId,
							SecretPath:   resource.VaultPath,
							Credentials: &nobs.RegisterProjectRequest_Credentials{
								Host:     postgresSecret.Host,
								Port:     postgresSecret.Port,
								Username: postgresSecret.Username,
								Password: postgresSecret.Password,
								Database: postgresSecret.Database,
							},
						},
					)
					if err != nil {
						s.logger.Error("--ReconnectResource-- [PostgresBuilderProjectService]", l.Error(err))
						res.ReconnectStatus = append(res.ReconnectStatus, &pb.ReconnectResourceRes_ReconnectStatus{
							ServiceType: arg.ServiceType,
							Status:      false,
						})
						//return nil, err
					} else {
						res.ReconnectStatus = append(res.ReconnectStatus, &pb.ReconnectResourceRes_ReconnectStatus{
							ServiceType: arg.ServiceType,
							Status:      true,
						})
					}
				}
			}(service)
		}
		wg.Wait()

	default:
		err = errors.New("not valid resource_type")
		s.logger.Error("--ReconnectResource--", l.Error(err))
		return nil, err
	}

	return &res, nil
}

func (s *ResourceService) AutoConnect(ctx context.Context, in *pb.GetProjectsRequest) (*pb.GetResourceManyWithPathResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.AutoConnect", in)
	defer dbSpan.Finish()

	s.logger.Info("--AutoConnect-- requested", l.Any("req", in))
	var wg sync.WaitGroup

	projectsId, err := s.storage.Project().GetProjectResources(ctx, &pb.GetProjectsRequest{K8SNamespace: in.GetK8SNamespace()})
	if err != nil {
		s.logger.Error("--AutoConnect--", l.Error(err))
		return nil, err
	}

	resource, err := s.storage.Resource().GetResourceManyWithPath(ctx, &pb.GetResourceManyRequest{Id: projectsId.Response, NodeType: in.GetNodeType()})
	if err != nil {
		s.logger.Error("--AutoConnect--", l.Error(err))
		return nil, err
	}

	for i := range resource.Res {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			secret, err := s.vault.Get(ctx, resource.Res[index].Path)
			if err != nil {
				s.logger.Error("--AutoConnect--", l.Error(err))
				return
			}

			bytes, err := json.Marshal(secret)
			if err != nil {
				s.logger.Error("--AutoConnect--", l.Error(err))
				return
			}
			postgresSecret := pb.PostgresSqlCredentials{}
			mongoSecret := pb.MongodbCredentials{}
			switch resource.Res[index].ResourceType {
			case pb.ResourceType_MONGODB:
				err = json.Unmarshal(bytes, &mongoSecret)
				if err != nil {
					s.logger.Error("--AutoConnect--", l.Error(err))
					return
				}
				resource.Res[index].Credentials.Password = mongoSecret.GetPassword()
			case pb.ResourceType_POSTGRESQL:
				postgresSecret = pb.PostgresSqlCredentials{}
				err = json.Unmarshal(bytes, &postgresSecret)
				if err != nil {
					s.logger.Error("--AutoConnect--", l.Error(err))
					return
				}
				// TODO DELETE : GetDatabase, GetHost, GetPassword, GetPort, GetUsername
				resource.Res[index].Credentials.Database = postgresSecret.GetDatabase()
				resource.Res[index].Credentials.Host = postgresSecret.GetHost()
				resource.Res[index].Credentials.Password = postgresSecret.GetPassword()
				resource.Res[index].Credentials.Port = postgresSecret.GetPort()
				resource.Res[index].Credentials.Username = postgresSecret.GetUsername()
			}

			resource.Res[index].ProjectId = resource.Res[index].Id
		}(i)
	}
	wg.Wait()

	return resource, nil
}

func (s *ResourceService) AutoConnectByProjectId(ctx context.Context, in *pb.AutoConnectByProjectIdRequest) (*pb.AutoConnectByProjectIdResponse, error) {
	s.logger.Info("--AutoConnect-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.AutoConnectByProjectId", in)
	defer dbSpan.Finish()

	resource := &pb.AutoConnectByProjectIdResponse{}

	resEnv, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{Id: in.ProjectId, NodeType: in.GetNodeType()})
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	secret, err := s.vault.Get(ctx, resEnv.VaultPath)
	if err != nil {
		s.logger.Error("--AutoConnectByProjectId--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if secret == nil {
		s.logger.Error("--AutoConnectByProjectId--")
		return nil, status.Error(codes.Internal, "No creds!")
	}

	ResourceEnvironment := &pb.AutoConnectByProjectIdResponse_ResourceEnvironment{}
	Credentials := &pb.AutoConnectByProjectIdResponse_ResourceEnvironment_Credentials{}
	Credentials.Database = cast.ToString(secret["database"])
	Credentials.Username = cast.ToString(secret["username"])
	Credentials.Port = cast.ToString(secret["port"])
	Credentials.Host = cast.ToString(secret["host"])
	Credentials.Password = cast.ToString(secret["password"])

	ResourceEnvironment.ProjectId = resEnv.ProjectId
	ResourceEnvironment.Id = resEnv.Id
	ResourceEnvironment.Credentials = Credentials
	ResourceEnvironment.ResourceType = pb.ResourceType(resEnv.ResourceType)
	ResourceEnvironment.ServiceType = pb.ServiceType(resEnv.ServiceType)

	resource.Res = ResourceEnvironment

	return resource, nil
}

func (s *ResourceService) UpsertResourceEnvironment(ctx context.Context, in *pb.UpsertResourceEnvironmentRequest) (*pb.UpsertResourceEnvironmentResponse, error) {
	s.logger.Info("--UpsertResourceEnvironment-- requested")
	// Does Not Implemented
	return nil, errors.New("err not implemented")
}

func (s *ResourceService) UpdateResource(ctx context.Context, req *pb.UpdateResourceRequest) (*pb.ResourceWithoutPassword, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.UpdateResource", req)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateResource-- requested", l.Any("req", req))

	secretPath, err := s.vault.GetSecretPath(ctx)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	resEnviron, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{Id: req.Id})
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	project, err := s.storage.Project().GetById(ctx, resEnviron.ProjectId)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	path := fmt.Sprintf("%s/%s/%s/%s/%s",
		strings.Trim(secretPath, "/"),
		project.CompanyId,
		project.ProjectId,
		resEnviron.ResourceId,
		req.ResourceType.String(),
	)
	data := map[string]any{}

	secret, err := s.vault.Get(ctx, path)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, err
	}

	credentialPassword, ok := secret["password"].(string)
	if !ok {
		err := errors.New("err password is missing in vault")
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, err
	}

	if req.GetCredentials().GetPassword() != "" {
		credentialPassword = req.GetCredentials().GetPassword()
	}

	var credentialData any

	servicePool, err := s.services.GetByNodeType(resEnviron.ProjectId, resEnviron.NodeType)
	if err != nil {
		s.logger.Error("-- Get from serviceNode --", l.Error(err))
		return nil, err
	}

	switch req.GetResourceType() {
	case pb.ResourceType_CLICKHOUSE:
		credentialData = pb.ClickhouseCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: credentialPassword,
			Database: req.Credentials.GetDatabase(),
		}
	case pb.ResourceType_MONGODB:
		credentialData = pb.MongodbCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: credentialPassword,
			Database: req.Credentials.GetDatabase(),
		}
	default:
		s.logger.Error("--UpdateResource--", l.Any("msg", "resource type not specified"))
		return nil, status.Error(codes.InvalidArgument, "resource type not specified")
	}

	switch resEnviron.ServiceType {
	case int32(pb.ServiceType_BUILDER_SERVICE):
		_, err := servicePool.GetBuilderProjectServiceByType(resEnviron.NodeType).Reconnect(
			ctx,
			&object_builder_service.RegisterProjectRequest{
				K8SNamespace: "cp-region-type-id",
				ProjectId:    resEnviron.GetId(),
				SecretPath:   path,
				Credentials: &object_builder_service.RegisterProjectRequest_Credentials{
					Host:     req.Credentials.GetHost(),
					Port:     req.Credentials.GetPort(),
					Username: req.Credentials.GetUsername(),
					Password: credentialPassword,
					Database: req.Credentials.GetDatabase(),
				},
				UserId:     "@TODO::", // @TODO
				ResourceId: req.Id,
			},
		)
		if err != nil {
			s.logger.Error("--UpdateResource--", l.Error(err))
			return nil, err
		}
	default:
		return nil, errors.New("wrong service type")
	}

	bytes, err := json.Marshal(credentialData)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, err
	}

	err = json.Unmarshal(bytes, &data)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, err
	}

	err = s.vault.Put(ctx, path, data)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, err
	}

	_, err = s.storage.Resource().UpdateResource(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateResource--", l.Error(err))
		return nil, err
	}

	_, err = s.storage.Resource().UpdateResourceEnvironment(ctx, &pb.ResourceEnvironment{
		Id:            resEnviron.Id,
		ProjectId:     resEnviron.ProjectId,
		ResourceId:    resEnviron.ResourceId,
		EnvironmentId: resEnviron.EnvironmentId,
		IsConfigured:  req.IsConfigured,
		VaultPath:     path,
		ServiceType:   resEnviron.ServiceType,
		ResourceType:  int32(req.ResourceType),
		Host:          req.Credentials.Host,
		Port:          req.Credentials.Port,
		Username:      req.Credentials.Username,
		Database:      req.Credentials.Database,
		Default:       req.GetDefault(),
		NodeType:      req.GetNodeType(),
	})
	if err != nil {
		s.logger.Error("--UpdateResourceEnvironment--", l.Error(err))
		return nil, err
	}

	if req.GetDefault() {
		_, err = s.storage.Resource().UpdateDefaultResourceEnvironment(
			ctx,
			&pb.ResourceEnvironment{
				ProjectId:     resEnviron.GetProjectId(),
				ResourceId:    resEnviron.GetResourceId(),
				EnvironmentId: resEnviron.GetEnvironmentId(),
			},
		)
		if err != nil {
			s.logger.Error("--UpdateResource--", l.Error(err))
		}
	}

	return &pb.ResourceWithoutPassword{
		ResourceType: req.ResourceType,
		Credentials: &pb.ResourceWithoutPassword_Credentials{
			Host:     req.Credentials.Host,
			Port:     req.Credentials.Port,
			Username: req.Credentials.Username,
			Database: req.Credentials.Database,
		},
		Id:           resEnviron.Id,
		ProjectId:    resEnviron.ProjectId,
		Title:        req.Title,
		Environments: []*pb.ResourceWithoutPassword_Environment{},
	}, nil
}

func (s *ResourceService) ConfigureResource(ctx context.Context, req *pb.ConfigureResourceRequest) (*pb.ConfigureResourceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.ConfigureResource", req)
	defer dbSpan.Finish()

	s.logger.Info("--ConfigureResource-- requested", l.Any("req", req))

	resource, err := s.storage.Resource().GetResource(ctx, &pb.GetResourceRequest{Id: req.ResourceId})
	if err != nil {
		s.logger.Error("--ConfigureResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	secretPath, err := s.vault.GetSecretPath(ctx)
	if err != nil {
		s.logger.Error("--ConfigureResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	var credentialData any
	resourceType := req.ResourceType
	serviceType := req.ServiceType

	switch req.GetResourceType() {
	case pb.ResourceType_CLICKHOUSE:
		credentialData = pb.ClickhouseCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: req.Credentials.GetPassword(),
			Database: req.Credentials.GetDatabase(),
		}

	case pb.ResourceType_MONGODB:
		credentialData = pb.MongodbCredentials{
			Host:     req.Credentials.GetHost(),
			Port:     req.Credentials.GetPort(),
			Username: req.Credentials.GetUsername(),
			Password: req.Credentials.GetPassword(),
			Database: req.Credentials.GetDatabase(),
		}
	default:
		s.logger.Error("--ConfigureResource--", l.Any("msg", "resource type not specified"))
		return nil, status.Error(codes.InvalidArgument, "resource type not specified")
	}

	path := fmt.Sprintf("%s/%s/%s/%s/%s", strings.Trim(secretPath, "/"),
		req.CompanyId,
		req.ProjectId,
		req.ResourceId,
		req.ResourceType.String())
	data := map[string]any{}

	servicePool, err := s.services.GetByNodeType(resource.ProjectId, resource.NodeType)
	if err != nil {
		s.logger.Error("-- Get from serviceNode --", l.Error(err))
		return nil, err
	}

	switch req.ServiceType {
	case pb.ServiceType_BUILDER_SERVICE:
		_, err := servicePool.GetBuilderProjectServiceByType(resource.NodeType).Register(
			ctx,
			&object_builder_service.RegisterProjectRequest{
				K8SNamespace: "cp-region-type-id",
				ProjectId:    req.ProjectId, // @TODO resource environment
				SecretPath:   path,
				Credentials: &object_builder_service.RegisterProjectRequest_Credentials{
					Host:     req.Credentials.GetHost(),
					Port:     req.Credentials.GetPort(),
					Username: req.Credentials.GetUsername(),
					Password: req.Credentials.GetPassword(),
					Database: req.Credentials.GetDatabase(),
				},
				UserId:     req.UserId,
				ResourceId: req.ResourceId,
			},
		)
		if err != nil {
			s.logger.Error("--ConfigureResource--", l.Error(err))
			return nil, err
		}
	default:
		return nil, errors.New("wrong service type")
	}

	bytes, err := json.Marshal(credentialData)
	if err != nil {
		s.logger.Error("--ConfigureResource--", l.Error(err))
		return nil, err
	}

	err = json.Unmarshal(bytes, &data)
	if err != nil {
		s.logger.Error("--ConfigureResource--", l.Error(err))
		return nil, err
	}

	err = s.vault.Put(ctx, path, data)
	if err != nil {
		s.logger.Error("--ConfigureResource--", l.Error(err))
		return nil, err
	}

	_, err = s.storage.Resource().InsertResourceEnvironment(ctx, &pb.ResourceEnvironment{
		ProjectId:     req.ProjectId,
		ResourceId:    req.ResourceId,
		EnvironmentId: req.EnvironmentId,
		IsConfigured:  req.IsConfigured,
		VaultPath:     path,
		ServiceType:   int32(serviceType),
		ResourceType:  int32(resourceType),
		Host:          req.Credentials.Host,
		Port:          req.Credentials.Port,
		Database:      req.Credentials.Database,
		Username:      req.Credentials.Username,
		Default:       req.GetDefault(),
	})
	if err != nil {
		s.logger.Error("--ConfigureResource--", l.Error(err))
		return nil, err
	}

	if req.GetDefault() {
		_, err = s.storage.Resource().UpdateDefaultResourceEnvironment(
			ctx,
			&pb.ResourceEnvironment{
				ProjectId:     req.GetProjectId(),
				ResourceId:    req.GetResourceId(),
				EnvironmentId: req.GetEnvironmentId(),
			},
		)
		if err != nil {
			s.logger.Error("--ConfigureResource--", l.Error(err))
		}
	}

	return &pb.ConfigureResourceResponse{
		Id: req.ResourceId,
	}, nil
}

func (s *ResourceService) GetResourceByResEnvironId(ctx context.Context, in *pb.GetResourceRequest) (*pb.ResourceWithoutPassword, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResourceByResEnvironId", in)
	defer dbSpan.Finish()

	s.logger.Info("--GetResourceByResEnvironId-- requested")

	resourceEnvironment, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{Id: in.Id})
	if err != nil {
		s.logger.Error("--GetResourceByResEnvironId--", l.Error(err))
		return nil, err
	}

	resource, err := s.storage.Resource().GetResource(ctx, &pb.GetResourceRequest{Id: resourceEnvironment.ResourceId})
	if err != nil {
		s.logger.Error("--GetResourceByResEnvironId--", l.Error(err))
		return nil, err
	}

	resourceEnvironments, err := s.storage.Resource().GetListResourceEnvironment(ctx, &pb.GetListResourceEnvironmentReq{
		ResourceId: resource.Id,
		ProjectId:  resource.ProjectId,
	})
	if err != nil {
		s.logger.Error("--GetResourceByResEnvironId--", l.Error(err))
		return nil, err
	}

	resourceEnvironmentsMap := map[string]*pb.ResourceEnvironment{}
	environmentIDs := []string{}
	for _, environ := range resourceEnvironments {
		environmentIDs = append(environmentIDs, environ.EnvironmentId)
		resourceEnvironmentsMap[environ.EnvironmentId] = environ
	}

	respEnvironments := []*pb.ResourceWithoutPassword_Environment{}

	environments, err := s.storage.Environment().GetListWithPKs(ctx, environmentIDs)
	if err != nil {
		s.logger.Error("--GetResourceByResEnvironId--", l.Error(err))
		return nil, err
	}

	for _, environ := range environments.Environments {
		respEnvironments = append(respEnvironments, &pb.ResourceWithoutPassword_Environment{
			ResourceEnvironmentId: resourceEnvironmentsMap[environ.Id].Id,
			Name:                  environ.Name,
			DisplayColor:          environ.DisplayColor,
			Description:           environ.Description,
			IsConfigured:          resourceEnvironmentsMap[environ.Id].IsConfigured,
			Id:                    environ.Id,
			Default:               resourceEnvironmentsMap[environ.GetId()].GetDefault(),
		})
	}

	noEnvironments, err := s.storage.Environment().GetList(ctx, &pb.GetEnvironmentListRequest{
		ProjectId: resource.ProjectId,
	})
	if err != nil {
		s.logger.Error("--GetResourceByResEnvironId--", l.Error(err))
		return nil, err
	}

	for _, environ := range noEnvironments.Environments {
		if _, ok := resourceEnvironmentsMap[environ.Id]; !ok {
			respEnvironments = append(respEnvironments, &pb.ResourceWithoutPassword_Environment{
				ResourceEnvironmentId: "",
				Name:                  environ.Name,
				DisplayColor:          environ.DisplayColor,
				Description:           environ.Description,
				IsConfigured:          false,
				Id:                    environ.Id,
			})
		}

	}

	if len(resourceEnvironments) < 1 {
		return &pb.ResourceWithoutPassword{
			ProjectId:    resource.ProjectId,
			Title:        resource.Title,
			Environments: respEnvironments,
		}, nil
	}

	resourceEnvironmet := resourceEnvironments[len(resourceEnvironments)-1]

	return &pb.ResourceWithoutPassword{
		ResourceType: pb.ResourceType(resourceEnvironment.ResourceType),
		Credentials: &pb.ResourceWithoutPassword_Credentials{
			Host:     resourceEnvironmet.Host,
			Port:     resourceEnvironmet.Port,
			Database: resourceEnvironmet.Database,
			Username: resourceEnvironmet.Username,
		},
		Id:           resourceEnvironmet.ResourceId,
		ProjectId:    resourceEnvironmet.ProjectId,
		Title:        resource.Title,
		Environments: respEnvironments,
	}, nil
}

func (s *ResourceService) GetResourceEnvironment(ctx context.Context, in *pb.GetResourceEnvironmentReq) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResourceEnvironment", in)
	defer dbSpan.Finish()

	s.logger.Info("--GetResourceEnvironment-- requested", l.Any("req", in))

	resourceEnvironment, err := s.storage.Resource().GetSingleResourceEnvironment(
		ctx,
		&pb.ResourceEnvironment{
			Id:            in.GetId(),
			ProjectId:     in.GetProjectId(),
			ResourceId:    in.GetResourceId(),
			EnvironmentId: in.GetEnvironmentId(),
			Username:      in.GetUsername(),
		},
	)
	if err != nil {
		s.logger.Error("--GetResourceEnvironment--", l.Error(err))
		return nil, err
	}

	return resourceEnvironment, nil
}

func (s *ResourceService) GetListResourceEnvironment(ctx context.Context, in *pb.GetListResourceEnvironmentReq) (*pb.GetListResourceEnvironmentRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetListResourceEnvironment", in)
	defer dbSpan.Finish()

	var res pb.GetListResourceEnvironmentRes

	resourceEnvironment, err := s.storage.Resource().GetListResourceEnvironment(ctx, in)
	if err != nil {
		s.logger.Error("--GetListResourceEnvironment--", l.Error(err))
		return nil, err
	}

	res.Data = resourceEnvironment

	res.Count = int32(len(res.GetData()))

	return &res, nil
}

func (s *ResourceService) GetListConfiguredResourceEnvironment(ctx context.Context, in *pb.GetListConfiguredResourceEnvironmentReq) (*pb.GetListConfiguredResourceEnvironmentRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetListConfiguredResourceEnvironment", in)
	defer dbSpan.Finish()

	res, err := s.storage.Resource().GetListConfiguredResourceEnvironment(
		ctx,
		&pb.GetListConfiguredResourceEnvironmentReq{
			Id:            in.GetId(),
			ProjectId:     in.GetProjectId(),
			ResourceId:    in.GetResourceId(),
			EnvironmentId: in.GetEnvironmentId(),
		},
	)
	if err != nil {
		s.logger.Error("--GetListConfiguredResourceEnvironment--", l.Error(err))
		return nil, err
	}

	return res, nil
}

func (s *ResourceService) GetDefaultResourceEnvironment(ctx context.Context, in *pb.GetDefaultResourceEnvironmentReq) (*pb.ResourceEnvironment, error) {
	s.logger.Info("--GetDefaultResourceEnvironment--", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetDefaultResourceEnvironment", in)
	defer dbSpan.Finish()

	resourceEnvironment, err := s.storage.Resource().GetDefaultResource(
		ctx,
		&pb.ResourceEnvironment{
			Id:            in.GetId(),
			ProjectId:     in.GetProjectId(),
			ResourceId:    in.GetResourceId(),
			EnvironmentId: in.GetEnvironmentId(),
		},
	)
	if err != nil {
		s.logger.Error("--GetDefaultResourceEnvironment--", l.Error(err))
		return nil, err
	}

	return resourceEnvironment, nil
}

func (s *ResourceService) GetResourceByEnvID(ctx context.Context, req *pb.GetResourceByEnvIDRequest) (resp *pb.GetResourceByEnvIDResponse, err error) {
	s.logger.Info("--GetResourceByEnvID--", l.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResourceByEnvID", req)
	defer dbSpan.Finish()

	resp, err = s.storage.Resource().GetResourceByEnvID(ctx, req)
	if err != nil {
		s.logger.Error("--GetResourceByEnvID--", l.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *ResourceService) GetResEnvByResIdEnvId(ctx context.Context, in *pb.GetResEnvByResIdEnvIdRequest) (*pb.ResourceEnvironment, error) {
	s.logger.Info("--GetResEnvByResIdEnvId-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResEnvByResIdEnvId", in)
	defer dbSpan.Finish()

	if !util.IsValidUUID(in.GetEnvironmentId()) {
		err := errors.New("invalid or missing environment_id")
		s.logger.Error("--GetResEnvByResIdEnvId--", l.Error(err))
		return nil, err
	}

	if !util.IsValidUUID(in.GetResourceId()) {
		err := errors.New("invalid or missing resource_id")
		s.logger.Error("--GetResEnvByResIdEnvId--", l.Error(err))
		return nil, err
	}

	resourceEnvironment, err := s.storage.Resource().GetResEnvByResIdEnvId(ctx, in)
	if err != nil {
		s.logger.Error("--GetResEnvByResIdEnvId--", l.Error(err))
		return nil, err
	}

	return resourceEnvironment, nil
}

func (s *ResourceService) GetServiceResources(ctx context.Context, in *pb.GetServiceResourcesReq) (*pb.GetServiceResourcesRes, error) {
	s.logger.Info("--GetServiceResources-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetServiceResources", in)
	defer dbSpan.Finish()

	var (
		res = pb.GetServiceResourcesRes{
			ServiceResources: []*pb.GetServiceResourcesRes_ServiceTypeResources{},
		}
	)

	m, err := s.storage.Resource().GetServiceResources(ctx, in)
	if err != nil {
		s.logger.Error("--GetServiceResources--", l.Error(err))
		return nil, err
	}

	for i := range m {
		var (
			value = pb.GetServiceResourcesRes_ServiceTypeResources{
				Resource: []*pb.GetServiceResourcesRes_ServiceTypeResources_ServiceResources{},
			}
		)

		value.ServiceType = i
		value.Resource = m[i]
		res.ServiceResources = append(res.ServiceResources, &value)
	}

	return &res, nil
}

func (s *ResourceService) SetDefaultResource(ctx context.Context, in *pb.SetDefaultResourceReq) (*pb.SetDefaultResourceRes, error) {
	s.logger.Info("--SetDefaultResource-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.SetDefaultResource", in)
	defer dbSpan.Finish()

	err := s.storage.Resource().RemoveDefaultResource(ctx, in)
	if err != nil {
		s.logger.Error("--SetDefaultResource--", l.Error(err))
		return nil, err
	}

	res, err := s.storage.Resource().SetDefaultResource(ctx, in)
	if err != nil {
		s.logger.Error("--SetDefaultResource--", l.Error(err))
	}

	return res, nil
}

func (s *ResourceService) GetResourceById(ctx context.Context, in *pb.GetResourceEnvironmentReq) (*pb.ResourceEnvironmentWithPassword, error) {
	s.logger.Info("--GetResourceById-- requested", l.Any("req", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetResourceById", in)
	defer dbSpan.Finish()

	response := pb.ResourceEnvironmentWithPassword{}

	resource, err := s.storage.Resource().GetResEnvByResIdEnvIdNotDefault(ctx, &pb.GetResEnvByResIdEnvIdRequest{
		ResourceId:    in.GetResourceId(),
		EnvironmentId: in.GetEnvironmentId(),
	})
	if err != nil {
		s.logger.Error("--GetResourceById--", l.Error(err))
		return nil, err
	}
	switch resource.ResourceType {
	case int32(pb.ResourceType_CLICKHOUSE.Number()):
		clickHouseSecret := pb.ClickhouseCredentials{}
		secret, err := s.vault.Get(ctx, resource.GetVaultPath())
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		bytes, err := json.Marshal(secret)
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		err = json.Unmarshal(bytes, &clickHouseSecret)
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		response.Credentials = &pb.ResourceEnvironmentWithPassword_Credentials{
			Database: resource.Database,
			Port:     resource.Port,
			Host:     resource.Host,
			Username: resource.Username,
			Password: clickHouseSecret.Password,
		}
	case int32(pb.ResourceType_MONGODB.Number()):
		mongoCredential := pb.MongodbCredentials{}
		secret, err := s.vault.Get(ctx, resource.GetVaultPath())
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		bytes, err := json.Marshal(secret)
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		err = json.Unmarshal(bytes, &mongoCredential)
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		response.Credentials = &pb.ResourceEnvironmentWithPassword_Credentials{
			Database: resource.Database,
			Port:     resource.Port,
			Host:     resource.Host,
			Username: resource.Username,
			Password: mongoCredential.Password,
		}
	case int32(pb.ResourceType_POSTGRESQL.Number()):
		postgresSqlSecret := pb.PostgresSqlCredentials{}
		secret, err := s.vault.Get(ctx, resource.GetVaultPath())
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		bytes, err := json.Marshal(secret)
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		err = json.Unmarshal(bytes, &postgresSqlSecret)
		if err != nil {
			s.logger.Error("--GetResourceById--", l.Error(err))
			return nil, err
		}

		response.Credentials = &pb.ResourceEnvironmentWithPassword_Credentials{
			Database: resource.Database,
			Port:     resource.Port,
			Host:     resource.Host,
			Username: resource.Username,
			Password: postgresSqlSecret.Password,
		}
	}

	return &response, nil
}

func (s *ResourceService) CreateVariableResource(ctx context.Context, in *pb.CreateVariableResourceRequest) (*pb.VariableResource, error) {
	s.logger.Info("--CreateVariableresource-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.CreateVariableResource", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().CreateVariableResource(ctx, in)
	if err != nil {
		s.logger.Error("--CreateVariableresource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) GetVariableResourceList(ctx context.Context, in *pb.GetVariableResourceListRequest) (*pb.GetVariableResourceListResponse, error) {
	s.logger.Info("--GetVariableResourceList-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetVariableResourceList", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().GetVariableResourceList(ctx, in)
	if err != nil {
		s.logger.Error("--GetVariableResourceList--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) GetSingleVariableResource(ctx context.Context, in *pb.PrimaryKeyVariableResource) (*pb.VariableResource, error) {
	s.logger.Info("--GetSingleVariableResource-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetSingleVariableResource", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().GetSingleVariableResource(ctx, in)
	if err != nil {
		s.logger.Error("--GetSingleVariableResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) UpdateVariableResource(ctx context.Context, in *pb.UpdateVariableResourceRequest) (*pb.Empty, error) {
	s.logger.Info("--UpdateVariableResource-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.UpdateVariableResource", in)
	defer dbSpan.Finish()

	for _, v := range in.Variables {
		if v.Id == "" {
			_, err := s.storage.Resource().CreateVariableResource(
				ctx,
				&pb.CreateVariableResourceRequest{
					ProjectId:         in.ProjectId,
					EnvironmentId:     in.EnvironmentId,
					Key:               v.Key,
					Value:             v.Value,
					ProjectResourceId: in.ProjectResourceId,
				},
			)
			if err != nil {
				s.logger.Error("--UpsertVariableresource--", l.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			_, err := s.storage.Resource().UpdateVariableResource(
				ctx,
				&pb.VariableResource{
					ProjectId:         in.ProjectId,
					EnvironmentId:     in.EnvironmentId,
					Key:               v.Key,
					Value:             v.Value,
					ProjectResourceId: in.ProjectResourceId,
					Id:                v.Id,
				},
			)
			if err != nil {
				s.logger.Error("--UpdateVariableresource--", l.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}

	return &pb.Empty{}, nil
}

func (s *ResourceService) DeleteVariableResource(ctx context.Context, in *pb.PrimaryKeyVariableResource) (*pb.Empty, error) {
	s.logger.Info("--DeleteVariableResource-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.DeleteVariableResource", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().DeleteVariableResource(ctx, in)
	if err != nil {
		s.logger.Error("--DeleteVariableResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) AddResourceToProject(ctx context.Context, in *pb.AddResourceToProjectRequest) (*pb.ProjectResource, error) {
	safeReq := projectResourceAddRequestForLog(in)
	s.logger.Info("--AddResourceToProject1-- requested", l.Any("req: ", safeReq))

	dbSpan, _ := span.StartSpanFromContext(ctx, "grpc_resource.AddResourceToProject", safeReq)
	defer dbSpan.Finish()

	switch in.GetType() {
	case pb.ResourceType_TRANSCODER:
		project, err := s.storage.Project().GetById(ctx, in.GetProjectId())
		if err != nil {
			s.logger.Error("--AddResourceToProject--cannot get project", l.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		projectName := strings.Split(project.GetTitle(), " ")

		resEnv, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{
			ProjectId:     in.GetProjectId(),
			EnvironmentId: in.GetEnvironmentId(),
		})
		if err != nil {
			s.logger.Error("--AddResourceToProject--cannot get resource environment", l.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		_, err = s.serviceManager.TranscoderService().CreateCompanyAndProject(ctx, &tb.CreateCompanyAndProjectRequest{
			ProjectId: resEnv.Id,
			Title:     projectName[0],
		})
		if err != nil {
			s.logger.Error("--AddResourceToProject--CreateCompanyAndProject", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case pb.ResourceType_METABASE:
		project, err := s.storage.Project().GetById(ctx, in.GetProjectId())
		if err != nil {
			errGetProjectInfo := errors.New("invalid or missing project_id")
			s.logger.Error("--CreateResourceProjectGetById--", l.Error(err))
			return nil, status.Error(codes.InvalidArgument, errGetProjectInfo.Error())
		}

		environment, err := s.storage.Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{Id: in.GetEnvironmentId()})
		if err != nil {
			s.logger.Error("--CreateResourceEnvironmentGetById--", l.Error(err))
			return nil, err
		}

		var (
			host, database, username, password string
			projectNameSlc                     = strings.Split(project.GetTitle(), " ")
			envNameSlc                         = strings.Split(environment.GetName(), " ")

			projectName = strings.ToLower(projectNameSlc[0][:helper.Min(len(projectNameSlc[0]), 10)])
			envName     = strings.ToLower(envNameSlc[0])[:helper.Min(len(envNameSlc), 4)]
			projectId   = strings.ReplaceAll(project.GetProjectId(), "-", "")
		)

		randomPassword, _ := helper.GenerateRandomPassword(14)
		randomWord, _ := helper.GenerateRandomWord(3)

		resEnv, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{
			ProjectId:     in.GetProjectId(),
			EnvironmentId: in.GetEnvironmentId(),
			ResourceType:  int32(*pb.ResourceType_CLICKHOUSE.Enum()),
			IsConfigured:  true,
		})

		if err == nil {
			secret, err := s.vault.Get(ctx, resEnv.GetVaultPath())
			if err != nil {
				s.logger.Error("--AddResourceToProject--Get vault", l.Error(err))
				return nil, err
			}

			host = resEnv.GetHost()
			database = resEnv.GetDatabase()
			username = resEnv.GetUsername()
			password = cast.ToString(secret["password"])
		} else {
			var (
				databaseName = fmt.Sprintf("%s_%s_%s_%s",
					projectName,
					projectId,
					envName,
					"obj_build_svcs",
				)
				databaseNameForClickHouse = fmt.Sprintf("%s_%s_%s_%s",
					projectName,
					projectId,
					envName,
					"clickhouse_svcs",
				)

				userName                  = databaseName
				pass                      = helper.GeneratePassword(10, true, true, true, false)
				serviceType, resourceType int32
				errAddResource            error
				isDefault                 = true
			)
			resourceType = int32(*pb.ResourceType_CLICKHOUSE.Enum())
			serviceType = int32(*pb.ServiceType_BUILDER_SERVICE.Enum())

			var credential = pb.Resource_Credentials{
				Host:     s.cfg.UcodeClickhouseHost,
				Port:     s.cfg.UcodeClickhousePort,
				Username: databaseNameForClickHouse,
				Password: pass,
				Database: databaseNameForClickHouse,
			}

			chClientConn, errConnect := ch.NewClickHouse(s.cfg)
			if errConnect != nil {
				errConnClickHouse := errors.New("error unable to connect clickhouse db")
				s.logger.Error("--CreateResourceClickHouseConnection--", l.Error(err))
				return nil, status.Error(codes.Internal, errConnClickHouse.Error())
			}

			errorClickHouseUserCreate := chClientConn.CreateUserAndDatabase(ctx, ch.CreateUser{
				UserName: databaseNameForClickHouse,
				Pass:     pass,
				DbName:   databaseNameForClickHouse,
			})
			if errorClickHouseUserCreate != nil {
				errCreateUser := errors.New("Error while creating clickhouse user ->" + errorClickHouseUserCreate.Error())
				s.logger.Error("--CreateResourceCreateUserAndDatabase--", l.Error(errorClickHouseUserCreate))
				return nil, status.Error(codes.Internal, errCreateUser.Error())
			}

			defer chClientConn.CloseDb()
			defer func(data ch.DeleteUser) {
				if errAddResource != nil {
					errDelete := chClientConn.DeleteUserAndDatabase(context.Background(), ch.DeleteUser{
						UserName: databaseNameForClickHouse,
						DbName:   databaseNameForClickHouse,
					})
					if err != nil {
						s.logger.Error("--CreateResourceDeleteUserAndDatabase--", l.Error(errDelete))
					}
				}
			}(ch.DeleteUser{
				DbName:   databaseName,
				UserName: userName,
			})

			isDefault = false

			_, errAddResource = s.AddResource(ctx, &pb.AddResourceRequest{
				CompanyId:     in.GetCompanyId(),
				ProjectId:     in.GetProjectId(),
				UserId:        in.GetUserId(),
				EnvironmentId: environment.GetId(),
				ResourceType:  pb.ResourceType(resourceType),
				ServiceType:   pb.ServiceType(serviceType),
				Title:         in.GetName(),
				IsConfigured:  true,
				IsDefault:     isDefault,
				Credentials:   &credential,
				NodeType:      in.GetNodeType(),
				ClientTypeId:  in.ClientTypeId,
				RoleId:        in.ClientTypeId,
			})

			if errAddResource != nil {
				s.logger.Error("--CreateResourceAddResource--", l.Error(err))
				return nil, err
			}

			host = s.cfg.UcodeClickhouseHost
			database = databaseNameForClickHouse
			username = databaseNameForClickHouse
			password = pass
		}

		var metabaseDbName = fmt.Sprintf("%s-%s", helper.CapitalizeFirst(projectName), envName)

		err = metabase.CreateConnection(metabase.CreateConnectionRequest{
			DatabaseName:     metabaseDbName,
			Host:             host,
			Port:             s.cfg.UcodeClickhouseHTTPPort,
			Database:         database,
			Username:         username,
			Password:         password,
			MetabaseName:     fmt.Sprintf("%v_%v@gmail.com", metabaseDbName, randomWord),
			MetabasePassword: randomPassword,
			Cfg:              s.cfg,
		})
		if err != nil {
			s.logger.Error("--AddResourceToProject--Create metabase", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		in.Settings = &pb.Settings{Metabase: &pb.MetabaseCredentials{}}
		in.Settings.Metabase.Username = fmt.Sprintf("%v_%v@gmail.com", metabaseDbName, randomWord)
		in.Settings.Metabase.Password = randomPassword
		in.Settings.Metabase.Url = s.cfg.MetabaseBaseUrl
	case pb.ResourceType_SUPERSET:
		project, err := s.storage.Project().GetById(ctx, in.GetProjectId())
		if err != nil {
			errGetProjectInfo := errors.New("invalid or missing project_id")
			s.logger.Error("--CreateResourceProjectGetById--", l.Error(err))
			return nil, status.Error(codes.InvalidArgument, errGetProjectInfo.Error())
		}

		var host, database, username, password string
		projectName := strings.Split(project.GetTitle(), " ")

		randomPassword, _ := helper.GenerateRandomPassword(9)
		supersetName, _ := helper.GenerateRandomWord(4)

		resEnv, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{
			ProjectId:     in.GetProjectId(),
			EnvironmentId: in.GetEnvironmentId(),
			ResourceType:  int32(*pb.ResourceType_CLICKHOUSE.Enum()),
			IsConfigured:  true,
		})
		if err == nil {
			secret, err := s.vault.Get(ctx, resEnv.GetVaultPath())
			if err != nil {
				s.logger.Error("--AddResourceToProject--Get vault", l.Error(err))
				return nil, err
			}

			host = resEnv.GetHost()
			database = resEnv.GetDatabase()
			username = resEnv.GetUsername()
			password = cast.ToString(secret["password"])
		} else {
			environment, err := s.storage.Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{Id: in.GetEnvironmentId()})
			if err != nil {
				s.logger.Error("--CreateResourceEnvironmentGetById--", l.Error(err))
				return nil, err
			}

			var (
				projectName = strings.Split(project.GetTitle(), " ")
				envName     = strings.Split(environment.GetName(), " ")

				databaseName = fmt.Sprintf("%s_%s_%s_%s",
					strings.ToLower(projectName[0][:helper.Min(len(projectName[0]), 10)]),
					strings.ReplaceAll(project.GetProjectId(), "-", ""),
					strings.ToLower(envName[0])[:helper.Min(len(envName), 4)],
					"obj_build_svcs",
				)
				databaseNameForClickHouse = fmt.Sprintf("%s_%s_%s_%s",
					strings.ToLower(projectName[0][:helper.Min(len(projectName[0]), 10)]),
					strings.ReplaceAll(project.GetProjectId(), "-", ""),
					strings.ToLower(envName[0])[:helper.Min(len(envName), 4)],
					"clickhouse_svcs",
				)

				userName                  = databaseName
				pass                      = helper.GeneratePassword(10, true, true, true, false)
				serviceType, resourceType int32
				errAddResource            error
				isDefault                 = true
			)
			resourceType = int32(*pb.ResourceType_CLICKHOUSE.Enum())
			serviceType = int32(*pb.ServiceType_BUILDER_SERVICE.Enum())

			var credential = pb.Resource_Credentials{
				Host:     s.cfg.UcodeClickhouseHost,
				Port:     s.cfg.UcodeClickhousePort,
				Username: databaseNameForClickHouse,
				Password: pass,
				Database: databaseNameForClickHouse,
			}

			chClientConn, errConnect := ch.NewClickHouse(s.cfg)
			if errConnect != nil {
				errConnClickHouse := errors.New("error unable to connect clickhouse db")
				s.logger.Error("--CreateResourceClickHouseConnection--", l.Error(err))
				return nil, status.Error(codes.Internal, errConnClickHouse.Error())
			}

			errorClickHouseUserCreate := chClientConn.CreateUserAndDatabase(ctx, ch.CreateUser{
				UserName: databaseNameForClickHouse,
				Pass:     pass,
				DbName:   databaseNameForClickHouse,
			})
			if errorClickHouseUserCreate != nil {
				errCreateUser := errors.New("Error while creating clickhouse user ->" + errorClickHouseUserCreate.Error())
				s.logger.Error("--CreateResourceCreateUserAndDatabase--", l.Error(errorClickHouseUserCreate))
				return nil, status.Error(codes.Internal, errCreateUser.Error())
			}

			defer chClientConn.CloseDb()
			defer func(data ch.DeleteUser) {
				if errAddResource != nil {
					errDelete := chClientConn.DeleteUserAndDatabase(context.Background(), ch.DeleteUser{
						UserName: databaseNameForClickHouse,
						DbName:   databaseNameForClickHouse,
					})
					if err != nil {
						s.logger.Error("--CreateResourceDeleteUserAndDatabase--", l.Error(errDelete))
					}
				}
			}(ch.DeleteUser{
				DbName:   databaseName,
				UserName: userName,
			})

			isDefault = false

			_, errAddResource = s.AddResource(ctx, &pb.AddResourceRequest{
				CompanyId:     in.GetCompanyId(),
				ProjectId:     in.GetProjectId(),
				UserId:        in.GetUserId(),
				EnvironmentId: environment.GetId(),
				ResourceType:  pb.ResourceType(resourceType),
				ServiceType:   pb.ServiceType(serviceType),
				Title:         in.GetName(),
				IsConfigured:  true,
				IsDefault:     isDefault,
				Credentials:   &credential,
				NodeType:      in.GetNodeType(),
				ClientTypeId:  in.ClientTypeId,
				RoleId:        in.ClientTypeId,
			})

			if errAddResource != nil {
				s.logger.Error("--CreateResourceAddResource--", l.Error(err))
				return nil, err
			}

			host = s.cfg.UcodeClickhouseHost
			database = databaseNameForClickHouse
			username = databaseNameForClickHouse
			password = pass
		}

		err = airbyte.CreateSupersetConnection(airbyte.CreateSupersetConnectionRequest{
			DatabaseName:     projectName[0],
			Host:             host,
			Port:             s.cfg.UcodeClickhouseHTTPPort,
			Database:         database,
			Username:         username,
			Password:         password,
			SupersetPassword: randomPassword,
			SupersetName:     fmt.Sprintf("%v_%v", projectName[0], supersetName),
			Cfg:              s.cfg,
		})
		if err != nil {
			s.logger.Error("--AddResourceToProject--Create superset", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		in.Settings = &pb.Settings{Superset: &pb.SupersetCredentials{}}
		in.Settings.Superset.Username = fmt.Sprintf("%v_%v", projectName[0], supersetName)
		in.Settings.Superset.Password = randomPassword
		in.Settings.Superset.Url = s.cfg.SupersetBaseUrl
	case pb.ResourceType_POSTGRESQL:
		resEnv, err := s.storage.Resource().GetSingleResourceEnvironment(ctx, &pb.ResourceEnvironment{
			ProjectId:     in.GetProjectId(),
			EnvironmentId: in.GetEnvironmentId(),
			ResourceType:  int32(*pb.ResourceType_POSTGRESQL.Enum()),
			Default:       true,
		})
		if err != nil {
			s.logger.Error("--AddResourceToProject--Get resource environment", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		service, err := s.services.GetByNodeType(in.GetProjectId(), in.GetNodeType())
		if err != nil {
			s.logger.Error("--Get from serviceNode --", l.Error(err))
			return nil, err
		}

		sslmode := "disable"
		if in.Settings.Postgres.SslMode {
			sslmode = "allow"
		}

		connectionString := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			in.Settings.Postgres.Username,
			in.Settings.Postgres.Password,
			in.Settings.Postgres.Host,
			in.Settings.Postgres.Port,
			in.Settings.Postgres.Database,
			sslmode,
		)

		_, err = service.GoTableService().CreateConnectionAndSchema(
			ctx, &nobs.CreateConnectionAndSchemaReq{
				ProjectId:        resEnv.Id,
				ConnectionString: connectionString,
				Name:             in.Settings.Postgres.ConnectionName,
			},
		)
		if err != nil {
			s.logger.Error("--AddResourceToProject-- CreateConnectionAndSchema", l.Error(err))
			return nil, err
		}
	}

	resp, err := s.storage.Resource().AddResourceToProject(ctx, in)
	if err != nil {
		s.logger.Error("--AddResourceToProject--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	if resp.Id != "" && len(in.Variables) > 0 {
		for _, v := range in.Variables {
			_, err := s.storage.Resource().CreateVariableResource(ctx,
				&pb.CreateVariableResourceRequest{
					ProjectId:         in.ProjectId,
					EnvironmentId:     in.EnvironmentId,
					Key:               v.Key,
					Value:             v.Value,
					ProjectResourceId: resp.Id,
				},
			)
			if err != nil {
				s.logger.Error("--Create many variables--", l.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}

	return resp, nil
}

func (s *ResourceService) GetProjectResourceList(ctx context.Context, in *pb.GetProjectResourceListRequest) (*pb.ListProjectResource, error) {
	s.logger.Info("--GetProjectResourceList-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetProjectResourceList", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().GetProjectResourceList(ctx, in)
	if err != nil {
		s.logger.Error("--GetProjectResourceList--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) GetSingleProjectResouece(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.ProjectResource, error) {
	s.logger.Info("--GetSingleProjectResouece-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetSingleProjectResouece", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().GetSingleProjectResouece(ctx, in)
	if err != nil {
		s.logger.Error("--GetSingleProjectResouece--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) UpsertProjectResource(ctx context.Context, in *pb.AddResourceToProjectRequest) (*pb.ProjectResource, error) {
	s.logger.Info("--UpsertProjectResource-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.UpsertProjectResource", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().UpsertProjectResource(ctx, in)
	if err != nil {
		s.logger.Error("--UpsertProjectResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) UpdateProjectResource(ctx context.Context, in *pb.ProjectResource) (*pb.Empty, error) {
	safeReq := projectResourceForLog(in)
	s.logger.Info("--UpdateProjectResource-- requested", l.Any("req: ", safeReq))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.UpdateProjectResource", safeReq)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().UpdateProjectResource(ctx, in)
	if err != nil {
		s.logger.Error("--UpdateProjectResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

// The two helpers below clone through proto.Clone rather than copying the
// struct: a generated message carries a mutex, and copying it by value is what
// `go vet` flags.
func projectResourceAddRequestForLog(in *pb.AddResourceToProjectRequest) *pb.AddResourceToProjectRequest {
	if in == nil {
		return nil
	}
	safe, _ := proto.Clone(in).(*pb.AddResourceToProjectRequest)
	if safe == nil {
		return nil
	}
	safe.Secret = nil
	return safe
}

func projectResourceForLog(in *pb.ProjectResource) *pb.ProjectResource {
	if in == nil {
		return nil
	}
	safe, _ := proto.Clone(in).(*pb.ProjectResource)
	if safe == nil {
		return nil
	}
	safe.Secret = nil
	return safe
}

func (s *ResourceService) DeleteProjectResource(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.Empty, error) {
	s.logger.Info("--DeleteProjectResource-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.DeleteProjectResource", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().DeleteProjectResource(ctx, in)
	if err != nil {
		s.logger.Error("--DeleteProjectResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ResourceService) GetProjectResourcesByExternalId(ctx context.Context, in *pb.GetByExternalIdRequest) (*pb.ListProjectResource, error) {
	s.logger.Info("--GetProjectResourcesByExternalId-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_resource.GetProjectResourcesByExternalId", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Resource().GetProjectResourcesByExternalId(ctx, in)
	if err != nil {
		s.logger.Error("--GetProjectResourcesByExternalId--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}
