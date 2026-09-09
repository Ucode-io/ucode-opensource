package chat_prompts

import (
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// PromptBuilderAgentIntegrator tells the model to mount the ready-made builder-
// assistant widget in the app shell. The widget, client and hook are all injected as
// authoritative template files, so the model only adds the import + one element —
// this keeps the UI and response parsing correct and identical on every app.
var PromptBuilderAgentIntegrator = `You are a senior React/TypeScript engineer. This application ships with a built-in AI builder assistant for its end-users (a floating chat widget). The complete, production-ready widget is ALREADY provided as a project file. Your ONLY job is to mount it in the app shell so it appears on every page. Do NOT build a chat UI yourself.

You MUST respond by calling the integrate_agent tool. Never reply with plain text.

====================================
WHAT ALREADY EXISTS (DO NOT RECREATE)
====================================
These files are CORRECT and AUTHORITATIVE — never re-emit, modify, or duplicate them:
- src/lib/builderAgentClient.ts and src/hooks/useBuilderAgent.ts — the networking + chat-session layer.
- src/components/BuilderAssistantWidget.tsx — the FINISHED widget: a floating launcher button that opens a chat panel with the transcript, live build steps, a summary of what was built, input, loading and error states, and mobile support. It is fully self-contained and self-styled.

    export interface BuilderAssistantWidgetProps {
      title?: string;        // header title; defaults to "AI-ассистент"
      accentColor?: string;  // launcher/header/user-bubble color; defaults to a neutral indigo
      greeting?: string;     // empty-state greeting line
    }
    export function BuilderAssistantWidget(props?: BuilderAssistantWidgetProps): JSX.Element

====================================
YOUR JOB (this is small — do exactly this)
====================================
1. Import the widget from '@/components/BuilderAssistantWidget'.
2. Render it EXACTLY ONCE in the app shell — the layout/root component that wraps every page (find it in the file graph, e.g. the file that renders the sidebar + <Outlet/> or {children}). Place <BuilderAssistantWidget /> at the end of that component's returned tree so it floats above the app on every screen. Do NOT put it on a single page.
3. To make it feel native, pass the app's primary/brand color as accentColor if you can read it from the project's theme/Tailwind config/design tokens (e.g. accentColor="#2563eb"). If you are unsure of the brand color, omit the prop — the default looks good. Optionally pass a title matching the app's tone.

Do NOT touch the widget's internals, add your own chat state, call the hook yourself, or restyle it. Mounting is the whole task.

====================================
RULES
====================================
- SURGICAL: change ONLY the shell file (add the import + the one element). Do not refactor, rename, reformat, or touch unrelated code. Preserve existing imports, structure and comments.
- TypeScript must stay correct (import path resolves via the '@/' alias, element is valid JSX).
- Return the COMPLETE content of every file you change — never a diff or snippet.
- Include ONLY the shell file you modified. Do NOT re-emit builderAgentClient.ts, useBuilderAgent.ts or BuilderAssistantWidget.tsx.
- change_summary: one sentence naming the shell file you mounted the widget in.`

// BuildBuilderAgentIntegrationMessage renders the user-side message for the
// integrate_agent call for the fixed builder assistant: the pre-injected template
// files the model must reuse, and enough of the project (file graph + selected file
// contents) to place and style the widget correctly.
func BuildBuilderAgentIntegrationMessage(view models.BuilderAgentIntegrationView) string {
	var sb strings.Builder

	if len(view.TemplateFiles) > 0 {
		sb.WriteString("====================================\n")
		sb.WriteString("ALREADY-PROVIDED FILES (import & reuse — never re-emit)\n")
		sb.WriteString("====================================\n")
		for _, path := range view.TemplateFiles {
			fmt.Fprintf(&sb, "- %s\n", path)
		}
	}

	sb.WriteString("\n====================================\n")
	sb.WriteString("PROJECT FILE GRAPH (find the app shell / layout that wraps every page)\n")
	sb.WriteString("====================================\n")
	sb.WriteString(view.FileGraphJSON)
	sb.WriteString("\n")

	sb.WriteString("\n====================================\n")
	sb.WriteString("RELEVANT CURRENT FILES (full content — the app shell/layout where you mount <BuilderAssistantWidget/>, plus any theme file that reveals the brand color for accentColor)\n")
	sb.WriteString("====================================\n")
	sb.WriteString(view.FilesContext)
	sb.WriteString("\n")

	sb.WriteString("\nMount the widget now by calling the integrate_agent tool. Return ONLY the shell file you changed, with its complete content.")
	return sb.String()
}
