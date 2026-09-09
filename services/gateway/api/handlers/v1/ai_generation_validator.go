package v1

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// ============================================================================
// POST-GENERATION IMPORT/EXPORT VALIDATOR
//
// Scans all generated files after merge and detects:
// 1. Imports from files that don't exist in the output
// 2. Named imports that aren't exported by the target file
// 3. Env variables referenced in code but missing from .env
// 4. JSX/TSX syntax errors — unbalanced braces/brackets/parens
//
// This catches every class of error we've encountered:
// - Missing exports (Badge.tsx lost QuoteStatusBadge)
// - API renames (TabList → TabsList)
// - Missing files (import from non-existent path)
// - Esbuild "Expected > but found }" crashes (brace mismatch in JSX)
// ============================================================================

// Compiled regexps for import/export parsing — built once at startup.
var (
	// Matches: import { X, Y, Z } from '@/path' or './path' or '../path'
	reImportNamed = regexp.MustCompile(`import\s*\{([^}]+)\}\s*from\s*['"]([^'"]+)['"]`)

	// Matches: import X from '@/path' (default import, PascalCase)
	reImportDefault = regexp.MustCompile(`import\s+([A-Z]\w+)\s+from\s*['"]([^'"]+)['"]`)

	// Matches: import X, { Y, Z } from '@/path' (mixed default + named)
	// Must run BEFORE reImportNamed/reImportDefault to avoid double-counting.
	reImportMixed = regexp.MustCompile(`import\s+([A-Za-z]\w*)\s*,\s*\{([^}]+)\}\s*from\s*['"]([^'"]+)['"]`)

	// Matches: export function X, export const X, export class X, export type X, export interface X
	reExportNamed = regexp.MustCompile(`export\s+(?:function|const|let|var|class|type|interface|enum)\s+(\w+)`)

	// Matches: export { X, Y, Z }
	reExportBraces = regexp.MustCompile(`export\s*\{([^}]+)\}`)

	// Matches: export default function X or export default class X
	reExportDefault = regexp.MustCompile(`export\s+default\s+(?:function|class)\s+(\w+)`)

	// Matches: export default IdentifierName; — bare identifier re-export.
	// Captures the identifier so the registry can record it as both `default` AND `IdentifierName`.
	reExportDefaultIdent = regexp.MustCompile(`export\s+default\s+([A-Z]\w*)\s*;`)

	// Matches: export default React.memo(X) / export default memo(X) / export default forwardRef(X) wrappers.
	// Captures the wrapped component name.
	reExportDefaultWrapped = regexp.MustCompile(`export\s+default\s+(?:React\.)?(?:memo|forwardRef|observer)\s*\(\s*([A-Z]\w*)`)

	// Matches: const X = lazy(() => import('@/path')[.then(...)]) — code-splitting page declarations.
	// Captures X so findSelfRecursiveComponents and lazy validators can find them.
	reLazyConstDecl = regexp.MustCompile(`(?:export\s+)?const\s+([A-Z]\w*)\s*=\s*lazy\s*\(`)

	// Matches a lazy import call: lazy(() => import('@/path')) or
	// lazy(() => import('@/path').then(m => ({ default: m.NamedExport }))).
	// Capture 1: import path.
	// Capture 2: named export from `m.<NamedExport>` (empty when the file's default is used).
	// Whitespace and async are tolerated; arbitrary arrow-arg names (`m`, `mod`, `x`) are tolerated.
	reImportLazy = regexp.MustCompile(`lazy\s*\(\s*(?:async\s*)?\(\s*\)\s*=>\s*import\s*\(\s*['"]([^'"]+)['"]\s*\)(?:\s*\.\s*then\s*\(\s*(?:\([^)]*\)|\w+)\s*=>\s*\(?\s*\{\s*default\s*:\s*\w+\s*\.\s*(\w+)\s*\}\s*\)?\s*\)\s*)?`)

	// Matches: X.displayName = 'X' pattern (React.forwardRef components)
	reDisplayName = regexp.MustCompile(`(\w+)\.displayName\s*=`)

	// Matches: import.meta.env.VITE_XXX
	reEnvUsage = regexp.MustCompile(`import\.meta\.env\.(\w+)`)

	// Matches: const X, let X, var X, function X, class X — local declarations
	reLocalDecl = regexp.MustCompile(`(?:const|let|var|function|class)\s+([A-Z]\w+)`)

	// Matches word-apostrophe-word: Ko'rildi, Og'zaki, it's — unquoted in JS = build crash.
	reWordApostropheWord = regexp.MustCompile(`[A-Za-z]'[A-Za-z]`)

	// Matches common React component declarations.
	reComponentFunctionDecl = regexp.MustCompile(`(?:export\s+default\s+|export\s+)?function\s+([A-Z]\w*)\s*\(`)
	reComponentConstArrow   = regexp.MustCompile(`(?s)(?:export\s+)?const\s+([A-Z]\w*)\s*=\s*(?:React\.memo\s*\()?[^=;]{0,240}=>`)

	// Browser-build hazards that are cheap to catch before the generated app is published.
	reNativeSelect     = regexp.MustCompile(`<\s*select(?:\s|>)`)
	reEmptySelectItem  = regexp.MustCompile(`<SelectItem\b[^>]*\bvalue\s*=\s*(?:""|''|\{\s*""\s*\}|\{\s*''\s*\})`)
	reInlineApiNesting = regexp.MustCompile(`data\?\.(?:data\?\.)?(?:data\?\.)?response|data\.data\.response|data\.data\.data\.response`)
)

// ImportStatement represents one parsed import.
type ImportStatement struct {
	Names    []string // named imports: {A, B, C}
	Default  string   // default import name (if any)
	Path     string   // import path: '@/components/ui/Button'
	FilePath string   // source file that has this import
}

// ValidationError is one detected issue in the generated code.
type ValidationError struct {
	Severity string // "error" or "warning"
	File     string // file where the issue was found
	Message  string
}

// LazyImport represents one `lazy(() => import('@/path')...)` call.
// NamedExport is empty when the call relies on the file's default export;
// otherwise it holds the identifier read from `.then(m => ({ default: m.X }))`.
type LazyImport struct {
	Path        string
	NamedExport string
	FilePath    string
}

