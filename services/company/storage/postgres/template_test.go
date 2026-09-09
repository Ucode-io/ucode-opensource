package postgres_test

import (
	"context"
	"testing"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

// Helper function to check if template table exists

// Helper function to create a template for testing
func createTemplate(t *testing.T) *pb.TemplateMetadata {
	ctx := context.Background()

	// Create sample JSONB data
	tables := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"users":    structpb.NewStringValue("users_table"),
			"products": structpb.NewStringValue("products_table"),
			"orders":   structpb.NewStringValue("orders_table"),
		},
	}

	functions := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"auth":      structpb.NewStringValue("authentication_function"),
			"payment":   structpb.NewStringValue("payment_processing"),
			"analytics": structpb.NewStringValue("analytics_function"),
		},
	}

	microfronts := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"catalog":  structpb.NewStringValue("product_catalog_mf"),
			"checkout": structpb.NewStringValue("checkout_mf"),
			"admin":    structpb.NewStringValue("admin_panel_mf"),
		},
	}

	req := &pb.CreateTemplateMetadataReq{
		Name:        faker.Name(),
		Description: faker.Sentence(),
		Photo:       faker.URL(),
		Tables:      tables,
		Functions:   functions,
		Microfronts: microfronts,
	}

	template, err := strg.TemplateMetadata().Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.NotEmpty(t, template.Id)
	assert.Equal(t, req.Name, template.Name)
	assert.Equal(t, req.Description, template.Description)
	assert.Equal(t, req.Photo, template.Photo)
	assert.Equal(t, int64(0), template.DeletedAt) // 0 means not deleted

	return template
}

// Helper function to delete a template for testing cleanup
func deleteTemplate(t *testing.T, id string) {
	ctx := context.Background()
	err := strg.TemplateMetadata().Delete(ctx, id)
	assert.NoError(t, err)
}

// TestCreateTemplate tests the Create operation
func TestCreateTemplate(t *testing.T) {
	template := createTemplate(t)
	assert.NotEmpty(t, template.Id)

	// Cleanup
	deleteTemplate(t, template.Id)
}

// TestCreateTemplateWithEmptyFields tests creating template with minimal data
func TestCreateTemplateWithEmptyFields(t *testing.T) {
	ctx := context.Background()

	req := &pb.CreateTemplateMetadataReq{
		Name: faker.Name(),
		// Other fields are optional
	}

	template, err := strg.TemplateMetadata().Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.NotEmpty(t, template.Id)
	assert.Equal(t, req.Name, template.Name)
	assert.Equal(t, int64(0), template.DeletedAt) // 0 means not deleted

	// Cleanup
	deleteTemplate(t, template.Id)
}

// TestGetTemplateById tests the GetById operation
func TestGetTemplateById(t *testing.T) {
	// Create a template first
	template := createTemplate(t)

	// Get the template by ID
	ctx := context.Background()
	retrievedTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedTemplate)
	assert.Equal(t, template.Id, retrievedTemplate.Id)
	assert.Equal(t, template.Name, retrievedTemplate.Name)
	assert.Equal(t, template.Description, retrievedTemplate.Description)
	assert.Equal(t, template.Photo, retrievedTemplate.Photo)
	assert.Equal(t, int64(0), retrievedTemplate.DeletedAt) // 0 means not deleted

	// Cleanup
	deleteTemplate(t, template.Id)
}

// TestGetTemplateByIdNotFound tests getting a non-existent template
func TestGetTemplateByIdNotFound(t *testing.T) {
	ctx := context.Background()

	// Try to get a template with a random ID
	randomID := "non-existent-id-12345"
	template, err := strg.TemplateMetadata().GetById(ctx, randomID)
	assert.Error(t, err)
	assert.Nil(t, template)
	assert.Contains(t, err.Error(), "template not found")
}

