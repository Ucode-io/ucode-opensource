package v1

import (
	"log"
	"regexp"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func validateAppEntryContract(files []models.ProjectFile) []ValidationError {
	for _, f := range files {
		if f.Path != "src/App.tsx" {
			continue
		}
		if appHasDefaultExport(f.Content) {
			return nil
		}
		return []ValidationError{{
			Severity: "error",
			File:     "src/App.tsx",
			Message:  "src/App.tsx must have a default export because the preview entry imports App as default from virtual:/src/App",
		}}
	}

	return nil
}

func ensureAppEntryDefaultExport(files []models.ProjectFile) []models.ProjectFile {
	for i := range files {
		if files[i].Path != "src/App.tsx" || appHasDefaultExport(files[i].Content) {
			continue
		}
		content, fixed := addAppDefaultExport(files[i].Content)
		if fixed {
			files[i].Content = content
			log.Printf("[entry-contract] normalized src/App.tsx default export")
		}
		break
	}
	return files
}

func appHasDefaultExport(content string) bool {
	return codeHasDefaultExport(maskTSNonCode(content))
}

func codeHasDefaultExport(code string) bool {
	return regexp.MustCompile(`(?m)\bexport\s+default\b`).MatchString(code) ||
		regexp.MustCompile(`(?m)\bexport\s*\{[^}]*\bas\s+default\b[^}]*\}`).MatchString(code) ||
		regexp.MustCompile(`(?m)\bexport\s*\{[^}]*\bdefault\b[^}]*\}`).MatchString(code)
}

func addAppDefaultExport(content string) (string, bool) {
	code := maskTSNonCode(content)
	if codeHasDefaultExport(code) {
		return content, false
	}

	namedFunction := regexp.MustCompile(`(?m)^(\s*)export\s+function\s+App\s*\(`)
	if loc := namedFunction.FindStringSubmatchIndex(code); loc != nil {
		return content[:loc[0]] + content[loc[2]:loc[3]] + "export default function App(" + content[loc[1]:], true
	}

	if regexp.MustCompile(`(?m)^\s*export\s+const\s+App(?:\s*:\s*[^=]+)?\s*=`).MatchString(code) ||
		regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+App(?:\s*:\s*[^=]+)?\s*=`).MatchString(code) ||
		regexp.MustCompile(`(?m)^\s*function\s+App\s*\(`).MatchString(code) {
		return appendDefaultAppExport(content), true
	}

	return content, false
}

func appendDefaultAppExport(content string) string {
	if strings.HasSuffix(content, "\n") {
		return content + "export default App;\n"
	}
	return content + "\nexport default App;\n"
}

func maskTSNonCode(content string) string {
	var out strings.Builder
	out.Grow(len(content))

	for i := 0; i < len(content); {
		if content[i] == '"' || content[i] == '\'' || content[i] == '`' {
			quote := content[i]
			out.WriteByte(quote)
			i++
			for i < len(content) {
				c := content[i]
				if c == '\n' {
					out.WriteByte('\n')
					i++
					continue
				}
				if c == quote {
					out.WriteByte(quote)
					i++
					break
				}
				out.WriteByte(' ')
				if c == '\\' && i+1 < len(content) {
					if content[i+1] == '\n' {
						out.WriteByte('\n')
					} else {
						out.WriteByte(' ')
					}
					i += 2
					continue
				}
				i++
			}
			continue
		}

		if i+1 < len(content) && content[i] == '/' && content[i+1] == '/' {
			out.WriteString("  ")
			i += 2
			for i < len(content) && content[i] != '\n' {
				out.WriteByte(' ')
				i++
			}
			continue
		}
		if i+1 < len(content) && content[i] == '/' && content[i+1] == '*' {
			out.WriteString("  ")
			i += 2
			for i+1 < len(content) {
				if content[i] == '\n' {
					out.WriteByte('\n')
				} else {
					out.WriteByte(' ')
				}
				if content[i] == '*' && content[i+1] == '/' {
					out.WriteString("  ")
					i += 2
					break
				}
				i++
			}
			continue
		}
		out.WriteByte(content[i])
		i++
	}

	return out.String()
}
