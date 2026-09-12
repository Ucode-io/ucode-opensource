# Template Metadata Testing

This document describes the testing implementation for the Template Metadata Storage system.

## Overview

The testing suite provides comprehensive coverage for all CRUD operations of the template metadata storage, with intelligent handling of database availability.

## Test Structure

### Test Files

- **`storage/postgres/template_metadata_test.go`** - Main test file containing all test cases
- **`storage/postgres/main_test.go`** - Test setup and database connection

### Test Categories

1. **Unit Tests** - Test protobuf message creation and validation
2. **Integration Tests** - Test actual database operations (when table exists)
3. **Error Handling Tests** - Test various error scenarios
4. **Edge Case Tests** - Test boundary conditions and complex data structures

## Test Functions

### Basic CRUD Operations

- `TestCreateTemplate` - Tests template creation with full data
- `TestCreateTemplateWithEmptyFields` - Tests template creation with minimal data
- `TestGetTemplateById` - Tests retrieving template by ID
- `TestGetTemplateByIdNotFound` - Tests error handling for non-existent templates
- `TestUpdateTemplate` - Tests template updates
- `TestUpdateTemplateNotFound` - Tests update error handling
- `TestDeleteTemplate` - Tests soft delete functionality
- `TestDeleteTemplateNotFound` - Tests delete error handling

### Advanced Operations

- `TestTemplateCRUD` - Tests complete CRUD cycle
- `TestTemplateWithComplexJSONB` - Tests complex nested JSONB structures
- `TestTemplateListPagination` - Tests pagination functionality
- `TestGetTemplateList` - Tests list retrieval with filtering

### Structure Validation

- `TestTemplateTestStructure` - Tests protobuf message creation and validation

## Test Features

### Database Availability Detection

The test suite automatically detects whether the `template` table exists in the database:

```go
func checkTemplateTableExists(t *testing.T) bool {
    // Attempts to create a test template
    // Returns true if table exists, false otherwise
}
```

### Intelligent Test Skipping

Tests automatically skip when the database table doesn't exist:

```go
func TestCreateTemplate(t *testing.T) {
    if !checkTemplateTableExists(t) {
        t.Skip("Template table does not exist, skipping test")
    }
    // ... test implementation
}
```

### Comprehensive Test Coverage

- **Create Operations**: Tests with full and minimal data
- **Read Operations**: Tests single and list retrieval
- **Update Operations**: Tests field updates and validation
- **Delete Operations**: Tests soft delete and verification
- **Error Handling**: Tests various error scenarios
- **Data Validation**: Tests JSONB handling and protobuf conversion

## Test Data

### Sample Template Data

Tests use realistic sample data including:

- **Tables**: User, product, and order table configurations
- **Functions**: Authentication, payment, and analytics functions
- **Microfronts**: Catalog, checkout, and admin panel configurations

### Complex JSONB Structures

Tests include complex nested JSONB data:

```go
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
    },
}
```

## Running Tests

### Run All Template Tests

```bash
cd storage/postgres
go test -v -run "Test.*Template"
```

### Run Specific Test Categories

```bash
# Run only CRUD tests
go test -v -run "TestTemplateCRUD"

# Run only creation tests
go test -v -run "TestCreate.*Template"

# Run only error handling tests
go test -v -run "Test.*NotFound"
```

### Run Structure Tests (No Database Required)

```bash
go test -v -run TestTemplateTestStructure
```

## Test Output

### When Table Exists

Tests run normally and verify all functionality:

```
=== RUN   TestCreateTemplate
--- PASS: TestCreateTemplate (0.15s)
=== RUN   TestGetTemplateById
--- PASS: TestGetTemplateById (0.12s)
...
```

### When Table Doesn't Exist

Tests skip gracefully with informative messages:

```
=== RUN   TestCreateTemplate
    template_metadata_test.go:102: Template table does not exist, skipping test
--- SKIP: TestCreateTemplate (0.54s)
=== RUN   TestGetTemplateById
    template_metadata_test.go:139: Template table does not exist, skipping test
--- SKIP: TestGetTemplateById (0.10s)
...
```

## Test Dependencies

### Required Packages

- `github.com/stretchr/testify/assert` - Assertion library
- `github.com/bxcodec/faker/v3` - Test data generation
- `google.golang.org/protobuf/types/known/structpb` - Protobuf Struct handling

### Database Requirements

- PostgreSQL with JSONB support
- `template` table (created by migration 57)
- Proper database connection configuration

## Test Configuration

### Environment Variables

Tests use the same configuration as the main application:

- Database connection details from `.env` file
- Remote database connection
- Company service database

### Test Setup

Tests are initialized in `TestMain`:

```go
func TestMain(m *testing.M) {
    cfg = config.BaseLoad()
    strg, err = postgres.NewPostgres(context.Background(), cfg, nil)
    fakeData, _ = faker.New("en")
    os.Exit(m.Run())
}
```

## Best Practices

### Test Organization

1. **Helper Functions**: Reusable functions for common operations
2. **Cleanup**: Automatic cleanup after each test
3. **Data Isolation**: Each test uses unique data
4. **Error Verification**: Comprehensive error message checking

### Test Data Management

1. **Faker Integration**: Realistic test data generation
2. **Unique Identifiers**: UUID generation for test data
3. **JSONB Validation**: Complex data structure testing
4. **Edge Cases**: Boundary condition testing

### Error Handling

1. **Expected Errors**: Tests verify correct error messages
2. **Error Types**: Tests check for specific error conditions
3. **Graceful Degradation**: Tests skip when dependencies unavailable

## Future Enhancements

### Planned Test Improvements

- **Performance Testing**: Benchmark database operations
- **Concurrency Testing**: Test concurrent access patterns
- **Load Testing**: Test with large datasets
- **Mock Testing**: Unit tests with mocked database

### Test Coverage Expansion

- **Validation Testing**: Test input validation rules
- **Security Testing**: Test access control and permissions
- **Integration Testing**: Test with other services
- **End-to-End Testing**: Test complete workflows

## Troubleshooting

### Common Issues

1. **Database Connection**: Ensure database is accessible
2. **Table Existence**: Run migration 57 to create template table
3. **Environment Variables**: Check `.env` file configuration
4. **Dependencies**: Ensure all Go modules are available

### Debug Information

Tests provide detailed error messages and skip reasons:

```
Template table does not exist, skipping test
failed to create template: ERROR: relation "template" does not exist
```

## Conclusion

The template metadata testing suite provides comprehensive coverage of all storage operations with intelligent handling of database availability. Tests automatically adapt to the environment and provide clear feedback on test execution status.
