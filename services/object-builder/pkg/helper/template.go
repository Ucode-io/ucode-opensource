package helper

import (
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"

	"github.com/google/uuid"
)

func CreateTemplate(roleID string) []models.FieldPermission {
	templates := []models.FieldPermission{
		{
			FieldId:        "834df8ef-edb7-4170-996c-9bd5431d9a62",
			TableSlug:      "template",
			ViewPermission: true,
			EditPermission: true,
			Guid:           uuid.NewString(),
			RoleId:         roleID,
			Label:          "Таблица",
		},
		{
			FieldId:        "5dda58a1-84ac-4c50-8993-02e2cefcb29a",
			TableSlug:      "template",
			ViewPermission: true,
			EditPermission: true,
			Guid:           uuid.NewString(),
			RoleId:         roleID,
			Label:          "Размер",
		},
		{
			FieldId:        "9772b679-33ec-4004-b527-317a1165575e",
			TableSlug:      "template",
			ViewPermission: true,
			EditPermission: true,
			Guid:           uuid.NewString(),
			RoleId:         roleID,
			Label:          "Название",
		},
		{
			FieldId:        "98279b02-10c0-409e-8303-14224fd76ec6",
			TableSlug:      "template",
			ViewPermission: true,
			EditPermission: true,
			Guid:           uuid.NewString(),
			RoleId:         roleID,
			Label:          "HTML",
		},
		{
			FieldId:        "494e1ad3-fce8-4e6c-921f-850d0ec73cc4",
			TableSlug:      "template",
			ViewPermission: true,
			EditPermission: true,
			Guid:           uuid.NewString(),
			RoleId:         roleID,
			Label:          "ID",
		},
		{
			FieldId:        "fd7f0fde-3de7-4073-b64d-bd3076c6e3fb",
			TableSlug:      "template",
			ViewPermission: true,
			EditPermission: true,
			Guid:           uuid.NewString(),
			RoleId:         roleID,
			Label:          "FROM VersionTable2.1 TO template",
		},
	}

	return templates
}