// validateGeneratedProject scans all merged files for import/export mismatches
// and env variable inconsistencies. Returns a list of validation errors.
//
// Call this after mergeChunks() and before publishing.
func validateGeneratedProject(files []models.ProjectFile, envVars map[string]any) []ValidationError {
	var errors []ValidationError

	// Step 0: Validate preview entry contract. The virtual host entry always
	// imports default from src/App.tsx, so this is a build-time hard failure.
	errors = append(errors, validateAppEntryContract(files)...)

	// Step 1: Build export registry — path → set of exported names.
	exportRegistry := buildExportRegistry(files)

	// Step 2: Build file path set for existence checks.
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f.Path] = true
	}

	// Step 3: Scan all files for imports and validate them.
	for _, f := range files {
		imports := parseImports(f.Path, f.Content)
		for _, imp := range imports {
			if isNPMImport(imp.Path) {
				continue
			}

			resolvedPath := resolveImportPath(f.Path, imp.Path)
			if resolvedPath == "" {
				continue // couldn't resolve — skip
			}

			// Check: does the target file exist?
			exportSet, exists := exportRegistry[resolvedPath]
			if !exists {
				// Try common alternatives (.tsx, .ts, /index.tsx, /index.ts)
				found := false
				for _, alt := range resolveAlternatives(resolvedPath) {
					if _, altExists := exportRegistry[alt]; altExists {
						found = true
						resolvedPath = alt
						exportSet = exportRegistry[alt]
						break
					}
				}
				if !found {
					if isTemplateFile(resolvedPath) {
						// Template file exists at runtime — but still validate named imports
						// against known exports so we catch "formatPrice" style errors.
						if templateExports := getTemplateExports(resolvedPath); templateExports != nil {
							for _, name := range imp.Names {
								if name == "" || name == "type" {
									continue
								}
								if !templateExports[name] {
									errors = append(errors, ValidationError{
										Severity: "error",
										File:     f.Path,
										Message:  fmt.Sprintf("imports {%s} from %q but it is not exported by the template (available: check @/lib/utils docs)", name, imp.Path),
									})
								}
							}
						}
						continue
					}
					errors = append(errors, ValidationError{
						Severity: "error",
						File:     f.Path,
						Message:  fmt.Sprintf("imports from %q but file does not exist in generated output", imp.Path),
					})
					continue
				}
			}

			// Check: default import — target file must have a default export.
			if imp.Default != "" && !exportSet["default"] {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  fmt.Sprintf("default-imports %q from %q but the file has no default export", imp.Default, imp.Path),
				})
			}

			// Check: named imports — each must be exported by the target file.
			// Names are already cleaned (aliases stripped, "type " removed) by parseImports.
			for _, name := range imp.Names {
				if name == "" || name == "type" {
					continue
				}
				if !exportSet[name] {
					errors = append(errors, ValidationError{
						Severity: "error",
						File:     f.Path,
						Message:  fmt.Sprintf("imports {%s} from %q but it is not exported", name, imp.Path),
					})
				}
			}
		}
	}

	// Step 4: Check for orphaned displayName assignments (e.g. Texarea.displayName where Texarea is not defined).
	// These cause ReferenceError at module load time — the whole page crashes before React renders.
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") && !strings.HasSuffix(f.Path, ".ts") {
			continue
		}
		// Collect all locally-declared names (PascalCase only — component names)
		declared := make(map[string]bool)
		for _, m := range reLocalDecl.FindAllStringSubmatch(f.Content, -1) {
			declared[m[1]] = true
		}
		// Also treat imported names as "declared"
		for _, imp := range parseImports(f.Path, f.Content) {
			if imp.Default != "" {
				declared[imp.Default] = true
			}
			for _, n := range imp.Names {
				declared[strings.TrimSpace(n)] = true
			}
		}
		// Check every X.displayName = '...' — X must be declared
		for _, m := range reDisplayName.FindAllStringSubmatch(f.Content, -1) {
			name := m[1]
			// Skip known globals
			if name == "React" || name == "module" || name == "exports" {
				continue
			}
			if !declared[name] {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  fmt.Sprintf("%s.displayName is assigned but %s is not declared in this file (likely a typo in component name)", name, name),
				})
			}
		}
	}

	// Step 5: Validate JSX/TSX syntax — brace/bracket/paren balance.
	// Unbalanced delimiters cause Esbuild crashes like "Expected > but found }".
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") && !strings.HasSuffix(f.Path, ".ts") {
			continue
		}
		if syntaxErr := checkBraceBalance(f.Content); syntaxErr != "" {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  syntaxErr,
			})
		}
		if apostropheErr := checkUnquotedApostropheWords(f.Content); apostropheErr != "" {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  apostropheErr,
			})
		}
	}

	// Step 6: Validate env variables.
	envErrors := validateEnvVars(files, envVars)
	errors = append(errors, envErrors...)

	// Step 7: Validate route/page and browser-build hazards that often only show up
	// after Vite starts rendering each page.
	pageErrors := validatePageAndRuntimeHazards(files)
	errors = append(errors, pageErrors...)

	// Step 8: Validate React.lazy() calls against the export registry.
	// Catches the common "lazy(m.HomePage) but HomePage is default export only" pattern
	// that causes React error #306 at runtime navigation. parseImports never sees these.
	lazyErrors := validateLazyImports(files, exportRegistry)
	errors = append(errors, lazyErrors...)

	// Step 9: Layout contract. Empty/Outlet-less Layout files silently swallow
	// every route and the admin shell never paints. Skip when no Layout exists
	// (landing / single-page projects are exempt).
	layoutErrors := validateLayoutShape(files)
	errors = append(errors, layoutErrors...)

	return errors
}

// validateLayoutShape ensures src/components/layout/Layout.tsx renders either
// <Outlet /> (parent-route pattern) or accepts a `children` prop. A Layout
// that does neither will render its header/sidebar but no page content — the
// classic "menus disappear, content blank" bug.
func validateLayoutShape(files []models.ProjectFile) []ValidationError {
	const layoutPath = "src/components/layout/Layout.tsx"
	for _, f := range files {
		if f.Path != layoutPath {
			continue
		}
		hasOutlet := strings.Contains(f.Content, "<Outlet")
		hasChildren := strings.Contains(f.Content, "children")
		if !hasOutlet && !hasChildren {
			return []ValidationError{{
				Severity: "error",
				File:     f.Path,
				Message:  "Layout must render <Outlet /> from react-router-dom OR accept a `children` prop and render it. Without either, all routed pages render blank.",
			}}
		}
		return nil
	}
	return nil
}

// validateAgainstManifest reports drift between what the manifest promised
// and what was actually generated. No-ops when manifest is nil.
func validateAgainstManifest(files []models.ProjectFile, manifest *models.ProjectManifest) []ValidationError {
	if manifest == nil {
		return nil
	}
	var errors []ValidationError
	registry := buildExportRegistry(files)
	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f.Path] = true
	}

	if manifest.ExportStyle == "named-lazy" {
		for _, route := range manifest.Routes {
			if route.FilePath == "" || route.PageName == "" {
				continue
			}
			exports, ok := registry[route.FilePath]
			if !ok {
				for _, alt := range resolveAlternatives(route.FilePath) {
					if found, exists := registry[alt]; exists {
						exports = found
						ok = true
						break
					}
				}
			}
			if !ok {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     "src/App.tsx",
					Message:  fmt.Sprintf("manifest route %q points to %q but that file is not in the generated output", route.Path, route.FilePath),
				})
				continue
			}
			if !exports[route.PageName] {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     route.FilePath,
					Message:  fmt.Sprintf("manifest expects this file to export named function %q (for route %q), but it does not", route.PageName, route.Path),
				})
			}
		}
	}

	if len(manifest.EntityTypes) > 0 && fileSet["src/types.ts"] {
		typesExports := registry["src/types.ts"]
		for _, entity := range manifest.EntityTypes {
			if entity.Name == "" {
				continue
			}
			if !typesExports[entity.Name] {
				errors = append(errors, ValidationError{
					Severity: "warning",
					File:     "src/types.ts",
					Message:  fmt.Sprintf("manifest expects entity interface %q but src/types.ts does not export it (features importing %q will fail)", entity.Name, entity.Name),
				})
			}
		}
	}

	for _, group := range manifest.Groups {
		for _, entry := range group.Files {
			if entry.Kind != "page" || len(entry.Exports) == 0 {
				continue
			}
			exports, ok := registry[entry.Path]
			if !ok {
				continue
			}
			expected := entry.Exports[0]
			if expected == "" {
				continue
			}
			if !exports[expected] {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     entry.Path,
					Message:  fmt.Sprintf("manifest declares this page exports %q but the file does not export that name (lazy(m.%s) will resolve to undefined)", expected, expected),
				})
			}
		}
	}

	return errors
}

