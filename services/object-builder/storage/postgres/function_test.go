package postgres_test

//
//import (
//	"context"
//	"testing"
//	"github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
//
//	"github.com/stretchr/testify/assert"
//)
//
///*
//	id,
//	name,
//	path,
//	type,
//	description,
//	project_id,
//	environment_id,
//	url,
//	password,
//	ssh_url,
//	gitlab_id,
//	gitlab_group_id,
//	request_time,
//	source_url,
//	branch,
//	error_message,
//	pipeline_status
//*/
//
//func createFunction(t *testing.T) *new_object_builder_service.Function {
//	req := &new_object_builder_service.CreateFunctionRequest{
//		// Name:           fakeData.City(),
//		// Path:           fakeData.Country(),
//		// Type:           "KNATIVE",
//		// ProjectId:      "633dc21e-addb-4708-8ef9-fd3cd8d76da2",
//		// EnvironmentId:  CreateRandomId(t),
//		// Url:            fakeData.URL(),
//		// ErrorMessage:   fakeData.DomainName(),
//		// PipelineStatus: fakeData.JobTitle(),
//	}
//
//	resp, err := strg.Function().Create(context.Background(), req)
//
//	assert.NoError(t, err)
//	assert.NotEmpty(t, resp)
//
//	return resp
//}
//
//func TestCreateFunction(t *testing.T) {
//	id := createFunction(t)
//	assert.NotEmpty(t, id)
//}
//
//func TestUpdateFunction(t *testing.T) {
//	id := createFunction(t)
//
//	err := strg.Function().Update(context.Background(), id)
//
//	assert.NoError(t, err)
//}
//
//func TestListFunctions(t *testing.T) {
//	resp, err := strg.Function().GetList(context.Background(), &new_object_builder_service.GetAllFunctionsRequest{
//		ProjectId: "633dc21e-addb-4708-8ef9-fd3cd8d76da2",
//	})
//	assert.NoError(t, err)
//	assert.NotEmpty(t, resp)
//}
