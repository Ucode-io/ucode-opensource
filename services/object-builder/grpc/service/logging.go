package service

import (
	"fmt"
	"sort"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"

	"google.golang.org/protobuf/types/known/structpb"
)

const maxLoggedDataKeys = 12

func compactRequest(req any) map[string]any {
	switch r := req.(type) {
	case *nb.CommonMessage:
		return map[string]any{
			"project_id":         r.GetProjectId(),
			"company_project_id": r.GetCompanyProjectId(),
			"env_id":             r.GetEnvId(),
			"table_slug":         r.GetTableSlug(),
			"version_id":         r.GetVersionId(),
			"is_cached":          r.GetIsCached(),
			"data":               compactStruct(r.GetData()),
		}
	case *nb.CommonForDocxMessage:
		return map[string]any{
			"project_id":  r.GetProjectId(),
			"table_slug":  r.GetTableSlug(),
			"table_slugs": r.GetTableSlugs(),
			"data":        compactStruct(r.GetData()),
		}
	case *nb.UserActivityReqeust:
		return map[string]any{
			"resource_env_id": r.GetResourceEnvId(),
			"user_id":         r.GetUserId(),
			"login_table":     r.GetLoginTable(),
		}
	case *nb.ExecuteSQLRequest:
		return map[string]any{
			"resource_env_id": r.GetResourceEnvId(),
			"sql_length":      len(r.GetSql()),
			"params_count":    len(r.GetParams()),
			"in_transaction":  r.GetInTransaction(),
		}
	case *nb.GetResourceUsageRequest:
		return map[string]any{
			"project_id": r.GetProjectId(),
		}
	case *nb.RegisterProjectRequest:
		creds := r.GetCredentials()
		return map[string]any{
			"project_id":     r.GetProjectId(),
			"user_id":        r.GetUserId(),
			"resource_id":    r.GetResourceId(),
			"client_type_id": r.GetClientTypeId(),
			"role_id":        r.GetRoleId(),
			"credentials": map[string]any{
				"host":     creds.GetHost(),
				"port":     creds.GetPort(),
				"username": creds.GetUsername(),
				"database": creds.GetDatabase(),
			},
		}
	default:
		return map[string]any{
			"type": fmt.Sprintf("%T", req),
		}
	}
}

func compactStruct(data *structpb.Struct) map[string]any {
	if data == nil {
		return map[string]any{
			"field_count": 0,
		}
	}

	keys := make([]string, 0, len(data.GetFields()))
	for key := range data.GetFields() {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	truncated := false
	if len(keys) > maxLoggedDataKeys {
		keys = keys[:maxLoggedDataKeys]
		truncated = true
	}

	return map[string]any{
		"field_count": len(data.GetFields()),
		"field_keys":  keys,
		"truncated":   truncated,
	}
}