func validatePageAndRuntimeHazards(files []models.ProjectFile) []ValidationError {
	var errors []ValidationError

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") && !strings.HasSuffix(f.Path, ".ts") {
			continue
		}

		if strings.Contains(f.Content, "This section is temporarily unavailable") {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "contains fallback stub UI instead of a real generated page; this page must be implemented before publish",
			})
		}

		if reNativeSelect.MatchString(f.Content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "uses native <select>, which breaks the design system; replace with @/components/ui/select primitives",
			})
		}

		if reEmptySelectItem.MatchString(f.Content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "uses <SelectItem value=\"\">, which crashes Radix Select at runtime; use non-empty sentinel values like 'all' or 'none' and map them back to empty filters in state/query logic",
			})
		}

		if reInlineApiNesting.MatchString(f.Content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "indexes API response manually with data.data.response; use extractList, extractSingle, or extractCount",
			})
		}

		for _, componentName := range findSelfRecursiveComponents(f.Path, f.Content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  fmt.Sprintf("component %s renders <%s> inside itself, causing infinite React recursion and Maximum call stack size exceeded", componentName, componentName),
			})
		}

		if strings.HasPrefix(f.Path, "src/pages/") && strings.HasSuffix(f.Path, ".tsx") {
			if strings.Contains(f.Content, "useApiQuery") && !strings.Contains(f.Content, "isLoading") {
				errors = append(errors, ValidationError{
					Severity: "warning",
					File:     f.Path,
					Message:  "fetches API data but does not appear to render a loading state",
				})
			}
			if strings.Contains(f.Content, "useApiQuery") && !strings.Contains(f.Content, "error") && !strings.Contains(f.Content, "isError") {
				errors = append(errors, ValidationError{
					Severity: "warning",
					File:     f.Path,
					Message:  "fetches API data but does not appear to render an error state",
				})
			}
		}
	}

	return errors
}

func validateAdminPanelUIQuality(files []models.ProjectFile) []ValidationError {
	var errors []ValidationError

	for _, f := range files {
		if !isGeneratedAdminScreen(f.Path) {
			continue
		}
		content := f.Content
		lowerPath := strings.ToLower(f.Path)
		lowerContent := strings.ToLower(content)

		if strings.Contains(lowerPath, "dashboard") {
			if !hasAny(content, "grid-cols-4", "lg:grid-cols-4", "xl:grid-cols-4") ||
				!hasAny(lowerContent, "pipeline", "funnel", "chart", "trend", "forecast", "queue", "timeline", "ledger") ||
				!hasAny(lowerContent, "recent", "activity", "upcoming", "alert", "risk", "insight") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "admin UI quality: dashboard is not product-grade enough; add 4 domain KPI cards, a primary operational surface, a secondary insight/alert panel, and recent activity while preserving existing APIs and data fields",
				})
			}
			continue
		}

		if looksLikeKanban(content) && (!hasAny(content, "Avatar", "AvatarFallback", "getInitials") || !hasAny(content, "Sheet", "Dialog", "selected", "details")) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "admin UI quality: kanban board is too basic; add stage aggregate headers, compact metadata chips, owner avatars/initials, responsive columns, and a right-side detail drawer/dialog without changing API hooks or entity fields",
			})
		}

		if looksLikeTableOnlyCRUD(content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "admin UI quality: page looks like generic table-only CRUD; add operational summary cards, grouped filters, status chips, hover row actions, pagination/empty/loading states, and a detail drawer/dialog while preserving API endpoints and JSON contracts",
			})
		}

		if strings.Contains(lowerPath, "calendar") && !hasAny(lowerContent, "agenda", "upcoming", "selected", "side", "details") {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "admin UI quality: calendar needs a product-grade agenda/detail side panel, event density cues, calendar controls, and empty/loading states while preserving existing integrations",
			})
		}

		if strings.Contains(lowerPath, "report") && !hasAny(lowerContent, "preview", "chart", "metric", "export", "insight", "analytics") {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "admin UI quality: reports page needs analytics depth; add saved report cards with metrics, preview/insight area, export actions, and status filters while preserving report fields",
			})
		}
	}

	return errors
}

// reIconStringValue matches an icon stored as a STRING literal in data (e.g. icon: "zap"),
// which is the root of the "icon name rendered as text" bug.
var reIconStringValue = regexp.MustCompile(`(?i)\bicon:\s*["'][a-z][a-z0-9 _-]*["']`)

// reIconRendered matches a ".icon" member used in a JSX expression (e.g. {tx.icon}).
var reIconRendered = regexp.MustCompile(`\{\s*[a-zA-Z_][\w.]*\.icon\s*\}`)

// reUnsafeNumberMethod matches a number method called directly on an identifier/member
// (e.g. sellerRating.toFixed(1), item.price.toLocaleString()) — which crashes when the API
// value is a string/null. A leading ")" (as in Number(x).toFixed) is intentionally NOT matched.
var reUnsafeNumberMethod = regexp.MustCompile(`[A-Za-z0-9_$\]]\s*\.\s*(toFixed|toLocaleString)\s*\(`)

// validateWebAppUIQuality enforces the MOBILE-APP identity for webapp projects:
// a centered phone frame, a fixed bottom tab bar (not a desktop side rail), a compact
// top bar (no ⌘K), mobile list/cards instead of data tables, no admin chrome, and no
// icon-name-as-text bug. Severity "error" triggers an auto-repair pass.
func validateWebAppUIQuality(files []models.ProjectFile) []ValidationError {
	var errors []ValidationError

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") {
			continue
		}
		content := f.Content
		lower := strings.ToLower(content)

		// Admin / marketing chrome leaked into any file → wrong product identity.
		if hasAny(content, "Admin Panel", "Admin Console", "Command Center", "Platform overview", "Start Free Trial", "Watch the Film") {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "webapp UI: remove admin/marketing chrome (Admin Panel/Admin Console/Command Center/Start Free Trial). This is an end-user MOBILE app — use a product home screen, a compact mobile top bar, and a fixed bottom tab bar.",
			})
		}

		// Icon name stored as a string AND rendered (the 'zap'/'shopping cart' text bug).
		if reIconStringValue.MatchString(content) && reIconRendered.MatchString(content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "webapp UI: an icon name is stored as a string and rendered as text (shows literal 'zap'/'shopping cart'). Map the name to an imported lucide component (const ICONS = { key: Zap }; const Icon = ICONS[k] ?? Circle; <Icon className=\"h-5 w-5\" />) — never render the icon-name string.",
			})
		}

		switch {
		case strings.HasSuffix(f.Path, "/layout/Layout.tsx"):
			// The phone frame: centered, mobile-width, full-height.
			if !hasAny(content, "max-w-md", "max-w-sm", "max-w-[") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: Layout must be a centered phone frame — wrap the app in mx-auto max-w-md min-h-[100dvh] flex flex-col, with a scrollable <main> (pb-24) and a fixed bottom tab bar.",
				})
			}
			continue

		case strings.HasSuffix(f.Path, "/layout/Sidebar.tsx"):
			// Must be a FIXED BOTTOM TAB BAR, not a desktop side rail.
			isBottomBar := strings.Contains(lower, "bottom-0")
			looksLikeSideRail := hasAny(content, "w-56", "w-60", "w-64", "flex-col h-full", "border-r")
			if !isBottomBar || looksLikeSideRail {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: navigation must be a FIXED BOTTOM TAB BAR (fixed inset-x-0 bottom-0, 3–5 icon+label tabs), NOT a desktop side rail (no w-56/w-60/w-64/border-r/full-height column).",
				})
				continue
			}
			if !strings.Contains(content, "safe-area-inset-bottom") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: the bottom tab bar must reserve the bottom safe area — add pb-[env(safe-area-inset-bottom)] so it clears the home indicator.",
				})
			}
			// The fixed bar MUST be fully opaque or page content shows through it.
			// Reliable signals only: an explicitly transparent/translucent BACKGROUND token, or no solid bg token at all.
			// (We do NOT flag bg-card/.. or bg-primary/.. — those are commonly used on inner pills/indicators, not the bar bg.)
			if hasAny(content, "bg-transparent", "bg-background/") || !hasAny(content, "bg-background", "bg-card") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: the bottom tab bar must have a SOLID opaque background (bg-background) — never transparent/translucent (no bg-background/NN, no bg-transparent, no blur-only). Content scrolling underneath must be fully hidden.",
				})
			}
			continue

		case strings.HasSuffix(f.Path, "/layout/Header.tsx"):
			// Compact mobile top bar — no command palette / ⌘K / desktop search.
			if hasAny(content, "⌘K", "⌘ K", "CommandPalette", "cmdk", "command palette") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: Header must be a compact mobile top bar (title/back + bell + avatar). Remove the ⌘K command palette and desktop search — those are desktop patterns.",
				})
			}
			// Top safe-area: header content must clear the status bar / notch.
			if !strings.Contains(content, "safe-area-inset-top") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: the top bar must reserve the top safe area — add pt-[max(env(safe-area-inset-top),3rem)] so the title/bell/avatar are not clipped by the status bar / notch / Dynamic Island.",
				})
			}
			headerTag := firstOpeningTag(content, "header")
			if headerTag == "" {
				headerTag = content
			}
			hasTranslucentBackground := hasAny(headerTag,
				"bg-transparent", "bg-background/", "bg-card/", "bg-primary/", "bg-secondary/", "bg-muted/")
			hasSolidBackground := hasAny(headerTag,
				"bg-background", "bg-card", "bg-primary", "bg-secondary", "bg-muted", "bg-sidebar", "bg-white", "bg-black", "bg-[")
			if hasTranslucentBackground || !hasSolidBackground {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: Header must have a SOLID opaque background on the <header> element (for example bg-background or bg-card). Never use transparent/translucent backgrounds or blur-only styling because the hero and scrolling content become unreadable behind it.",
				})
			}
			if hasAny(headerTag, "absolute", "fixed") {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  "webapp UI: Header must be sticky and remain in normal layout flow, not absolute/fixed over the hero or page content. Use sticky top-0 with the hero rendered below it.",
				})
			}
			continue
		}

		// Screen files (pages / features).
		if !isGeneratedAdminScreen(f.Path) {
			continue
		}

		// Desktop data table → should be mobile list rows / stacked cards.
		if hasAny(content, "<table", "<Table ", "<Table>", "DataTable") {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "webapp UI: replace the data <table> with mobile list rows / stacked cards (full-width rows: leading icon/avatar + title/subtitle + trailing value, tap → bottom sheet/detail route).",
			})
		}

		// Desktop KPI dashboard grid as a screen.
		if hasAny(content, "grid-cols-4", "lg:grid-cols-4", "xl:grid-cols-4") && hasAny(lower, "kpi", "total ", "metric", "stat card", "overview") {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "webapp UI: this looks like a desktop KPI dashboard. The home screen must be a glanceable mobile surface (hero card + quick-action tiles + list), not a 4-column metrics grid.",
			})
		}

		// Number method on a raw API value → "x.toFixed is not a function" runtime crash.
		if reUnsafeNumberMethod.MatchString(content) {
			errors = append(errors, ValidationError{
				Severity: "error",
				File:     f.Path,
				Message:  "webapp build: a number method (.toFixed/.toLocaleString) is called directly on a value that may be a string/null from the API — this crashes at runtime ('toFixed is not a function'). Coerce first: Number(value ?? 0).toFixed(1), or use the null-safe helpers formatCurrency(value) / formatNumber(value).",
			})
		}
	}

	return errors
}

