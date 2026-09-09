package v1

import (
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// ============================================================================
// VALIDATOR UNIT TESTS
//
// These tests simulate real error classes from past generation failures.
// Run with: go test -v ./api/handlers/v1/ -run TestValidat
// ============================================================================

// TestValidate_MissingFile — import from a file that doesn't exist in output.
// Real failure: feature chunk imports from '@/components/ui/calendar' but calendar.tsx was never generated.
func TestValidate_MissingFile(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path:    "src/pages/OrdersPage.tsx",
			Content: `import React from 'react';\nimport { Calendar } from '@/components/ui/calendar';\nexport default function OrdersPage() { return <Calendar />; }`,
		},
		{
			Path:    "src/components/ui/button.tsx",
			Content: `import React from 'react';\nexport const Button = React.forwardRef(() => null);\nButton.displayName = 'Button';`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "does not exist") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for missing '@/components/ui/calendar', got %d errors: %v", len(errors), errors)
	}
}

// TestValidate_MissingExport — import a named export that doesn't exist.
// Real failure: QuoteStatusBadge was imported from Badge.tsx but Badge.tsx only exports Badge + BadgeProps.
func TestValidate_MissingExport(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/components/ui/badge.tsx",
			Content: `import React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

export const badgeVariants = cva('inline-flex items-center');

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement>,
  VariantProps<typeof badgeVariants> {}

export const Badge = React.forwardRef<HTMLDivElement, BadgeProps>(
  ({ className, variant, ...props }, ref) => (
    <div ref={ref} className={badgeVariants({ variant })} {...props} />
  )
);
Badge.displayName = 'Badge';`,
		},
		{
			Path: "src/features/quotes/components/QuoteList.tsx",
			Content: `import React from 'react';
import { Badge, QuoteStatusBadge } from '@/components/ui/badge';
export function QuoteList() { return <Badge />; }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "QuoteStatusBadge") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for missing export 'QuoteStatusBadge', got %d errors: %v", len(errors), errors)
	}
}

// TestValidate_ValidProject — no errors for a correctly wired project.
func TestValidate_ValidProject(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/components/ui/button.tsx",
			Content: `import React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';

export const buttonVariants = cva('btn');

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>,
  VariantProps<typeof buttonVariants> {}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button ref={ref} {...props} />
  )
);
Button.displayName = 'Button';`,
		},
		{
			Path: "src/components/ui/badge.tsx",
			Content: `import React from 'react';
export function Badge({ children }: { children: React.ReactNode }) { return <span>{children}</span>; }`,
		},
		{
			Path: "src/pages/DashboardPage.tsx",
			Content: `import React from 'react';
import { Button, buttonVariants } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
export default function DashboardPage() { return <div><Button /><Badge>OK</Badge></div>; }`,
		},
		{
			Path: "src/App.tsx",
			Content: `import React from 'react';
import './index.css';
import DashboardPage from './pages/DashboardPage';
const baseUrl = import.meta.env.VITE_API_BASE_URL;
export default function App() { return <DashboardPage />; }`,
		},
		{
			Path:    "src/index.css",
			Content: `:root { --primary: 220 80% 50%; }`,
		},
		{
			Path:    ".env",
			Content: "VITE_API_BASE_URL=https://api.example.com\nVITE_X_API_KEY=test-key",
		},
	}

	errors := validateGeneratedProject(files, nil)
	errorCount := 0
	for _, e := range errors {
		if e.Severity == "error" {
			errorCount++
			t.Errorf("unexpected error: [%s] %s", e.File, e.Message)
		}
	}
	if errorCount > 0 {
		t.Errorf("expected 0 errors for valid project, got %d", errorCount)
	}
}

// TestValidate_EnvMismatch — env var used in code but not defined anywhere.
func TestValidate_EnvMismatch(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path:    "src/config/axios.ts",
			Content: `import axios from 'axios';\nconst api = axios.create({ baseURL: import.meta.env.VITE_CUSTOM_URL });\nexport default api;`,
		},
		{
			Path:    ".env",
			Content: "VITE_API_BASE_URL=https://api.example.com",
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if contains(e.Message, "VITE_CUSTOM_URL") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for undefined VITE_CUSTOM_URL, got %d errors: %v", len(errors), errors)
	}
}

// TestValidate_RelativeImport — relative import './utils' resolves correctly.
func TestValidate_RelativeImport(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path:    "src/features/orders/api.ts",
			Content: `import { OrderType } from './types';\nexport function getOrders() { return null; }`,
		},
		{
			Path:    "src/features/orders/types.ts",
			Content: `export interface OrderType { id: string; }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	for _, e := range errors {
		if e.Severity == "error" {
			t.Errorf("unexpected error for valid relative import: [%s] %s", e.File, e.Message)
		}
	}
}

