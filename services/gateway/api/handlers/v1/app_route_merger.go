package v1

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// resolvedRoute is a manifest route that has been verified against the
// generated file set: the file exists and the named export is present.
type resolvedRoute struct {
	Path       string
	PageName   string
	FilePath   string
	ImportFrom string // value used inside lazy(import('...')), without .tsx, with @/ prefix
}

// layoutBinding describes how Layout.tsx should be imported and used to wrap
// routes. Detected from the generated Layout file content + export registry.
type layoutBinding struct {
	ImportLine string // e.g. `import Layout from '@/components/layout/Layout';`
	UsesOutlet bool   // true → parent-route pattern; false → children wrapper
}

// detectLayoutBinding inspects the generated Layout file (if any) and decides
// how App.tsx should integrate it. Two valid Layout contracts (both produced
// by our prompts):
//
//   - Outlet pattern:  export default function Layout() { return <main><Outlet /></main>; }
//     → App.tsx wraps with parent route: <Route element={<Layout />}><Route ... /></Route>
//   - Children pattern: export default function Layout({ children }) { return <div>{children}</div>; }
//     → App.tsx wraps routes:           <Layout><Routes><Route ... /></Routes></Layout>
//
// Returns nil when Layout is absent or has no usable export; caller falls back
// to the unwrapped template (correct for landing/single-page projects).
func detectLayoutBinding(files []models.ProjectFile, registry map[string]map[string]bool) *layoutBinding {
	const layoutPath = "src/components/layout/Layout.tsx"

	exports, ok := registry[layoutPath]
	if !ok {
		return nil
	}
	var layoutContent string
	for _, f := range files {
		if f.Path == layoutPath {
			layoutContent = f.Content
			break
		}
	}
	if layoutContent == "" {
		return nil
	}

	var importLine string
	switch {
	case exports["default"]:
		importLine = "import Layout from '@/components/layout/Layout';\n"
	case exports["Layout"]:
		importLine = "import { Layout } from '@/components/layout/Layout';\n"
	default:
		return nil
	}

	// Outlet wins when both signals are present — it's the only correct fit
	// for React Router v6 nested routes regardless of whether `children` also
	// appears in unrelated typing.
	usesOutlet := strings.Contains(layoutContent, "<Outlet")
	return &layoutBinding{ImportLine: importLine, UsesOutlet: usesOutlet}
}

