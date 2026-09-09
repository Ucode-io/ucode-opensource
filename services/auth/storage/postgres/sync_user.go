package postgres

import (
	"context"
	"strings"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
)

func (r *userRepo) UpdateSyncUser(ctx context.Context, req *pb.UpdateSyncUserRequest, loginType string) (*pb.SyncUserResponse, error) {
	var (
		loginValue, userId string
		resp               = &pb.SyncUserResponse{}
		resetPasswordReq   *pb.ResetPasswordRequest
	)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `SELECT
		id
	FROM
		"user"
	WHERE`

	switch loginType {
	case "email":
		query = query + ` LOWER(email) = $1`
		loginValue = strings.ToLower(req.GetEmail())
		resetPasswordReq = &pb.ResetPasswordRequest{
			Email:    req.GetEmail(),
			UserId:   req.GetGuid(),
			Password: req.GetPassword(),
		}
	case "phone":
		query = query + ` phone = $1`
		loginValue = strings.ToLower(req.GetPhone())
		resetPasswordReq = &pb.ResetPasswordRequest{
			Phone:    req.GetPhone(),
			UserId:   req.GetGuid(),
			Password: req.GetPassword(),
		}
	case "login":
		query = query + ` LOWER(login) = $1`
		loginValue = strings.ToLower(req.GetLogin())
		resetPasswordReq = &pb.ResetPasswordRequest{
			Login:    req.GetLogin(),
			UserId:   req.GetGuid(),
			Password: req.GetPassword(),
		}
	case "tin":
		query = query + ` tin = $1`
		loginValue = req.GetTin()
		resetPasswordReq = &pb.ResetPasswordRequest{
			Tin:    req.GetTin(),
			UserId: req.GetGuid(),
		}
	}

	err = tx.QueryRow(ctx, query, loginValue).Scan(&userId)
	if err == pgx.ErrNoRows {
		pKey, err := r.UpdateLoginStrategy(ctx, req, resetPasswordReq, tx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to reset password")
		}

		resp.UserId = pKey
	} else if err == nil {
		if loginType == "login" {
			return nil, errors.New("login already exists")
		}
		// The contact (phone/email/tin) being added already belongs to another
		// auth user (userId). Instead of the old silent redirect — which pointed
		// this project at a foreign identity and left two split records — fold the
		// current account (req.Guid) into that existing user, which survives and
		// absorbs the current account's login fields. Ownership of the contact is
		// verified upstream by the caller (the OTP-verified add-contact flow).
		if _, err = uuid.Parse(userId); err == nil {
			coords := mergeCoords{
				projectID:    req.GetProjectId(),
				companyID:    req.GetCompanyId(),
				clientTypeID: req.GetClientTypeId(),
				roleID:       req.GetRoleId(),
				envID:        req.GetEnvId(),
			}
			if err = r.mergeContactTx(ctx, tx, req.GetGuid(), userId, coords); err != nil {
				return nil, errors.Wrap(err, "failed to merge contact")
			}

			resp.UserId = userId
		}
	} else {
		return nil, errors.Wrap(err, "failed to get user")
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return resp, nil
}

func (r *userRepo) UpdateLoginStrategy(ctx context.Context, req *pb.UpdateSyncUserRequest, user *pb.ResetPasswordRequest, tx pgx.Tx) (string, error) {
	var (
		count  int
		userId string
	)

	query := `
		SELECT
			COUNT(*)
		FROM
			"user_project"
		WHERE
			user_id = $1`

	err := tx.QueryRow(ctx, query, req.GetGuid()).Scan(&count)
	if err != nil {
		return "", errors.Wrap(err, "failed to get user_project count")
	}

	switch {
	case count == 0:
		var (
			clientTypeId, roleId, envId pgtype.UUID
		)

		pKey, err := r.CreateWithTx(ctx, &pb.CreateUserRequest{
			Login:     req.GetLogin(),
			Password:  req.GetPassword(),
			Email:     req.GetEmail(),
			Phone:     req.GetPhone(),
			CompanyId: req.GetCompanyId(),
			Tin:       req.GetTin(),
		}, tx)
		if err != nil {
			return "", errors.Wrap(err, "failed to create user")
		}

		userId = pKey.GetId()

		if req.GetClientTypeId() != "" {
			err := clientTypeId.Set(req.GetClientTypeId())
			if err != nil {
				return "", errors.Wrap(err, "failed to set client type id")
			}
		} else {
			clientTypeId.Status = pgtype.Null
		}
		if req.GetRoleId() != "" {
			err := roleId.Set(req.GetRoleId())
			if err != nil {
				return "", errors.Wrap(err, "failed to set role id")
			}
		} else {
			roleId.Status = pgtype.Null
		}
		if req.GetEnvId() != "" {
			err := envId.Set(req.GetEnvId())
			if err != nil {
				return "", errors.Wrap(err, "failed to set env id")
			}
		} else {
			envId.Status = pgtype.Null
		}

		query = `INSERT INTO
				user_project(user_id, company_id, project_id, client_type_id, role_id, env_id)
				VALUES ($1, $2, $3, $4, $5, $6)`

		_, err = tx.Exec(ctx,
			query,
			userId,
			req.GetCompanyId(),
			req.GetProjectId(),
			clientTypeId,
			roleId,
			envId,
		)
		if err != nil {
			return "", errors.Wrap(err, "failed to insert user to project")
		}
	case count >= 1:
		// Add/change the login strategy (phone/email/login/tin) on the EXISTING
		// auth user in place, keeping a single identity — even when the user
		// belongs to more than one project.
		//
		// Previously count > 1 forked a brand-new auth user for this project and
		// moved the membership onto it. Because email/login/phone are globally
		// unique, that fork could only carry the value being changed and NOT the
		// user's existing email — so adding a phone split the account (old user
		// kept the email, the fork got the phone, the project's user_id_auth
		// repointed to the fork). update_phone/update_email then produced a
		// login-broken, two-record identity. Updating in place avoids that split.
		//
		// Trade-off (accepted): for an identity shared across projects/companies
		// the change now applies to the shared auth row rather than being
		// isolated per project.
		_, err = r.ResetPassword(ctx, user, tx)
		if err != nil {
			return "", errors.Wrap(err, "failed to reset password")
		}
		userId = req.GetGuid()
	}

	return userId, nil
}

func (r *userRepo) CreateWithTx(ctx context.Context, entity *pb.CreateUserRequest, tx pgx.Tx) (pKey *pb.UserPrimaryKey, err error) {
	query := `INSERT INTO "user" (
		id,
		phone,
		email,
		login,
		password,
		company_id,
		hash_type
	) VALUES (
		$1,
		$2,
		$3,
		$4,
		$5,
		$6,
		'bcrypt'
	)`

	id, err := uuid.NewRandom()
	if err != nil {
		return pKey, errors.Wrap(err, "failed to generate uuid")
	}

	_, err = tx.Exec(ctx, query,
		id.String(),
		entity.GetPhone(),
		entity.GetEmail(),
		entity.GetLogin(),
		entity.GetPassword(),
		entity.GetCompanyId(),
	)
	if err != nil {
		return pKey, errors.Wrap(err, "failed to create user")
	}

	pKey = &pb.UserPrimaryKey{
		Id: id.String(),
	}

	return pKey, nil
}

// MergeContact consolidates the current auth user into the found auth user
// (the one that already owns a just-added phone/email). The found user survives.
//
// Steps, in one transaction:
//  1. Move the current user's membership in THIS project onto the survivor.
//  2. Guard: the current user must have no memberships left anywhere else, or we
//     cannot safely delete it (see ErrMergeAccountInUse).
//  3. Absorb the current user's login fields (email/login/password) into the
//     survivor, then delete the current user. The current row is deleted before
//     the survivor is updated so a unique email/login is freed first.
//
// Ownership of the contact is the CALLER's responsibility (e.g. an OTP-verified
// add-contact flow); this method trusts current/found as given. The returned
// SyncUserResponse.UserId is the survivor, which the caller (object-builder)
// writes back into this project's users.user_id_auth.
func (r *userRepo) MergeContact(ctx context.Context, req *pb.MergeContactRequest) (*pb.SyncUserResponse, error) {
	current, found := req.GetCurrentUserId(), req.GetFoundUserId()
	if current == "" || found == "" {
		return nil, errors.New("current_user_id and found_user_id are required")
	}
	// Nothing to merge: the project already points at the survivor.
	if current == found {
		return &pb.SyncUserResponse{UserId: found}, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	coords := mergeCoords{
		projectID:    req.GetProjectId(),
		companyID:    req.GetCompanyId(),
		clientTypeID: req.GetClientTypeId(),
		roleID:       req.GetRoleId(),
		envID:        req.GetEnvId(),
	}
	if err = r.mergeContactTx(ctx, tx, current, found, coords); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return &pb.SyncUserResponse{UserId: found}, nil
}

// mergeCoords locates the current membership row being moved onto the survivor.
type mergeCoords struct {
	projectID    string
	companyID    string
	clientTypeID string
	roleID       string
	envID        string
}

// mergeContactTx folds `current` into `found` (survivor) inside an existing tx.
// It is shared by MergeContact (dedicated RPC) and UpdateSyncUser (the phone/
// email/tin collision path that a normal contact update reaches).
//
//  1. Move current's membership in THIS project onto the survivor.
//  2. Guard: current must have no memberships left anywhere else — otherwise
//     deleting it would dangle its user_id_auth in other projects' builder DBs
//     (which auth cannot reach). Returns storage.ErrMergeAccountInUse instead.
//  3. Absorb current's login fields (email/login/password) into the survivor and
//     delete current. Current is deleted before the survivor is updated so a
//     unique email/login is freed first.
//
// The caller repoints this project's users.user_id_auth to `found`.
func (r *userRepo) mergeContactTx(ctx context.Context, tx pgx.Tx, current, found string, c mergeCoords) error {
	if current == "" || found == "" || current == found {
		return nil
	}

	// 1. Move this project's membership from the current user to the survivor.
	if _, err := tx.Exec(ctx, `
		DELETE FROM user_project
		WHERE user_id = $1
		  AND project_id = $2
		  AND client_type_id = $3
		  AND role_id = $4
		  AND env_id = $5
		  AND company_id = $6`,
		current, c.projectID, c.clientTypeID, c.roleID, c.envID, c.companyID,
	); err != nil {
		return errors.Wrap(err, "failed to drop current membership")
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_project (user_id, company_id, project_id, client_type_id, role_id, env_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'ACTIVE')
		ON CONFLICT DO NOTHING`,
		found, c.companyID, c.projectID, c.clientTypeID, c.roleID, c.envID,
	); err != nil {
		return errors.Wrap(err, "failed to ensure survivor membership")
	}

	// 2. The current user must be referenced nowhere else before we delete it.
	var remaining int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM user_project WHERE user_id = $1`, current,
	).Scan(&remaining); err != nil {
		return errors.Wrap(err, "failed to count current memberships")
	}
	if remaining > 0 {
		return storage.ErrMergeAccountInUse
	}

	// 3. Absorb the current user's login fields into the survivor, then delete it.
	var email, login, password, hashType string
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(email, ''), COALESCE(login, ''), COALESCE(password, ''), COALESCE(hash_type::text, '')
		FROM "user" WHERE id = $1`, current,
	).Scan(&email, &login, &password, &hashType); err != nil {
		if err == pgx.ErrNoRows {
			return nil // current auth user already gone; survivor keeps the membership
		}
		return errors.Wrap(err, "failed to load current user")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, current); err != nil {
		return errors.Wrap(err, "failed to delete current user")
	}

	if _, err := tx.Exec(ctx, `
		UPDATE "user" SET
			email     = COALESCE(NULLIF($2, ''), email),
			login     = COALESCE(NULLIF($3, ''), login),
			password  = COALESCE(NULLIF($4, ''), password),
			hash_type = COALESCE(NULLIF($5, '')::hash_type, hash_type),
			updated_at = now()
		WHERE id = $1`,
		found, email, login, password, hashType,
	); err != nil {
		return errors.Wrap(err, "failed to absorb fields into survivor")
	}

	return nil
}
