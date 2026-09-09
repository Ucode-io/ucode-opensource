package v1

import (
	"reflect"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestFilterProjectFilesByPath(t *testing.T) {
	files := []models.ProjectFile{
		{Path: "src/selected.tsx", Content: "selected"},
		{Path: "src/not-selected.tsx", Content: "not selected"},
		{Path: "src/other-selected.tsx", Content: "other selected"},
	}
	allowed := map[string]bool{
		"src/selected.tsx":       true,
		"src/other-selected.tsx": true,
	}

	want := []models.ProjectFile{files[0], files[2]}
	if got := filterProjectFilesByPath(files, allowed); !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered files mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestFilterProjectFilesByPathRejectsEmptyScope(t *testing.T) {
	files := []models.ProjectFile{{Path: "src/App.tsx", Content: "content"}}
	if got := filterProjectFilesByPath(files, nil); len(got) != 0 {
		t.Fatalf("empty selection must reject all files, got %#v", got)
	}
}
