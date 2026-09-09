package postgres

import (
	"context"
	"errors"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/emptypb"
)

type appleSettingsRepo struct {
	db *Pool
}

func NewAppleSettingsRepo(db *Pool) storage.AppleSettingsI {
	return &appleSettingsRepo{
		db: db,
	}
}

func (e *appleSettingsRepo) Create(ctx context.Context, input *pb.AppleIdSettings) (*pb.AppleIdSettings, error) {

	query := `INSERT INTO "apple_settings" (
		id,
		team_id,
		client_id,
		key_id,
		secret,
		project_id
	) VALUES (
		$1,
		$2,
		$3,
		$4,
		$5,
		$6
	)`

	_, err := e.db.Exec(ctx, query,
		input.Id,
		input.TeamId,
		input.ClientId,
		input.KeyId,
		input.Secret,
		input.ProjectId,
	)
	if err != nil {
		return nil, err
	}

	return input, nil
}

func (e *appleSettingsRepo) GetByPK(ctx context.Context, pKey *pb.AppleIdSettingsPrimaryKey) (res *pb.AppleIdSettings, err error) {

	res = &pb.AppleIdSettings{}
	query := `SELECT
	                id,
					team_id,
					client_id,
					key_id,
					secret
	FROM
		"apple_settings"
	WHERE
		id = $1 `

	err = e.db.QueryRow(ctx, query, pKey.Id).Scan(
		&res.Id,
		&res.TeamId,
		&res.ClientId,
		&res.KeyId,
		&res.Secret,
	)
	if err == pgx.ErrNoRows {
		err := errors.New("apple settings not found")
		return nil, err
	} else if err != nil {
		return res, err
	}

	return res, nil
}

func (e *appleSettingsRepo) UpdateAppleSettings(ctx context.Context, input *pb.AppleIdSettings) (string, error) {

	var resp = &pb.AppleIdSettings{}

	query := `UPDATE "apple_settings" SET
		team_id = $1,
		client_id = $2,
		key_id = $3,
		secret = $4
	WHERE
		id = $5
	RETURNING id`

	err := e.db.QueryRow(ctx, query,
		input.TeamId,
		input.ClientId,
		input.KeyId,
		input.Secret,
		input.Id,
	).Scan(
		&resp.Id,
	)
	if err != nil {
		return "", err
	}

	return resp.Id, nil
}

func (e *appleSettingsRepo) GetListAppleSettings(ctx context.Context, input *pb.GetListAppleIdSettingsRequest) (*pb.GetListAppleIdSettingsResponse, error) {

	arr := &pb.GetListAppleIdSettingsResponse{}

	query := `SELECT
		id,
		project_id,
		team_id,
		client_id,
		key_id,
		secret
	FROM
		"apple_settings"
	WHERE
		project_id = $1`

	rows, err := e.db.Query(ctx, query, input.ProjectId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		obj := &pb.AppleIdSettings{}
		err = rows.Scan(
			&obj.Id,
			&obj.ProjectId,
			&obj.TeamId,
			&obj.ClientId,
			&obj.KeyId,
			&obj.Secret,
		)

		if err != nil {
			return nil, err
		}

		arr.Items = append(arr.Items, obj)
	}

	return arr, nil
}

func (e *appleSettingsRepo) DeleteAppleSettings(ctx context.Context, input *pb.AppleIdSettingsPrimaryKey) (*emptypb.Empty, error) {

	var resp = &emptypb.Empty{}

	query := `DELETE FROM "apple_settings" WHERE id = $1`

	_, err := e.db.Query(ctx, query, input.Id)

	if err != nil {
		return nil, err
	}

	return resp, nil
}