// TestValidate_RelativeImportMissing — relative import to non-existent file.
func TestValidate_RelativeImportMissing(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path:    "src/features/orders/api.ts",
			Content: `import { OrderType } from './types';\nimport { formatOrder } from './formatters';\nexport function getOrders() { return null; }`,
		},
		{
			Path:    "src/features/orders/types.ts",
			Content: `export interface OrderType { id: string; }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "formatters") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for missing './formatters', got %d errors: %v", len(errors), errors)
	}
}

func TestValidate_AbsoluteSrcImportMissing(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import React from 'react';
import Layout from '/src/components/layout/Layout';
export default function App() { return <Layout />; }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "/src/components/layout/Layout") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for missing absolute /src import, got %d errors: %v", len(errors), errors)
	}
}

func TestValidate_SelfRecursiveComponent(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/components/layout/Layout.tsx",
			Content: `import React from 'react';
export default function Layout() {
  return <Layout><main>Dashboard</main></Layout>;
}`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "renders <Layout> inside itself") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error for self-recursive Layout component, got %d errors: %v", len(errors), errors)
	}
}

// TestValidate_TemplateFilesSkipped — imports from template files should NOT be flagged.
func TestValidate_TemplateFilesSkipped(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/features/users/api.ts",
			Content: `import { useApiQuery, useApiMutation } from '@/hooks/useApi';
import { extractList, extractCount } from '@/lib/apiUtils';
import { cn } from '@/lib/utils';
export function useUsers() { return useApiQuery(['users'], '/v2/items/users'); }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	for _, e := range errors {
		if e.Severity == "error" {
			t.Errorf("template file import flagged as error: [%s] %s", e.File, e.Message)
		}
	}
}

func TestValidate_RuntimeHazards(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/pages/OrdersPage.tsx",
			Content: `import React from 'react';
export default function OrdersPage() {
  const rows = data?.data?.data?.response ?? [];
  return <select>{rows.map((r: any) => <option key={r.guid}>{r.name}</option>)}</select>;
}`,
		},
		{
			Path: "src/pages/FallbackPage.tsx",
			Content: `import React from 'react';
export default function FallbackPage() {
  return <p>This section is temporarily unavailable.</p>;
}`,
		},
		{
			Path: "src/pages/LeadsPage.tsx",
			Content: `import React from 'react';
import { SelectItem } from '@/components/ui/select';
export default function LeadsPage() {
  return <SelectItem value="">All statuses</SelectItem>;
}`,
		},
	}

	errors := validateGeneratedProject(files, nil)

	expectMessages := []string{
		"native <select>",
		"data.data.response",
		"fallback stub",
		"SelectItem value=\"\"",
	}
	for _, msg := range expectMessages {
		found := false
		for _, e := range errors {
			if e.Severity == "error" && contains(e.Message, msg) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected runtime hazard error containing %q, got %v", msg, errors)
		}
	}
}

