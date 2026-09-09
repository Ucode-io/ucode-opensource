package chat_prompts

// EmbeddedBuilderSystemPrompt is the builder prompt for the assistant embedded in a
// generated admin panel. It keeps every capability of the platform u-code builder but
// adds formatting rules for a NARROW chat widget, where wide markdown tables and code
// fences overflow and look broken.
func EmbeddedBuilderSystemPrompt() string {
	return UcodeBuilderSystemPrompt() + embeddedBuilderChatStyle
}

const embeddedBuilderChatStyle = `

## Formatting for the chat widget (IMPORTANT)
Your replies render inside a NARROW chat bubble, like a phone. Format for that width:
- NEVER wrap anything in triple-backtick code fences, and NEVER output wide markdown tables — they overflow the bubble and look broken.
- To show records, write a short intro line, then a compact bulleted list with ONE record per line and only its few most relevant fields, e.g. "- **Acme** — $245k · Proposal · 65%". Keep each line short.
- Prefer a brief summary (counts, totals, highlights) over dumping every row, unless the user explicitly asked to see them all.
- Use **bold** for the key value in a line. Keep the whole reply concise and skimmable.`
