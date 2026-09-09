package helper

import (
	"regexp"
	"strings"
)

var reFolderName = regexp.MustCompile(`[^a-z0-9_]`)

type UcodeUploadConf struct {
	ResourceEnvId string // MinIO bucket = project's resource_env_id
	FolderName    string // subfolder inside bucket, e.g. "myapp_images"
}

// SanitizeFolderName converts a project name to a safe MinIO folder name.
func SanitizeFolderName(name string) string {
	s := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	s = reFolderName.ReplaceAllString(s, "")
	if s == "" {
		s = "project"
	}
	return s + "_images"
}