func TestValidateAdminPanelUIQuality_GenericTableOnly(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/pages/ContactsPage.tsx",
			Content: `import React from 'react';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
export default function ContactsPage() {
  return <div><h1>Contacts</h1><Table><TableHeader><TableRow><TableHead>Name</TableHead></TableRow></TableHeader><TableBody><TableRow><TableCell>Anna</TableCell></TableRow></TableBody></Table></div>;
}`,
		},
	}

	errors := validateAdminPanelUIQuality(files)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "generic table-only CRUD") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected admin UI quality error for table-only CRUD, got %d errors: %v", len(errors), errors)
	}
}

func TestValidateAdminPanelUIQuality_BasicKanban(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/pages/LeadsPage.tsx",
			Content: `import React from 'react';
export default function LeadsPage() {
  return <div><h1>Kanban</h1><section><h2>Discovery</h2><div>Wayne Security Audit</div></section><section><h2>Qualification</h2></section><section><h2>Proposal</h2></section></div>;
}`,
		},
	}

	errors := validateAdminPanelUIQuality(files)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "kanban board is too basic") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected admin UI quality error for basic kanban, got %d errors: %v", len(errors), errors)
	}
}

func TestValidateAdminPanelUIQuality_PremiumTablePasses(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/pages/ContactsPage.tsx",
			Content: `import React from 'react';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
export default function ContactsPage() {
  const isLoading = false;
  return <div>
    <div className="grid grid-cols-4"><Card>Total contacts</Card><Card>Qualified</Card><Card>Needs follow-up</Card><Card>New this week</Card></div>
    <div><Input placeholder="Search" /><Select /></div>
    <Table><TableHeader><TableRow><TableHead>Name</TableHead></TableRow></TableHeader><TableBody>{isLoading ? <TableRow><TableCell><Skeleton /></TableCell></TableRow> : <TableRow className="group hover:bg-muted/40"><TableCell><Badge>Active</Badge><span className="group-hover:opacity-100">Actions</span></TableCell></TableRow>}</TableBody></Table>
    <div>Pagination Previous Next empty state</div>
  </div>;
}`,
		},
	}

	errors := validateAdminPanelUIQuality(files)
	for _, e := range errors {
		if e.Severity == "error" {
			t.Errorf("unexpected admin UI quality error: [%s] %s", e.File, e.Message)
		}
	}
}

// TestValidate_ExportBraces — export { X, Y, Z } pattern detected.
func TestValidate_ExportBraces(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/lib/helpers.ts",
			Content: `function formatDate(d: string) { return d; }
function formatCurrency(n: number) { return n.toString(); }
export { formatDate, formatCurrency };`,
		},
		{
			Path:    "src/pages/ReportPage.tsx",
			Content: `import { formatDate, formatCurrency } from '@/lib/helpers';\nexport default function ReportPage() { return null; }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	for _, e := range errors {
		if e.Severity == "error" {
			t.Errorf("unexpected error for export {} pattern: [%s] %s", e.File, e.Message)
		}
	}
}

// TestValidate_DisplayNamePattern — React.forwardRef with displayName should be detected as export.
func TestValidate_DisplayNamePattern(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/components/ui/input.tsx",
			Content: `import React from 'react';
export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {}
export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, ...props }, ref) => <input ref={ref} {...props} />
);
Input.displayName = 'Input';`,
		},
		{
			Path:    "src/features/users/UserForm.tsx",
			Content: `import { Input, InputProps } from '@/components/ui/input';\nexport function UserForm() { return <Input />; }`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	for _, e := range errors {
		if e.Severity == "error" {
			t.Errorf("unexpected error for forwardRef/displayName: [%s] %s", e.File, e.Message)
		}
	}
}

// TestBuildUIKitAPISummary — verifies the API summary extraction.
func TestBuildUIKitAPISummary(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/components/ui/button.tsx",
			Content: `import React from 'react';