func isGeneratedAdminScreen(path string) bool {
	if !strings.HasSuffix(path, ".tsx") {
		return false
	}
	if path == "src/App.tsx" || strings.Contains(path, "/components/ui/") || strings.Contains(path, "/components/layout/") || strings.Contains(path, "/components/shared/") {
		return false
	}
	return strings.Contains(path, "/pages/") || strings.Contains(path, "/features/")
}

func looksLikeKanban(content string) bool {
	lower := strings.ToLower(content)
	return strings.Contains(lower, "kanban") ||
		(strings.Contains(lower, "discovery") && strings.Contains(lower, "qualification") && strings.Contains(lower, "proposal")) ||
		(strings.Contains(lower, "to do") && strings.Contains(lower, "in progress") && strings.Contains(lower, "completed"))
}

func looksLikeTableOnlyCRUD(content string) bool {
	hasTable := hasAny(content, "<Table", "DataTable", "<table")
	if !hasTable {
		return false
	}
	hasOperationalContext := hasAny(content, "Card", "Tabs", "Sheet", "Dialog", "Badge", "Avatar", "Progress", "Skeleton")
	hasGroupedControls := hasAny(content, "Select", "TabsList", "filter", "Search", "Input")
	hasActionsAndStates := hasAny(content, "group-hover", "isLoading", "Skeleton", "empty", "No ", "Pagination", "Previous", "Next")
	return !hasOperationalContext || !hasGroupedControls || !hasActionsAndStates
}

func hasAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func firstOpeningTag(content, tag string) string {
	start := strings.Index(content, "<"+tag)
	if start == -1 {
		return ""
	}
	end := strings.Index(content[start:], ">")
	if end == -1 {
		return ""
	}
	return content[start : start+end+1]
}

func findSelfRecursiveComponents(path, content string) []string {
	componentNames := make(map[string]bool)

	base := path
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.TrimSuffix(strings.TrimSuffix(base, ".tsx"), ".ts")
	if base != "" && base != "index" && base[0] >= 'A' && base[0] <= 'Z' {
		componentNames[base] = true
	}

	for _, match := range reComponentFunctionDecl.FindAllStringSubmatch(content, -1) {
		componentNames[match[1]] = true
	}
	for _, match := range reComponentConstArrow.FindAllStringSubmatch(content, -1) {
		componentNames[match[1]] = true
	}
	// const X = lazy(...) — also a valid component declaration for App.tsx style.
	for _, match := range reLazyConstDecl.FindAllStringSubmatch(content, -1) {
		componentNames[match[1]] = true
	}

	var recursive []string
	for name := range componentNames {
		tagRe := regexp.MustCompile(`<\s*/?\s*` + regexp.QuoteMeta(name) + `(?:\s|>|/)`)
		if tagRe.MatchString(content) {
			recursive = append(recursive, name)
		}
	}
	return recursive
}

// buildExportRegistry scans all files and returns a map of path → exported names.
func buildExportRegistry(files []models.ProjectFile) map[string]map[string]bool {
	registry := make(map[string]map[string]bool, len(files))

	for _, f := range files {
		exports := make(map[string]bool)

		// export function X / export const X / export class X / export type X / export interface X
		for _, match := range reExportNamed.FindAllStringSubmatch(f.Content, -1) {
			exports[match[1]] = true
		}

		// export { X, Y, Z }
		for _, match := range reExportBraces.FindAllStringSubmatch(f.Content, -1) {
			for _, name := range strings.Split(match[1], ",") {
				name = strings.TrimSpace(name)
				// Handle "X as Y" — export the aliased name
				if parts := strings.SplitN(name, " as ", 2); len(parts) == 2 {
					exports[strings.TrimSpace(parts[1])] = true
				} else {
					exports[name] = true
				}
			}
		}

		// All four `export default` shapes only expose `default`. The inner identifier
		// is module-local — `import { X } from '...'` against any of these would fail.
		if reExportDefault.MatchString(f.Content) ||
			reExportDefaultIdent.MatchString(f.Content) ||
			reExportDefaultWrapped.MatchString(f.Content) ||
			strings.Contains(f.Content, "export default") {
			exports["default"] = true
		}

		// X.displayName = 'X' — React.forwardRef pattern
		for _, match := range reDisplayName.FindAllStringSubmatch(f.Content, -1) {
			exports[match[1]] = true
		}

		registry[f.Path] = exports
	}

	return registry
}

