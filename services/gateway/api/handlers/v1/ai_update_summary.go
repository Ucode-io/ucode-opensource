package v1

import (
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// buildUpdateSummary builds a user-facing summary of what changed after a code edit.
// The summary language matches the language the user wrote in.
func buildUpdateSummary(plan *models.SonnetPlanResult, editedFiles []models.ProjectFile, userMessage string) string {
	lang := detectLang(userMessage)

	changed := plan.FilesToChange
	created := plan.FilesToCreate

	// Fall back to actual file list if planner data is empty.
	if len(changed) == 0 && len(created) == 0 && len(editedFiles) > 0 {
		for _, f := range editedFiles {
			changed = append(changed, models.FilePlan{Path: f.Path})
		}
	}

	if len(changed) == 0 && len(created) == 0 {
		return updateDoneLabel(lang)
	}

	var sb strings.Builder

	sb.WriteString(updateDoneLabel(lang))
	sb.WriteString("\n\n")

	// Stats line: "📝 3 файла изменено · 1 создан"
	sb.WriteString(updateStats(lang, len(changed), len(created)))
	sb.WriteString("\n")

	if len(changed) > 0 {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "**%s:**\n", changedLabel(lang))
		for _, f := range changed {
			if f.Description != "" {
				fmt.Fprintf(&sb, "• `%s` — %s\n", f.Path, f.Description)
			} else {
				fmt.Fprintf(&sb, "• `%s`\n", f.Path)
			}
		}
	}

	if len(created) > 0 {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "**%s:**\n", createdLabel(lang))
		for _, f := range created {
			if f.Description != "" {
				fmt.Fprintf(&sb, "• `%s` — %s\n", f.Path, f.Description)
			} else {
				fmt.Fprintf(&sb, "• `%s`\n", f.Path)
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// actually applied and appends a note for any that failed, so a partial success
// is never shown as a full one. Reports failure or "no changes" when nothing applied.
func buildChunkedUpdateSummary(plan *models.SonnetPlanResult, edited []models.ProjectFile, failed []models.FilePlan, userMessage string) string {
	lang := detectLang(userMessage)

	if len(edited) == 0 {
		if len(failed) > 0 {
			return updateFailedLabel(lang) + "\n\n" + buildFailedFilesNote(lang, failed)
		}
		return updateNoChangesLabel(lang)
	}

	editedPaths := make(map[string]bool, len(edited))
	for _, f := range edited {
		editedPaths[f.Path] = true
	}

	// Keep only the plan entries the editor actually produced.
	applied := &models.SonnetPlanResult{}
	for _, fp := range plan.FilesToChange {
		if editedPaths[fp.Path] {
			applied.FilesToChange = append(applied.FilesToChange, fp)
		}
	}
	for _, fp := range plan.FilesToCreate {
		if editedPaths[fp.Path] {
			applied.FilesToCreate = append(applied.FilesToCreate, fp)
		}
	}

	summary := buildUpdateSummary(applied, edited, userMessage)
	if len(failed) > 0 {
		summary += "\n\n" + buildFailedFilesNote(lang, failed)
	}
	return summary
}

// buildFailedFilesNote lists files that could not be updated after retries, so a
// partial success is never presented as complete. Returns "" when nothing failed.
func buildFailedFilesNote(lang string, failed []models.FilePlan) string {
	if len(failed) == 0 {
		return ""
	}

	var sb strings.Builder
	if lang == "ru" {
		fmt.Fprintf(&sb, "⚠️ **Не удалось обновить %d %s** (попробуйте повторить запрос):\n", len(failed), ruFiles(len(failed)))
	} else {
		fmt.Fprintf(&sb, "⚠️ **Could not update %d %s** (try again):\n", len(failed), plural(len(failed), "file", "files"))
	}
	for _, f := range failed {
		if f.Description != "" {
			fmt.Fprintf(&sb, "• `%s` — %s\n", f.Path, f.Description)
		} else {
			fmt.Fprintf(&sb, "• `%s`\n", f.Path)
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// detectLang returns "ru" if the text is predominantly Cyrillic, "en" otherwise.
func detectLang(text string) string {
	var cyrillic, latin int
	for _, r := range text {
		switch {
		case r >= 'а' && r <= 'я' || r >= 'А' && r <= 'Я' || r == 'ё' || r == 'Ё':
			cyrillic++
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
			latin++
		}
	}
	if cyrillic > latin {
		return "ru"
	}
	return "en"
}

func updateDoneLabel(lang string) string {
	if lang == "ru" {
		return "✅ Готово!"
	}
	return "✅ Done!"
}

func updateFailedLabel(lang string) string {
	if lang == "ru" {
		return "⚠️ Не удалось применить изменения."
	}
	return "⚠️ Could not apply the changes."
}

func updateNoChangesLabel(lang string) string {
	if lang == "ru" {
		return "ℹ️ Изменения не потребовались."
	}
	return "ℹ️ No changes were needed."
}

func updateStats(lang string, changed, created int) string {
	if lang == "ru" {
		parts := []string{}
		if changed > 0 {
			parts = append(parts, fmt.Sprintf("**%d %s**", changed, ruFileWord(changed, "изменён", "изменено", "изменено")))
		}
		if created > 0 {
			parts = append(parts, fmt.Sprintf("**%d %s**", created, ruFileWord(created, "создан", "создано", "создано")))
		}
		return "📝 " + strings.Join(parts, " · ")
	}
	parts := []string{}
	if changed > 0 {
		parts = append(parts, fmt.Sprintf("**%d %s**", changed, plural(changed, "file changed", "files changed")))
	}
	if created > 0 {
		parts = append(parts, fmt.Sprintf("**%d %s**", created, plural(created, "file created", "files created")))
	}
	return "📝 " + strings.Join(parts, " · ")
}

func changedLabel(lang string) string {
	if lang == "ru" {
		return "Изменено"
	}
	return "Changed"
}

func createdLabel(lang string) string {
	if lang == "ru" {
		return "Создано"
	}
	return "Created"
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// ruPluralIndex maps a count to its Russian plural form: 0 = one (1, 21),
// 1 = few (2–4, 22–24), 2 = many (0, 5–20, 11–14).
func ruPluralIndex(n int) int {
	mod10, mod100 := n%10, n%100
	switch {
	case mod100 >= 11 && mod100 <= 19:
		return 2
	case mod10 == 1:
		return 0
	case mod10 >= 2 && mod10 <= 4:
		return 1
	default:
		return 2
	}
}

// ruFiles returns the noun "файл" agreeing with n: 1 → "файл", 2 → "файла", 5 → "файлов".
func ruFiles(n int) string {
	return [3]string{"файл", "файла", "файлов"}[ruPluralIndex(n)]
}

// ruFileWord prefixes the agreeing "файл" noun to the agreeing verb form.
func ruFileWord(n int, form1, form2, form5 string) string {
	return ruFiles(n) + " " + [3]string{form1, form2, form5}[ruPluralIndex(n)]
}