export const buttonVariants = cva('btn');
export interface ButtonProps {}
export const Button = React.forwardRef(() => null);
Button.displayName = 'Button';`,
		},
	}

	summary := buildUIKitAPISummary(files)
	if summary == "" {
		t.Error("expected non-empty UI Kit API summary")
	}
	if !contains(summary, "buttonVariants") {
		t.Error("expected summary to contain 'buttonVariants'")
	}
	if !contains(summary, "ButtonProps") {
		t.Error("expected summary to contain 'ButtonProps'")
	}
	if !contains(summary, "Button") {
		t.Error("expected summary to contain 'Button'")
	}
}

// TestValidateLazyImports_HappyPath — App.tsx uses lazy with named extractor; page exports named — no errors.
func TestValidateLazyImports_HappyPath(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import { lazy, Suspense } from 'react';
const HomePage = lazy(() => import('@/pages/HomePage').then(m => ({ default: m.HomePage })));
export default function App() { return <Suspense><HomePage/></Suspense>; }`,
		},
		{
			Path:    "src/pages/HomePage.tsx",
			Content: `export function HomePage() { return <div>Home</div>; }`,
		},
	}
	errors := validateGeneratedProject(files, nil)
	for _, e := range errors {
		if contains(e.Message, "lazy") {
			t.Errorf("expected no lazy-import errors, got: %s", e.Message)
		}
	}
}

// TestValidateLazyImports_MissingNamedExport — App.tsx expects m.HomePage, file only has default. Should error.
// This is the exact root cause of React error #306 the user reported.
func TestValidateLazyImports_MissingNamedExport(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import { lazy } from 'react';
const HomePage = lazy(() => import('@/pages/HomePage').then(m => ({ default: m.HomePage })));`,
		},
		{
			Path:    "src/pages/HomePage.tsx",
			Content: `export default function HomePage() { return <div>Home</div>; }`,
		},
	}
	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "lazy import expects named export") && contains(e.Message, "HomePage") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected lazy-named-export error for default-only file, got %d errors: %v", len(errors), errors)
	}
}

// TestValidateLazyImports_WrongName — typo in extractor (m.Home vs HomePage). Should error.
func TestValidateLazyImports_WrongName(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import { lazy } from 'react';
const HomePage = lazy(() => import('@/pages/HomePage').then(m => ({ default: m.Home })));`,
		},
		{
			Path:    "src/pages/HomePage.tsx",
			Content: `export function HomePage() { return <div/>; }`,
		},
	}
	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "named export") && contains(e.Message, "Home") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected lazy-import error for m.Home typo, got %d errors: %v", len(errors), errors)
	}
}

