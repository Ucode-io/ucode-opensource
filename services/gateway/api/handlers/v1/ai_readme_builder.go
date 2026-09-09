package v1

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// buildReadmeFile generates a comprehensive README.md for the project repo.
// It is injected into generated.Project.Files before publishing to GitLab.
// The README is written for both humans and AI agents — detailed enough that
// an AI can safely read it and modify the project without breaking anything.
func buildReadmeFile(plan *models.ArchitectPlan, files []models.ProjectFile) models.ProjectFile {
	return models.ProjectFile{
		Path:    "README.md",
		Content: buildReadmeContent(plan, files),
	}
}

func buildReadmeContent(plan *models.ArchitectPlan, files []models.ProjectFile) string {
	var sb strings.Builder

	// ── Header ──────────────────────────────────────────────────────────────────

	fmt.Fprintf(&sb, "# %s\n\n", plan.ProjectName)

	tagline := readmeTagline(plan)
	if tagline != "" {
		fmt.Fprintf(&sb, "> %s\n\n", tagline)
	}

	sb.WriteString("![Built with AI](https://img.shields.io/badge/Built%20with-AI-6366f1?style=flat-square) ")
	sb.WriteString("![React](https://img.shields.io/badge/React-18-61dafb?style=flat-square&logo=react) ")
	sb.WriteString("![TypeScript](https://img.shields.io/badge/TypeScript-5-3178c6?style=flat-square&logo=typescript) ")
	sb.WriteString("![Tailwind](https://img.shields.io/badge/Tailwind-3-06b6d4?style=flat-square&logo=tailwindcss) ")
	if plan.ProjectType == mobileProjectType {
		fmt.Fprintf(&sb, "![Capacitor](https://img.shields.io/badge/Capacitor-%s-119eff?style=flat-square)\n\n", capacitorRuntimeVersion)
	} else {
		sb.WriteString("\n\n")
	}
	sb.WriteString("---\n\n")

	// ── Overview ─────────────────────────────────────────────────────────────────

	sb.WriteString("## Overview\n\n")
	sb.WriteString(readmeOverview(plan))
	sb.WriteString("\n\n")

	// ── Architecture ─────────────────────────────────────────────────────────────

	sb.WriteString("## Architecture\n\n")
	sb.WriteString(readmeArchitecture(plan))
	sb.WriteString("\n")

	// ── Quick Start ───────────────────────────────────────────────────────────────

	sb.WriteString("## Quick Start\n\n")
	sb.WriteString("```bash\nnpm install\nnpm run dev\n```\n\n")
	sb.WriteString("App runs at **http://localhost:5173**\n\n")
	if plan.ProjectType == mobileProjectType {
		sb.WriteString("Create and synchronize native projects from a trusted build environment with Node.js 22+:\n\n")
		sb.WriteString("```bash\nnpm run cap:add:android\nnpm run cap:add:ios\nnpm run cap:sync\n```\n\n")
	}

	// ── Environment Variables ─────────────────────────────────────────────────────

	sb.WriteString("## Environment Variables\n\n")
	sb.WriteString("Create a `.env` file in the project root:\n\n")
	sb.WriteString("```env\n")
	sb.WriteString(readmeEnvVars(plan))
	sb.WriteString("```\n\n")

	// ── Tech Stack ────────────────────────────────────────────────────────────────

	sb.WriteString("## Tech Stack\n\n")
	sb.WriteString("| Technology | Version | Purpose |\n|---|---|---|\n")
	sb.WriteString("| React | 18 | UI framework |\n")
	sb.WriteString("| TypeScript | 5.x | Type safety across the entire codebase |\n")
	sb.WriteString("| Tailwind CSS | 3.x | Utility-first styling — no custom CSS files |\n")
	sb.WriteString("| Vite | 5.x | Build tool and dev server |\n")
	sb.WriteString("| React Router | 6.x | Client-side routing with nested routes |\n")
	if plan.ProjectType == mobileProjectType {
		fmt.Fprintf(&sb, "| Capacitor | %s.x | Native iOS/Android container and device APIs |\n", capacitorRuntimeVersion)
	}
	sb.WriteString("| Axios / Fetch | — | API communication layer |\n\n")

	// ── Project Structure ─────────────────────────────────────────────────────────

	sb.WriteString("## Project Structure\n\n")
	sb.WriteString("```\n")
	sb.WriteString(readmeFileTree(files))
	sb.WriteString("```\n\n")

	// ── Routing ───────────────────────────────────────────────────────────────────

	if routeTable := readmeRouteTable(plan, files); routeTable != "" {
		sb.WriteString("## Routing\n\n")
		sb.WriteString(routeTable)
		sb.WriteString("\n")
	}

	// ── File Inventory ────────────────────────────────────────────────────────────

	sb.WriteString("## File Inventory\n\n")
	sb.WriteString(readmeFileInventory(plan, files))

	// ── Database Schema ───────────────────────────────────────────────────────────

	if plan.ProjectType != "landing" && len(plan.Tables) > 0 {
		sb.WriteString("## Database Schema\n\n")
		sb.WriteString(readmeDatabaseSection(plan))
	}

	// ── User Roles ────────────────────────────────────────────────────────────────

	if len(plan.ClientTypes) > 0 {
		sb.WriteString("## User Roles & Access\n\n")
		sb.WriteString("| Role | Permissions |\n|---|---|\n")
		for i, role := range plan.ClientTypes {
			if i == 0 {
				fmt.Fprintf(&sb, "| %s | Full access to all modules |\n", role)
			} else {
				fmt.Fprintf(&sb, "| %s | Standard access — defined per module |\n", role)
			}
		}
		sb.WriteString("\n")
	}

	// ── Design System ─────────────────────────────────────────────────────────────

	if section := readmeDesignSection(plan.Design); section != "" {
		sb.WriteString(section)
	}

	// ── Modification Guide ────────────────────────────────────────────────────────

	sb.WriteString("## Modification Guide\n\n")
	sb.WriteString(readmeModificationGuide(plan))
	sb.WriteString("\n")

	// ── Footer ───────────────────────────────────────────────────────────────────

	sb.WriteString("---\n\n")
	sb.WriteString("*Generated by [u-code AI](https://ucode.io) — production-ready React application*\n")

	return sb.String()
}

