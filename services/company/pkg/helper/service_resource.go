package helper

import "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

func MakeBodyServiceResource(resourceId string) []*company_service.ServiceResourceModel {
	resp := []*company_service.ServiceResourceModel{
		{
			Title:        "ANALYTICS_SERVICE",
			ServiceType:  2,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "API_REF_SERVICE",
			ServiceType:  7,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "BUILDER_SERVICE",
			ServiceType:  1,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "FUNCTION_SERVICE",
			ServiceType:  5,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "POSTGRES_BUILDER",
			ServiceType:  8,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "QUERY_SERVICE",
			ServiceType:  4,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "TEMPLATE_SERVICE",
			ServiceType:  3,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
		{
			Title:        "WEB_PAGE_SERVICE",
			ServiceType:  6,
			ResourceId:   resourceId,
			ResourceType: 1,
		},
	}

	return resp
}
