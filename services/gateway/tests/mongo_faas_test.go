package tests

var FaaSNameForMongo = "wellplayed-earth"

// // Done
// func TestOpenFaaS(t *testing.T) {
// 	_, err := UcodeApi.DoRequest(BaseUrl+"/v1/invoke_function/"+FaaSNameForMongo, http.MethodPost, map[string]any{},
// 		map[string]string{
// 			"Resource-Id":    ResourceIdMongo,
// 			"Environment-Id": EnvironmentIdMongo,
// 			"X-API-KEY":      UcodeApiForStaging.Config().AppId,
// 			"Authorization":  "API-KEY",
// 		},
// 	)
// 	assert.NoError(t, err)
// }
