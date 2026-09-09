package person

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"fmt"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	pa "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/util"
)

func CreateSyncWithLoginTable(ctx context.Context, req models.CreateSyncWithLoginTableRequest) error {
	var (
		authInfo                     = cast.ToStringMap(req.TableAttributesMap["auth_info"])
		clientTypeID, clientTypeIDOk = authInfo[config.CLIENT_TYPE_ID].(string)
		roleID, roleIDOk             = authInfo[config.ROLE_ID].(string)
		phone, phoneOk               = authInfo["phone"].(string)
		email, emailOk               = authInfo["email"].(string)
		login, loginOk               = authInfo["login"].(string)
		password, passwordOk         = authInfo["password"].(string)

		queryFields = []string{"guid, user_id_auth"}
		queryValues = []string{"$1, $2"}
		args        = []any{req.Guid, req.UserIdAuth}
		argCount    = 3
	)

	if clientTypeIDOk {
		queryFields = append(queryFields, clientTypeID)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		args = append(args, req.Data[config.CLIENT_TYPE_ID])
		argCount++
	}

	if roleIDOk {
		queryFields = append(queryFields, roleID)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		args = append(args, req.Data[config.ROLE_ID])
		argCount++
	}

	if phoneOk {
		queryFields = append(queryFields, phone)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		args = append(args, req.Data["phone_number"])
		argCount++
	}

	if emailOk {
		queryFields = append(queryFields, email)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		args = append(args, req.Data["email"])
		argCount++
	}

	if loginOk {
		queryFields = append(queryFields, login)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		args = append(args, req.Data["login"])
		argCount++
	}

	if passwordOk {
		queryFields = append(queryFields, password)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		args = append(args, req.Data["password"])
		argCount++
	}

	query := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
		req.LoginTableSlug,
		strings.Join(queryFields, ", "),
		strings.Join(queryValues, ", "),
	)

	_, err := req.Tx.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "when insert to login table")
	}

	return nil
}

