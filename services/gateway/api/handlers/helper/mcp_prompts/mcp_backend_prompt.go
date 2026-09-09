package mcp_prompts

import (
	"errors"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

var (
	SystemPromptBackend = `You are connected to an MCP server named "ucode" and have access to tools via the mcp_toolset.
AVAILABLE TOOLS:
- get_dbml: Get database schema
- create_table: Create a new table
- update_table: Add fields and relations to a table (bulk)
- create_table_item: Create a row/record in a table (CRITICAL - must use for test data)
- create_menu: Create a menu
CRITICAL RULES:
1. For field creation, ALWAYS use update_table (bulk), NOT create_field
2. After creating tables and fields, you MUST call create_table_item to create test data
3. Each table MUST have at least 3 test records
4. Do not invent results — call the appropriate tool with exact parameters`

	SystemPromptClassifyRequest = `You are a request classifier for a full-stack admin panel system.

Your task: Analyze user request and determine if it requires BACKEND operations, FRONTEND operations, or BOTH.

BACKEND operations include:
- Creating/deleting tables (e.g., "add table users", "create table products")
- Creating/updating/deleting fields (e.g., "add field email to users", "remove field phone")
- Creating/modifying menus (e.g., "create menu orders")
- Database schema changes (e.g., "add relation between users and orders")
- Any operation involving database structure

FRONTEND operations include:
- UI/visual changes (e.g., "change sidebar color", "make header sticky")
- Layout modifications (e.g., "add search bar", "change button size")
- Styling updates (e.g., "use blue theme", "increase font size")
- Component behavior (e.g., "add loading spinner", "make table sortable")
- Any operation involving visual presentation

IMPORTANT RULES:
1. A request can require BOTH backend and frontend operations
2. If user mentions table/field/menu creation AND UI changes → both=true
3. If image is provided, it typically indicates frontend changes (but can still have backend operations in text)
4. Backend operations use MCP tools, frontend operations modify React code
5. Analyze the COMPLETE request - don't ignore any part

Examples:

Request: "add table users"
→ requires_backend=true, requires_frontend=false

Request: "change sidebar to dark blue"
→ requires_backend=false, requires_frontend=true

Request: "add table products with fields name, price AND make sidebar blue"
→ requires_backend=true, requires_frontend=true

Request: "create menu orders" [with image showing UI design]
→ requires_backend=true, requires_frontend=true (image suggests UI changes)

Analyze this request carefully and return ONLY valid JSON (no markdown, no explanation):

{
  "requires_backend": true/false,
  "requires_frontend": true/false,
  "backend_reason": "brief explanation why backend is/isn't needed",
  "frontend_reason": "brief explanation why frontend is/isn't needed",
  "confidence": "high/medium/low"
}

User Request: "%s"
Has Images: %v

Return ONLY the JSON object. Start with { and end with }.`
)

func BuildClassificationPrompt(userRequest string, hasImages bool) string {
	return fmt.Sprintf(SystemPromptClassifyRequest, userRequest, hasImages)
}

func BuildBackendPrompt(request models.GeneratePromptRequest) (content, message string, err error) {
	switch request.Method {
	case "project":
		content = fmt.Sprintf(`You are creating a complete u-code project from scratch. Analyze the user's request and determine:
- Project Type (CRM, ERP, E-commerce, Admin Panel, etc.)
- Management Systems needed (ProjectManagement, Payroll, Inventory, Sales, etc.)
- Industry (IT, Healthcare, Finance, Retail, etc.)

Then follow these steps EXACTLY:

STEP 1: Retrieve current DBML schema (if any exists)
- Use get_dbml tool with:
  project-id = %s
  environment-id = %s
  x-api-key = %s

STEP 2: Analyze user request and generate comprehensive DBML schema
User Request: "%s"

Based on the user's request, determine:
1. What type of project/system they need (CRM, ERP, Admin Panel, etc.)
2. What functional areas/modules are required
3. What industry/domain this is for
4. What tables, relationships, and data structures are needed

Generate a complete DBML schema that:
- Matches the user's requirements EXACTLY
- Uses PostgreSQL syntax
- **EXCLUDES all existing tables** from current schema (if retrieved in STEP 1)
- Includes industry-specific functional areas based on your analysis
- DO NOT include Users or Roles tables (they already exist in u-code)
- Use proper Ref definitions: Ref fk_name: table1.column1 < table2.column2
- For status/type fields, use realistic Enum definitions: Enum "status" { "Active" "Inactive" "Pending" }
- Use camelCase or snake_case consistently
- No indexes, no quotes, no cascade options, no default values
- Use descriptive field names, NO comments
- Follow relational design principles
- If user mentions specific number of menus/tables (e.g., "10 menus"), create exactly that many

STEP 3: Create tables directly using create_table tool
- **Quantity Rule**: If the user did NOT specify a number of tables/menus, strictly create approximately **10 tables** suitable for the project logic.
- **Icon Rule**: You **MUST** provide an 'icon' parameter for EVERY table. Use a valid full URL from Iconify (e.g., "https://api.iconify.design/mdi:account.svg"). Pick an icon relevant to the table's function.
- **Menu ID Rule**: You **MUST** provide 'menu_id' for every table. Use the root parent_id = "%s".
- Call create_table with: label, slug (snake_case), icon, menu_id, x-api-key.
- IMPORTANT: Save the response from each create_table call - you need table_id and slug (collection) for next steps
- Example: create_table({label: "Customers", slug: "customers", icon: "https://api.iconify.design/mdi:account.svg", menu_id: "%s", x_api_key: "%s"})
- Do NOT create folders, all tables are created at root level using the provided menu_id.

STEP 4: Create fields for each table (IMPORTANT: use update_table, not create_field)
- IMPORTANT INSTRUCTION FOR FIELD CREATION:
  - Do NOT call the create_field tool under any circumstances for automated field creation.
  - Instead, always aggregate fields for a table and call update_table with a fields array and relations array.
  - If the plan would create several individual fields, combine them into a single update_table call per table (bulk create).
  - If update_table cannot perform a specific field creation (server returns error), STOP and report the error back (do not retry individual create_field).
  - Use create_field only if a human explicitly requests a single manual field creation and only after explicit confirmation.
- Then use update_table to add all fields and relations for each table.

- For each table created, add ALL necessary fields based on DBML schema
- Use update_table with: tableSlug (collection slug), xapikey, fields array, relations array
- Field types: SINGLE_LINE, TEXT, NUMBER, DATE, BOOLEAN, ENUM, etc.
- Add standard fields: id (auto), name/title (required), created_at, updated_at
- Add all fields from your DBML schema
- Example: update_table({tableSlug: "customers", xapikey: "%s", fields: [...], relations: [...]})

STEP 5: Organize tables into menus
- Since no folders are created, all tables use parent_id = "%s"
- Provide view_fields JSON: { "table_slug": "primary_field_slug" }
- Example: { "customers": "name", "orders": "order_number" }

STEP 6: Execute DBML (optional - if you prefer bulk creation)
- If you want to use dbml_to_ucode for bulk creation, provide:
  - dbml: (full DBML string)
  - view_fields: (JSON object)
  - menus: (JSON object where keys are table names, values are table slugs)
  - x-api-key: %s

STEP 7: Create test data for each table (CRITICAL: Create exactly 3 items per table)
- After ALL tables and fields are created, you MUST populate EVERY SINGLE TABLE with exactly **3 test rows**.
- Use create_table_item tool with: table_slug, data (object with field values), x_api_key
- Example: create_table_item({table_slug: "customers", data: {name: "John Doe", email: "john@example.com", status: "Active"}, x_api_key: "%s"})

CRITICAL RULES FOR TEST DATA CREATION:
1. **Iterate ALL Tables**: Do not stop after creating data for just 2 or 3 tables. If you created 10 tables in Step 3, you MUST create test data for all 10 tables.
2. **Quantity**: Create exactly **3 records** per table.
3. **Dependency Order (Crucial)**:
   - First create records in parent tables (tables referenced by others).
   - Then create records in child tables (tables containing foreign keys).
   - SAVE the 'id' from parent records to use in child records.
4. **Realistic Data**:
   - Use realistic names, dates, and numbers suitable for the industry (Finance, Healthcare, etc.).
   - For ENUMs, use valid options defined in Step 2.
   - For RELATIONS, use only valid IDs from previously created records.
5. **Orphans**: Never create a child record without a valid parent ID.

CRITICAL RULES:
1. All tables are created at root level: parent_id = "%s"
2. Save table_id and collection (slug) from create_table responses
3. Use collection (slug) as the "collection" parameter in create_field (only for manual single-field requests)
4. If any tool call fails, STOP and report the error - do not retry
5. Create a COMPLETE, working project - not just partial structure
6. Analyze user request carefully - if they say "10 menus", create exactly 10 tables
7. Remember: Empty folders are not used, all tables are at root level
8. **Final Verification**: Ensure that Step 7 was executed for ALL tables before finishing.

Context:
project-id = %s
environment-id = %s
x-api-key = %s
main-menu-parent-id = %s

User Request: "%s"

Now analyze the user's request, determine project type/systems/industry, create the complete project structure directly as tables at root level, and finally populate ALL tables with 3 test items each.`,
			request.ProjectId, request.EnvironmentId, request.APIKey, request.UserPrompt,

			config.MainMenuID, config.MainMenuID, request.APIKey,

			request.APIKey, // 145

			config.MainMenuID, // 148

			request.APIKey, // 157

			request.APIKey, // 162

			config.MainMenuID,

			request.ProjectId, request.EnvironmentId, request.APIKey, config.MainMenuID,
			request.UserPrompt,
		)

		message = "Your project has been successfully created with test data."

	case "table":
		content = fmt.Sprintf(`You are managing backend operations for a u-code project.

USER REQUEST: "%s"

AVAILABLE MCP TOOLS (via mcp_toolset):
You have access to these tools through the MCP server "ucode":

1. get_dbml - Get current database schema
   Parameters:
     - project-id: "%s"
     - environment-id: "%s"
     - x-api-key: "%s"

2. create_table - Create new table
   Parameters:
     - label: Display name (e.g., "Users", "Products")
     - slug: URL-safe name in snake_case (e.g., "users", "products")
     - icon: Valid Iconify URL (e.g., "https://api.iconify.design/mdi:account.svg")
     - menu_id: "%s" (ALWAYS use this value)
     - x-api-key: "%s"
   Example: create_table({label: "Users", slug: "users", icon: "https://api.iconify.design/mdi:account.svg", menu_id: "%s", x_api_key: "%s"})

3. update_table - Add or modify fields and relations in bulk
   Parameters:
     - tableSlug: Table slug (collection name)
     - xapikey: "%s"
     - fields: Array of field objects
     - relations: Array of relation objects

   Field types available:
     - SINGLE_LINE: Short text (names, titles)
     - TEXT: Long text (descriptions)
     - NUMBER: Integers
     - FLOAT: Decimal numbers
     - DATE: Date values
     - BOOLEAN: True/false
     - ENUM: Predefined options
     - RELATION: Foreign key to another table

   Field object structure:
   {
     "type": "SINGLE_LINE",
     "label": "Full Name",
     "slug": "full_name",
     "required": true,
     "attributes": {}
   }

   Example: update_table({tableSlug: "users", xapikey: "%s", fields: [{type: "SINGLE_LINE", label: "Name", slug: "name", required: true}, {type: "TEXT", label: "Bio", slug: "bio"}], relations: []})

4. create_field - Create single field (USE ONLY FOR EXPLICIT SINGLE FIELD REQUESTS)
   IMPORTANT: Do NOT use this for bulk operations. Use update_table instead.
   Only call this when user explicitly says "add one field" or "create single field"
   Parameters:
     - collection: Table slug
     - xapikey: "%s"
     - type: Field type
     - label: Display name
     - slug: Field slug
     - required: boolean
     - attributes: Field-specific settings

CRITICAL RULES FOR OPERATIONS:

For TABLE CREATION:
1. ALWAYS provide a valid icon URL from Iconify (https://api.iconify.design/...)
2. ALWAYS use menu_id = "%s"
3. Slug must be snake_case (e.g., "user_profiles", not "UserProfiles")
4. After creating table, use update_table to add fields

For FIELD CREATION:
1. PREFER update_table for adding multiple fields at once
2. Use create_field ONLY when explicitly requested for single field
3. Always include standard fields: name/title, created_at, updated_at
4. For ENUM fields, define options in attributes

For MENU CREATION:
1. Creating a menu means creating a table (menus and tables are same in u-code)
2. Follow same rules as table creation

OPERATION WORKFLOW:

Step 1: Understand the request
- Is this creating new table/menu?
- Is this adding fields to existing table?
- Is this modifying existing structure?

Step 2: Check current schema (if modifying existing)
- Call get_dbml to see existing structure
- Identify what needs to change

Step 3: Execute appropriate MCP tool
- create_table for new tables/menus
- update_table for adding/modifying fields in bulk
- create_field ONLY for single manual field creation

Step 4: Return clear result
- Report what was created/modified
- Include any relevant IDs or slugs
- Report any errors clearly

CONTEXT:
project-id: %s
environment-id: %s
x-api-key: %s
main-menu-id: %s

TASK:
Analyze the user request above and execute the appropriate MCP tool calls to fulfill it.
Return a clear, concise message about what was done.
If any operation fails, report the error clearly.

Execute now.`,
			request.UserPrompt,
			request.ProjectId, request.EnvironmentId, request.APIKey,
			config.MainMenuID, request.APIKey, config.MainMenuID, request.APIKey,
			request.APIKey,
			request.APIKey,
			request.APIKey,
			config.MainMenuID,
			request.ProjectId, request.EnvironmentId, request.APIKey, config.MainMenuID,
		)

		message = "Backend operation completed successfully."

	default:
		message = fmt.Sprintf("method not implemented: %s", request.Method)
		err = errors.New(message)
	}

	return content, message, err
}