// TestGetTemplateList tests the GetList operation
func TestGetTemplateList(t *testing.T) {

	// Create multiple templates for testing
	template1 := createTemplate(t)
	template2 := createTemplate(t)

	ctx := context.Background()

	// Test getting active templates
	req := &pb.GetTemplateMetadataListReq{
		Limit:  10,
		Offset: 0,
	}

	result, err := strg.TemplateMetadata().GetList(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.Count, int32(2))
	assert.GreaterOrEqual(t, len(result.Templates), 2)

	// Test pagination
	req.Limit = 1
	req.Offset = 0
	result, err = strg.TemplateMetadata().GetList(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Templates, 1)

	// Test getting active templates (should still work)
	result, err = strg.TemplateMetadata().GetList(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Cleanup
	deleteTemplate(t, template1.Id)
	deleteTemplate(t, template2.Id)
}

// TestUpdateTemplate tests the Update operation
func TestUpdateTemplate(t *testing.T) {
	// Create a template first
	template := createTemplate(t)

	ctx := context.Background()

	// Update the template
	updateReq := &pb.UpdateTemplateMetadataReq{
		Id:          template.Id,
		Name:        faker.Name() + " updated",
		Description: faker.Sentence() + " updated",
		Photo:       faker.URL() + " updated",
		Tables: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"users":     structpb.NewStringValue("users_table_v2"),
				"products":  structpb.NewStringValue("products_table_v2"),
				"orders":    structpb.NewStringValue("orders_table_v2"),
				"inventory": structpb.NewStringValue("inventory_table"),
			},
		},
		Functions: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"auth":      structpb.NewStringValue("authentication_function_v2"),
				"payment":   structpb.NewStringValue("payment_processing_v2"),
				"analytics": structpb.NewStringValue("analytics_function_v2"),
				"reporting": structpb.NewStringValue("reporting_function"),
			},
		},
		Microfronts: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"catalog":   structpb.NewStringValue("product_catalog_mf_v2"),
				"checkout":  structpb.NewStringValue("checkout_mf_v2"),
				"admin":     structpb.NewStringValue("admin_panel_mf_v2"),
				"dashboard": structpb.NewStringValue("dashboard_mf"),
			},
		},
	}

	updatedTemplate, err := strg.TemplateMetadata().Update(ctx, updateReq)
	assert.NoError(t, err)
	assert.NotNil(t, updatedTemplate)
	assert.Equal(t, updateReq.Name, updatedTemplate.Name)
	assert.Equal(t, updateReq.Description, updatedTemplate.Description)
	assert.Equal(t, updateReq.Photo, updatedTemplate.Photo)
	assert.Equal(t, int64(0), updatedTemplate.DeletedAt) // 0 means not deleted

	// Verify the update by getting the template again
	retrievedTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.NoError(t, err)
	assert.Equal(t, updateReq.Name, retrievedTemplate.Name)
	assert.Equal(t, updateReq.Description, retrievedTemplate.Description)

	// Cleanup
	deleteTemplate(t, template.Id)
}

// TestUpdateTemplateNotFound tests updating a non-existent template
func TestUpdateTemplateNotFound(t *testing.T) {

	ctx := context.Background()

	updateReq := &pb.UpdateTemplateMetadataReq{
		Id:   "non-existent-id-12345",
		Name: faker.Name(),
	}

	template, err := strg.TemplateMetadata().Update(ctx, updateReq)
	assert.Error(t, err)
	assert.Nil(t, template)
	assert.Contains(t, err.Error(), "template not found")
}

// TestDeleteTemplate tests the Delete operation
func TestDeleteTemplate(t *testing.T) {

	// Create a template first
	template := createTemplate(t)

	ctx := context.Background()

	// Delete the template
	err := strg.TemplateMetadata().Delete(ctx, template.Id)
	assert.NoError(t, err)

	// Verify the template is soft deleted (not found when getting by ID)
	retrievedTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.Error(t, err)
	assert.Nil(t, retrievedTemplate)
	assert.Contains(t, err.Error(), "template not found")

	// Note: Since we only get active templates now, we can't verify deleted templates in the list
	// The template should not be found in the active templates list
	listReq := &pb.GetTemplateMetadataListReq{
		Limit:  10,
		Offset: 0,
	}

	result, err := strg.TemplateMetadata().GetList(ctx, listReq)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Since we only get active templates, our deleted template should not be in the list
	found := false
	for _, tmpl := range result.Templates {
		if tmpl.Id == template.Id {
			found = true
			break
		}
	}
	assert.False(t, found, "Deleted template should not be found in active templates list")
}

// TestDeleteTemplateNotFound tests deleting a non-existent template
func TestDeleteTemplateNotFound(t *testing.T) {

	ctx := context.Background()

	randomID := "non-existent-id-12345"
	err := strg.TemplateMetadata().Delete(ctx, randomID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "template not found")
}

// TestTemplateCRUD tests the complete CRUD cycle
func TestTemplateCRUD(t *testing.T) {

	// Create
	template := createTemplate(t)
	assert.NotEmpty(t, template.Id)

	// Read
	ctx := context.Background()
	retrievedTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.NoError(t, err)
	assert.Equal(t, template.Id, retrievedTemplate.Id)

	// Update
	updateReq := &pb.UpdateTemplateMetadataReq{
		Id:          template.Id,
		Name:        faker.Name() + " CRUD updated",
		Description: faker.Sentence() + " CRUD updated",
	}

	updatedTemplate, err := strg.TemplateMetadata().Update(ctx, updateReq)
	assert.NoError(t, err)
	assert.Equal(t, updateReq.Name, updatedTemplate.Name)

	// Verify update
	finalTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.NoError(t, err)
	assert.Equal(t, updateReq.Name, finalTemplate.Name)

	// Delete
	err = strg.TemplateMetadata().Delete(ctx, template.Id)
	assert.NoError(t, err)

	// Verify deletion
	deletedTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.Error(t, err)
	assert.Nil(t, deletedTemplate)
}