// ── Section builders ──────────────────────────────────────────────────────────

func readmeTagline(plan *models.ArchitectPlan) string {
	switch plan.ProjectType {
	case "admin_panel":
		if len(plan.Tables) > 0 {
			entities := make([]string, 0, 3)
			for _, t := range plan.Tables {
				if !t.IsLoginTable {
					entities = append(entities, t.Label)
				}
				if len(entities) == 3 {
					break
				}
			}
			if len(entities) > 0 {
				return fmt.Sprintf("Full-featured admin panel for managing %s", strings.Join(entities, ", "))
			}
		}
		return "Full-featured admin panel with role-based access control"
	case "landing":
		return fmt.Sprintf("Modern, conversion-focused landing page — %s", plan.ProjectName)
	case mobileProjectType:
		return fmt.Sprintf("Installable iOS and Android app powered by Capacitor — %s", plan.ProjectName)
	default:
		return fmt.Sprintf("Modern web application — %s", plan.ProjectName)
	}
}

func readmeOverview(plan *models.ArchitectPlan) string {
	var sb strings.Builder

	switch plan.ProjectType {
	case "admin_panel":
		fmt.Fprintf(&sb, "**%s** is a full-featured admin panel built with React 18 and TypeScript.\n", plan.ProjectName)
		userTables := make([]string, 0, len(plan.Tables))
		for _, t := range plan.Tables {
			if !t.IsLoginTable {
				userTables = append(userTables, t.Label)
			}
		}
		if len(userTables) > 0 {
			fmt.Fprintf(&sb, "It provides complete CRUD management for: **%s**.\n", strings.Join(userTables, "**, **"))
		}
		if len(plan.ClientTypes) > 0 {
			fmt.Fprintf(&sb, "Access is role-based with %d role(s): **%s**.\n",
				len(plan.ClientTypes), strings.Join(plan.ClientTypes, "**, **"))
		}
	case "landing":
		fmt.Fprintf(&sb, "**%s** is a modern landing page designed to present the product and convert visitors.\n", plan.ProjectName)
	case mobileProjectType:
		fmt.Fprintf(&sb, "**%s** is a phone-first React web application packaged for iOS and Android with Capacitor.\n", plan.ProjectName)
	default:
		fmt.Fprintf(&sb, "**%s** is a modern web application built with React 18, TypeScript, and Tailwind CSS.\n", plan.ProjectName)
	}

	if first := firstMeaningfulLine(plan.UIStructure); first != "" {
		sb.WriteString("\n")
		sb.WriteString(first)
	}

	return strings.TrimRight(sb.String(), "\n")
}

func readmeArchitecture(plan *models.ArchitectPlan) string {
	var sb strings.Builder

	switch plan.ProjectType {
	case "landing":
		sb.WriteString("The project follows a section-based structure where each visual block is an isolated component:\n\n")
		sb.WriteString("- **`src/sections/`** — one component per visual section (Hero, Features, Pricing, etc.)\n")
		sb.WriteString("- **`src/components/`** — shared UI elements (Button, Card, Icon, etc.)\n")
		sb.WriteString("- **`src/styles/`** — global CSS and Tailwind configuration\n\n")
		sb.WriteString("Each section is independently composable — you can reorder, remove, or add sections in `src/App.tsx`.\n")
	default:
		sb.WriteString("The project follows a feature-based layered architecture:\n\n")
		sb.WriteString("- **`src/pages/`** — one file per route; handles layout and data orchestration only\n")
		sb.WriteString("- **`src/components/`** — reusable UI, organized by feature (`/ui/`, `/users/`, `/orders/`, etc.)\n")
		sb.WriteString("- **`src/hooks/`** — data fetching and business logic; one hook per resource (e.g. `useUsers`)\n")
		sb.WriteString("- **`src/services/`** — API communication; one service per resource with typed request/response\n")
		sb.WriteString("- **`src/types/`** — TypeScript interfaces for all data models and API responses\n")
		sb.WriteString("- **`src/router/`** — React Router v6 configuration with protected routes\n\n")
		sb.WriteString("**Key conventions:**\n")
		sb.WriteString("- Pages are dumb — they delegate data to hooks, rendering to components\n")
		sb.WriteString("- Hooks own all async logic — no `useEffect` + `fetch` in components\n")
		sb.WriteString("- All Tailwind — zero custom CSS files; design tokens live in `tailwind.config.ts`\n")
		if plan.ProjectType == "admin_panel" {
			sb.WriteString("- Authentication state is managed globally; protected routes redirect to `/login` if unauthenticated\n")
		}
		if plan.ProjectType == mobileProjectType {
			sb.WriteString("- **`capacitor.config.ts`** — native container identity and bundled web output configuration\n")
			sb.WriteString("- **`src/lib/capacitor.ts`** — browser-safe wrappers around approved Capacitor device APIs\n")
			sb.WriteString("- Native `ios/` and `android/` projects are generated by the build worker, not by AI\n")
		}
	}

	return sb.String()
}

