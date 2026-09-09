package v1

import (
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// buildProjectSummary produces a rich Markdown message shown to the user after
// project generation. All data comes from plan + files — zero extra API calls.
func buildProjectSummary(plan *models.ArchitectPlan, files []models.ProjectFile, durationSec int) string {
	var sb strings.Builder

	sb.WriteString(summaryHeader(plan))
	sb.WriteString(summaryStats(files, plan.ProjectType, durationSec))

	if line := summaryDesign(plan.Design); line != "" {
		sb.WriteString("\n")
		sb.WriteString(line)
	}

	if plan.ProjectType != "landing" && len(plan.Tables) > 0 {
		sb.WriteString("\n")
		sb.WriteString(summaryDatabase(plan))
	}

	sb.WriteString("\n")
	sb.WriteString(summaryPages(plan, files))

	if plan.ProjectType == mobileProjectType {
		fmt.Fprintf(&sb, "\n⚙️ React 18 · TypeScript · Tailwind CSS · Vite · Capacitor %s\n", capacitorRuntimeVersion)
	} else {
		sb.WriteString("\n⚙️ React 18 · TypeScript · Tailwind CSS · Vite\n")
	}

	if len(plan.ClientTypes) > 0 {
		fmt.Fprintf(&sb, "👥 **Роли:** %s\n", strings.Join(plan.ClientTypes, ", "))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// ── Summary sub-builders ──────────────────────────────────────────────────────

func summaryHeader(plan *models.ArchitectPlan) string {
	switch plan.ProjectType {
	case "admin_panel":
		return fmt.Sprintf("✅ **%s** сгенерирован и готов к работе!\n", plan.ProjectName)
	case "web":
		return fmt.Sprintf("✅ **%s** — сайт готов!\n", plan.ProjectName)
	case "webapp":
		return fmt.Sprintf("✅ **%s** — мобильное приложение готово!\n", plan.ProjectName)
	case mobileProjectType:
		return fmt.Sprintf("✅ **%s** — устанавливаемое мобильное приложение готово!\n", plan.ProjectName)
	default:
		return fmt.Sprintf("✅ **%s** — лендинг готов!\n", plan.ProjectName)
	}
}

func summaryStats(files []models.ProjectFile, projectType string, durationSec int) string {
	pages, components, hooks := categorizeFiles(files)

	parts := []string{fmt.Sprintf("%d файлов", len(files))}
	if components > 0 {
		parts = append(parts, fmt.Sprintf("%d компонентов", components))
	}
	if pages > 0 {
		label := "страниц"
		if projectType == "admin_panel" {
			label = "модулей"
		} else if projectType == mobileProjectType {
			label = "экранов"
		}
		parts = append(parts, fmt.Sprintf("%d %s", pages, label))
	}
	if hooks > 0 {
		parts = append(parts, fmt.Sprintf("%d хуков", hooks))
	}
	switch {
	case durationSec >= 60:
		parts = append(parts, fmt.Sprintf("⏱ %d мин %d сек", durationSec/60, durationSec%60))
	case durationSec > 0:
		parts = append(parts, fmt.Sprintf("⏱ %d сек", durationSec))
	}

	return fmt.Sprintf("📊 %s\n", strings.Join(parts, " · "))
}

func summaryDesign(d models.DesignSpec) string {
	if d.DesignInspiration == "" {
		return ""
	}
	parts := []string{slugToTitle(d.DesignInspiration)}
	switch {
	case d.FontFamily != "" && d.BodyFont != "" && d.FontFamily != d.BodyFont:
		parts = append(parts, d.FontFamily+" / "+d.BodyFont)
	case d.FontFamily != "":
		parts = append(parts, d.FontFamily)
	}
	if d.PrimaryColor != "" {
		parts = append(parts, d.PrimaryColor)
	}
	return fmt.Sprintf("🎨 **Дизайн** — %s\n", strings.Join(parts, " · "))
}

func summaryDatabase(plan *models.ArchitectPlan) string {
	var sb strings.Builder

	var loginTable *models.TablePlan
	var userTables []models.TablePlan
	slugLabel := make(map[string]string, len(plan.Tables))
	mockCount := 0

	for i, t := range plan.Tables {
		slugLabel[t.Slug] = t.Label
		if len(t.MockData) > 0 {
			mockCount++
		}
		if t.IsLoginTable {
			loginTable = &plan.Tables[i]
		} else {
			userTables = append(userTables, t)
		}
	}

	if loginTable != nil {
		fmt.Fprintf(&sb, "🗄 **База данных** — %d таблиц, авторизация через *%s*\n", len(plan.Tables), loginTable.Label)
	} else {
		fmt.Fprintf(&sb, "🗄 **База данных** — %d таблиц\n", len(plan.Tables))
	}

	for _, t := range userTables {
		labels := tableFieldLabels(t.Fields, 5)
		if len(labels) > 0 {
			fmt.Fprintf(&sb, "• **%s** — %s\n", t.Label, strings.Join(labels, ", "))
		} else {
			fmt.Fprintf(&sb, "• **%s**\n", t.Label)
		}
	}
	if loginTable != nil {
		fmt.Fprintf(&sb, "• **%s** — авторизация (login, email, password)\n", loginTable.Label)
	}

	if len(plan.Relations) > 0 {
		sb.WriteString(summaryRelations(plan.Relations, slugLabel))
	}
	if mockCount > 0 {
		fmt.Fprintf(&sb, "📋 Тестовые данные загружены в %d таблицах\n", mockCount)
	}

	return sb.String()
}

// tableFieldLabels returns up to max field labels with an overflow "+N" suffix.
func tableFieldLabels(fields []models.TableFieldPlan, max int) []string {
	labels := make([]string, 0, max+1)
	for i, f := range fields {
		if i >= max {
			labels = append(labels, fmt.Sprintf("+%d", len(fields)-max))
			break
		}
		labels = append(labels, f.Label)
	}
	return labels
}

// summaryRelations formats FK relations as "*Orders* → Customers, Products".
func summaryRelations(relations []models.TableRelationPlan, slugLabel map[string]string) string {
	grouped := make(map[string][]string)
	order := make([]string, 0, len(relations))
	seen := make(map[string]bool)

	for _, r := range relations {
		toLabel := slugLabel[r.TableTo]
		if toLabel == "" {
			toLabel = r.TableTo
		}
		grouped[r.TableFrom] = append(grouped[r.TableFrom], toLabel)
		if !seen[r.TableFrom] {
			seen[r.TableFrom] = true
			order = append(order, r.TableFrom)
		}
	}

	parts := make([]string, 0, len(order))
	for _, from := range order {
		fromLabel := slugLabel[from]
		if fromLabel == "" {
			fromLabel = from
		}
		parts = append(parts, fmt.Sprintf("*%s* → %s", fromLabel, strings.Join(grouped[from], ", ")))
	}
	return fmt.Sprintf("🔗 %s\n", strings.Join(parts, " · "))
}

func summaryPages(plan *models.ArchitectPlan, files []models.ProjectFile) string {
	var sb strings.Builder
	switch plan.ProjectType {
	case "landing":
		sections := parseSectionsFromUIStructure(plan.UIStructure)
		if len(sections) == 0 {
			sections = extractComponentNames(files, []string{"/sections/", "/components/sections/", "/blocks/"})
		}
		if len(sections) > 0 {
			fmt.Fprintf(&sb, "🖥 **Секции** (%d)\n", len(sections))
			for _, s := range sections {
				fmt.Fprintf(&sb, "• %s\n", s)
			}
		}
	case "webapp":
		names := extractProjectPages(files)
		if len(names) == 0 {
			names = parseSectionsFromUIStructure(plan.UIStructure)
		}
		if len(names) > 0 {
			fmt.Fprintf(&sb, "📱 **Экраны** (%d)\n%s\n", len(names), strings.Join(names, " · "))
		}
	case mobileProjectType:
		names := extractProjectPages(files)
		if len(names) == 0 {
			names = parseSectionsFromUIStructure(plan.UIStructure)
		}
		if len(names) > 0 {
			fmt.Fprintf(&sb, "📱 **Экраны** (%d)\n%s\n", len(names), strings.Join(names, " · "))
		}
	case "web":
		names := extractProjectPages(files)
		if len(names) == 0 {
			names = parseSectionsFromUIStructure(plan.UIStructure)
		}
		if len(names) > 0 {
			fmt.Fprintf(&sb, "🖥 **Страницы** (%d)\n%s\n", len(names), strings.Join(names, " · "))
		}
	default: // admin_panel
		names := extractProjectPages(files)
		if len(names) > 0 {
			fmt.Fprintf(&sb, "🖥 **Модули** (%d)\n%s\n", len(names), strings.Join(names, " · "))
		}
	}
	return sb.String()
}

// ── File analysis helpers ─────────────────────────────────────────────────────

// categorizeFiles counts .tsx page files, other .tsx components, and .ts hook files.
func categorizeFiles(files []models.ProjectFile) (pages, components, hooks int) {
	for _, f := range files {
		switch {
		case strings.HasSuffix(f.Path, ".tsx") && (strings.Contains(f.Path, "/pages/") || strings.Contains(f.Path, "/screens/")):
			pages++
		case strings.HasSuffix(f.Path, ".tsx"):
			components++
		case strings.HasSuffix(f.Path, ".ts") && strings.Contains(f.Path, "/hooks/"):
			hooks++
		}
	}
	return
}

// extractProjectPages returns human-readable page names from src/pages/*Page.tsx.
func extractProjectPages(files []models.ProjectFile) []string {
	skip := map[string]bool{
		"": true, "index": true, "Index": true,
		"NotFound": true, "Error": true, "Loading": true,
		"Login": true, "Auth": true, "Register": true,
	}
	seen := make(map[string]bool)
	var names []string

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") || !strings.Contains(f.Path, "/pages/") {
			continue
		}
		base := f.Path[strings.LastIndex(f.Path, "/")+1:]
		name := strings.TrimSuffix(strings.TrimSuffix(base, ".tsx"), "Page")
		if skip[name] || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, camelCaseToWords(name))
	}
	return names
}

