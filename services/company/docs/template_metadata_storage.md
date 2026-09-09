# Template Metadata Storage

This document describes the Template Metadata Storage implementation for the Ucode Company Service.

## Overview

The Template Metadata Storage provides CRUD operations for managing template metadata in the `template` table. Templates contain metadata about application templates including tables, functions, and microfronts configurations.

## Table Structure

The storage operates on the `template` table with the following structure:

```sql
CREATE TABLE IF NOT EXISTS "template" (
    "id" UUID PRIMARY KEY,
    "name" VARCHAR NOT NULL,
    "description" TEXT,
    "photo" VARCHAR,
    "tables" JSONB NOT NULL DEFAULT '{}',
    "functions" JSONB NOT NULL DEFAULT '{}',
    "microfronts" JSONB NOT NULL DEFAULT '{}',
    "created_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "is_active" BOOLEAN NOT NULL DEFAULT TRUE
);
```

## Features

- **Create**: Add new template metadata with JSONB fields for tables, functions, and microfronts
- **Read**: Retrieve templates by ID or get paginated lists
- **Update**: Modify existing template metadata
- **Delete**: Soft delete templates by setting `is_active` to false
- **JSONB Support**: Full support for structured data in tables, functions, and microfronts fields

## API Interface

### Storage Interface

```go
type TemplateMetadataStorageI interface {
    Create(ctx context.Context, template *pb.CreateTemplateMetadataReq) (*pb.TemplateMetadata, error)
    GetById(ctx context.Context, id string) (*pb.TemplateMetadata, error)
    GetList(ctx context.Context, req *pb.GetTemplateMetadataListReq) (*pb.GetTemplateMetadataListRes, error)
    Update(ctx context.Context, template *pb.UpdateTemplateMetadataReq) (*pb.TemplateMetadata, error)
    Delete(ctx context.Context, id string) error
}
```

### Protobuf Messages

#### CreateTemplateMetadataReq
- `name`: Template name (required)
- `description`: Template description
- `photo`: Photo/image URL
- `tables`: JSONB structure for table configurations
- `functions`: JSONB structure for function configurations
- `microfronts`: JSONB structure for microfrontend configurations

#### TemplateMetadata
- `id`: Unique template identifier
- `name`: Template name
- `description`: Template description
- `photo`: Photo/image URL
- `tables`: Table configurations as protobuf Struct
- `functions`: Function configurations as protobuf Struct
- `microfronts`: Microfrontend configurations as protobuf Struct
- `created_at`: Creation timestamp
- `updated_at`: Last update timestamp
- `is_active`: Active status flag

## Usage Examples

### Creating a Template

```go
req := &pb.CreateTemplateMetadataReq{
    Name:        "E-commerce Template",
    Description: "A comprehensive template for e-commerce applications",
    Photo:       "ecommerce.jpg",
    Tables: &structpb.Struct{
        Fields: map[string]*structpb.Value{
            "users": structpb.NewStringValue("users_table"),
            "products": structpb.NewStringValue("products_table"),
        },
    },
    Functions: &structpb.Struct{
        Fields: map[string]*structpb.Value{
            "auth": structpb.NewStringValue("authentication_function"),
        },
    },
    Microfronts: &structpb.Struct{
        Fields: map[string]*structpb.Value{
            "catalog": structpb.NewStringValue("product_catalog_mf"),
        },
    },
}

template, err := storage.TemplateMetadata().Create(ctx, req)
```

### Retrieving a Template

```go
template, err := storage.TemplateMetadata().GetById(ctx, "template-id")
if err != nil {
    // Handle error
}
```

### Getting Template List

```go
req := &pb.GetTemplateMetadataListReq{
    Limit:    10,
    Offset:   0,
    IsActive: true,
}

result, err := storage.TemplateMetadata().GetList(ctx, req)
```

### Updating a Template

```go
req := &pb.UpdateTemplateMetadataReq{
    Id:          "template-id",
    Name:        "Updated Template Name",
    Description: "Updated description",
    // ... other fields
}

template, err := storage.TemplateMetadata().Update(ctx, req)
```

### Deleting a Template

```go
err := storage.TemplateMetadata().Delete(ctx, "template-id")
```

## Implementation Details

### Database Operations

- **Create**: Generates UUID, converts protobuf Struct to JSONB, inserts into database
- **Read**: Retrieves data, converts JSONB to protobuf Struct, handles timestamps
- **Update**: Updates fields, maintains `updated_at` timestamp, converts data types
- **Delete**: Soft delete using `is_active` flag and updates `updated_at`

### JSONB Handling

The storage includes helper functions for converting between protobuf Struct and JSONB:

- `structToJSON()`: Converts protobuf Struct to JSON bytes
- `jsonToStruct()`: Converts JSON bytes to protobuf Struct

### Error Handling

- Comprehensive error messages with context
- Proper handling of database-specific errors
- Validation of required fields
- Graceful handling of missing records

### Performance Features

- Pagination support for list operations
- Efficient JSONB queries
- Proper indexing on `id` and `is_active` fields
- Connection pooling through the existing Pool implementation

## Integration

The Template Metadata Storage is integrated into the main storage system:

1. **Interface**: Added to `storage.StorageI`
2. **Implementation**: Added to `postgres.Store`
3. **Repository**: Available through `storage.TemplateMetadata()`

## Testing

The storage includes comprehensive test coverage for all CRUD operations. Tests cover:

- Successful operations
- Error conditions
- Edge cases
- Data type conversions

## Dependencies

- `github.com/google/uuid` for ID generation
- `github.com/opentracing/opentracing-go` for tracing
- `google.golang.org/protobuf/types/known/structpb` for protobuf Struct handling
- PostgreSQL with JSONB support

## Future Enhancements

- Search and filtering capabilities
- Template versioning
- Template categories and tags
- Bulk operations
- Template import/export functionality