// TestValidateLazyImports_DefaultLazy_NoDefault — lazy without .then expects default export.
func TestValidateLazyImports_DefaultLazy_NoDefault(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import { lazy } from 'react';
const HomePage = lazy(() => import('@/pages/HomePage'));`,
		},
		{
			Path:    "src/pages/HomePage.tsx",
			Content: `export function HomePage() { return <div/>; }`,
		},
	}
	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "no default export") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected lazy-without-then default-export error, got %d errors: %v", len(errors), errors)
	}
}

// TestExportDefaultRelaxed_AllFourForms — buildExportRegistry recognises all four export-default shapes.
// All four register `default` only; the identifier inside `export default X` is module-local and
// is NOT exposed as a named export (named import would fail).
func TestExportDefaultRelaxed_AllFourForms(t *testing.T) {
	cases := []struct {
		name    string
		content string
		// expectNamedAlso=true means the identifier ALSO appears as a normal named export
		// (e.g. `export const Foo = ...; export default Foo;`).
		expectNamedAlso bool
		identifier      string
	}{
		{"function form", `export default function Foo() {}`, false, "Foo"},
		{"class form", `export default class Bar {}`, false, "Bar"},
		{"identifier form", `const Baz = () => null; export default Baz;`, false, "Baz"},
		{"memo wrapped", `const Qux = () => null; export default React.memo(Qux);`, false, "Qux"},
		{"named + default", `export const Foo = () => null; export default Foo;`, true, "Foo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := []models.ProjectFile{{Path: "src/X.tsx", Content: c.content}}
			reg := buildExportRegistry(files)
			ex := reg["src/X.tsx"]
			if !ex["default"] {
				t.Errorf("[%s] expected default export to be registered", c.name)
			}
			if c.expectNamedAlso && !ex[c.identifier] {
				t.Errorf("[%s] expected named export %q ALSO registered, got: %v", c.name, c.identifier, ex)
			}
			if !c.expectNamedAlso && ex[c.identifier] {
				t.Errorf("[%s] expected %q NOT registered as named (it is module-local), got: %v", c.name, c.identifier, ex)
			}
		})
	}
}

// TestParseLazyImports_VariousSpacings — regex tolerates whitespace variations.
func TestParseLazyImports_VariousSpacings(t *testing.T) {
	cases := []string{
		`lazy(() => import('@/pages/A').then(m => ({ default: m.A })))`,
		`lazy( ( ) => import( '@/pages/B' ).then( m => ( { default: m.B } ) ) )`,
		`lazy(() => import("@/pages/C"))`, // no .then
		`lazy(async () => import('@/pages/D').then((mod) => ({ default: mod.D })))`,
	}
	expected := []struct{ Path, Name string }{
		{"@/pages/A", "A"},
		{"@/pages/B", "B"},
		{"@/pages/C", ""},
		{"@/pages/D", "D"},
	}
	for i, c := range cases {
		got := parseLazyImports("src/App.tsx", c)
		if len(got) == 0 {
			t.Errorf("[case %d] failed to parse: %q", i, c)
			continue
		}
		if got[0].Path != expected[i].Path {
			t.Errorf("[case %d] path mismatch: want %q got %q", i, expected[i].Path, got[0].Path)
		}
		if got[0].NamedExport != expected[i].Name {
			t.Errorf("[case %d] name mismatch: want %q got %q", i, expected[i].Name, got[0].NamedExport)
		}
	}
}

// TestMergeAppRoutes_RebuildsFromManifest — the merger should overwrite App.tsx
// with the canonical lazy + Route shape derived from manifest.Routes + actual page exports.
func TestMergeAppRoutes_RebuildsFromManifest(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// will be overwritten`},
		{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() { return <div/>; }`},
		{Path: "src/pages/AboutPage.tsx", Content: `export function AboutPage() { return <div/>; }`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes: []models.ManifestRoute{
			{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"},
			{Path: "/about", PageName: "AboutPage", FilePath: "src/pages/AboutPage.tsx"},
		},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
			break
		}
	}
	if !contains(app, "m.HomePage") || !contains(app, "m.AboutPage") {
		t.Errorf("App.tsx missing expected lazy named extractors:\n%s", app)
	}
	if !contains(app, `path="/"`) || !contains(app, `path="/about"`) {
		t.Errorf("App.tsx missing expected routes:\n%s", app)
	}
}

// TestMergeAppRoutes_SkipsLegacyManifest — without ExportStyle="named-lazy" the merger no-ops.
func TestMergeAppRoutes_SkipsLegacyManifest(t *testing.T) {
	original := `legacy App.tsx untouched`
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: original},
		{Path: "src/pages/HomePage.tsx", Content: `export default function HomePage() {}`},
	}
	manifest := &models.ProjectManifest{
		Routes: []models.ManifestRoute{{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"}},
		// ExportStyle deliberately empty → legacy.
	}
	got := mergeAppRoutes(files, manifest)
	for _, f := range got {
		if f.Path == "src/App.tsx" && f.Content != original {
			t.Errorf("legacy App.tsx was mutated. before=%q after=%q", original, f.Content)
		}
	}
}

// TestMergeAppRoutes_DropsRoutesWithMissingExports — if a page file lacks the named
// export the route refers to, merger must skip it rather than emit a broken App.tsx.
func TestMergeAppRoutes_DropsRoutesWithMissingExports(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// will be overwritten`},
		{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
		// AboutPage exports nothing useful — should be skipped.
		{Path: "src/pages/AboutPage.tsx", Content: `export default function AboutPage() {}`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes: []models.ManifestRoute{
			{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"},
			{Path: "/about", PageName: "AboutPage", FilePath: "src/pages/AboutPage.tsx"},
		},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
		}
	}
	if !contains(app, "m.HomePage") {
		t.Errorf("expected HomePage to be present; got:\n%s", app)
	}
	if contains(app, "m.AboutPage") {
		t.Errorf("AboutPage should be dropped (no named export); got:\n%s", app)
	}
}

