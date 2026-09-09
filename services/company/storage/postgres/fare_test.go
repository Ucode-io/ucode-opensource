//go:build integration

package postgres_test

import (
	"context"
	"testing"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/stretchr/testify/assert"
)

func createFareItem(t *testing.T) string {
	fare, err := strg.Billing().CreateFareItem(context.Background(), &pb.CreateFareItemRequest{
		Name: fakeData.Name(),
		Info: fakeData.Sentence(4, true),
		Type: "request_per_second",
	})

	assert.NoError(t, err)
	assert.NotNil(t, fare)
	return fare.Id
}

func TestFareItemCRUD(t *testing.T) {
	id := createFareItem(t)
	assert.NotEmpty(t, id)

	item, err := strg.Billing().GetFareItem(context.Background(), &pb.PrimaryKey{Id: id})
	assert.NoError(t, err)
	assert.NotNil(t, item)

	err = strg.Billing().DeleteFareItem(context.Background(), &pb.PrimaryKey{Id: id})
	assert.NoError(t, err)

	listResp, err := strg.Billing().ListFareItem(context.Background(), &pb.ListRequest{
		Offset: 0,
		Limit:  10,
	})
	assert.NoError(t, err)
	assert.NotNil(t, listResp)

	item.Name = fakeData.Name() + " updated"
	_, err = strg.Billing().UpdateFareItem(context.Background(), item)
	assert.NoError(t, err)
}

func TestProjectsRPS(t *testing.T) {
	projects, err := strg.Project().ListProjectRPS(context.Background(), &pb.GetProjectListRequest{
		Offset: 0,
		Limit:  10,
	})

	assert.NoError(t, err)
	assert.NotNil(t, projects)
}