func UpdateSyncWithLoginTable(ctx context.Context, grpcClient client.ServiceManagerI, req models.UpdateSyncWithLoginTableRequest) error {
	var data = req.Data

	if data[config.CLIENT_TYPE_ID] == nil || data[config.ROLE_ID] == nil {
		return errors.New(config.ErrAuthInfo)
	}

	var (
		loginTableSlug sql.NullString
		clientTypeId   = data[config.CLIENT_TYPE_ID].(string)
	)

	query := `SELECT table_slug FROM "client_type" WHERE guid = $1`

	if err := req.Tx.QueryRow(ctx, query, clientTypeId).Scan(&loginTableSlug); err != nil {
		return errors.Wrap(err, "when select client_type for person")
	}

	var (
		tableAttributes, tableId sql.NullString
		tableAttributesMap       = make(map[string]any, 0)
	)

	if !loginTableSlug.Valid {
		loginTableSlug.String = config.USER
	}

	query = `SELECT id, attributes FROM "table" WHERE slug = $1`

	if err := req.Tx.QueryRow(ctx, query, loginTableSlug.String).Scan(&tableId, &tableAttributes); err != nil {
		return errors.Wrap(err, "when select table for person")
	}

	if err := json.Unmarshal([]byte(tableAttributes.String), &tableAttributesMap); err != nil {
		return errors.Wrap(err, "when unmarshal login table attributes")
	}

	response, err := helper.GetItemWithTx(ctx, req.Tx, loginTableSlug.String, req.Guid, false)
	if err != nil {
		return errors.Wrap(err, "error while getting item")
	}

	updateUserRequest := &pa.UpdateSyncUserRequest{
		Guid:         cast.ToString(response["user_id_auth"]),
		RoleId:       cast.ToString(response[config.ROLE_ID]),
		ClientTypeId: cast.ToString(response[config.CLIENT_TYPE_ID]),
		EnvId:        req.EnvId,
		ProjectId:    req.ProjectId,
	}

	if len(cast.ToString(data["password"])) != config.BcryptHashPasswordLength && len(cast.ToString(data["password"])) != 0 {
		err := util.ValidStrongPassword(cast.ToString(data["password"]))
		if err != nil {
			return errors.Wrap(err, "strong password checker")
		}
		updateUserRequest.Password = cast.ToString(data["password"])
	}

	if cast.ToString(data["email"]) != cast.ToString(response["email"]) {
		updateUserRequest.Email = cast.ToString(data["email"])
	}

	if cast.ToString(data["login"]) != cast.ToString(response["login"]) {
		updateUserRequest.Login = cast.ToString(data["login"])
	}

	if cast.ToString(data["phone_number"]) != cast.ToString(response["phone_number"]) {
		updateUserRequest.Phone = cast.ToString(data["phone_number"])
	}

	var (
		authInfo                     = cast.ToStringMap(tableAttributesMap["auth_info"])
		clientTypeID, clientTypeIDOk = authInfo[config.CLIENT_TYPE_ID].(string)
		roleID, roleIDOk             = authInfo[config.ROLE_ID].(string)
		phone, phoneOk               = authInfo["phone"].(string)
		email, emailOk               = authInfo["email"].(string)
		login, loginOk               = authInfo["login"].(string)
		password, passwordOk         = authInfo["password"].(string)

		queryFields  = []string{"guid, user_id_auth"}
		queryValues  = []string{"$1, $2"}
		updateFields = []string{"user_id_auth = EXCLUDED.user_id_auth"}
		args         = []any{req.Guid, cast.ToString(response["user_id_auth"])}
		argCount     = 3
	)

	if clientTypeIDOk {
		queryFields = append(queryFields, clientTypeID)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		updateFields = append(updateFields, fmt.Sprintf("%s = EXCLUDED.%s", clientTypeID, clientTypeID))
		args = append(args, data[config.CLIENT_TYPE_ID])
		argCount++
	}

	if roleIDOk {
		queryFields = append(queryFields, roleID)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		updateFields = append(updateFields, fmt.Sprintf("%s = EXCLUDED.%s", roleID, roleID))
		args = append(args, data[config.ROLE_ID])
		argCount++
	}

	if phoneOk {
		queryFields = append(queryFields, phone)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		updateFields = append(updateFields, fmt.Sprintf("%s = EXCLUDED.%s", phone, phone))
		args = append(args, data["phone_number"])
		argCount++
	}

	if emailOk {
		queryFields = append(queryFields, email)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		updateFields = append(updateFields, fmt.Sprintf("%s = EXCLUDED.%s", email, email))
		args = append(args, data["email"])
		argCount++
	}

	if loginOk {
		queryFields = append(queryFields, login)
		queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
		updateFields = append(updateFields, fmt.Sprintf("%s = EXCLUDED.%s", login, login))
		args = append(args, data["login"])
		argCount++
	}

	if passwordOk {
		dataPassword := cast.ToString(data["password"])
		if len(dataPassword) != config.BcryptHashPasswordLength {
			hashedPassword, err := security.HashPasswordBcrypt(dataPassword)
			if err != nil {
				return errors.Wrap(err, "error when hash password")
			}

			queryFields = append(queryFields, password)
			queryValues = append(queryValues, fmt.Sprintf("$%d", argCount))
			updateFields = append(updateFields, fmt.Sprintf("%s = EXCLUDED.%s", password, password))
			args = append(args, hashedPassword)
			argCount++
		}
	}

	query = fmt.Sprintf(`
	INSERT INTO "%s" (%s) VALUES (%s)
	ON CONFLICT (guid) DO UPDATE SET %s`,
		loginTableSlug.String,
		strings.Join(queryFields, ", "),
		strings.Join(queryValues, ", "),
		strings.Join(updateFields, ", "),
	)

	_, err = req.Tx.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "when upserting to login table")
	}

	_, err = grpcClient.SyncUserService().UpdateUser(ctx, updateUserRequest)
	if err != nil {
		return errors.Wrap(err, "error while updating user")
	}

	return nil
}

func DeleteSyncWithLoginTable(ctx context.Context, grpcClient client.ServiceManagerI, req models.DeleteSyncWithLoginTableRequest) (err error) {
	if req.Response[config.CLIENT_TYPE_ID] == nil || req.Response[config.ROLE_ID] == nil || req.Response["user_id_auth"] == nil {
		return errors.New(config.ErrAuthInfo)
	}

	var (
		loginTableSlug sql.NullString
		softTable      sql.NullBool
		userIdAuth     = req.Response["user_id_auth"].(string)
		clientTypeId   = req.Response[config.CLIENT_TYPE_ID].(string)
		query          = `SELECT table_slug from client_type where guid = $1`
	)

	if err = req.Tx.QueryRow(ctx, query, clientTypeId).Scan(&loginTableSlug); err != nil {
		return errors.Wrap(err, "when select client_type for person")
	}

	if !loginTableSlug.Valid {
		loginTableSlug.String = "user"
	}

	query = `SELECT soft_delete FROM "table" WHERE slug = $1`

	if err = req.Tx.QueryRow(ctx, query, loginTableSlug.String).Scan(&softTable); err != nil {
		return errors.Wrap(err, "when select table for person")
	}

	if len(userIdAuth) == 0 {
		return errors.New(config.ErrInvalidUserId)
	}

	if softTable.Valid {
		if softTable.Bool {
			query = fmt.Sprintf(`UPDATE "%s" SET deleted_at = CURRENT_TIMESTAMP WHERE guid = $1`, loginTableSlug.String)
		} else if !softTable.Bool {
			query = fmt.Sprintf(`DELETE FROM "%s" WHERE guid = $1`, loginTableSlug.String)
		}

		_, err := req.Tx.Exec(ctx, query, req.Id)
		if err != nil {
			return errors.Wrap(err, "when delete loginTable row")
		}
	}

	_, err = grpcClient.SyncUserService().DeleteUser(ctx, &pa.DeleteSyncUserRequest{
		UserId:       userIdAuth,
		ClientTypeId: cast.ToString(req.Response[config.CLIENT_TYPE_ID]),
	})
	if err != nil {
		return errors.New("when delete user from auth")
	}

	return nil
}