// TestMergeAppRoutes_WrapsInLayoutOutlet — admin Layout uses <Outlet />; merger
// must emit parent-route pattern so the sidebar/header shell paints around pages.
func TestMergeAppRoutes_WrapsInLayoutOutlet(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// overwritten`},
		{Path: "src/components/layout/Layout.tsx", Content: `import { Outlet } from 'react-router-dom';
export default function Layout() { return <main><Outlet /></main>; }`},
		{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes:      []models.ManifestRoute{{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"}},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
		}
	}
	if !contains(app, "import Layout from '@/components/layout/Layout'") {
		t.Errorf("expected Layout import; got:\n%s", app)
	}
	if !contains(app, "<Route element={<Layout />}>") {
		t.Errorf("expected outlet parent-route wrapping; got:\n%s", app)
	}
}

// TestMergeAppRoutes_WrapsInLayoutChildren — website Layout uses { children };
// merger must emit <Layout> wrapping <Routes>.
func TestMergeAppRoutes_WrapsInLayoutChildren(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// overwritten`},
		{Path: "src/components/layout/Layout.tsx", Content: `export default function Layout({ children }: { children: React.ReactNode }) { return <div>{children}</div>; }`},
		{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes:      []models.ManifestRoute{{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"}},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
		}
	}
	if !contains(app, "import Layout from '@/components/layout/Layout'") {
		t.Errorf("expected Layout import; got:\n%s", app)
	}
	if !contains(app, "<Layout>") || !contains(app, "</Layout>") {
		t.Errorf("expected <Layout> wrapper around <Routes>; got:\n%s", app)
	}
	if contains(app, "<Route element={<Layout />}>") {
		t.Errorf("children-Layout must not use outlet pattern; got:\n%s", app)
	}
}

// TestMergeAppRoutes_NoLayoutNoWrap — landing-style projects have no Layout;
// merger must fall back to the unwrapped template instead of inventing imports.
func TestMergeAppRoutes_NoLayoutNoWrap(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// overwritten`},
		{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes:      []models.ManifestRoute{{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"}},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
		}
	}
	if contains(app, "from '@/components/layout/Layout'") {
		t.Errorf("must not import non-existent Layout; got:\n%s", app)
	}
	if contains(app, "<Layout") {
		t.Errorf("must not reference Layout when file is absent; got:\n%s", app)
	}
}

// TestEnsureDefaultRoutes_InjectsRootAndDashboard — Nexus-ERP scenario: the
// architect drops Dashboard from the manifest entirely but DashboardPage.tsx
// still gets generated. Sidebar links to "/dashboard" → blank page in prod.
// Merger must rescue: synthesize "/" + "/dashboard" pointing at the orphan page.
func TestEnsureDefaultRoutes_InjectsRootAndDashboard(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// overwritten`},
		{Path: "src/pages/DashboardPage.tsx", Content: `export function DashboardPage() {}`},
		{Path: "src/pages/OrdersPage.tsx", Content: `export function OrdersPage() {}`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes: []models.ManifestRoute{
			// Note: DashboardPage is orphaned — file exists but no route.
			{Path: "/orders", PageName: "OrdersPage", FilePath: "src/pages/OrdersPage.tsx"},
		},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
		}
	}
	if !contains(app, `path="/"`) {
		t.Errorf("expected synthesized root route; got:\n%s", app)
	}
	if !contains(app, `path="/dashboard"`) {
		t.Errorf("expected synthesized /dashboard route for orphan DashboardPage; got:\n%s", app)
	}
}

// TestEnsureDefaultRoutes_RespectsExistingDashboardMapping — when architect
// deliberately maps DashboardPage to a non-"/dashboard" path, merger must NOT
// fight that choice by inventing a duplicate route.
func TestEnsureDefaultRoutes_RespectsExistingDashboardMapping(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/App.tsx", Content: `// overwritten`},
		{Path: "src/pages/DashboardPage.tsx", Content: `export function DashboardPage() {}`},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes: []models.ManifestRoute{
			{Path: "/overview", PageName: "DashboardPage", FilePath: "src/pages/DashboardPage.tsx"},
		},
	}
	got := mergeAppRoutes(files, manifest)
	var app string
	for _, f := range got {
		if f.Path == "src/App.tsx" {
			app = f.Content
		}
	}
	if contains(app, `path="/dashboard"`) {
		t.Errorf("must not invent /dashboard when DashboardPage already has a route; got:\n%s", app)
	}
}

// TestMergeAppRoutes_EmitsWildcardCatch — every shape (outlet, children, none)
// must end with <Route path="*" .../> so typos and stale sidebar links redirect
// to "/" instead of rendering a blank screen.
func TestMergeAppRoutes_EmitsWildcardCatch(t *testing.T) {
	cases := []struct {
		name  string
		files []models.ProjectFile
	}{
		{
			name: "outlet",
			files: []models.ProjectFile{
				{Path: "src/App.tsx", Content: ``},
				{Path: "src/components/layout/Layout.tsx", Content: `import { Outlet } from 'react-router-dom'; export default function Layout() { return <Outlet />; }`},
				{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
			},
		},
		{
			name: "children",
			files: []models.ProjectFile{
				{Path: "src/App.tsx", Content: ``},
				{Path: "src/components/layout/Layout.tsx", Content: `export default function Layout({ children }) { return <div>{children}</div>; }`},
				{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
			},
		},
		{
			name: "no-layout",
			files: []models.ProjectFile{
				{Path: "src/App.tsx", Content: ``},
				{Path: "src/pages/HomePage.tsx", Content: `export function HomePage() {}`},
			},
		},
	}
	manifest := &models.ProjectManifest{
		ExportStyle: "named-lazy",
		Routes:      []models.ManifestRoute{{Path: "/", PageName: "HomePage", FilePath: "src/pages/HomePage.tsx"}},
	}
	for _, c := range cases {
		got := mergeAppRoutes(c.files, manifest)
		var app string
		for _, f := range got {
			if f.Path == "src/App.tsx" {
				app = f.Content
			}
		}
		if !contains(app, `path="*"`) || !contains(app, `<Navigate to="/" replace />`) {
			t.Errorf("[%s] missing wildcard fallback route; got:\n%s", c.name, app)
		}
		if !contains(app, "Navigate") || !contains(app, "react-router-dom") {
			t.Errorf("[%s] Navigate must be imported from react-router-dom; got:\n%s", c.name, app)
		}
	}
}

// TestValidateLayoutShape_FlagsBareLayout — Layout that neither uses Outlet
// nor accepts children leaves the app shell empty; validator must catch it.
func TestValidateLayoutShape_FlagsBareLayout(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/components/layout/Layout.tsx", Content: `export default function Layout() { return <aside>menu</aside>; }`},
	}
	errs := validateLayoutShape(files)
	if len(errs) == 0 {
		t.Errorf("expected validator to flag Outlet-less Layout, got no errors")
	}
}

// TestValidateLayoutShape_AcceptsOutletAndChildren — both contracts are valid.
func TestValidateLayoutShape_AcceptsOutletAndChildren(t *testing.T) {
	cases := []string{
		`import { Outlet } from 'react-router-dom'; export default function Layout() { return <main><Outlet /></main>; }`,
		`export default function Layout({ children }) { return <div>{children}</div>; }`,
	}
	for i, content := range cases {
		errs := validateLayoutShape([]models.ProjectFile{{Path: "src/components/layout/Layout.tsx", Content: content}})
		if len(errs) != 0 {
			t.Errorf("[case %d] expected no errors, got %v", i, errs)
		}
	}
}

// TestDedupTsTsxPairs_DropsShadowedTs — the bug class from the portfolio
// landing report: Vite picks .tsx, the .ts shadow drifts in exports, repair
// loop chases its tail. Dedup must drop the .ts when .tsx of same stem exists.
func TestDedupTsTsxPairs_DropsShadowedTs(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/hooks/use-mobile.ts", Content: `export function useMobile() { return false; }`},
		{Path: "src/hooks/use-mobile.tsx", Content: `export function useIsMobile() { return false; }`},
		{Path: "src/components/sections/Community.tsx", Content: `import { useIsMobile } from '@/hooks/use-mobile';`},
	}
	out := dedupTsTsxPairs(files)
	for _, f := range out {
		if f.Path == "src/hooks/use-mobile.ts" {
			t.Errorf("expected use-mobile.ts to be dropped (shadowed by .tsx); still present")
		}
	}
	foundTsx := false
	for _, f := range out {
		if f.Path == "src/hooks/use-mobile.tsx" {
			foundTsx = true
		}
	}
	if !foundTsx {
		t.Errorf("expected use-mobile.tsx to survive; got %d files", len(out))
	}
}

// TestDedupTsTsxPairs_KeepsTsWhenNoTsxSibling — pure utility files in .ts
// without a .tsx counterpart must pass through untouched.
func TestDedupTsTsxPairs_KeepsTsWhenNoTsxSibling(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/lib/utils.ts", Content: `export function cn() {}`},
		{Path: "src/types.ts", Content: `export interface Foo {}`},
		{Path: "src/App.tsx", Content: ``},
	}
	out := dedupTsTsxPairs(files)
	if len(out) != 3 {
		t.Errorf("expected all 3 files to survive (no .tsx shadows present); got %d", len(out))
	}
}