// extractComponentNames returns component names from files under any of the given subpaths.
func extractComponentNames(files []models.ProjectFile, subpaths []string) []string {
	skipNames := map[string]bool{
		"App": true, "Layout": true, "Navbar": true, "Footer": true,
		"Header": true, "Sidebar": true, "index": true, "Index": true,
	}
	seen := make(map[string]bool)
	var names []string

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".tsx") {
			continue
		}
		matched := false
		for _, sp := range subpaths {
			if strings.Contains(f.Path, sp) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		base := f.Path[strings.LastIndex(f.Path, "/")+1:]
		name := strings.TrimSuffix(base, ".tsx")
		for _, suffix := range []string{"Section", "Block", "Component", "View"} {
			name = strings.TrimSuffix(name, suffix)
		}
		if skipNames[name] || seen[name] || name == "" {
			continue
		}
		seen[name] = true
		names = append(names, camelCaseToWords(name))
	}
	return names
}

// parseSectionsFromUIStructure extracts section names from the architect's free-form
// UIStructure text. "- Hero Section with background" → "Hero Section".
func parseSectionsFromUIStructure(uiStructure string) []string {
	layoutWords := map[string]bool{
		"navbar": true, "navigation": true, "footer": true,
		"header": true, "sidebar": true, "menu": true,
	}
	seen := make(map[string]bool)
	var sections []string

	for _, line := range strings.Split(uiStructure, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, prefix := range []string{"- ", "• ", "* ", "· "} {
			line = strings.TrimPrefix(line, prefix)
		}
		if len(line) > 2 && line[0] >= '0' && line[0] <= '9' {
			if idx := strings.Index(line, ". "); idx >= 0 && idx <= 2 {
				line = line[idx+2:]
			}
		}
		for _, sep := range []string{":", " — ", " - ", " with ", " (", " including "} {
			if idx := strings.Index(line, sep); idx > 3 && idx < 60 {
				line = line[:idx]
				break
			}
		}
		line = strings.Trim(strings.TrimSpace(line), "*_`")
		if len(line) < 3 || len(line) > 60 {
			continue
		}
		lower := strings.ToLower(line)
		skip := false
		for w := range layoutWords {
			if strings.Contains(lower, w) {
				skip = true
				break
			}
		}
		if skip || seen[line] {
			continue
		}
		seen[line] = true
		sections = append(sections, line)
	}
	return sections
}

// ── String utilities ──────────────────────────────────────────────────────────

// slugToTitle converts "dark-space-theme" → "Dark Space Theme".
func slugToTitle(s string) string {
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// camelCaseToWords splits CamelCase into words: "AttendanceLeave" → "Attendance Leave".
func camelCaseToWords(s string) string {
	runes := []rune(s)
	var out []rune
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if prev >= 'a' && prev <= 'z' {
				out = append(out, ' ')
			} else if i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z' && prev >= 'A' && prev <= 'Z' {
				out = append(out, ' ')
			}
		}
		out = append(out, r)
	}
	return string(out)
}
