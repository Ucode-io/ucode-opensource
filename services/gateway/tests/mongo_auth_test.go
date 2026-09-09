package tests

// // Done
// func TestEmailRegister(t *testing.T) {
// 	faker1, err := faker.New("en")
// 	assert.NoError(t, err, "faker error")

// 	body, err := UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/register?project-id="+ProjectIdMongo, "POST",
// 		map[string]any{
// 			"data": map[string]any{"type": "email", "client_type_id": ClientTypeIdMongo, "role_id": RoleIdMongo, "email": Email, "name": faker1.Name()},
// 		},
// 		map[string]string{"Resource-Id": ResourceIdMongo, "Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	var registerResponse RegisterResponse
// 	err = json.Unmarshal(body, &registerResponse)
// 	assert.NoError(t, err)

// 	userId := registerResponse.Data.UserID

// 	body, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/send-code", "POST", map[string]any{
// 		"recipient": Email,
// 		"text":      "This is your code",
// 		"type":      "EMAIL",
// 	}, map[string]string{"Resource-Id": ResourceIdMongo, "Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	var smsResponse SmsResponse
// 	err = json.Unmarshal(body, &smsResponse)
// 	assert.NoError(t, err)

// 	smsId := smsResponse.Data.SmsID

// 	_, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/login/with-option?project-id="+ProjectIdMongo, "POST", map[string]any{
// 		"data": map[string]any{
// 			"sms_id":         smsId,
// 			"otp":            "111111",
// 			"email":          Email,
// 			"client_type_id": ClientTypeIdMongo,
// 			"role_id":        RoleIdMongo,
// 		},
// 		"login_strategy": "EMAIL_OTP",
// 	}, map[string]string{"Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.Delete(&sdk.Argument{
// 		TableSlug: "user_email",
// 		Request: sdk.Request{Data: map[string]any{
// 			"guid": userId,
// 		}},
// 	})
// 	assert.NoError(t, err)
// }

// // Done
// func TestPhoneRegister(t *testing.T) {
// 	faker1, err := faker.New("en")
// 	assert.NoError(t, err, "faker error")
// 	phone := "+998946236953"

// 	body, err := UcodeApi.DoRequest(BaseUrlAuthStaging+"/v2/register?project-id="+ProjectIdMongo, "POST",
// 		map[string]any{
// 			"data": map[string]any{"type": "phone", "client_type_id": ClientTypeIdMongo, "role_id": RoleIdMongo, "phone": phone, "name": faker1.Name()},
// 		},
// 		map[string]string{"Resource-Id": ResourceIdMongo, "Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	var registerResponse RegisterResponse
// 	err = json.Unmarshal(body, &registerResponse)
// 	assert.NoError(t, err)

// 	userId := registerResponse.Data.UserID

// 	body, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/send-code", "POST", map[string]any{
// 		"recipient": phone,
// 		"text":      "This is your code",
// 		"type":      "PHONE",
// 	}, map[string]string{"Resource-Id": ResourceIdMongo, "Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	var smsResponse SmsResponse
// 	err = json.Unmarshal(body, &smsResponse)
// 	assert.NoError(t, err)

// 	smsId := smsResponse.Data.SmsID

// 	_, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/login/with-option?project-id="+ProjectIdPg, "POST", map[string]any{
// 		"data": map[string]any{
// 			"sms_id":         smsId,
// 			"otp":            "111111",
// 			"phone":          phone,
// 			"client_type_id": ClientTypeIdMongo,
// 			"role_id":        RoleIdMongo,
// 		},
// 		"login_strategy": "PHONE_OTP",
// 	}, map[string]string{"Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.Delete(&sdk.Argument{
// 		TableSlug: "user_email",
// 		Request: sdk.Request{Data: map[string]any{
// 			"guid": userId,
// 		}},
// 	})
// 	assert.NoError(t, err)
// }

// // Done
// func TestLoginRegister(t *testing.T) {
// 	var (
// 		guid  = uuid.New().String()
// 		login = "test_login123"
// 	)

// 	_, _, err := UcodeApiForStaging.CreateObject(&sdk.Argument{
// 		TableSlug: "employee",
// 		Request: sdk.Request{
// 			Data: map[string]any{
// 				"guid":           guid,
// 				"login":          login,
// 				"password":       login,
// 				"email":          Email,
// 				"role_id":        EmployeeRoleIdMongo,
// 				"client_type_id": EmployeeClientTypeIdMongo,
// 			},
// 		},
// 		DisableFaas: true,
// 	})
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/login/with-option?project-id="+ProjectIdMongo, "POST", map[string]any{
// 		"data": map[string]any{
// 			"username":       login,
// 			"password":       login,
// 			"client_type_id": EmployeeClientTypeIdMongo,
// 			"role_id":        EmployeeRoleIdMongo,
// 		},
// 		"login_strategy": "LOGIN_PWD",
// 	}, map[string]string{"Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	userResp, _, err := UcodeApiForStaging.GetSingleSlim(&sdk.Argument{
// 		TableSlug:   "employee",
// 		DisableFaas: true,
// 		Request: sdk.Request{
// 			Data: map[string]any{"guid": guid},
// 		},
// 	})
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/reset-password", "PUT", map[string]any{
// 		"password": "12345678",
// 		"user_id":  userResp.Data.Data.Response["user_id_auth"],
// 	}, map[string]string{})
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/login/with-option?project-id="+ProjectIdMongo, "POST", map[string]any{
// 		"data": map[string]any{
// 			"username":       login,
// 			"password":       "12345678",
// 			"client_type_id": EmployeeClientTypeIdMongo,
// 			"role_id":        EmployeeRoleIdMongo,
// 		},
// 		"login_strategy": "LOGIN_PWD",
// 	}, map[string]string{"Environment-Id": EnvironmentIdMongo, "X-API-KEY": UcodeApiForStaging.Config().AppId},
// 	)
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.DoRequest(BaseUrlAuthStaging+"/v2/forgot-password", "POST", map[string]any{"login": login}, map[string]string{})
// 	assert.NoError(t, err)

// 	_, err = UcodeApiForStaging.Delete(&sdk.Argument{
// 		TableSlug: "employee",
// 		Request: sdk.Request{
// 			Data: map[string]any{
// 				"guid": guid,
// 			},
// 		},
// 	})
// 	assert.NoError(t, err)
// }