// cleanImportNames normalises a raw comma-separated names string from inside { }.
// It strips TypeScript "type " prefix and "as Alias" renaming so only the
// exported identifier (the name the target file must actually export) remains.
func cleanImportNames(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		n := strings.TrimSpace(p)
		// "type X" / "type X as Y" — strip leading "type " keyword
		n = strings.TrimPrefix(n, "type ")
		n = strings.TrimSpace(n)
		// "X as Y" — we want to check the exported name X, not the local alias Y
		if idx := strings.Index(n, " as "); idx >= 0 {
			n = strings.TrimSpace(n[:idx])
		}
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

// parseImports extracts all import statements from a file.
func parseImports(filePath, content string) []ImportStatement {
	var imports []ImportStatement

	// Track ranges already consumed by reImportMixed so we don't double-count.
	mixedRanges := make([][2]int, 0)

	// Mixed imports FIRST: import Default, { X, Y } from 'path'
	for _, loc := range reImportMixed.FindAllStringSubmatchIndex(content, -1) {
		match := reImportMixed.FindStringSubmatch(content[loc[0]:loc[1]])
		if match == nil {
			continue
		}
		mixedRanges = append(mixedRanges, [2]int{loc[0], loc[1]})
		imports = append(imports, ImportStatement{
			Default:  match[1],
			Names:    cleanImportNames(match[2]),
			Path:     match[3],
			FilePath: filePath,
		})
	}

	isMixed := func(start, end int) bool {
		for _, r := range mixedRanges {
			if start >= r[0] && end <= r[1] {
				return true
			}
		}
		return false
	}

	// Named imports: import { X, Y } from 'path'
	for _, loc := range reImportNamed.FindAllStringSubmatchIndex(content, -1) {
		if isMixed(loc[0], loc[1]) {
			continue
		}
		match := reImportNamed.FindStringSubmatch(content[loc[0]:loc[1]])
		if match == nil {
			continue
		}
		imports = append(imports, ImportStatement{
			Names:    cleanImportNames(match[1]),
			Path:     match[2],
			FilePath: filePath,
		})
	}

	// Default imports: import X from 'path'
	for _, loc := range reImportDefault.FindAllStringSubmatchIndex(content, -1) {
		if isMixed(loc[0], loc[1]) {
			continue
		}
		match := reImportDefault.FindStringSubmatch(content[loc[0]:loc[1]])
		if match == nil {
			continue
		}
		imports = append(imports, ImportStatement{
			Default:  match[1],
			Path:     match[2],
			FilePath: filePath,
		})
	}

	return imports
}

// parseLazyImports extracts every `lazy(() => import(...)...)` call.
// parseImports cannot see these because they are not top-level `import` statements.
func parseLazyImports(filePath, content string) []LazyImport {
	if !strings.Contains(content, "lazy(") {
		return nil
	}
	var imports []LazyImport
	for _, m := range reImportLazy.FindAllStringSubmatch(content, -1) {
		imports = append(imports, LazyImport{
			Path:        m[1],
			NamedExport: m[2],
			FilePath:    filePath,
		})
	}
	return imports
}

// validateLazyImports verifies, for each `lazy(...)` call:
//   - the target file exists in the generated output
//   - if `.then(m => ({ default: m.X }))` is used, X is exported as a NAMED export
//     (default-only files cause `m.X === undefined` → React error #306 at navigation)
//   - if no `.then`, the target file has a `default` export
func validateLazyImports(files []models.ProjectFile, registry map[string]map[string]bool) []ValidationError {
	var errors []ValidationError
	for _, f := range files {
		lazyImports := parseLazyImports(f.Path, f.Content)
		for _, imp := range lazyImports {
			if isNPMImport(imp.Path) {
				continue
			}
			resolvedPath := resolveImportPath(f.Path, imp.Path)
			if resolvedPath == "" {
				continue
			}
			exportSet, exists := registry[resolvedPath]
			if !exists {
				for _, alt := range resolveAlternatives(resolvedPath) {
					if e, ok := registry[alt]; ok {
						exportSet = e
						exists = true
						break
					}
				}
			}
			if !exists {
				errors = append(errors, ValidationError{
					Severity: "error",
					File:     f.Path,
					Message:  fmt.Sprintf("lazy import from %q but file does not exist in generated output", imp.Path),
				})
				continue
			}
			if imp.NamedExport != "" {
				if !exportSet[imp.NamedExport] {
					errors = append(errors, ValidationError{
						Severity: "error",
						File:     f.Path,
						Message:  fmt.Sprintf("lazy import expects named export %q from %q but the file does not export it (m.%s will be undefined → React error #306)", imp.NamedExport, imp.Path, imp.NamedExport),
					})
				}
			} else {
				if !exportSet["default"] {
					errors = append(errors, ValidationError{
						Severity: "error",
						File:     f.Path,
						Message:  fmt.Sprintf("lazy import from %q has no .then resolver and the target file has no default export", imp.Path),
					})
				}
			}
		}
	}
	return errors
}

// resolveImportPath converts an import path to a file path relative to project root.
// @/components/ui/Button → src/components/ui/Button
// /src/components/layout/Layout → src/components/layout/Layout
// ./utils → (resolved relative to importer)
func resolveImportPath(importerPath, importPath string) string {
	// @/ alias → src/
	if strings.HasPrefix(importPath, "@/") {
		return "src/" + strings.TrimPrefix(importPath, "@/")
	}

	// Vite absolute-from-root imports. The generated virtual FS stores paths
	// without a leading slash, so "/src/..." must resolve to "src/...".
	if strings.HasPrefix(importPath, "/src/") {
		return strings.TrimPrefix(importPath, "/")
	}

	// Relative imports
	if strings.HasPrefix(importPath, "./") || strings.HasPrefix(importPath, "../") {
		dir := importerPath
		if idx := strings.LastIndex(dir, "/"); idx >= 0 {
			dir = dir[:idx]
		} else {
			dir = ""
		}

		parts := strings.Split(importPath, "/")
		for _, part := range parts {
			switch part {
			case ".":
				// stay
			case "..":
				if idx := strings.LastIndex(dir, "/"); idx >= 0 {
					dir = dir[:idx]
				} else {
					dir = ""
				}
			default:
				if dir == "" {
					dir = part
				} else {
					dir = dir + "/" + part
				}
			}
		}
		return dir
	}

	return "" // npm or unresolvable
}

// resolveAlternatives returns possible file paths for an import
// (TypeScript resolves .tsx, .ts, /index.tsx, /index.ts automatically).
func resolveAlternatives(path string) []string {
	return []string{
		path + ".tsx",
		path + ".ts",
		path + "/index.tsx",
		path + "/index.ts",
	}
}

// isNPMImport returns true for imports from node_modules (no ./ or @/ prefix).
func isNPMImport(path string) bool {
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "@/") || strings.HasPrefix(path, "/src/") {
		return false
	}
	// Scoped npm packages: @radix-ui/*, @tanstack/*, etc.
	if strings.HasPrefix(path, "@") && !strings.HasPrefix(path, "@/") {
		return true
	}
	return true
}

// templateFileExports maps known template file paths to their exported names.
// Used to validate named imports against template files — catches "formatPrice" style errors
// where the model imports a function that doesn't exist in the pre-built template.
var templateFileExports = map[string]map[string]bool{
	"src/hooks/useApi": {
		"useApiQuery": true, "useApiMutation": true, "useApiInfiniteQuery": true,
	},
	"src/hooks/useApi.ts": {
		"useApiQuery": true, "useApiMutation": true, "useApiInfiniteQuery": true,
	},
	"src/hooks/useAppForm":    {"useAppForm": true},
	"src/hooks/useAppForm.ts": {"useAppForm": true},
	"src/lib/apiUtils": {
		"extractList": true, "extractSingle": true, "extractCount": true,
	},
	"src/lib/apiUtils.ts": {
		"extractList": true, "extractSingle": true, "extractCount": true,
	},
	"src/lib/utils": {
		"cn": true, "formatDate": true, "formatCurrency": true, "formatNumber": true,
		"getInitials": true, "truncate": true, "generateId": true, "sleep": true, "debounce": true,
	},
	"src/lib/utils.ts": {
		"cn": true, "formatDate": true, "formatCurrency": true, "formatNumber": true,
		"getInitials": true, "truncate": true, "generateId": true, "sleep": true, "debounce": true,
	},
	"src/types/common": {
		"NavItem": true, "TableColumn": true, "ApiResponse": true, "ApiError": true,
		"SelectOption": true, "PaginationParams": true, "LatLng": true, "MapMarker": true,
	},
	"src/types/common.ts": {
		"NavItem": true, "TableColumn": true, "ApiResponse": true, "ApiError": true,
		"SelectOption": true, "PaginationParams": true, "LatLng": true, "MapMarker": true,
	},
	"src/config/axios":                       {"default": true, "apiClient": true},
	"src/config/axios.ts":                    {"default": true, "apiClient": true},
	"src/components/shared/AppProviders":     {"AppProviders": true},
	"src/components/shared/AppProviders.tsx": {"AppProviders": true},
}

