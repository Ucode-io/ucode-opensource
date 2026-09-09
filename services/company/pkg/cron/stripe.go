package cron

//func CreatePaymentIntent(key string, amount int64, customerID, paymentMethodID string) (*stripe.PaymentIntent, error) {
//	stripe.Key = key
//
//	params := &stripe.PaymentIntentParams{
//		Amount:        stripe.Int64(amount),
//		Currency:      stripe.String(string(stripe.CurrencyUSD)),
//		Customer:      stripe.String(customerID),
//		PaymentMethod: stripe.String(paymentMethodID),
//		Confirm:       stripe.Bool(true),
//		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
//			Enabled: stripe.Bool(true),
//		},
//		OffSession: stripe.Bool(true),
//	}
//
//	pi, err := paymentintent.New(params)
//	if err != nil {
//		return nil, err
//	}
//
//	return pi, nil
//}