func readmeEnvVars(plan *models.ArchitectPlan) string {
	var sb strings.Builder

	sb.WriteString("# Base API URL for the ucode backend\n")
	sb.WriteString("VITE_API_URL=https://your-project.ucode.io/v2\n\n")

	if plan.ProjectType != "landing" {
		sb.WriteString("# ucode project credentials\n")
		sb.WriteString("VITE_PROJECT_ID=your_project_id\n")
		sb.WriteString("VITE_API_KEY=your_api_key\n\n")
	}

	sb.WriteString("# App base path (used by Vite for sub-path deployments)\n")
	sb.WriteString("VITE_BASE_URL=/\n")

	return sb.String()
}

// readmeRouteTable derives the routing table from page files and plan data.
func readmeRouteTable(plan *models.ArchitectPlan, files []models.ProjectFile) string {
	if plan.ProjectType == "landing" {
		return ""
	}

	type route struct {
		path      string
		component string
		access    string
	}

	routes := []route{}

	publicPages := map[string]bool{
		"Login": true, "Auth": true, "Register": true, "Signup": true, "ForgotPassword": true,
	}
	dashboardNames := map[string]bool{
		"Dashboard": true, "Home": true, "Overview": true, "Main": true,
	}

	var hasLogin bool
	var hasDashboard bool

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") || !strings.Contains(f.Path, "/pages/") {
			continue
		}
		base := f.Path[strings.LastIndex(f.Path, "/")+1:]
		name := strings.TrimSuffix(strings.TrimSuffix(base, ".tsx"), "Page")

		if name == "" || name == "index" || name == "Index" || name == "NotFound" || name == "Error" {
			continue
		}

		path := "/" + strings.ToLower(name)
		access := "Authenticated"
		if publicPages[name] {
			path = "/" + strings.ToLower(name)
			access = "Public"
			hasLogin = true
		}
		if dashboardNames[name] {
			path = "/dashboard"
			hasDashboard = true
		}

		routes = append(routes, route{
			path:      path,
			component: name + "Page",
			access:    access,
		})
	}

	if len(routes) == 0 {
		return ""
	}

	// Sort: public first, then dashboard, then alphabetical.
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].access != routes[j].access {
			return routes[i].access == "Public"
		}
		if routes[i].path == "/dashboard" {
			return true
		}
		return routes[i].path < routes[j].path
	})

	var sb strings.Builder
	sb.WriteString("| Path | Component | Access |\n|---|---|---|\n")

	// Redirect root → dashboard if applicable.
	if hasDashboard && hasLogin {
		sb.WriteString("| `/` | → `/login` (redirect) | — |\n")
	} else if hasDashboard {
		sb.WriteString("| `/` | → `/dashboard` (redirect) | — |\n")
	}

	for _, r := range routes {
		fmt.Fprintf(&sb, "| `%s` | `%s` | %s |\n", r.path, r.component, r.access)
	}

	return sb.String()
}