// isTemplateFile returns true for files that exist in the pre-built template.
func isTemplateFile(path string) bool {
	_, ok := templateFileExports[path]
	return ok ||
		path == "src/config/env" ||
		path == "src/config/env.ts" ||
		path == "src/config/queryClient" ||
		path == "src/config/queryClient.ts"
}

// getTemplateExports returns the known exports for a template file, or nil if unknown.
func getTemplateExports(path string) map[string]bool {
	return templateFileExports[path]
}

// validateEnvVars checks that env variables used in code are defined.
func validateEnvVars(files []models.ProjectFile, envVars map[string]any) []ValidationError {
	var errors []ValidationError

	// Collect all env vars used in code.
	usedVars := make(map[string]string) // var name → first file that uses it
	for _, f := range files {
		for _, match := range reEnvUsage.FindAllStringSubmatch(f.Content, -1) {
			varName := match[1]
			if _, exists := usedVars[varName]; !exists {
				usedVars[varName] = f.Path
			}
		}
	}

	// Check against provided env vars.
	for varName, firstFile := range usedVars {
		if _, defined := envVars[varName]; !defined {
			// Check .env files in the output
			found := false
			for _, f := range files {
				if strings.HasSuffix(f.Path, ".env") || strings.HasSuffix(f.Path, ".env.production") {
					if strings.Contains(f.Content, varName+"=") {
						found = true
						break
					}
				}
			}
			if !found {
				errors = append(errors, ValidationError{
					Severity: "warning",
					File:     firstFile,
					Message:  fmt.Sprintf("uses import.meta.env.%s but it is not defined in env vars or .env files", varName),
				})
			}
		}
	}

	return errors
}

func isAlphaNumeric(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '$'
}

// maskDoubleAndBacktickStrings replaces content inside "..." and `...` with spaces
// so apostrophe checks don't produce false positives on properly-quoted strings.
func maskDoubleAndBacktickStrings(line string) string {
	var b strings.Builder
	i := 0
	for i < len(line) {
		c := line[i]
		switch c {
		case '"':
			b.WriteByte('"')
			i++
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					i++
					b.WriteByte(' ')
				}
				b.WriteByte(' ')
				i++
			}
			if i < len(line) {
				b.WriteByte('"')
				i++
			}
		case '`':
			b.WriteByte('`')
			i++
			for i < len(line) && line[i] != '`' {
				if line[i] == '\\' && i+1 < len(line) {
					i += 2
					b.WriteString("  ")
					continue
				}
				b.WriteByte(' ')
				i++
			}
			if i < len(line) {
				b.WriteByte('`')
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// checkUnquotedApostropheWords detects words like Ko'rildi, Og'zaki used as bare
// JavaScript identifiers (not wrapped in string quotes). Esbuild treats the apostrophe
// as a string opener, producing "Expected } but found [word]" build crashes.
// Returns a single error string listing ALL affected lines so the repair pass fixes them all.
func checkUnquotedApostropheWords(content string) string {
	lines := strings.Split(content, "\n")
	var badLines []int
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "*") ||
			strings.HasPrefix(trimmed, "/*") {
			continue
		}
		// Mask properly-quoted strings so "Ko'rildi" or `Ko'rildi` don't trigger.
		stripped := maskDoubleAndBacktickStrings(line)
		if reWordApostropheWord.MatchString(stripped) {
			badLines = append(badLines, i+1)
		}
	}
	if len(badLines) == 0 {
		return ""
	}
	lineNums := make([]string, len(badLines))
	for i, ln := range badLines {
		lineNums[i] = fmt.Sprintf("%d", ln)
	}
	return fmt.Sprintf(
		"SYNTAX ERROR (lines %s): word(s) with apostrophe used as unquoted JS expression (e.g. Ko'rildi, Og'zaki, Ko'rib chiqilmoqda) — esbuild crashes with 'Expected } but found [word]'. Scan the ENTIRE file and wrap EVERY such word in double quotes: Ko'rildi → \"Ko'rildi\", Ko'rib chiqilmoqda → \"Ko'rib chiqilmoqda\". Check arrays, object values, const assignments, JSX attributes. JSX text nodes like <Badge>Ko'rildi</Badge> are the only exception.",
		strings.Join(lineNums, ", "),
	)
}

