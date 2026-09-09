package v1

import (
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

type crmFieldMention struct {
	field string
	start int
	end   int
}

type crmTextOccurrence struct {
	start int
	end   int
}

func buildCommonCRMFieldSettings(
	req models.CRMAssistantRequest,
	schema []models.TableSchema,
) (*crmAssistantResult, bool) {
	// Screenshot interpretation stays with the multimodal model because visible
	// labels and layout are part of the user's instruction.
	if len(req.Images) > 0 {
		return nil, false
	}

	table := strings.TrimSpace(req.PageContext.Table)
	if table == "" {
		table = "deals"
	}
	aliases, ok := crmFieldAliases(schema, table)
	if !ok {
		return nil, false
	}
	message := strings.ToLower(strings.TrimSpace(req.Message))
	if !crmRequestLooksLikeFieldSettings(message) {
		return nil, false
	}
	mentions := findCRMFieldMentions(message, aliases)
	if len(mentions) == 0 {
		return nil, false
	}

	hide := fieldsNearestVerbs(message, mentions, []string{
		"ko‘rsatma", "ko'rsatma", "korsatma", "yashir", "berkit", "hide", "remove", "скрой", "не показы",
	})
	show := fieldsNearestVerbs(message, mentions, []string{
		"ko‘rin", "ko'rin", "korin",
		"ko‘rsat", "ko'rsat", "korsat", "show", "visible", "покажи", "отобраз",
	})
	order := fieldsByOrdinals(message, mentions)
	if len(hide) == 0 && len(show) == 0 && len(order) == 0 {
		return nil, false
	}

	// A negative form such as "ko‘rsatma" contains the positive stem
	// "ko‘rsat" too. Resolve that overlap in favour of the explicit hide intent.
	hideSet := make(map[string]struct{}, len(hide))
	for _, field := range hide {
		hideSet[field] = struct{}{}
	}
	filteredShow := show[:0]
	for _, field := range show {
		if _, hidden := hideSet[field]; !hidden {
			filteredShow = append(filteredShow, field)
		}
	}
	show = filteredShow

	// Ordering a field on a card implies that it must be visible there. An
	// explicit ordinal therefore wins over a contradictory visibility phrase.
	show = appendUniqueCRMFields(show, order...)
	showSet := make(map[string]struct{}, len(show))
	for _, field := range show {
		showSet[field] = struct{}{}
	}
	filteredHide := hide[:0]
	for _, field := range hide {
		if _, orderedAndShown := showSet[field]; !orderedAndShown {
			filteredHide = append(filteredHide, field)
		}
	}

	reply := "Kartochka maydonlari sozlandi."
	switch detectCRMRequestLanguage(message) {
	case "ru":
		reply = "Поля карточки настроены."
	case "en":
		reply = "The card fields have been configured."
	}
	return &crmAssistantResult{
		reply: reply,
		clientActions: []models.CRMClientAction{{
			Type:       "set_card_field_visibility",
			Table:      table,
			ShowFields: show,
			HideFields: filteredHide,
			FieldOrder: order,
		}},
	}, true
}

func crmFieldAliases(schema []models.TableSchema, table string) (map[string]string, bool) {
	tableSchema, ok := findCRMSchemaTable(schema, table)
	if !ok {
		return nil, false
	}
	aliases := make(map[string]string, len(tableSchema.Fields)*2)
	for _, field := range tableSchema.Fields {
		aliases[strings.ToLower(strings.TrimSpace(field.Slug))] = field.Slug
		if label := strings.ToLower(strings.TrimSpace(field.Label)); label != "" {
			aliases[label] = field.Slug
		}
		for _, alias := range commonCRMFieldAliases(field.Slug) {
			aliases[strings.ToLower(alias)] = field.Slug
		}
	}
	for alias, field := range builtInCRMFieldAliases(table) {
		aliases[strings.ToLower(alias)] = field
	}
	return aliases, true
}

func findCRMFieldMentions(message string, aliases map[string]string) []crmFieldMention {
	aliasList := make([]string, 0, len(aliases))
	for alias := range aliases {
		// Very short schema slugs such as "id" are too ambiguous for a
		// deterministic natural-language match; the LLM can clarify those.
		if len([]rune(alias)) >= 3 {
			aliasList = append(aliasList, alias)
		}
	}
	sort.Slice(aliasList, func(i, j int) bool {
		if len(aliasList[i]) == len(aliasList[j]) {
			return aliasList[i] < aliasList[j]
		}
		return len(aliasList[i]) > len(aliasList[j])
	})

	mentions := make([]crmFieldMention, 0)
	seenAt := make(map[struct {
		field string
		start int
	}]struct{})
	for _, alias := range aliasList {
		startAt := 0
		for startAt < len(message) {
			relative := strings.Index(message[startAt:], alias)
			if relative < 0 {
				break
			}
			start := startAt + relative
			end := start + len(alias)
			key := struct {
				field string
				start int
			}{field: aliases[alias], start: start}
			if _, duplicate := seenAt[key]; !duplicate {
				seenAt[key] = struct{}{}
				mentions = append(mentions, crmFieldMention{field: aliases[alias], start: start, end: end})
			}
			startAt = end
		}
	}
	sort.Slice(mentions, func(i, j int) bool {
		if mentions[i].start == mentions[j].start {
			return mentions[i].end > mentions[j].end
		}
		return mentions[i].start < mentions[j].start
	})

	// A long alias and its shorter synonym can overlap at the same position.
	result := mentions[:0]
	for _, mention := range mentions {
		overlaps := false
		for _, existing := range result {
			if mention.start < existing.end && mention.end > existing.start {
				overlaps = true
				break
			}
		}
		if !overlaps {
			result = append(result, mention)
		}
	}
	return result
}

func fieldsNearestVerbs(message string, mentions []crmFieldMention, verbs []string) []string {
	result := make([]string, 0)
	for _, verb := range verbs {
		for _, occurrence := range findCRMTextOccurrences(message, verb) {
			bestIndex := -1
			bestDistance := 1 << 30
			for index, mention := range mentions {
				distance := mention.start - occurrence.end
				if distance < 0 {
					distance = occurrence.start - mention.end
				}
				if distance < 0 {
					distance = 0
				}
				if distance < bestDistance {
					bestDistance = distance
					bestIndex = index
				}
			}
			if bestIndex >= 0 && bestDistance <= 40 {
				result = appendUniqueCRMFields(result, mentions[bestIndex].field)
			}
		}
	}
	return result
}

func fieldsByOrdinals(message string, mentions []crmFieldMention) []string {
	ordinalTerms := [][]string{
		{"birinchi", "1-chi", "1chi", "first", "перв"},
		{"ikkinchi", "2-chi", "2chi", "second", "втор"},
		{"uchinchi", "3-chi", "3chi", "third", "трет"},
	}
	ordered := make([]string, len(ordinalTerms))
	for ordinalIndex, terms := range ordinalTerms {
		occurrences := make([]crmTextOccurrence, 0)
		for _, term := range terms {
			occurrences = append(occurrences, findCRMTextOccurrences(message, term)...)
		}
		for _, occurrence := range occurrences {
			beforeIndex, beforeDistance := -1, 1<<30
			afterIndex, afterDistance := -1, 1<<30
			for index, mention := range mentions {
				if mention.end <= occurrence.start && occurrence.start-mention.end < beforeDistance {
					beforeIndex, beforeDistance = index, occurrence.start-mention.end
				}
				if mention.start >= occurrence.end && mention.start-occurrence.end < afterDistance {
					afterIndex, afterDistance = index, mention.start-occurrence.end
				}
			}
			// Uzbek commonly puts the ordinal after the field ("telefonni
			// birinchi"). Prefer that form when it is nearby; otherwise support
			// English/Russian prefix ordering ("first phone").
			if beforeIndex >= 0 && beforeDistance <= 24 {
				ordered[ordinalIndex] = mentions[beforeIndex].field
			} else if afterIndex >= 0 && afterDistance <= 24 {
				ordered[ordinalIndex] = mentions[afterIndex].field
			}
		}
	}

	result := make([]string, 0, len(ordered))
	for _, field := range ordered {
		if field != "" {
			result = appendUniqueCRMFields(result, field)
		}
	}
	return result
}

func findCRMTextOccurrences(message, text string) []crmTextOccurrence {
	result := make([]crmTextOccurrence, 0)
	startAt := 0
	for startAt < len(message) {
		relative := strings.Index(message[startAt:], text)
		if relative < 0 {
			break
		}
		start := startAt + relative
		end := start + len(text)
		result = append(result, crmTextOccurrence{start: start, end: end})
		startAt = end
	}
	return result
}

func appendUniqueCRMFields(fields []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(fields)+len(additions))
	for _, field := range fields {
		seen[field] = struct{}{}
	}
	for _, field := range additions {
		if _, duplicate := seen[field]; duplicate {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	return fields
}
