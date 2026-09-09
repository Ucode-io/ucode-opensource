package storage

import (
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"
)

// StorageI ...
type StorageI interface {
	CloseDB()
	Company() repo.CompanyStorageI
	Project() repo.ProjectStorageI
	Environment() repo.EnvironmentStorageI
	Resource() repo.ResourceStorageI
	ServiceResource() repo.ServiceResourceStorageI
	Redirect() repo.RedirectStorageI
	Airbyte() repo.AirbyteStorageI
	Billing() repo.BillingStorageI
	TemplateMetadata() repo.TemplateStorageI
	IntegrationResource() repo.IntegrationResourceStorageI
	UgenTemplate() repo.UgenTemplateStorageI
	MfeShortLink() repo.MfeShortLinkStorageI
}