// checkBraceBalance scans a TypeScript/TSX file and returns an error message
// if braces {}, brackets [], or parentheses () are unbalanced.
// It respects string literals (single/double/backtick), comments (// and /* */),
// and regex literals so that delimiters inside them are not counted.
//
// This detects the root cause of Esbuild crashes like "Expected > but found }".
func checkBraceBalance(content string) string {
	type delimInfo struct {
		char byte
		line int
	}
	var stack []delimInfo
	line := 1
	i := 0
	n := len(content)

	for i < n {
		c := content[i]

		// Track line numbers.
		if c == '\n' {
			line++
			i++
			continue
		}

		// Skip single-line comments.
		if c == '/' && i+1 < n && content[i+1] == '/' {
			for i < n && content[i] != '\n' {
				i++
			}
			continue
		}

		// Skip block comments.
		if c == '/' && i+1 < n && content[i+1] == '*' {
			i += 2
			for i+1 < n {
				if content[i] == '\n' {
					line++
				}
				if content[i] == '*' && content[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		// Skip string literals (double quote).
		if c == '"' {
			i++
			for i < n && content[i] != '"' {
				if content[i] == '\\' {
					i++ // skip escaped char
				}
				if i < n && content[i] == '\n' {
					line++
				}
				i++
			}
			i++ // skip closing "
			continue
		}

		// Skip string literals (single quote).
		// EXCEPTION: if ' is immediately preceded by a letter/digit it is an apostrophe
		// inside a word (Uzbek: Ko'rildi, French: it's) — NOT a JS string opener.
		// Real JS string openers are always preceded by whitespace or a punctuator.
		if c == '\'' {
			if i > 0 && isAlphaNumeric(content[i-1]) {
				i++ // treat as ordinary character, not string delimiter
				continue
			}
			i++
			for i < n && content[i] != '\'' {
				if content[i] == '\\' {
					i++
				}
				if i < n && content[i] == '\n' {
					line++
				}
				i++
			}
			i++
			continue
		}

		// Skip template literals (backtick) — track ${} depth.
		if c == '`' {
			i++
			tmplDepth := 0
			for i < n {
				if content[i] == '\n' {
					line++
				}
				if content[i] == '\\' {
					i += 2
					continue
				}
				if content[i] == '$' && i+1 < n && content[i+1] == '{' {
					tmplDepth++
					i += 2
					continue
				}
				if content[i] == '}' && tmplDepth > 0 {
					tmplDepth--
					i++
					continue
				}
				if content[i] == '`' && tmplDepth == 0 {
					i++
					break
				}
				i++
			}
			continue
		}

		// Opening delimiters.
		if c == '{' || c == '(' || c == '[' {
			stack = append(stack, delimInfo{char: c, line: line})
			i++
			continue
		}

		// Closing delimiters.
		if c == '}' || c == ')' || c == ']' {
			if len(stack) == 0 {
				return fmt.Sprintf("SYNTAX ERROR (line ~%d): unexpected closing '%c' with no matching opener — this will crash Esbuild build", line, c)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			var expected byte
			switch c {
			case '}':
				expected = '{'
			case ')':
				expected = '('
			case ']':
				expected = '['
			}
			if top.char != expected {
				return fmt.Sprintf("SYNTAX ERROR (line ~%d): closing '%c' does not match opening '%c' at line ~%d — this will crash Esbuild build", line, c, top.char, top.line)
			}
			i++
			continue
		}

		i++
	}

	if len(stack) > 0 {
		top := stack[len(stack)-1]
		var closerName string
		switch top.char {
		case '{':
			closerName = "}"
		case '(':
			closerName = ")"
		case '[':
			closerName = "]"
		}
		return fmt.Sprintf("SYNTAX ERROR: unclosed '%c' opened at line ~%d is never closed (missing '%s') — this will crash Esbuild build", top.char, top.line, closerName)
	}

	return "" // balanced
}

// logValidationResults logs all validation errors and returns counts.
func logValidationResults(errors []ValidationError) (errorCount, warningCount int) {
	for _, e := range errors {
		switch e.Severity {
		case "error":
			errorCount++
			log.Printf("[VALIDATE] ❌ %s: %s", e.File, e.Message)
		case "warning":
			warningCount++
			log.Printf("[VALIDATE] ⚠️  %s: %s", e.File, e.Message)
		}
	}
	if errorCount == 0 && warningCount == 0 {
		log.Printf("[VALIDATE] ✅ All imports/exports verified — 0 issues")
	} else {
		log.Printf("[VALIDATE] Summary: %d errors, %d warnings", errorCount, warningCount)
	}
	return
}

// ============================================================================
// REPAIR LOOP
//
// If validateGeneratedProject returns errors, repairBrokenFiles sends each
// broken file to Haiku with a targeted prompt showing exactly what's wrong
// and what exports are available in the referenced files.
//
// maxDeployErrors: if error count exceeds this threshold the caller should
// refuse deployment entirely (too broken to be useful).
// ============================================================================

// repairFileResult is the tool-use response from Haiku for a single file fix.
type repairFileResult struct {
	Content string `json:"content"`
}

// repairBrokenFiles attempts to fix all files that have validation errors.
// Returns a slice of repaired files (only those that were successfully fixed).
// The caller is responsible for patching these back into the merged file list.
func (p *ChatProcessor) repairBrokenFiles(ctx context.Context, files []models.ProjectFile, validationErrors []ValidationError) []models.ProjectFile {
	exportRegistry := buildExportRegistry(files)

	// Group errors by file path.
	errorsByFile := make(map[string][]string)
	for _, e := range validationErrors {
		if e.Severity == "error" {
			errorsByFile[e.File] = append(errorsByFile[e.File], e.Message)
		}
	}

	// Build path → file index for fast lookup.
	fileMap := make(map[string]models.ProjectFile, len(files))
	for _, f := range files {
		fileMap[f.Path] = f
	}

	type repairResult struct {
		file models.ProjectFile
		ok   bool
	}
	results := make(chan repairResult, len(errorsByFile))

	var wg sync.WaitGroup
	for filePath, errs := range errorsByFile {
		f, ok := fileMap[filePath]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(f models.ProjectFile, errs []string) {
			defer wg.Done()
			p.emitter().Emit(SSEEvent{Type: EvRepair, Message: "Исправляю: " + f.Path, Percent: 86})
			fixed, err := p.repairSingleFile(ctx, f, errs, exportRegistry, files, p.currentManifest)
			if err != nil {
				log.Printf("[repair] ⚠️ failed to repair %s: %v", f.Path, err)
				results <- repairResult{ok: false}
				return
			}
			log.Printf("[repair] ✅ repaired %s", f.Path)
			results <- repairResult{file: fixed, ok: true}
		}(f, errs)
	}

	wg.Wait()
	close(results)

	var repaired []models.ProjectFile
	for r := range results {
		if r.ok {
			repaired = append(repaired, r.file)
		}
	}
	return repaired
}

// repairSingleFile sends one broken file to Haiku and returns the fixed version.
// allFiles + manifest are used to enrich the prompt with App.tsx context, manifest
// entry, and expected import/export pairs — so repair can fix lazy/named mismatches
// even when the error originates in App.tsx (which itself is read-only here).
func (p *ChatProcessor) repairSingleFile(
	ctx context.Context,
	f models.ProjectFile,
	errs []string,
	exportRegistry map[string]map[string]bool,
	allFiles []models.ProjectFile,
	manifest *models.ProjectManifest,
) (models.ProjectFile, error) {
	var sb strings.Builder

	sb.WriteString("Fix the TypeScript/TSX file below. It has the following errors:\n\n")
	for _, e := range errs {
		fmt.Fprintf(&sb, "  - %s\n", e)
	}

	// Inject available exports from target files so Haiku knows what's actually there.
	sb.WriteString("\nAVAILABLE EXPORTS in the referenced files (use ONLY these names):\n")
	imports := parseImports(f.Path, f.Content)
	seen := make(map[string]bool)
	for _, imp := range imports {
		resolved := resolveImportPath(f.Path, imp.Path)
		if resolved == "" || isNPMImport(imp.Path) {
			continue
		}
		for _, alt := range append([]string{resolved}, resolveAlternatives(resolved)...) {
			if exports, ok := exportRegistry[alt]; ok && !seen[alt] {
				seen[alt] = true
				names := make([]string, 0, len(exports))
				for name := range exports {
					names = append(names, name)
				}
				fmt.Fprintf(&sb, "  %s → [%s]\n", imp.Path, strings.Join(names, ", "))
				break
			}
		}
	}

	// Manifest entry for the target file — gives the agent the authoritative
	// kind/route/exports contract instead of guessing from naming.
	if manifest != nil {
		for _, g := range manifest.Groups {
			for _, mf := range g.Files {
				if mf.Path == f.Path {
					fmt.Fprintf(&sb, "\nMANIFEST ENTRY for this file: kind=%s, exports=[%s]", mf.Kind, strings.Join(mf.Exports, ", "))
					if mf.Route != "" {
						fmt.Fprintf(&sb, ", route=%s", mf.Route)
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	// App.tsx reference — most import errors trace back to a wrong lazy(m.X) in App.tsx.
	// We attach it so the agent sees the contract it must satisfy.
	if f.Path != "src/App.tsx" {
		for _, af := range allFiles {
			if af.Path == "src/App.tsx" {
				sb.WriteString("\n=== APP.TSX (READ-ONLY reference — do not modify this file; align your exports to match what App.tsx expects) ===\n```typescript\n")
				sb.WriteString(af.Content)
				sb.WriteString("\n```\n")
				break
			}
		}
	}

	// If repairing App.tsx, list the full lazy→page contract from the manifest.
	if f.Path == "src/App.tsx" && manifest != nil && len(manifest.Routes) > 0 {
		sb.WriteString("\n=== EXPECTED ROUTES (manifest contract — App.tsx must contain exactly these) ===\n")
		for _, r := range manifest.Routes {
			fmt.Fprintf(&sb, "  path=%q  →  const %s = lazy(() => import('@/pages/%s').then(m => ({ default: m.%s })));  // file: %s\n",
				r.Path, r.PageName, strings.TrimSuffix(strings.TrimPrefix(r.FilePath, "src/pages/"), ".tsx"), r.PageName, r.FilePath)
		}
	}

	sb.WriteString("\nRULES:\n")
	sb.WriteString("  - Fix ONLY the listed errors. Do not rewrite unrelated code.\n")
	sb.WriteString("  - src/App.tsx is the preview entry. It MUST export default App (`export default function App()` or `export default App;`). The host imports default from virtual:/src/App, so a named-only App export is a build failure.\n")
	sb.WriteString("  - For import errors: use correct exported names from the AVAILABLE EXPORTS list above.\n")
	sb.WriteString("  - For 'lazy import expects named export X but the file does not export it' errors: the lazy resolver in App.tsx wants a NAMED export. If THIS file is the page and currently uses `export default function X`, rewrite it as `export function X` (named export). Keep the function body identical. Do NOT touch App.tsx.\n")
	sb.WriteString("  - For 'X.displayName assigned but X not declared': it is a typo in the component name — rename the const/variable to match the displayName assignment, or fix the displayName to match the const name.\n")
	sb.WriteString("  - For 'component X renders <X> inside itself': this is infinite React recursion. Replace the inner <X> with the intended wrapper element (<div>, <main>, <Outlet />) or import the correct different component name. A component must never render itself directly.\n")
	sb.WriteString("  - For '<SelectItem value=\"\">' errors: Radix SelectItem values cannot be empty strings. Replace empty option values with non-empty sentinel strings such as 'all', 'none', or 'unassigned'. Update state/filter logic so the sentinel means no filter / empty relation, but NEVER render value=\"\" on SelectItem.\n")
	sb.WriteString("  - For 'admin UI quality' errors: perform a focused visual/product polish pass on this file. Preserve every API endpoint, hook, mutation, entity field, JSON extraction, route, and generated type. Improve layout density, hierarchy, cards, filters, status chips, detail drawer/dialog, states, and domain-specific widgets only.\n")
	sb.WriteString("  - For 'word with apostrophe used as unquoted JS expression' errors: the file contains Uzbek/non-ASCII text like Ko'rildi, Ko'rib chiqilmoqda, Og'zaki used directly as JavaScript identifiers without string quotes. This crashes esbuild. Find EVERY such word in arrays, object property values, variable assignments, JSX attribute values — wrap each one in double quotes. Example: { label: Ko'rildi } → { label: \"Ko'rildi\" }, [Ko'rib] → [\"Ko'rib\"]. JSX text nodes are fine: <Badge>Ko'rildi</Badge> does NOT need change, only JS expression contexts.\n")
	sb.WriteString("  - For 'webapp UI' errors: this is a MOBILE APP (responsive web). Fix toward a phone layout — a centered max-w-md min-h-[100dvh] frame, a fixed bottom tab bar (NOT a desktop side rail) with a SOLID opaque bg-background (no /opacity, no transparent, no blur-only), and a compact sticky in-flow Header with a SOLID opaque background plus pt-[max(env(safe-area-inset-top),3rem)] (never absolute/fixed/transparent over the hero). Use single-column stacked cards/list rows (NOT data tables or KPI dashboards), bottom-sheet/detail-route for item details, and lucide icons rendered as <Icon/> components (never icon-name strings). EVERY button/tab/tile/row/FAB must be wired (onClick navigate via useNavigate/NavLink, or open a Sheet via useState, or fire a useApiMutation) — no dead buttons. Render real API data (useApiQuery + extractList), not hardcoded values. Preserve all APIs, hooks, fields, routes, and types.\n")
	sb.WriteString("  - For brace/bracket/paren imbalance: carefully trace through the file and find the exact location of the missing or extra delimiter. Common causes: unclosed ternary in JSX, missing closing brace in .map() callback, extra } after a component return, unclosed template literal.\n")
	sb.WriteString("  - NEVER use angle-bracket type assertions in .tsx files (const x = <Type>value). ALWAYS use 'as' syntax (const x = value as Type).\n")
	sb.WriteString("  - Output the complete corrected file. Never truncate.\n")

	fmt.Fprintf(&sb, "\nFILE: %s\n```typescript\n%s\n```\n", f.Path, f.Content)

	return p.agent.RepairFile(ctx, models.RepairFileInput{
		File:       f,
		UserPrompt: sb.String(),
	})
}

// applyRepairs patches repaired file contents back into the file list in-place.
// isProtectedTemplateFile reports whether a path is a pre-built file that must
// always keep its template content (auth runtime + axios), never model output.
func isProtectedTemplateFile(path string) bool {
	if forceTemplatePaths[path] {
		return true
	}
	for _, p := range adminPanelRuntimeTemplateFiles {
		if p == path {
			return true
		}
	}
	return false
}

func applyRepairs(files []models.ProjectFile, repaired []models.ProjectFile) {
	patchMap := make(map[string]string, len(repaired))
	for _, f := range repaired {
		// Never let a repair pass overwrite the pre-built auth runtime. The
		// repair model is usually triggered by an App.tsx wiring error, but it
		// often re-emits a stubbed LoginPage ("simulate login", no /v2/login)
		// alongside the legit App.tsx fix. mergeTemplateScaffold and
		// injectMissingCriticalFiles already forced the template BEFORE repair;
		// applying the model's version here silently breaks sign-in again.
		if isProtectedTemplateFile(f.Path) {
			log.Printf("[quality-gate] skipping repair of protected template file: %s", f.Path)
			continue
		}
		patchMap[f.Path] = f.Content
	}
	for i := range files {
		if newContent, ok := patchMap[files[i].Path]; ok {
			files[i].Content = newContent
		}
	}
}

// buildUIKitAPISummary extracts a compact API reference from generated UI Kit files
// (both ui/* primitives and components/shared/* composite patterns).
// Injected into feature chunk prompts so they know exact component APIs and variant values.
func buildUIKitAPISummary(uiKitFiles []models.ProjectFile) string {
	if len(uiKitFiles) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("====================================\n")
	sb.WriteString("UI KIT + SHARED PATTERNS — API REFERENCE\n")
	sb.WriteString("====================================\n")
	sb.WriteString("Already generated. Use EXACTLY these names, props, and variant values.\n\n")

	reInterface := regexp.MustCompile(`(?m)export\s+(?:interface|type)\s+(\w+(?:Props|Column|State))\s*(?:[<{]|extends)`)
	reVariants := regexp.MustCompile(`(?m)export\s+const\s+(\w+Variants)\s*=`)
	// Extracts variant KEYS from cva variant blocks: variant: { default: '...', outline: '...' }
	reVariantBlock := regexp.MustCompile(`(?s)variants\s*:\s*\{(.+?)\}\s*,?\s*defaultVariants`)
	reVariantEntry := regexp.MustCompile(`(?m)^\s*(\w+)\s*:\s*\{([^}]+)\}`)
	reVariantKeys := regexp.MustCompile(`(?m)^\s*(\w+)\s*:`)

	for _, f := range uiKitFiles {
		var exports []string
		for _, match := range reExportNamed.FindAllStringSubmatch(f.Content, -1) {
			exports = append(exports, match[1])
		}
		for _, match := range reExportBraces.FindAllStringSubmatch(f.Content, -1) {
			for _, name := range strings.Split(match[1], ",") {
				if n := strings.TrimSpace(name); n != "" {
					exports = append(exports, n)
				}
			}
		}
		if len(exports) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "### %s\n", f.Path)
		fmt.Fprintf(&sb, "  Exports: [%s]\n", strings.Join(exports, ", "))

		for _, match := range reInterface.FindAllStringSubmatch(f.Content, -1) {
			fmt.Fprintf(&sb, "  Props: %s\n", match[1])
		}

		// Show variant definitions with actual key values so chunks use correct variant names.
		for _, varMatch := range reVariants.FindAllStringSubmatch(f.Content, -1) {
			fmt.Fprintf(&sb, "  Variants const: %s\n", varMatch[1])
		}
		if blockMatch := reVariantBlock.FindStringSubmatch(f.Content); len(blockMatch) > 1 {
			for _, entryMatch := range reVariantEntry.FindAllStringSubmatch(blockMatch[1], -1) {
				variantName := entryMatch[1]
				var keys []string
				for _, keyMatch := range reVariantKeys.FindAllStringSubmatch(entryMatch[2], -1) {
					keys = append(keys, keyMatch[1])
				}
				if len(keys) > 0 {
					fmt.Fprintf(&sb, "  %s values: [%s]\n", variantName, strings.Join(keys, ", "))
				}
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