// readmeFileInventory produces a grouped list of all generated files with inferred descriptions.
func readmeFileInventory(plan *models.ArchitectPlan, files []models.ProjectFile) string {
	// Build a slug→label map for description inference.
	slugLabel := make(map[string]string, len(plan.Tables))
	for _, t := range plan.Tables {
		slugLabel[t.Slug] = t.Label
		// Also map lowercase label to label for matching by component name.
		slugLabel[strings.ToLower(t.Label)] = t.Label
		slugLabel[strings.ToLower(strings.ReplaceAll(t.Label, " ", ""))] = t.Label
	}

	type fileEntry struct {
		path string
		desc string
	}

	groups := map[string][]fileEntry{
		"pages":      {},
		"hooks":      {},
		"services":   {},
		"components": {},
		"types":      {},
		"router":     {},
		"utils":      {},
		"other":      {},
	}
	groupOrder := []string{"pages", "hooks", "services", "components", "types", "router", "utils", "other"}

	for _, f := range files {
		entry := fileEntry{path: f.Path, desc: inferFileDescription(f.Path, slugLabel, plan)}

		switch {
		case strings.Contains(f.Path, "/pages/"):
			groups["pages"] = append(groups["pages"], entry)
		case strings.Contains(f.Path, "/hooks/"):
			groups["hooks"] = append(groups["hooks"], entry)
		case strings.Contains(f.Path, "/services/") || strings.Contains(f.Path, "/api/"):
			groups["services"] = append(groups["services"], entry)
		case strings.Contains(f.Path, "/types/") || strings.Contains(f.Path, "/interfaces/"):
			groups["types"] = append(groups["types"], entry)
		case strings.Contains(f.Path, "/router/") || f.Path == "src/router.tsx" || strings.Contains(f.Path, "/routes/"):
			groups["router"] = append(groups["router"], entry)
		case strings.Contains(f.Path, "/utils/") || strings.Contains(f.Path, "/helpers/") || strings.Contains(f.Path, "/lib/"):
			groups["utils"] = append(groups["utils"], entry)
		case strings.Contains(f.Path, "/components/"):
			groups["components"] = append(groups["components"], entry)
		}
	}

	var sb strings.Builder
	groupTitles := map[string]string{
		"pages":      "Pages",
		"hooks":      "Hooks — Data Layer",
		"services":   "Services — API Layer",
		"components": "Components",
		"types":      "Types",
		"router":     "Router",
		"utils":      "Utilities",
		"other":      "Other",
	}

	for _, g := range groupOrder {
		entries := groups[g]
		if len(entries) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "### %s\n\n", groupTitles[g])
		sb.WriteString("| File | Description |\n|---|---|\n")
		for _, e := range entries {
			fmt.Fprintf(&sb, "| `%s` | %s |\n", e.path, e.desc)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// inferFileDescription produces a one-line description for a file based on its path and the project plan.
func inferFileDescription(path string, slugLabel map[string]string, plan *models.ArchitectPlan) string {
	base := path[strings.LastIndex(path, "/")+1:]
	name := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, ".tsx"), ".ts"), ".jsx")

	lower := strings.ToLower(name)

	// Router / App shell.
	switch lower {
	case "app":
		return "App root — renders router and global providers"
	case "main":
		return "Entry point — mounts React app to DOM"
	case "router", "routes", "approuter":
		return "Route definitions — maps URL paths to page components"
	case "layout", "applayout":
		return "Shared layout wrapper — sidebar, header, and content area"
	case "sidebar":
		return "Navigation sidebar with menu items and active-route highlighting"
	case "header", "navbar", "topbar":
		return "Top navigation bar"
	case "footer":
		return "Page footer"
	case "privateRoute", "protectedRoute", "authGuard":
		return "HOC — redirects unauthenticated users to /login"
	}

	// Auth pages.
	if lower == "login" || lower == "loginpage" {
		return "Login page — email/password form, sets auth token on success"
	}
	if lower == "register" || lower == "registerpage" || lower == "signup" || lower == "signuppage" {
		return "Registration page — creates new user account"
	}
	if lower == "dashboard" || lower == "dashboardpage" || lower == "home" || lower == "homepage" {
		return "Dashboard — summary stats and quick-access widgets"
	}

	// Match against known table labels.
	nameLower := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(name, "Page"), "page"))

	// Pages.
	if strings.HasSuffix(name, "Page") || strings.HasSuffix(name, "page") {
		entity := strings.TrimSuffix(strings.TrimSuffix(name, "Page"), "page")
		if label, ok := slugLabel[strings.ToLower(entity)]; ok {
			return fmt.Sprintf("%s management page — list with search, filters, and CRUD actions", label)
		}
		return fmt.Sprintf("%s page", camelCaseToWords(entity))
	}

	// Hooks.
	if strings.HasPrefix(lower, "use") {
		resource := strings.TrimPrefix(name, "use")
		resource = strings.TrimPrefix(resource, "Use")
		if label, ok := slugLabel[strings.ToLower(resource)]; ok {
			return fmt.Sprintf("Data hook for %s — exposes list, create, update, delete with loading/error state", label)
		}
		switch strings.ToLower(resource) {
		case "auth", "session":
			return "Authentication hook — exposes user, login(), logout(), isAuthenticated"
		case "debounce":
			return "Debounce hook — delays value update for search inputs"
		case "localstorage":
			return "Persistent state hook backed by localStorage"
		case "pagination":
			return "Pagination state hook — page, pageSize, total, navigation helpers"
		}
		return fmt.Sprintf("Hook — %s state and logic", camelCaseToWords(resource))
	}

	// Services.
	if strings.HasSuffix(lower, "service") || strings.HasSuffix(lower, "api") {
		resource := strings.TrimSuffix(strings.TrimSuffix(name, "Service"), "Api")
		resource = strings.TrimSuffix(resource, "service")
		resource = strings.TrimSuffix(resource, "api")
		if label, ok := slugLabel[strings.ToLower(resource)]; ok {
			return fmt.Sprintf("API service for %s — getList, getById, create, update, delete", label)
		}
		if strings.ToLower(resource) == "api" || strings.ToLower(resource) == "client" || strings.ToLower(resource) == "http" {
			return "Base HTTP client with auth headers, base URL, and error interceptors"
		}
		return fmt.Sprintf("API service — %s endpoints", camelCaseToWords(resource))
	}

	// Type files.
	if strings.Contains(path, "/types/") || strings.HasSuffix(lower, "types") || strings.HasSuffix(lower, "type") {
		if label, ok := slugLabel[nameLower]; ok {
			return fmt.Sprintf("TypeScript interfaces for %s data model and API responses", label)
		}
		return fmt.Sprintf("TypeScript type definitions — %s", camelCaseToWords(name))
	}

	// Components — try to infer from name patterns.
	for suffix, pattern := range map[string]string{
		"Modal":  "%s create/edit modal dialog",
		"Form":   "%s create/edit form with validation",
		"Table":  "%s data table with pagination and row actions",
		"List":   "%s list view",
		"Card":   "%s card component",
		"Detail": "%s detail/view panel",
		"Filter": "Filter controls for %s list",
		"Row":    "Single row component for %s table",
		"Item":   "Single item component for %s list",
	} {
		if strings.HasSuffix(name, suffix) {
			entity := strings.TrimSuffix(name, suffix)
			if label, ok := slugLabel[strings.ToLower(entity)]; ok {
				return fmt.Sprintf(pattern, label)
			}
		}
	}

	// Generic UI components.
	uiComponents := map[string]string{
		"Button":        "Reusable button with variants (primary, secondary, danger, ghost)",
		"Input":         "Form input with label, error state, and helper text",
		"Select":        "Dropdown select with search support",
		"Modal":         "Generic modal dialog with backdrop and close button",
		"Table":         "Generic data table with sorting and pagination",
		"Badge":         "Status badge component with color variants",
		"Avatar":        "User avatar with fallback initials",
		"Spinner":       "Loading spinner",
		"Toast":         "Toast notification component",
		"Dropdown":      "Dropdown menu component",
		"Pagination":    "Pagination controls with page size selector",
		"SearchInput":   "Debounced search input",
		"EmptyState":    "Empty state placeholder with icon and CTA",
		"ErrorBoundary": "React error boundary — catches render errors",
		"Skeleton":      "Loading skeleton placeholder",
		"Breadcrumb":    "Breadcrumb navigation component",
		"Tooltip":       "Tooltip wrapper with hover trigger",
		"Tabs":          "Tab panel component",
		"Accordion":     "Expandable accordion section",
		"DataTable":     "Feature-rich data table with sorting, filtering, pagination",
		"ConfirmDialog": "Confirmation dialog for destructive actions",
	}
	if desc, ok := uiComponents[name]; ok {
		return desc
	}

	// Tailwind / config files.
	switch base {
	case "tailwind.config.ts", "tailwind.config.js":
		return "Tailwind CSS config — custom colors, fonts, and design tokens"
	case "vite.config.ts":
		return "Vite build config — aliases, plugins, and proxy settings"
	case "tsconfig.json":
		return "TypeScript compiler config"
	case "package.json":
		return "Project dependencies and npm scripts"
	case "index.html":
		return "HTML entry point — Vite root template"
	case ".env.example":
		return "Example environment variables — copy to .env and fill in values"
	}

	// Fallback.
	return camelCaseToWords(name)
}