// dedupTsTsxPairs removes `X.ts` when `X.tsx` exists with the same stem.
// Vite resolves `import 'foo'` to `foo.tsx` first, then `foo.ts`, so a stray
// `.ts` shadow file is dead code that confuses both validators and repair LLMs:
// they cannot tell which version is canonical, may patch the wrong one, and
// the build keeps failing because Vite still imports the unmodified `.tsx`.
//
// Concrete failure this prevents (real user report, portfolio landing):
// Claude generated both `use-mobile.ts` (exporting `useMobile`) and
// `use-mobile.tsx` (exporting `useIsMobile`). Community.tsx imported
// `useMobile`, Vite picked `.tsx`, and every repair pass produced "Done"
// while the same error returned. The fix is structural: the .ts shadow must
// go before validation, so the repair sees a single source of truth.
func dedupTsTsxPairs(files []models.ProjectFile) []models.ProjectFile {
	tsxPaths := make(map[string]bool, len(files))
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".tsx") {
			tsxPaths[strings.TrimSuffix(f.Path, ".tsx")] = true
		}
	}
	out := files[:0]
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".ts") {
			stem := strings.TrimSuffix(f.Path, ".ts")
			if tsxPaths[stem] {
				log.Printf("[dedup] dropping %s — shadowed by %s.tsx", f.Path, stem)
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// stripPageTsx drops `src/Page.tsx` from the file list. The model sometimes
// invents it as a microfrontend entry with MemoryRouter, which conflicts with
// the federation contract (`./App` → `./src/App.tsx`) and steals `import './index.css'`.
func stripPageTsx(files []models.ProjectFile) []models.ProjectFile {
	out := files[:0]
	for _, f := range files {
		if f.Path == "src/Page.tsx" {
			log.Printf("[merge-routes] dropping stray src/Page.tsx (federation entry is src/App.tsx)")
			continue
		}
		out = append(out, f)
	}
	return out
}

// ensureDefaultRoutes guarantees that the resolved set contains a root ("/")
// route and — when a Dashboard page exists — a "/dashboard" route. The architect
// LLM is told to include these but occasionally drops them; without "/" the app
// renders blank on first load, and sidebars that link to "/dashboard" 404.
//
// Strategy:
//   - Pick the best landing candidate among already-resolved pages, preferring
//     Dashboard > Home > Index > first available.
//   - If no "/" exists, alias the best candidate as "/".
//   - If a *DashboardPage export exists somewhere but no route reaches it,
//     synthesize "/dashboard".
//
// Only existing pages are routed — we never invent a file.
func ensureDefaultRoutes(resolved []resolvedRoute, files []models.ProjectFile, registry map[string]map[string]bool) []resolvedRoute {
	hasRoot := false
	hasDashboardPath := false
	for _, r := range resolved {
		if r.Path == "/" {
			hasRoot = true
		}
		if r.Path == "/dashboard" {
			hasDashboardPath = true
		}
	}

	score := func(name string) int {
		lower := strings.ToLower(name)
		switch {
		case strings.HasPrefix(lower, "dashboard"):
			return 3
		case strings.HasPrefix(lower, "home"):
			return 2
		case strings.HasPrefix(lower, "index"), strings.HasPrefix(lower, "main"):
			return 1
		}
		return 0
	}

	var best *resolvedRoute
	bestScore := -1
	for i := range resolved {
		s := score(resolved[i].PageName)
		if s > bestScore {
			bestScore = s
			best = &resolved[i]
		}
	}

	if !hasRoot && best != nil {
		resolved = append(resolved, resolvedRoute{
			Path:       "/",
			PageName:   best.PageName,
			FilePath:   best.FilePath,
			ImportFrom: best.ImportFrom,
		})
		log.Printf("[merge-routes] no '/' in manifest; aliasing %s as root", best.PageName)
	}

	if !hasDashboardPath {
		dashboardRouted := false
		for _, r := range resolved {
			if strings.HasPrefix(strings.ToLower(r.PageName), "dashboard") {
				dashboardRouted = true
				break
			}
		}
		if !dashboardRouted {
			// Map iteration is non-deterministic, so prefer specific names by priority:
			// "DashboardPage" first, then "Dashboard", then any Dashboard* fallback.
			// Without this the same project could route /dashboard to DashboardWidget
			// vs DashboardPage between runs.
			pickDashboardExport := func(exports map[string]bool) string {
				if exports["DashboardPage"] {
					return "DashboardPage"
				}
				if exports["Dashboard"] {
					return "Dashboard"
				}
				// Sort candidates for deterministic fallback.
				var candidates []string
				for name := range exports {
					if strings.HasPrefix(strings.ToLower(name), "dashboard") {
						candidates = append(candidates, name)
					}
				}
				if len(candidates) == 0 {
					return ""
				}
				sort.Strings(candidates)
				return candidates[0]
			}

			for _, f := range files {
				name := pickDashboardExport(registry[f.Path])
				if name == "" {
					continue
				}
				importFrom := strings.TrimSuffix(f.Path, ".tsx")
				importFrom = strings.TrimSuffix(importFrom, ".ts")
				importFrom = "@/" + strings.TrimPrefix(importFrom, "src/")
				resolved = append(resolved, resolvedRoute{
					Path:       "/dashboard",
					PageName:   name,
					FilePath:   f.Path,
					ImportFrom: importFrom,
				})
				log.Printf("[merge-routes] no '/dashboard' in manifest; routing %s from %s", name, f.Path)
				dashboardRouted = true
				break
			}
		}
	}

	return resolved
}

// writeRoutes emits the <Routes>...</Routes> block, optionally wrapped in Layout,
// always terminated by a wildcard "*" route that redirects unknown paths to "/".
// The wildcard is the safety net for stale sidebar links, typo navigation, and
// missing /dashboard scenarios — without it those land on a blank screen.
//
//	layout==nil          → flat routes  (landing / no-shell projects)
//	layout.UsesOutlet    → nested parent route around all children
//	!layout.UsesOutlet   → <Layout> wraps the <Routes> block
func writeRoutes(sb *strings.Builder, resolved []resolvedRoute, layout *layoutBinding) {
	const indent = "          "
	const wildcard = `<Route path="*" element={<Navigate to="/" replace />} />`

	emitPageRoutes := func(prefix string) {
		for _, r := range resolved {
			fmt.Fprintf(sb, "%s<Route path=%q element={<%s />} />\n", prefix, r.Path, r.PageName)
		}
	}

	switch {
	case layout == nil:
		fmt.Fprintf(sb, "%s<Routes>\n", indent)
		emitPageRoutes(indent + "  ")
		fmt.Fprintf(sb, "%s  %s\n", indent, wildcard)
		fmt.Fprintf(sb, "%s</Routes>\n", indent)
	case layout.UsesOutlet:
		fmt.Fprintf(sb, "%s<Routes>\n", indent)
		fmt.Fprintf(sb, "%s  <Route element={<Layout />}>\n", indent)
		emitPageRoutes(indent + "    ")
		fmt.Fprintf(sb, "%s  </Route>\n", indent)
		fmt.Fprintf(sb, "%s  %s\n", indent, wildcard)
		fmt.Fprintf(sb, "%s</Routes>\n", indent)
	default:
		fmt.Fprintf(sb, "%s<Layout>\n", indent)
		fmt.Fprintf(sb, "%s  <Routes>\n", indent)
		emitPageRoutes(indent + "    ")
		fmt.Fprintf(sb, "%s    %s\n", indent, wildcard)
		fmt.Fprintf(sb, "%s  </Routes>\n", indent)
		fmt.Fprintf(sb, "%s</Layout>\n", indent)
	}
}

const pageLoaderSource = `export function PageLoader() {
  return (
    <div className="flex h-screen w-full items-center justify-center">
      <div className="h-10 w-10 animate-spin rounded-full border-4 border-muted border-t-primary" />
    </div>
  );
}
`

// mergeAppRoutes deterministically rebuilds src/App.tsx from the manifest
// after parallel chunk generation has produced all page files.
//
// Why: Foundation emits App.tsx BEFORE the page files exist, so it has to
// guess at lazy-import names. A single typo (Home vs HomePage, default vs
// named) crashes the runtime with React error #306 on navigation. Rebuilding
// from manifest.Routes + real page exports as ground truth eliminates the
// whole class of bugs.
//
// Gated on manifest.ExportStyle == "named-lazy". Legacy manifests fall through.
func mergeAppRoutes(files []models.ProjectFile, manifest *models.ProjectManifest) []models.ProjectFile {
	if manifest == nil || manifest.ExportStyle != "named-lazy" {
		return files
	}
	if len(manifest.Routes) == 0 {
		return files
	}

	// Strip any stray src/Page.tsx the model invented as an alternate microfrontend entry.
	// Vite federation exposes './App' → './src/App.tsx'; Page.tsx is dead code and
	// occasionally absorbs `import './index.css'`, which then never reaches the host bundle.
	files = stripPageTsx(files)

	appIdx := -1
	for i, f := range files {
		if f.Path == "src/App.tsx" {
			appIdx = i
			break
		}
	}
	if appIdx == -1 {
		log.Printf("[merge-routes] src/App.tsx not found, skipping")
		return files
	}

	registry := buildExportRegistry(files)

	var resolved []resolvedRoute
	var skipped []string

	for _, route := range manifest.Routes {
		filePath := route.FilePath
		if filePath == "" {
			skipped = append(skipped, fmt.Sprintf("%s (no file_path)", route.Path))
			continue
		}
		exports, ok := registry[filePath]
		if !ok {
			for _, alt := range resolveAlternatives(filePath) {
				if found, exists := registry[alt]; exists {
					exports = found
					filePath = alt
					ok = true
					break
				}
			}
			if !ok {
				skipped = append(skipped, fmt.Sprintf("%s (file %s missing)", route.Path, route.FilePath))
				continue
			}
		}
		if !exports[route.PageName] {
			skipped = append(skipped, fmt.Sprintf("%s (file exists but missing named export %s)", route.Path, route.PageName))
			continue
		}

		importFrom := strings.TrimSuffix(filePath, ".tsx")
		importFrom = strings.TrimSuffix(importFrom, ".ts")
		importFrom = "@/" + strings.TrimPrefix(importFrom, "src/")

		resolved = append(resolved, resolvedRoute{
			Path:       route.Path,
			PageName:   route.PageName,
			FilePath:   filePath,
			ImportFrom: importFrom,
		})
	}

	if len(resolved) == 0 {
		log.Printf("[merge-routes] no routes resolved against generated files, leaving App.tsx as-is (skipped: %v)", skipped)
		return files
	}

	resolved = ensureDefaultRoutes(resolved, files, registry)

	layout := detectLayoutBinding(files, registry)

	var sb strings.Builder
	// index.css must be imported here — App.tsx is the federation entry (vite.config.ts
	// exposes './App'). Without this the host loads the remote without Tailwind variables.
	sb.WriteString("import './index.css';\n")
	sb.WriteString("import { lazy, Suspense } from 'react';\n")
	sb.WriteString("import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';\n")
	sb.WriteString("import { AppProviders } from '@/components/shared/AppProviders';\n")
	sb.WriteString("import { PageLoader } from '@/components/shared/PageLoader';\n")
	if layout != nil {
		sb.WriteString(layout.ImportLine)
	}
	sb.WriteString("\n")

	emittedLazy := make(map[string]bool, len(resolved))
	for _, route := range resolved {
		if emittedLazy[route.PageName] {
			continue
		}
		emittedLazy[route.PageName] = true
		fmt.Fprintf(&sb, "const %s = lazy(() => import('%s').then((m) => ({ default: m.%s })));\n",
			route.PageName, route.ImportFrom, route.PageName)
	}

	sb.WriteString("\nexport default function App() {\n")
	sb.WriteString("  return (\n")
	sb.WriteString("    <BrowserRouter>\n")
	sb.WriteString("      <AppProviders>\n")
	sb.WriteString("        <Suspense fallback={<PageLoader />}>\n")
	writeRoutes(&sb, resolved, layout)
	sb.WriteString("        </Suspense>\n")
	sb.WriteString("      </AppProviders>\n")
	sb.WriteString("    </BrowserRouter>\n")
	sb.WriteString("  );\n")
	sb.WriteString("}\n")

	files[appIdx].Content = sb.String()

	// Foundation occasionally drops PageLoader; the rebuilt App.tsx imports it, so re-inject if missing.
	if _, ok := registry["src/components/shared/PageLoader.tsx"]; !ok {
		files = append(files, models.ProjectFile{
			Path:    "src/components/shared/PageLoader.tsx",
			Content: pageLoaderSource,
		})
	}

	layoutMode := "none"
	if layout != nil {
		if layout.UsesOutlet {
			layoutMode = "outlet"
		} else {
			layoutMode = "children"
		}
	}
	log.Printf("[merge-routes] rebuilt App.tsx with %d routes, layout=%s (skipped %d: %v)", len(resolved), layoutMode, len(skipped), skipped)
	return files
}
