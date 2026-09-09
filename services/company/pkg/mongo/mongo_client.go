package mongo

import (
	"context"
	"fmt"
	"log"
	"github.com/Ucode-io/ucode-opensource/services/company/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Commands struct {
	client *mongo.Client
}

type CreateUser struct {
	Db       string
	UserName string
	Pass     string
}

type DeleteUser struct {
	Db       string
	UserName string
}

func Connection(ctx context.Context, cfg config.Config, nodeType string) (*Commands, error) {

	namespace := "mongodb:"
	cfg.UcodeMongoPort = ":" + cfg.UcodeMongoPort

	if nodeType == config.EnterPriceNodeType {
		namespace = "mongodb+srv:"
		cfg.UcodeMongoPort = ""
	}

	uri := fmt.Sprintf("%s//%s:%s@%s%s/admin",
		namespace,
		cfg.UcodeMongoUser,
		cfg.UcodeMongoPassword,
		cfg.UcodeMongoHost,
		cfg.UcodeMongoPort,
	)

	clientOptions := options.Client().ApplyURI(uri)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}
	return &Commands{
		client: client,
	}, nil
}

func (c *Commands) Disconnect(ctx context.Context) {
	err := c.client.Disconnect(ctx)
	if err != nil {
		log.Println("mongo disconnection error", err.Error())
	}
}

func (c *Commands) CreateUser(ctx context.Context, user CreateUser) error {
	r := c.client.Database(user.Db).RunCommand(ctx, bson.D{
		{Key: "createUser", Value: user.UserName},
		{Key: "pwd", Value: user.Pass},
		{Key: "roles", Value: []bson.M{
			{
				"role": "readWrite",
				"db":   user.Db,
			},
		}},
	})

	if r.Err() != nil {
		log.Printf("credentials(username,password,db): %s:%s:%s", user.UserName, user.Pass, user.Db)
		log.Println("error while create user", r.Err())
		return r.Err()
	}
	return nil
}

func (c *Commands) DeleteUser(ctx context.Context, user DeleteUser) error {
	r := c.client.Database(user.Db).RunCommand(ctx, bson.D{
		{Key: "dropUser", Value: user.UserName},
	})

	if r.Err() != nil {
		return r.Err()
	}
	return nil
}
