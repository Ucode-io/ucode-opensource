//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
)

func deleteFare(id string) error {
	err := strg.Billing().DeleteFare(context.Background(), &pb.PrimaryKey{
		Id: id,
	})
	return err
}

func createFare(t *testing.T) string {
	var (
		ctx  = context.Background()
		fare = &pb.CreateFareRequest{
			Name:           faker.Name(),
			CurrencyId:     config.PaymeCurrencyId,
			Price:          faker.Latitude(),
			TrialDays:      int32(faker.RandomUnixTime()),
			DisactivateDay: int32(faker.UnixTime()),
			Description:    faker.Sentence(),
			FareItemPrices: []*pb.FareItemPrice{
				{
					FareItemId:     createFareItem(t),
					Value:          fmt.Sprintf("%d", faker.RandomUnixTime()),
					Price:          faker.Latitude(),
					OverLimitPrice: faker.Longitude(),
				},
			},
		}
	)

	f, err := strg.Billing().CreateFare(ctx, fare)
	assert.NoError(t, err)
	assert.NotNil(t, f)
	assert.Len(t, f.FareItemPrices, len(fare.FareItemPrices))

	return f.Id
}

func TestCreateFare(t *testing.T) {
	id := createFare(t)

	err = deleteFare(id)
	assert.NoError(t, err)
}

func TestGetFareById(t *testing.T) {
	id := createFare(t)
	fare, err := strg.Billing().GetFareById(context.Background(), &pb.PrimaryKey{
		Id:        id,
		ProjectId: projectId,
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, fare)

	deleteFare(id)
}

func TestUpdateTransaction(t *testing.T) {
	transaction, err := strg.Billing().UpdateTransaction(context.Background(), &pb.Transaction{
		Id:            createTransaction(t),
		Comment:       "Updated comment",
		PaymentStatus: "accepted",
	})
	assert.NoError(t, err)
	assert.NotNil(t, transaction)
}

func TestGetTransactions(t *testing.T) {
	list, err := strg.Billing().GetListTransactions(context.Background(), &pb.ListRequest{
		Limit:     10,
		Offset:    0,
		ProjectId: projectId,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, list)
}

func TestProcessUcodeRenewals(t *testing.T) {

	err := strg.Billing().ProcessUcodeRenewals(context.Background())
	assert.NoError(t, err)
}

func TestCalculatePrice(t *testing.T) {
	fareId := createFare(t)
	resp, err := strg.Billing().CalculatePrice(context.Background(), &pb.CalculatePriceRequest{
		ProjectId: projectId,
		FareId:    fareId,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
	err = deleteFare(fareId)
	assert.NoError(t, err)
}

func TestListDiscounts(t *testing.T) {
	resp, err := strg.Billing().ListDiscounts(context.Background(), &pb.ListRequest{
		Limit:  10,
		Offset: 0,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
}

func TestListFares(t *testing.T) {
	resp, err := strg.Billing().GetListFares(context.Background(), &pb.ListRequest{
		Limit:  10,
		Offset: 0,
	})

	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
}

func createTransaction(t *testing.T) string {
	fareId := createFare(t)
	tr, err := strg.Billing().CreateTransaction(context.Background(), &pb.CreateTransactionRequest{
		ProjectId:       projectId,
		CreatorId:       CreateRandomId(t),
		PaymentStatus:   "pending",
		Amount:          fakeData.Latitude(),
		TransactionType: "topup",
		CreatorType:     "user",
		FareId:          fareId,
		CurrencyId:      config.PaymeCurrencyId,
		PaymentType:     "Payme",
	})

	deleteFare(fareId)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.NotNil(t, tr)

	return tr.Id
}

func TestCreateTransaction(t *testing.T) {
	trId := createTransaction(t)
	assert.NotEmpty(t, trId)
}

// func TestAttachFare(t *testing.T) {
// 	// 	discount_id
// 	// :
// 	// "ce1809e3-85db-4f94-b7fd-a8623530297b"
// 	// fare_id
// 	// :
// 	// "7d958abe-08df-4735-a4f3-d275d1482b47"
// 	// project_id
// 	// :
// 	// "81f457e0-a137-4e48-8ddc-2abfcd04bd84"

// 	_, err := strg.Project().AttachFare(context.Background(), &pb.AttachFareRequest{
// 		DiscountId: "ce1809e3-85db-4f94-b7fd-a8623530297b",
// 		FareId:     "7d958abe-08df-4735-a4f3-d275d1482b47",
// 		ProjectId:  "81f457e0-a137-4e48-8ddc-2abfcd04bd84",
// 	})

// 	assert.NoError(t, err)

// }