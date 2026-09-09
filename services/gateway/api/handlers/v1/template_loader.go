package v1

import (
	_ "embed"
	"encoding/json"
	"log"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

//go:embed project_template.json
var projectTemplateRaw []byte

var projectTemplate []models.ProjectFile

func init() {
	var tmpl struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"files"`
	}
	if err := json.Unmarshal(projectTemplateRaw, &tmpl); err != nil {
		log.Printf("[TEMPLATE] failed to parse project_template.json: %v", err)
		return
	}
	projectTemplate = make([]models.ProjectFile, 0, len(tmpl.Files))
	for _, f := range tmpl.Files {
		projectTemplate = append(projectTemplate, models.ProjectFile{
			Path:    f.Path,
			Content: f.Content,
		})
	}
	log.Printf("[TEMPLATE] loaded %d files", len(projectTemplate))
}

func GetTemplateScaffold() []models.ProjectFile {
	return projectTemplate
}

// forceTemplatePaths lists pre-built files that must ALWAYS come from the
// template, even when the model regenerated them. The auth runtime is the
// motivating case: despite the PRE-BUILT contract (manifest + prompt both say
// "do NOT re-emit them"), the model sometimes still emits a stubbed LoginPage
// ("simulate login", no real /v2/login call) that silently breaks sign-in on
// login-gated panels. Skip-if-present injection then keeps the broken stub.
// Overwriting guarantees the working runtime ships regardless of model output.
var forceTemplatePaths = map[string]bool{
	"src/lib/auth.ts":                        true,
	"src/lib/permissions.ts":                 true,
	"src/components/auth/LoginPage.tsx":      true,
	"src/components/auth/ProtectedRoute.tsx": true,
}

// forcedTemplateFile returns the template version of a forceTemplatePaths file.
func forcedTemplateFile(path string) (models.ProjectFile, bool) {
	if !forceTemplatePaths[path] {
		return models.ProjectFile{}, false
	}
	for _, f := range projectTemplate {
		if f.Path == path {
			return f, true
		}
	}
	return models.ProjectFile{}, false
}

// enforceAuthRuntime guarantees the forceTemplatePaths auth files match the
// template on the EDIT path (full generation already forces them via
// mergeTemplateScaffold). The editor agent can still emit a stubbed LoginPage
// ("simulate login", no /v2/login call) or rewrite auth.ts/permissions.ts, and
// panels generated before the force-overwrite landed carry that stub forever
// because edits never re-run the scaffold merge. This:
//   - overwrites any edited protected file with the template version, and
//   - heals a drifted/missing live file by injecting the template version,
// so sign-in works regardless of model output or panel age.
func enforceAuthRuntime(edited []models.ProjectFile, existing []models.GitlabFileChange) []models.ProjectFile {
	existingByPath := make(map[string]string, len(existing))
	for _, f := range existing {
		existingByPath[f.FilePath] = f.Content
	}

	idx := make(map[string]int, len(edited))
	for i, f := range edited {
		idx[f.Path] = i
	}

	for path := range forceTemplatePaths {
		tmpl, ok := forcedTemplateFile(path)
		if !ok {
			continue
		}
		if i, editedHere := idx[path]; editedHere {
			edited[i] = tmpl
			continue
		}
		if cur, exists := existingByPath[path]; !exists || cur != tmpl.Content {
			edited = append(edited, tmpl)
		}
	}
	return edited
}

// mergeTemplateScaffold merges template scaffold files into the generated set:
// files in forceTemplatePaths always win (overwriting model output); every other
// scaffold file is added only when the model did not already produce it.
func mergeTemplateScaffold(files, scaffold []models.ProjectFile) []models.ProjectFile {
	if len(scaffold) == 0 {
		return files
	}

	indexByPath := make(map[string]int, len(files))
	for i, f := range files {
		indexByPath[f.Path] = i
	}

	for _, sf := range scaffold {
		if idx, exists := indexByPath[sf.Path]; exists {
			if forceTemplatePaths[sf.Path] {
				files[idx] = sf
			}
			continue
		}
		files = append(files, sf)
		indexByPath[sf.Path] = len(files) - 1
	}

	return files
}

func GetTemplateContext() []models.ProjectFile {
	skip := map[string]bool{
		"package.json":       true,
		"tailwind.config.js": true,
		"tsconfig.json":      true,
		"vite.config.ts":     true,
		"src/App.tsx":        true,
		"src/index.css":      true,
		"src/main.tsx":       true,
	}
	out := make([]models.ProjectFile, 0, len(projectTemplate))
	for _, f := range projectTemplate {
		if !skip[f.Path] {
			out = append(out, f)
		}
	}
	return out
}