// TestTemplateWithComplexJSONB tests creating template with complex JSONB data
func TestTemplateWithComplexJSONB(t *testing.T) {

	ctx := context.Background()

	// Create complex nested JSONB structures
	complexTables := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"users": structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"table_name": structpb.NewStringValue("users"),
					"columns": structpb.NewListValue(&structpb.ListValue{
						Values: []*structpb.Value{
							structpb.NewStringValue("id"),
							structpb.NewStringValue("name"),
							structpb.NewStringValue("email"),
						},
					}),
					"indexes": structpb.NewListValue(&structpb.ListValue{
						Values: []*structpb.Value{
							structpb.NewStringValue("idx_users_email"),
							structpb.NewStringValue("idx_users_name"),
						},
					}),
				},
			}),
			"products": structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"table_name": structpb.NewStringValue("products"),
					"columns": structpb.NewListValue(&structpb.ListValue{
						Values: []*structpb.Value{
							structpb.NewStringValue("id"),
							structpb.NewStringValue("name"),
							structpb.NewStringValue("price"),
							structpb.NewStringValue("category_id"),
						},
					}),
				},
			}),
		},
	}

	req := &pb.CreateTemplateMetadataReq{
		Name:        faker.Name() + " Complex JSONB",
		Description: faker.Sentence(),
		Tables:      complexTables,
		Functions:   &structpb.Struct{},
		Microfronts: &structpb.Struct{},
	}

	template, err := strg.TemplateMetadata().Create(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, template)
	assert.NotEmpty(t, template.Id)

	// Verify the complex JSONB data is preserved
	retrievedTemplate, err := strg.TemplateMetadata().GetById(ctx, template.Id)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedTemplate.Tables)

	// Cleanup
	deleteTemplate(t, template.Id)
}

// TestTemplateListPagination tests pagination functionality
func TestTemplateListPagination(t *testing.T) {

	// Create multiple templates
	templates := make([]*pb.TemplateMetadata, 5)
	for i := 0; i < 5; i++ {
		templates[i] = createTemplate(t)
	}

	ctx := context.Background()

	// Test different pagination scenarios
	testCases := []struct {
		limit         int32
		offset        int32
		expectedCount int
	}{
		{2, 0, 2},  // First page, 2 items
		{2, 2, 2},  // Second page, 2 items
		{2, 4, 1},  // Third page, 1 item
		{10, 0, 5}, // All items
		{1, 0, 1},  // Single item
	}

	for _, tc := range testCases {
		req := &pb.GetTemplateMetadataListReq{
			Limit:  tc.limit,
			Offset: tc.offset,
		}

		result, err := strg.TemplateMetadata().GetList(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Templates, tc.expectedCount)
	}

	// Cleanup
	for _, template := range templates {
		deleteTemplate(t, template.Id)
	}
}

// TestTemplateTestStructure tests that the test structure is correct
func TestTemplateTestStructure(t *testing.T) {
	// This test doesn't require database connection
	// It just verifies that our test structure is correct

	// Test that we can create protobuf messages
	req := &pb.CreateTemplateMetadataReq{
		Name:        "Test Template",
		Description: "Test Description",
		Photo:       "test.jpg",
		Tables: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"test_table": structpb.NewStringValue("test_value"),
			},
		},
		Functions:   &structpb.Struct{},
		Microfronts: &structpb.Struct{},
	}

	assert.NotNil(t, req)
	assert.Equal(t, "Test Template", req.Name)
	assert.Equal(t, "Test Description", req.Description)
	assert.Equal(t, "test.jpg", req.Photo)
	assert.NotNil(t, req.Tables)
	assert.NotNil(t, req.Functions)
	assert.NotNil(t, req.Microfronts)

	// Test that we can create list request
	listReq := &pb.GetTemplateMetadataListReq{
		Limit:  10,
		Offset: 0,
	}

	assert.NotNil(t, listReq)
	assert.Equal(t, int32(10), listReq.Limit)
	assert.Equal(t, int32(0), listReq.Offset)

	// Test that we can create update request
	updateReq := &pb.UpdateTemplateMetadataReq{
		Id:          "test-id",
		Name:        "Updated Template",
		Description: "Updated Description",
	}

	assert.NotNil(t, updateReq)
	assert.Equal(t, "test-id", updateReq.Id)
	assert.Equal(t, "Updated Template", updateReq.Name)
	assert.Equal(t, "Updated Description", updateReq.Description)

	// Test that we can create delete request
	deleteReq := &pb.DeleteTemplateMetadataReq{
		Id: "test-id",
	}

	assert.NotNil(t, deleteReq)
	assert.Equal(t, "test-id", deleteReq.Id)
}
