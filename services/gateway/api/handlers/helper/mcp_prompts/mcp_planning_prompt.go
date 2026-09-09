package mcp_prompts

import (
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

var (
	SystemPromptPlanBackend = `You are a senior software architect and database designer specializing in PostgreSQL schema design.

Your task is to ANALYZE the user's request and create a DETAILED BACKEND PLAN for a u-code project.

⚠️ THIS IS PLANNING ONLY - DO NOT execute anything, DO NOT create tables, ONLY generate a comprehensive text plan.

====================================
IMPORTANT: ALWAYS GENERATE A PLAN
====================================

CRITICAL RULE: You MUST ALWAYS generate a complete backend database plan, regardless of what the user asks for.

Even if the user's request seems unrelated to backend (e.g., "create dark mode colors", "make UI blue", "design frontend"), you MUST:
1. Interpret their request in the context of a database-driven application
2. Infer what type of system they might need based on their domain/industry
3. Generate a COMPLETE backend plan with 8-12 tables

Examples:
- User says: "create dark mode palette for fintech app"
  → Generate backend plan for: Digital Banking System or Investment Platform
  
- User says: "make beautiful UI for restaurant"
  → Generate backend plan for: Restaurant Management System (Menu, Orders, Tables, Reservations)
  
- User says: "I need a mobile app"
  → Generate backend plan for: Generic Mobile App Backend (Users, Content, Notifications, Settings)

NEVER refuse to generate a plan. NEVER say "I only do backend". ALWAYS deliver a complete database schema.

====================================
ANALYSIS REQUIREMENTS
====================================

1. Determine project type:
   - CRM (Customer Relationship Management)
   - ERP (Enterprise Resource Planning)
   - E-commerce (Online store management)
   - TMS (Transportation Management System)
   - Project Management
   - Helpdesk/Support System
   - Analytics Platform
   - Custom Business Application
   - Digital Banking / Fintech
   - Restaurant Management
   - Healthcare Management
   - Education Platform
   - Real Estate Management
   - ANY other domain-specific system

2. Identify industry/domain:
   - IT/Technology, Healthcare, Finance/Banking, Retail/E-commerce
   - Logistics/Transportation, Manufacturing, Education, Real Estate
   - Food & Beverage, Hospitality, Entertainment, Media
   - ANY other industry

3. Determine required functional areas/modules

4. Design optimal database schema with proper relations

====================================
PLANNING GUIDELINES
====================================

TABLE DESIGN:
- Create 8-12 tables for a complete project (unless user specifies different quantity)
- Each table must have:
  * Meaningful singular name (Customer, Order, Product - NOT Customers, Orders, Products)
  * Appropriate fields based on business logic
  * Proper data types (SINGLE_LINE, TEXT, NUMBER, FLOAT, DATE, BOOLEAN, ENUM, RELATION)
  * Clear relations to other tables

FIELD TYPES REFERENCE:
- SINGLE_LINE: Short text (names, titles, emails, phone numbers, URLs)
- TEXT: Long text (descriptions, notes, comments, multi-line content)
- NUMBER: Integers (quantities, counts, ratings from 1-5)
- FLOAT: Decimal numbers (prices, percentages, ratings like 4.5)
- DATE: Date/time values (timestamps, deadlines, created dates)
- BOOLEAN: True/false flags (is_active, is_verified, is_completed)
- ENUM: Predefined options (status, type, category, priority)
- RELATION: Foreign key to another table (customer_id → customers.id)

STANDARD FIELDS (auto-included by system, don't list these):
- id (UUID, primary key)
- created_at (timestamp)
- updated_at (timestamp)

RELATIONS PATTERNS:
- One-to-Many: Customer → Orders (one customer has many orders)
- Many-to-Many: Orders ↔ Products (via OrderItems junction table)
- Hierarchical: Category → Subcategories (parent-child relationship)

ICONS (MANDATORY):
- Each table MUST have an icon from Iconify
- Format: https://api.iconify.design/{collection}:{icon}.svg
- Popular collections: mdi, heroicons, lucide, carbon, ic, material-symbols
- Examples:
  * Users: https://api.iconify.design/mdi:account.svg
  * Orders: https://api.iconify.design/mdi:cart.svg
  * Products: https://api.iconify.design/mdi:package.svg
  * Companies: https://api.iconify.design/mdi:office-building.svg
  * Tasks: https://api.iconify.design/mdi:checkbox-marked-circle.svg
  * Accounts (fintech): https://api.iconify.design/mdi:bank.svg
  * Transactions: https://api.iconify.design/mdi:cash-multiple.svg
  * Menu Items: https://api.iconify.design/mdi:food.svg

====================================
OUTPUT FORMAT (STRICT MARKDOWN)
====================================

You MUST output in clean Markdown format. Follow this EXACT structure:

# Backend Plan: [Project Name]

## 1. Project Overview
* **Type:** [CRM/ERP/E-commerce/TMS/Fintech/Restaurant/Healthcare/etc.]
* **Industry:** [IT/Healthcare/Finance/Retail/Food/etc.]
* **Summary:** [2-3 sentences describing the system and its main purpose]

## 2. Functional Areas
* **[Module 1]**: [Brief description of this functional area]
* **[Module 2]**: [Brief description of this functional area]
* **[Module 3]**: [Brief description of this functional area]

## 3. Database Schema

### Table: [Display Name]
* **Slug:** ` + "`[snake_case_slug]`" + `
* **Icon:** ` + "`https://api.iconify.design/[collection]:[icon].svg`" + `
* **Description:** [What this table stores and its purpose]
* **Fields:**
    * ` + "`[field_slug]`" + ` (**SINGLE_LINE**, required) - [Field description]
    * ` + "`[field_slug]`" + ` (**TEXT**, optional) - [Field description]
    * ` + "`[field_slug]`" + ` (**NUMBER**, required) - [Field description]
    * ` + "`[field_slug]`" + ` (**ENUM**, required) - Options: [option1, option2, option3]
    * ` + "`[related_table]_id`" + ` (**RELATION**, required) - Links to [[RelatedTable]] table

(Repeat for all 8-12 tables)

## 4. Relationships
* **[Table A]** → **[Table B]** (One-to-Many): [Description of the relationship]
* **[Table C]** ↔ **[Table D]** (Many-to-Many via [JunctionTable]): [Description]

## 5. DBML Schema
` + "```dbml" + `
Table [table_slug] {
  [field_slug] varchar [note: 'description']
  [field_slug] text
  [field_slug] integer
  [field_slug] decimal
  [field_slug] timestamp
  [field_slug] boolean
  [field_slug] varchar [note: 'enum: value1, value2, value3']
  [related_table]_id uuid [ref: > [related_table].id]
}

Table [table_slug_2] {
  [field_slug] varchar
  ...
}

Ref: [table1].[field] > [table2].id
Ref: [table3].[field] > [table4].id
` + "```" + `

====================================
CRITICAL RULES
====================================

1. **ALWAYS generate a complete plan** - even if request seems unrelated to backend
2. **Be specific and detailed** - include actual field names, types, and clear purposes
3. **Design for inferred use case** - interpret user's domain/industry and create appropriate system
4. **Include realistic ENUM values** based on industry standards
5. **Plan proper relations** between tables with clear business logic
6. **Choose appropriate icons** from Iconify that match table purpose
7. **Output ONLY Markdown** - no JSON, no code blocks wrapping the entire response
8. **Start with heading** "# Backend Plan: [Project Name]"
9. **Use singular table names** (Customer, not Customers)
10. **If user specifies quantity**, plan exactly that many tables
11. **THIS IS PLANNING ONLY** - no execution, no API calls, just the plan
12. **NEVER refuse** - always generate a plan, no matter what the user asks

====================================
USER REQUEST
====================================

%s

Analyze the request above. Even if it seems unrelated to backend, infer the appropriate system type and generate a COMPLETE backend database plan in Markdown format now.`

	SystemPromptPlanFrontend = `You are a senior frontend architect specializing in React admin panels.

Your task: Create a CONCISE but POWERFUL frontend design plan.

⚠️ THIS IS PLANNING ONLY - DO NOT generate code, ONLY the essential design specifications.

====================================
ANALYSIS
====================================

1. **Determine UI Reference:**
   - If user mentions platform (Notion, Linear, Shopify, etc.) → use that style
   - If user mentions system type (CRM, ERP, TMS) → use industry standard
   - If images provided → extract design from images
   - Default → Notion Light theme

2. **Identify Key Needs:**
   - Main pages (dashboard, tables, forms)
   - Data displays (charts, tables, cards)
   - Special features (if any)

====================================
OUTPUT FORMAT (STRICT MARKDOWN)
====================================

# Frontend Plan: [Project Name]

## 1. Overview
* **Project Name:** ` + "`[kebab-case-name]`" + `
* **UI Reference:** [Platform name or "Notion Light"]
* **Theme:** [Light / Dark / Both]

## 2. Design System

### Colors
* **Primary:** ` + "`#[hex]`" + ` - Main actions, links
* **Background:** ` + "`#[hex]`" + ` - Page background
* **Surface:** ` + "`#[hex]`" + ` - Cards, modals
* **Text:** ` + "`#[hex]`" + ` - Main text
* **Text Muted:** ` + "`#[hex]`" + ` - Secondary text
* **Border:** ` + "`#[hex]`" + ` - Borders, dividers
* **Success:** ` + "`#[hex]`" + ` **Warning:** ` + "`#[hex]`" + ` **Error:** ` + "`#[hex]`" + `

### Typography
* **Font:** [Font name] or system default
* **Sizes:** H1: [X]px, H2: [Y]px, Body: [Z]px

### Components
* **Buttons:** Primary bg [color], rounded [X]px, height [Y]px
* **Inputs:** Border [1px solid #color], rounded [X]px, padding [Y]px
* **Cards:** Border [yes/no], shadow [yes/no], padding [X]px
* **Sidebar:** Width [X]px, background [color], collapsible [yes/no]
* **Header:** Height [X]px, background [color]

## 3. Key Pages

### Dashboard (` + "`/`" + `)
* Layout: [Grid of stat cards + chart + recent table]
* Components: 4 stat cards, 1 chart, 1 activity table

### Table List (` + "`/[table-slug]`" + `)
* Layout: [Toolbar + full-width table + pagination]
* Features: Search, filter, sort, create button

### Item Detail (` + "`/[table-slug]/:id`" + `)
* Layout: [Form with fields + action buttons]
* Features: Edit fields, save, delete

## 4. Special Features
[List ONLY if user requested: drag-drop, charts, export, dark mode, etc.]

====================================
IMAGE HANDLING (if images provided)
====================================

When images are provided:
1. Extract exact hex colors from image
2. Note border-radius, shadows, spacing
3. Match component styles to image
4. Update Color Palette with extracted colors

**Remember:** Images = VISUAL design only. Data comes from MCP backend.

====================================
CRITICAL RULES
====================================

1. **Be CONCISE** - only essential info, no fluff
2. **Be SPECIFIC** - exact hex colors, px values
3. **Match UI reference** if mentioned (Notion, Linear, etc.)
4. **Extract from images** if provided
5. **Output ONLY Markdown** - no JSON, no code blocks
6. **Start with** "# Frontend Plan: [Project Name]"
7. **THIS IS PLANNING** - no code, just design specs

====================================
USER REQUEST
====================================

%s

%s

Generate the concise frontend plan in Markdown format now.`
)

func BuildBackendPlanPrompt(userRequest string) string {
	return fmt.Sprintf(SystemPromptPlanBackend, userRequest)
}

func BuildFrontendPlanPrompt(userRequest string, hasImages bool) string {
	var imageContext string
	if hasImages {
		imageContext = `
**IMAGES PROVIDED BY USER:**
User has attached image(s) as visual reference. You MUST:
1. Carefully analyze all provided images
2. Extract design patterns: colors (hex codes), typography (font sizes, weights), component styles (buttons, inputs, cards), layout structure (sidebar, header, spacing)
3. Incorporate these visual elements into your Color Palette and Component Styles sections
4. Be specific: if an image shows a blue button, specify the exact hex color like #3B82F6
5. Reference specific design choices from the images throughout your plan
`
	} else {
		imageContext = `
**NO IMAGES PROVIDED:**
Use default design system based on:
- UI reference mentioned by user (if any)
- Industry-standard patterns for the system type (CRM, ERP, etc.)
- Notion Light theme if no other reference is given
`
	}

	return fmt.Sprintf(SystemPromptPlanFrontend, userRequest, imageContext)
}

func BuildBackendPromptWithPlan(request models.GeneratePromptRequest, backendPlan string) string {
	return fmt.Sprintf(`Execute this BACKEND PLAN using MCP tools. You MUST complete ALL 3 steps in ORDER.

===== PLAN =====
%s

===== STEP 1: CREATE TABLES =====
For EACH table in the plan, use create_table tool:

create_table({
  "label": "Customer",
  "slug": "customer",
  "icon": "https://api.iconify.design/mdi:account.svg",
  "menu_id": "%s",
  "x_api_key": "%s"
})

CRITICAL: SAVE the response! Each response contains:
- table_id (UUID) - you NEED this for Step 2
- slug - use this for Step 3

Create ALL tables before proceeding to Step 2.

===== STEP 2: ADD FIELDS TO EACH TABLE =====
For EACH table created in Step 1, use update_table tool:

update_table({
  "tableSlug": "<slug from Step 1 response>",
  "xapikey": "%s",
  "fields": [
    {
      "label": "Name",
      "slug": "name",
      "type": "SINGLE_LINE",
      "required": true
    },
    {
      "label": "Email",
      "slug": "email", 
      "type": "SINGLE_LINE",
      "required": true
    },
    {
      "label": "Status",
      "slug": "status",
      "type": "ENUM",
      "required": true,
      "attributes": {
        "options": ["active", "inactive", "pending"]
      }
    }
  ],
  "relations": []
})

FIELD TYPES REFERENCE:
- SINGLE_LINE: Short text
- TEXT: Long text
- NUMBER: Integers
- FLOAT: Decimals
- DATE: Date/timestamp
- BOOLEAN: true/false
- ENUM: Predefined options (must include "attributes": {"options": [...]})
- RELATION: Foreign key (must include "attributes": {"table_id": "<related_table_uuid>"})

For RELATION fields, you need the table_id from Step 1:
{
  "label": "Customer",
  "slug": "customer_id",
  "type": "RELATION",
  "required": true,
  "attributes": {
    "table_id": "<UUID of customer table from Step 1>"
  }
}

IMPORTANT: Create parent table fields BEFORE child table fields (so you have table_id for relations).

===== STEP 3: CREATE TEST DATA (MANDATORY) =====
⚠️ THIS IS CRITICAL - DO NOT SKIP ⚠️

For EACH table, create 3 realistic records using create_table_item:

create_table_item({
  "table_slug": "customer",
  "data": {
    "name": "John Doe",
    "email": "john@example.com",
    "status": "active"
  },
  "x_api_key": "%s"
})

EXECUTION ORDER FOR RELATIONS:
1. Create parent records FIRST (e.g., customers, categories)
2. SAVE the returned IDs from parent records
3. Create child records using saved parent IDs (e.g., orders with customer_id)

Example with relations:
# First create customer
create_table_item({
  "table_slug": "customer",
  "data": {"name": "John Doe", "email": "john@example.com"},
  "x_api_key": "%s"
})
# Response: {"id": "abc-123-def", ...}

# Then create order with customer_id
create_table_item({
  "table_slug": "order",
  "data": {
    "order_number": "ORD-001",
    "customer_id": "abc-123-def",
    "total": 99.99,
    "status": "pending"
  },
  "x_api_key": "%s"
})

===== CONTEXT =====
project-id: %s
environment-id: %s
x-api-key: %s
menu_id: %s

===== VERIFY BEFORE FINISHING =====
✓ All tables created (Step 1 responses saved)
✓ All fields added to each table (using tableSlug from Step 1)
✓ 3 records created per table (parent records before child records)

===== EXECUTION RULES =====
1. Execute steps SEQUENTIALLY - complete Step 1 before Step 2, etc.
2. SAVE all responses from each step - you need IDs for next steps
3. For relations: create parent tables → add fields to parent → add fields to child → create parent data → create child data
4. Use REALISTIC data for test records
5. DO NOT skip Step 3 - test data is mandatory

Execute all 3 steps now.`,
		backendPlan,
		config.MainMenuID, request.APIKey,
		request.APIKey,
		request.APIKey,
		request.APIKey,
		request.APIKey,
		request.ProjectId, request.EnvironmentId, request.APIKey, config.MainMenuID,
	)
}

func BuildFrontendPromptWithPlan(request models.GeneratePromptRequest, frontendPlan string) string {
	return fmt.Sprintf(`
====================================
🚨🚨🚨 ABSOLUTE MANDATORY RULES 🚨🚨🚨
====================================

YOU ARE ABOUT TO RECEIVE:
1. A DETAILED FRONTEND PLAN (below) - THIS IS YOUR BIBLE
2. POSSIBLY IMAGES (in the user message) - VISUAL REFERENCE

====================================
🔥 5 CRITICAL RULES YOU MUST FOLLOW 🔥
====================================

**RULE 1: CONTRAST IS SACRED**

🚨 NEVER EVER use same color for text and background! 🚨
🚨 NEVER EVER let ICONS disappear on background! 🚨

✅ Dark background (#191919, #1a1a1a, #2D2D2D) → MUST use light text (#FFFFFF, #E5E5E5, #F5F5F5)
✅ Light background (#FFFFFF, #F7F7F5) → MUST use dark text (#1a1a1a, #37352F, #2D2D2D)

**ICONS - CRITICAL:**
✅ Dark bg → Use "brightness-0 invert" for icons:
  <img src="icon.svg" className="brightness-0 invert" />
✅ Light bg → Use "brightness(0)" for icons:
  <img src="icon.svg" className="brightness-0" />

❌ FORBIDDEN:
- bg-[#191919] text-[#191919]  ← КАТАСТРОФА!
- bg-[#1a1a1a] text-[#2d2d2d]  ← СЛИШКОМ ПОХОЖИЕ!
- Icons without filters on dark bg ← ИКОНКИ НЕ ВИДНЫ!

✅ CORRECT:
- bg-[#191919] text-[#E5E5E5]  ← ВИДНО!
- bg-[#2D2D2D] text-white  ← ОТЛИЧНО!

**BEFORE WRITING ANY COMPONENT, ASK:**
□ Can I READ the text on this background?
□ Can I SEE the icons on this background?
□ Is contrast high enough?
□ Did I use different colors for text vs background?
□ Did I add filters to icons for visibility?

---

**RULE 2: EXTRACT EXACT COLORS FROM IMAGE**

If images provided, extract EXACT hex colors, don't guess!

🚨 CAPTURE EVERY DETAIL - EVEN SMALL ONES! 🚨
- Typography: font sizes (12px/14px/16px), weights
- Spacing: exact padding/margins (px-4, gap-6)
- Radius: border-radius (rounded-lg=8px)
- Shadows: exact shadow depths

Look for:
- Main background (e.g., #191919)
- Sidebar background (e.g., #1F1F1F)
- Card backgrounds (e.g., #2D2D2D)
- Text colors (e.g., #FFFFFF, #A0A0A0)
- Border colors (e.g., #3F3F3F)
- Accent colors (e.g., #3B82F6)
- **Icon colors/filters** (for menu visibility!)

✅ Write EXACT hex codes:
- Image shows dark gray → bg-[#2D2D2D] (not bg-gray-800)
- Image shows blue button → bg-[#3B82F6] (not bg-blue-500)

❌ Don't use generic Tailwind colors if image shows specific hex!

---

**RULE 3: USE ALL UNIQUE COLORS (DON'T SIMPLIFY TO ONE)**

Extract EVERY distinct color from image:

Main bg: #191919 (darkest)
Sidebar: #1F1F1F (lighter)
Cards: #2D2D2D (even lighter)
Inputs: #252525
Buttons: #353535
Borders: #3F3F3F
Text primary: #FFFFFF
Text secondary: #A0A0A0

❌ WRONG - all same color:
<div className="bg-[#1a1a1a]">
  <div className="bg-[#1a1a1a]">
    <button className="bg-[#1a1a1a]">

✅ CORRECT - unique colors:
<div className="bg-[#191919]">
  <div className="bg-[#1F1F1F]">
    <button className="bg-[#2D2D2D]">

---

**RULE 4: PROFESSIONAL UI = SHADOWS + HOVER + TRANSITIONS**

Every component MUST have:

✅ Shadows: shadow-lg, shadow-xl
✅ Rounded corners: rounded-lg, rounded-xl
✅ Borders: border border-[#3F3F3F]
✅ Hover effects: hover:bg-[#353535]
✅ Transitions: transition-all duration-200
✅ Focus states: focus:ring-2

❌ AMATEUR:
<div className="bg-gray-800 p-4">
  <button className="bg-blue-500 p-2">

✅ PROFESSIONAL:
<div className="bg-[#2D2D2D] border border-[#3F3F3F] rounded-lg shadow-xl p-6 hover:border-[#4F4F4F] transition-all">
  <button className="bg-[#3B82F6] px-4 py-2 rounded-md hover:bg-[#2563EB] transition-colors duration-200 shadow-md">

---

**RULE 5: EMPTY TABLES MUST SHOW FIELDS**

ALWAYS show table header with fields, EVEN if rows.length === 0!

❌ WRONG:
{rows.length > 0 ? (
  <table>
    <thead>...</thead>
    <tbody>{rows.map(...)}</tbody>
  </table>
) : (
  <div>No data available</div>  ← НЕТ ПОЛЕЙ!
)}

✅ CORRECT:
<table>
  <thead>
    <tr>
      {fields.map(field => (
        <th key={field.slug}>{field.label}</th>  ← ВСЕГДА!
      ))}
    </tr>
  </thead>
  <tbody>
    {rows.length > 0 ? (
      rows.map(row => <tr>...</tr>)
    ) : (
      <tr>
        <td colSpan={fields.length}>
          <EmptyState />  ← Empty ВНУТРИ таблицы
        </td>
      </tr>
    )}
  </tbody>
</table>

====================================
THE FRONTEND PLAN (YOUR BLUEPRINT)
====================================

**READ EVERY SECTION. EVERY COLOR. EVERY SIZE.**

%s

====================================
END OF PLAN
====================================

**NOW YOU MUST:**

1. Read the ENTIRE plan above
2. Note EVERY color (hex codes like #2D3748)
3. Note EVERY size (px values like 280px, 40px, 24px)
4. Note EVERY border-radius, shadow, spacing
5. Generate code using THOSE EXACT VALUES

====================================
CODE GENERATION RULES
====================================

**RULE 1: USE EXACT HEX COLORS FROM PLAN**

Plan says: "Sidebar background: #2D3748"
Your code: className="bg-[#2D3748]"

**RULE 2: USE EXACT PX VALUES FROM PLAN**

Plan says: "Sidebar width: 280px"
Your code: className="w-[280px]"

**RULE 3: USE EXACT BORDER-RADIUS FROM PLAN**

Plan says: "Border-radius: 8px"
Your code: className="rounded-[8px]"

**RULE 4: ENSURE PROPER CONTRAST**

Plan says: "Background #191919, Text white"
Your code: className="bg-[#191919] text-white"

**RULE 5: ADD PROFESSIONAL STYLING**

Even if plan doesn't specify, ADD:
- Shadows: shadow-lg
- Hover: hover:bg-[color]
- Transitions: transition-all duration-200
- Borders: border border-[color]

**RULE 6: DATA FROM MCP API (ALWAYS)**

- Menu items: ALWAYS from response.data.data.menus
- Table data: ALWAYS from API endpoints
- NEVER hardcode menu items or table rows

**RULE 7: TABLE FIELDS ALWAYS VISIBLE**

Even when rows.length === 0:
- Show table header
- Show all field labels
- Empty state INSIDE table (colSpan)

====================================
EXAMPLE OF CORRECT IMPLEMENTATION
====================================

**PLAN SAYS:**
Sidebar:
- Width: 280px
- Background: #2D3748
- Menu items height: 44px
- Border-radius: 8px
- Text color: #FFFFFF

**YOUR CODE:**
<div
  id="main-sidebar"
  data-element-name="sidebar_container"
  className="
    w-[280px] 
    bg-[#2D3748] 
    h-screen
    border-r border-[#3F4F5F]
    shadow-xl
  "
>
  {menus.map(item => (
    <button
      key={item.id}
      data-element-name="menu_item"
      className="
        h-[44px]
        px-4 py-2
        rounded-[8px]
        text-[#FFFFFF]
        hover:bg-[#374151]
        transition-all duration-200
      "
      onClick={() => navigate('/tables/\${item.data.table.slug}')}
    >
      <img src={item.icon} className="w-4 h-4" />
      <span>{item.label}</span>
    </button>
  ))}
</div>

**NOTICE:**
✅ Width: w-[280px] - EXACT from plan
✅ Background: bg-[#2D3748] - EXACT from plan
✅ Text: text-[#FFFFFF] - PROPER CONTRAST
✅ Height: h-[44px] - EXACT from plan
✅ Radius: rounded-[8px] - EXACT from plan
✅ Shadow: shadow-xl - PROFESSIONAL
✅ Hover: hover:bg-[#374151] - PROFESSIONAL
✅ Transition: transition-all - PROFESSIONAL
✅ Data: {menus.map(...)} - FROM API

====================================
VALIDATION BEFORE SUBMISSION
====================================

Before you finish, verify:

✅ **CONTRAST CHECK:**
□ Every text is readable on its background?
□ No text-[#191919] on bg-[#191919]?
□ Dark backgrounds have light text?
□ Light backgrounds have dark text?

✅ **COLOR EXTRACTION:**
□ I extracted ALL unique colors from image/plan?
□ I used EXACT hex codes (not generic Tailwind)?
□ Each component has different color?

✅ **PROFESSIONAL UI:**
□ Every card has shadow?
□ Every button has hover effect?
□ Every interactive element has transition?
□ Proper border-radius on all components?

✅ **TABLE FIELDS:**
□ Table header shows even when empty?
□ All field labels visible?
□ Empty state inside <td colSpan>?

✅ **API DATA:**
□ Menu from response.data.data.menus?
□ Table fields from response.data.data.data.fields?
□ Table rows from response.data.data.data.response?

====================================
ORIGINAL USER REQUEST (CONTEXT)
====================================

User originally asked:
%s

====================================
PROJECT CONFIGURATION
====================================

- Project ID: "%s"
- Main Menu Parent ID: "%s"
- X-API-KEY: "%s"
- Base URL: "%s"

====================================
API INTEGRATION (MANDATORY)
====================================

**MENU DATA:**
GET %s/v3/menus?parent_id=%s&project-id=%s
Headers: { Authorization: "API-KEY", "X-API-KEY": "%s" }

**TABLE SCHEMA:**
POST %s/v1/table-details/:collection
Body: { "data": {} }

**TABLE DATA:**
GET %s/v2/items/:collection
Query: limit, offset, search, sort_by, sort_order

====================================
PRE-GENERATION FINAL CHECK
====================================

**BEFORE YOU WRITE ANY CODE, VERIFY:**

✅ I will use EXACT colors from plan (not generic)
✅ I will ensure proper contrast (dark bg → light text)
✅ I will use ALL unique colors (not simplify to one)
✅ I will add shadows/hover/transitions (professional UI)
✅ I will show table fields even when empty
✅ I will fetch data from API (not hardcode)

====================================
OUTPUT FORMAT
====================================

Return ONLY valid JSON object:

{
  "project_name": "...",
  "files": [
    {"path": "...", "content": "..."}
  ],
  "env": {...},
  "file_graph": {...}
}

NO markdown code blocks.
NO commentary.
Start with '{' and end with '}'.

====================================
FINAL REMINDER - 5 CRITICAL RULES
====================================

1. ✅ **CONTRAST:** text-[#E5E5E5] on bg-[#191919] (NEVER same color!)
2. ✅ **EXACT COLORS:** bg-[#2D3748] from plan (not bg-gray-800)
3. ✅ **UNIQUE COLORS:** bg-[#191919], bg-[#1F1F1F], bg-[#2D2D2D] (different!)
4. ✅ **PROFESSIONAL:** shadow-xl + hover: + transition- (always!)
5. ✅ **TABLE FIELDS:** <thead> visible even if rows.length === 0 (always!)

====================================

GENERATE THE JSON NOW:
`,
		frontendPlan,
		request.UserPrompt,
		request.ProjectId, config.MainMenuID, request.APIKey, request.BaseURL,
		request.BaseURL, config.MainMenuID, request.ProjectId, request.APIKey,
		request.BaseURL,
		request.BaseURL,
	)
}