func DeleteManySyncWithLoginTable(ctx context.Context, grpcClient client.ServiceManagerI, req models.DeleteManySyncWithLoginTableRequest) (err error) {
	query := fmt.Sprintf(`
	SELECT 
		t.guid, 
		t.user_id_auth, 
		t.client_type_id, 
		t.role_id, 
		ct.table_slug, 
		ta.soft_delete 
	FROM "%s" t 
	JOIN client_type ct ON t.client_type_id = ct.guid 
	JOIN "table" as ta ON ta.slug = ct.table_slug 
	WHERE t.guid = ANY($1)`, req.Table.Slug,
	)

	rows, err := req.Tx.Query(ctx, query, req.Ids)
	if err != nil {
		return err
	}

	defer rows.Close()

	var (
		tableSlugUsers       = make(map[string][]string, 0)
		tableSlugsSoftDelete = make(map[string]bool, 0)
	)

	for rows.Next() {
		var (
			guid, id, roleId, clientTypeId, loginTableSlug sql.NullString
			softDelete                                     sql.NullBool
		)

		err = rows.Scan(
			&guid,
			&id,
			&clientTypeId,
			&roleId,
			&loginTableSlug,
			&softDelete,
		)
		if err != nil {
			return err
		}

		if !id.Valid || !roleId.Valid || !clientTypeId.Valid {
			return errors.New(config.ErrAuthInfo)
		}

		if !loginTableSlug.Valid {
			loginTableSlug.String = config.USER
		}

		tableSlugsSoftDelete[loginTableSlug.String] = softDelete.Bool
		if userIds, ok := tableSlugUsers[loginTableSlug.String]; ok {
			userIds = append(userIds, guid.String)
			tableSlugUsers[loginTableSlug.String] = userIds
		} else {
			tableSlugUsers[loginTableSlug.String] = []string{guid.String}
		}

		req.Users = append(req.Users, &pa.DeleteManyUserRequest_User{
			UserId:       id.String,
			RoleId:       roleId.String,
			ClientTypeId: clientTypeId.String,
		})
	}

	if req.Table.SoftDelete {
		query = fmt.Sprintf(`UPDATE "%s" SET deleted_at = CURRENT_TIMESTAMP WHERE guid = ANY($1)`, req.Table.Slug)
	} else {
		query = fmt.Sprintf(`DELETE FROM "%s" WHERE guid = ANY($1)`, req.Table.Slug)
	}

	_, err = req.Tx.Exec(ctx, query, req.Ids)
	if err != nil {
		return errors.Wrap(err, "error while executing")
	}

	// I have no idea exec this query without iteration
	for tableSlug, userIds := range tableSlugUsers {
		if tableSlugsSoftDelete[tableSlug] {
			query = fmt.Sprintf(`UPDATE "%s" SET deleted_at = CURRENT_TIMESTAMP WHERE guid = ANY($1)`, tableSlug)

			_, err = req.Tx.Exec(ctx, query, userIds)
			if err != nil {
				return errors.Wrap(err, "when deleting loginTable row")
			}
		} else {
			query = fmt.Sprintf(`DELETE FROM "%s" WHERE guid = ANY($1)`, tableSlug)
			_, err = req.Tx.Exec(ctx, query, userIds)
			if err != nil {
				return errors.Wrap(err, "when deleting loginTable row")
			}
		}
	}

	_, err = grpcClient.SyncUserService().DeleteManyUser(ctx, &pa.DeleteManyUserRequest{
		Users:         req.Users,
		ProjectId:     cast.ToString(req.Data["company_service_project_id"]),
		EnvironmentId: cast.ToString(req.Data["company_service_environment_id"]),
	})
	if err != nil {
		return errors.Wrap(err, "when delete users from auth")
	}

	return nil
}
