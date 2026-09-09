package clickhouse

import (
	"context"
	"fmt"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/company/config"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseStore struct {
	db driver.Conn
}

func NewClickHouse(cfg config.Config) (*ClickHouseStore, error) {
	var store = &ClickHouseStore{}

	conn, err := ch.Open(&ch.Options{
		Addr: []string{cfg.UcodeClickhouseHost + ":" + cfg.UcodeClickhousePort},
		Auth: ch.Auth{
			Database: "default",
			Username: cfg.UcodeClickhouseUser,
			Password: cfg.UcodeClickhousePassword,
		},
		DialTimeout: 5 * time.Second,
		Compression: &ch.Compression{
			Method: ch.CompressionLZ4,
		},
	})
	if err != nil {
		return &ClickHouseStore{}, err
	}

	if err := conn.Ping(context.Background()); err != nil {
		return &ClickHouseStore{}, err
	}

	store.db = conn

	return store, err
}

func (c *ClickHouseStore) CloseDb() {
	_ = c.db.Close()
}

func (c *ClickHouseStore) CreateUserAndDatabase(ctx context.Context, req CreateUser) error {
	var query = fmt.Sprintf(`CREATE DATABASE "%s"`, req.DbName)

	if err := c.db.Exec(ctx, query); err != nil {
		return err
	}

	query = fmt.Sprintf(`CREATE USER "%s" IDENTIFIED WITH plaintext_password BY '%s'`, req.UserName, req.Pass)

	if err := c.db.Exec(ctx, query); err != nil {
		return err
	}

	query = fmt.Sprintf(`GRANT ALL PRIVILEGES ON "%s".* TO "%s"`, req.DbName, req.UserName)
	if err := c.db.Exec(ctx, query); err != nil {
		return err
	}

	return nil
}

func (c *ClickHouseStore) DeleteUserAndDatabase(ctx context.Context, req DeleteUser) error {
	var query = fmt.Sprintf(`DROP DATABASE "%s"`, req.DbName)

	if err := c.db.Exec(ctx, query); err != nil {
		return err
	}

	query = fmt.Sprintf(`DROP USER "%s"`, req.UserName)

	if err := c.db.Exec(ctx, query); err != nil {
		return err
	}

	return nil
}