// readmeDatabaseSection renders a full schema — one subsection per table.
func readmeDatabaseSection(plan *models.ArchitectPlan) string {
	var sb strings.Builder

	slugLabel := make(map[string]string, len(plan.Tables))
	for _, t := range plan.Tables {
		slugLabel[t.Slug] = t.Label
	}

	fmt.Fprintf(&sb, "%d table(s)", len(plan.Tables))
	if len(plan.Relations) > 0 {
		fmt.Fprintf(&sb, " · %d relation(s)", len(plan.Relations))
	}
	sb.WriteString("\n\n")

	// Login table first (auth), then alphabetical.
	printTable := func(t models.TablePlan) {
		if t.IsLoginTable {
			fmt.Fprintf(&sb, "### %s *(authentication)*\n\n", t.Label)
		} else {
			fmt.Fprintf(&sb, "### %s\n\n", t.Label)
		}

		sb.WriteString("| Column | Type | Notes |\n|---|---|---|\n")
		sb.WriteString("| id | UUID | Primary key, auto-generated |\n")

		for _, f := range t.Fields {
			notes := ""
			if t.IsLoginTable && (strings.ToLower(f.Slug) == "login" || strings.ToLower(f.Slug) == "email") {
				notes = "Used for login"
			}
			fmt.Fprintf(&sb, "| %s | %s | %s |\n", f.Label, readableFieldType(f.Type), notes)
		}

		if t.IsLoginTable {
			sb.WriteString("| password | Text | Hashed — never returned in API responses |\n")
		}

		sb.WriteString("| created_at | DateTime | Auto-set on creation |\n")
		sb.WriteString("| updated_at | DateTime | Auto-updated on every write |\n")
		sb.WriteString("\n")
	}

	// Auth table first.
	for _, t := range plan.Tables {
		if t.IsLoginTable {
			printTable(t)
		}
	}
	// Then the rest.
	for _, t := range plan.Tables {
		if !t.IsLoginTable {
			printTable(t)
		}
	}

	// Relations.
	if len(plan.Relations) > 0 {
		sb.WriteString("### Foreign Key Relations\n\n")
		sb.WriteString("| From | To | Type | FK Column |\n|---|---|---|---|\n")
		for _, r := range plan.Relations {
			from := slugLabel[r.TableFrom]
			if from == "" {
				from = r.TableFrom
			}
			to := slugLabel[r.TableTo]
			if to == "" {
				to = r.TableTo
			}
			fkCol := r.TableTo + "_id"
			fmt.Fprintf(&sb, "| %s | %s | Many → One | `%s` on %s |\n", from, to, fkCol, from)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// readableFieldType converts ucode field type slugs to human-readable names.
func readableFieldType(t string) string {
	types := map[string]string{
		"SINGLE_LINE":     "Text",
		"MULTI_LINE":      "Long Text",
		"NUMBER":          "Integer",
		"FLOAT":           "Decimal",
		"EMAIL":           "Email",
		"PHONE":           "Phone",
		"DATE":            "Date",
		"DATE_TIME":       "DateTime",
		"BOOLEAN":         "Boolean",
		"IMAGE":           "Image URL",
		"FILE":            "File URL",
		"PICK_LIST":       "Select (enum)",
		"MULTI_PICK_LIST": "Multi Select",
		"RELATION":        "Foreign Key (UUID)",
		"MULTI_RELATION":  "Foreign Keys (array)",
		"LOOKUP":          "Lookup (computed)",
		"FORMULA":         "Formula (computed)",
		"AUTO_INCREMENT":  "Auto-increment Integer",
		"SLUG":            "Slug (URL-safe string)",
		"PASSWORD":        "Password (hashed)",
		"ICON":            "Icon identifier",
		"COLOR":           "Color (hex/rgb)",
		"MAP":             "Geo coordinates (lat/lng)",
		"BARCODE":         "Barcode string",
		"JSON":            "JSON object",
		"ARRAY":           "Array",
	}
	if readable, ok := types[t]; ok {
		return readable
	}
	return t
}

// readmeDesignSection renders the full design system — color palette, CSS variables,
// Tailwind config snippet, and typography. Enough for an AI agent to write correct styles.
func readmeDesignSection(d models.DesignSpec) string {
	if d.DesignInspiration == "" && d.PrimaryColor == "" && d.FontFamily == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Design System\n\n")

	if d.DesignInspiration != "" {
		fmt.Fprintf(&sb, "Style: **%s**\n\n", slugToTitle(d.DesignInspiration))
	}

	// ── Color palette ─────────────────────────────────────────────────────────

	type colorRow struct {
		name string
		hex  string
		hsl  string
	}
	colors := []colorRow{
		{"Primary", d.PrimaryColor, d.PrimaryHSL},
		{"Accent", d.AccentColor, d.AccentHSL},
		{"Background", d.BackgroundColor, d.BackgroundHSL},
		{"Surface / card", d.SurfaceColor, d.SurfaceHSL},
		{"Sidebar", d.SidebarBackground, d.SidebarBackgroundHSL},
		{"Text", d.TextColor, ""},
		{"Text muted", d.TextMutedColor, ""},
		{"Border", d.BorderColor, ""},
	}

	hasColors := false
	for _, c := range colors {
		if c.hex != "" {
			hasColors = true
			break
		}
	}

	if hasColors {
		sb.WriteString("### Color Palette\n\n")
		sb.WriteString("| Token | Hex | HSL |\n|---|---|---|\n")
		for _, c := range colors {
			if c.hex == "" {
				continue
			}
			hsl := c.hsl
			if hsl == "" {
				hsl = "—"
			}
			fmt.Fprintf(&sb, "| %s | `%s` | `%s` |\n", c.name, c.hex, hsl)
		}
		sb.WriteString("\n")
	}

	// ── CSS custom properties ─────────────────────────────────────────────────
	// These are the variables used in globals.css / index.css.

	hasCSSVars := d.PrimaryHSL != "" || d.BackgroundHSL != "" || d.AccentHSL != ""
	if hasCSSVars {
		sb.WriteString("### CSS Variables\n\n")
		sb.WriteString("Defined in `src/index.css` (or `src/styles/globals.css`).\n")
		sb.WriteString("Use these in Tailwind classes via `hsl(var(--token))`.\n\n")
		sb.WriteString("```css\n:root {\n")
		if d.PrimaryHSL != "" {
			fmt.Fprintf(&sb, "  --primary:    %s;\n", d.PrimaryHSL)
		}
		if d.AccentHSL != "" {
			fmt.Fprintf(&sb, "  --accent:     %s;\n", d.AccentHSL)
		}
		if d.BackgroundHSL != "" {
			fmt.Fprintf(&sb, "  --background: %s;\n", d.BackgroundHSL)
		}
		if d.SurfaceHSL != "" {
			fmt.Fprintf(&sb, "  --surface:    %s;\n", d.SurfaceHSL)
		}
		if d.SidebarBackgroundHSL != "" {
			fmt.Fprintf(&sb, "  --sidebar:    %s;\n", d.SidebarBackgroundHSL)
		}
		if d.BorderRadius != "" {
			fmt.Fprintf(&sb, "  --radius:     %s;\n", d.BorderRadius)
		}
		sb.WriteString("}\n```\n\n")
	}

	// ── Tailwind config snippet ───────────────────────────────────────────────

	sb.WriteString("### Tailwind Config\n\n")
	sb.WriteString("Defined in `tailwind.config.ts` under `theme.extend`.\n\n")
	sb.WriteString("```ts\ntheme: {\n  extend: {\n")

	// Colors
	sb.WriteString("    colors: {\n")
	if d.PrimaryHSL != "" {
		fmt.Fprintf(&sb, "      primary:    'hsl(%s)',\n", d.PrimaryHSL)
	} else if d.PrimaryColor != "" {
		fmt.Fprintf(&sb, "      primary:    '%s',\n", d.PrimaryColor)
	}
	if d.AccentHSL != "" {
		fmt.Fprintf(&sb, "      accent:     'hsl(%s)',\n", d.AccentHSL)
	} else if d.AccentColor != "" {
		fmt.Fprintf(&sb, "      accent:     '%s',\n", d.AccentColor)
	}
	if d.BackgroundHSL != "" {
		fmt.Fprintf(&sb, "      background: 'hsl(%s)',\n", d.BackgroundHSL)
	} else if d.BackgroundColor != "" {
		fmt.Fprintf(&sb, "      background: '%s',\n", d.BackgroundColor)
	}
	if d.SurfaceHSL != "" {
		fmt.Fprintf(&sb, "      surface:    'hsl(%s)',\n", d.SurfaceHSL)
	} else if d.SurfaceColor != "" {
		fmt.Fprintf(&sb, "      surface:    '%s',\n", d.SurfaceColor)
	}
	if d.TextColor != "" {
		fmt.Fprintf(&sb, "      foreground: '%s',\n", d.TextColor)
	}
	if d.TextMutedColor != "" {
		fmt.Fprintf(&sb, "      muted:      '%s',\n", d.TextMutedColor)
	}
	if d.BorderColor != "" {
		fmt.Fprintf(&sb, "      border:     '%s',\n", d.BorderColor)
	}
	sb.WriteString("    },\n")

	// Fonts
	if d.FontFamily != "" || d.BodyFont != "" {
		sb.WriteString("    fontFamily: {\n")
		if d.FontFamily != "" {
			fmt.Fprintf(&sb, "      heading: ['%s', 'sans-serif'],\n", d.FontFamily)
		}
		if d.BodyFont != "" && d.BodyFont != d.FontFamily {
			fmt.Fprintf(&sb, "      body:    ['%s', 'sans-serif'],\n", d.BodyFont)
		} else if d.FontFamily != "" {
			fmt.Fprintf(&sb, "      body:    ['%s', 'sans-serif'],\n", d.FontFamily)
		}
		sb.WriteString("    },\n")
	}

	// Border radius
	if d.BorderRadius != "" {
		sb.WriteString("    borderRadius: {\n")
		fmt.Fprintf(&sb, "      DEFAULT: '%s',\n", d.BorderRadius)
		sb.WriteString("    },\n")
	}

	sb.WriteString("  },\n},\n```\n\n")

	// ── Usage hint ────────────────────────────────────────────────────────────

	sb.WriteString("### Usage in Components\n\n")
	sb.WriteString("```tsx\n")
	sb.WriteString("// ✅ correct — use semantic token names\n")
	sb.WriteString("<div className=\"bg-surface text-foreground border border-border\" />\n")
	sb.WriteString("<button className=\"bg-primary text-white hover:bg-primary/90\" />\n\n")
	sb.WriteString("// ❌ wrong — never hardcode hex colors in components\n")
	sb.WriteString("<div className=\"bg-[#1e293b]\" />\n")
	sb.WriteString("```\n\n")

	return sb.String()
}

// readmeModificationGuide gives step-by-step instructions for common changes.
func readmeModificationGuide(plan *models.ArchitectPlan) string {
	var sb strings.Builder

	switch plan.ProjectType {
	case "landing":
		sb.WriteString("### Adding a New Section\n\n")
		sb.WriteString("1. Create `src/sections/NewSection.tsx`\n")
		sb.WriteString("2. Import and add it to `src/App.tsx` at the desired position\n")
		sb.WriteString("3. Keep all styles as Tailwind classes — do not create separate CSS files\n\n")

		sb.WriteString("### Editing Text / Copy\n\n")
		sb.WriteString("All text content is defined inline in each section component.\n")
		sb.WriteString("Search for the text string across `src/sections/` and edit directly.\n\n")

		sb.WriteString("### Changing Colors\n\n")
		sb.WriteString("1. Update custom color values in `tailwind.config.ts` under `theme.extend.colors`\n")
		sb.WriteString("2. All components use semantic color names (e.g. `bg-primary`, `text-accent`) — updating the config updates the whole app\n\n")

	default:
		sb.WriteString("### Adding a New Page / Module\n\n")
		sb.WriteString("1. Create `src/pages/NewEntityPage.tsx` — layout and data orchestration only\n")
		sb.WriteString("2. Create `src/hooks/useNewEntity.ts` — fetch/mutate logic, exposes `data`, `isLoading`, `error`, and mutation functions\n")
		sb.WriteString("3. Create `src/services/newEntityService.ts` — raw API calls with typed request/response\n")
		sb.WriteString("4. Create `src/types/newEntity.ts` — TypeScript interfaces for the entity\n")
		sb.WriteString("5. Add the route to `src/router/index.tsx` (or equivalent router file)\n")
		sb.WriteString("6. Add a navigation entry in the Sidebar component\n\n")

		sb.WriteString("### Adding a Column to a Table\n\n")
		sb.WriteString("1. Add the field in the ucode admin panel (or via the AI chat)\n")
		sb.WriteString("2. Update the TypeScript interface in `src/types/`\n")
		sb.WriteString("3. Add the column to the `<Table>` component in `src/components/[entity]/`\n")
		sb.WriteString("4. If the field is editable, update the form/modal component as well\n\n")

		sb.WriteString("### Changing a Component Style\n\n")
		sb.WriteString("All styles are Tailwind utility classes — edit them directly in the component file.\n")
		sb.WriteString("Global design tokens (colors, fonts, border-radius) are in `tailwind.config.ts`.\n\n")

		sb.WriteString("### Adding a New User Role\n\n")
		if len(plan.ClientTypes) > 0 {
			sb.WriteString("1. Add the role in the ucode admin panel\n")
			sb.WriteString("2. Update the auth hook to expose the user's role\n")
			sb.WriteString("3. Add conditional rendering / route guards where needed\n\n")
		}

		sb.WriteString("### ⚠️ What NOT to change\n\n")
		sb.WriteString("- Do not modify `package-lock.json` manually — run `npm install` instead\n")
		sb.WriteString("- Do not add inline `<style>` tags or separate `.css` files — use Tailwind classes\n")
		sb.WriteString("- Do not move files between directories without updating all imports\n")
		sb.WriteString("- The `id`, `created_at`, `updated_at` columns are auto-managed by ucode — do not include them in create/update API calls\n")
	}

	return sb.String()
}

// ── File tree ─────────────────────────────────────────────────────────────────

func readmeFileTree(files []models.ProjectFile) string {
	dirCount := make(map[string]int)
	var rootConfigFiles []string

	for _, f := range files {
		idx := strings.Index(f.Path, "src/")
		if idx < 0 {
			base := f.Path[strings.LastIndex(f.Path, "/")+1:]
			rootConfigFiles = append(rootConfigFiles, base)
			continue
		}
		rest := f.Path[idx+4:]
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			continue
		}
		dirCount[rest[:slashIdx]]++
	}

	ordered := []string{"pages", "components", "sections", "blocks", "hooks", "services", "api",
		"store", "context", "router", "routes", "utils", "lib", "helpers", "types", "interfaces", "config", "assets", "styles"}
	shown := make(map[string]bool)

	var sb strings.Builder
	sb.WriteString("src/\n")

	isLast := func(dir string) bool {
		for i := len(ordered) - 1; i >= 0; i-- {
			if shown[ordered[i]] {
				return ordered[i] == dir
			}
		}
		return false
	}
	_ = isLast

	for _, dir := range ordered {
		if cnt, ok := dirCount[dir]; ok {
			shown[dir] = true
			fmt.Fprintf(&sb, "├── %s/\t\t# %d file(s)\n", dir, cnt)
		}
	}

	extras := make([]string, 0)
	for dir := range dirCount {
		if !shown[dir] {
			extras = append(extras, dir)
		}
	}
	sort.Strings(extras)
	for _, dir := range extras {
		fmt.Fprintf(&sb, "├── %s/\t\t# %d file(s)\n", dir, dirCount[dir])
	}

	// Config files at root.
	sort.Strings(rootConfigFiles)
	seen := make(map[string]bool)
	for _, f := range rootConfigFiles {
		if !seen[f] && isConfigFile(f) {
			seen[f] = true
			sb.WriteString(f + "\n")
		}
	}

	return sb.String()
}

func isConfigFile(name string) bool {
	configs := map[string]bool{
		"package.json": true, "vite.config.ts": true, "tsconfig.json": true,
		"tailwind.config.ts": true, "tailwind.config.js": true,
		"postcss.config.js": true, "index.html": true, ".env.example": true,
	}
	return configs[name]
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func firstMeaningfulLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		for _, pfx := range []string{"- ", "• ", "* ", "· "} {
			line = strings.TrimPrefix(line, pfx)
		}
		line = strings.Trim(line, "*_`")
		low := strings.ToLower(line)
		if len(line) > 15 &&
			!strings.HasPrefix(low, "navbar") &&
			!strings.HasPrefix(low, "footer") &&
			!strings.HasPrefix(low, "sidebar") {
			return line + "."
		}
	}
	return ""
}
