package helper

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
)

func GetRecordPermission(ctx context.Context, req models.GetRecordPermissionRequest) (*models.GetRecordPermissionResponse, error) {
	var recordPermission = models.GetRecordPermissionResponse{}

	if len(req.RoleId) == 0 {
		return &recordPermission, nil
	}

	query := `
		SELECT
			"guid",
			"role_id",
			"table_slug",
			"read",
			"write",
			"update",
			"delete",
			"is_public",
			"is_have_condition"
		FROM "record_permission"
		WHERE table_slug = $1 AND role_id = $2
	`

	err := req.Conn.QueryRow(ctx, query, req.TableSlug, req.RoleId).Scan(
		&recordPermission.Guid,
		&recordPermission.RoleId,
		&recordPermission.TableSlug,
		&recordPermission.Read,
		&recordPermission.Write,
		&recordPermission.Update,
		&recordPermission.Delete,
		&recordPermission.IsPublic,
		&recordPermission.IsHaveCondition,
	)
	if err != nil {
		if err.Error() != config.ErrNoRows {
			return &models.GetRecordPermissionResponse{}, err
		}
	}

	return &recordPermission, nil
}