// TestDedupTsTsxPairs_OnlyMatchesExactStem — `use-mobile.ts` must NOT be
// dropped by the unrelated `use-mobile-state.tsx` (different stem).
func TestDedupTsTsxPairs_OnlyMatchesExactStem(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/hooks/use-mobile.ts", Content: `export function useMobile() {}`},
		{Path: "src/hooks/use-mobile-state.tsx", Content: `export function useMobileState() {}`},
	}
	out := dedupTsTsxPairs(files)
	if len(out) != 2 {
		t.Errorf("expected both files to survive (stems don't match); got %d", len(out))
	}
}

func TestValidateWebAppUIQualityRejectsTransparentOverlayHeader(t *testing.T) {
	files := []models.ProjectFile{{
		Path: "src/components/layout/Header.tsx",
		Content: `export function Header() {
  return <header className="fixed top-0 bg-background/80 backdrop-blur pt-[max(env(safe-area-inset-top),3rem)]">Header</header>
}`,
	}}

	errors := validateWebAppUIQuality(files)
	var messages []string
	for _, validationError := range errors {
		messages = append(messages, validationError.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, expected := range []string{"SOLID opaque background", "sticky and remain in normal layout flow"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("expected %q validation error, got: %s", expected, joined)
		}
	}
}

func TestValidateWebAppUIQualityAcceptsSolidStickyHeader(t *testing.T) {
	files := []models.ProjectFile{{
		Path: "src/components/layout/Header.tsx",
		Content: `export function Header() {
  return <header className="sticky top-0 bg-background pt-[max(env(safe-area-inset-top),3rem)]">Header</header>
}`,
	}}

	if errors := validateWebAppUIQuality(files); len(errors) != 0 {
		t.Fatalf("expected solid sticky Header to pass, got: %+v", errors)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
