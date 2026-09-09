package v1

import (
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestValidateAppEntryContract_MissingDefaultExport(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import './index.css';
export function App() {
  return <div />;
}`,
		},
	}

	errors := validateGeneratedProject(files, nil)
	found := false
	for _, e := range errors {
		if e.Severity == "error" && contains(e.Message, "must have a default export") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected src/App.tsx default export error, got %v", errors)
	}
}

func TestEnsureAppEntryDefaultExport_ConvertsNamedFunction(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import './index.css';
export function App() {
  return <div />;
}`,
		},
	}

	got := ensureAppEntryDefaultExport(files)
	if !contains(got[0].Content, "export default function App()") {
		t.Fatalf("expected named App function to become default export, got:\n%s", got[0].Content)
	}
}

func TestEnsureAppEntryDefaultExport_AppendsForConstApp(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import './index.css';
const App = () => <div />;
`,
		},
	}

	got := ensureAppEntryDefaultExport(files)
	if !contains(got[0].Content, "export default App;") {
		t.Fatalf("expected default App export to be appended, got:\n%s", got[0].Content)
	}
}

func TestValidateAppEntryContract_IgnoresCommentedDefaultExport(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `// export default function App() {}
export function App() {
  return <div />;
}`,
		},
	}

	errors := validateAppEntryContract(files)
	if len(errors) == 0 {
		t.Fatal("expected commented default export to be ignored")
	}
}

func TestValidateAppEntryContract_AcceptsDefaultReExport(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `function App() {
  return <div />;
}
export { App as default };`,
		},
	}

	errors := validateAppEntryContract(files)
	if len(errors) != 0 {
		t.Fatalf("expected default re-export to satisfy entry contract, got %v", errors)
	}
}

func TestEnsureAppEntryDefaultExport_AppendsForTypedConstApp(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `import type { FC } from 'react';
const App: FC = () => <div />;
`,
		},
	}

	got := ensureAppEntryDefaultExport(files)
	if !contains(got[0].Content, "export default App;") {
		t.Fatalf("expected typed const App to get default export, got:\n%s", got[0].Content)
	}
}

func TestEnsureAppEntryDefaultExport_DoesNotRewriteCommentedNamedFunction(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `// export function App() {}
const App = () => <div />;
`,
		},
	}

	got := ensureAppEntryDefaultExport(files)
	if contains(got[0].Content, "// export default function App()") {
		t.Fatalf("commented named function was rewritten:\n%s", got[0].Content)
	}
	if !contains(got[0].Content, "export default App;") {
		t.Fatalf("expected real const App to get default export, got:\n%s", got[0].Content)
	}
}

func TestValidateAppEntryContract_IgnoresDefaultExportInsideString(t *testing.T) {
	files := []models.ProjectFile{
		{
			Path: "src/App.tsx",
			Content: `const text = "export default function App() {}";
export function App() {
  return <div>{text}</div>;
}`,
		},
	}

	errors := validateAppEntryContract(files)
	if len(errors) == 0 {
		t.Fatal("expected default export inside string to be ignored")
	}
}

func TestAddAppDefaultExport_DoesNotDuplicateExistingDefaultAfterImportString(t *testing.T) {
	content := `import './index.css';
export default function App() {
  return <div />;
}`

	got, fixed := addAppDefaultExport(content)
	if fixed {
		t.Fatalf("expected existing default export to stay untouched, got:\n%s", got)
	}
}
