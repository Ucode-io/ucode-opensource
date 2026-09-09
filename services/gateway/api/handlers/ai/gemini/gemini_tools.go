package gemini

// Tool definitions mirror claude_tools.go but use Gemini's funcDeclaration format.
// Names and JSON Schema parameter maps are identical to the Anthropic tools so both
// providers produce output that unmarshals into the same Go structs.

var toolArchitectPlan = funcDeclaration{
	Name:        "plan_architecture",
	Description: "Return the complete project architecture: database tables with fields and mock data, all relations between tables, a rich UI structure description, and a complete design system for the frontend developer.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"project_name", "project_type", "tables", "relations", "ui_structure", "design", "image_keywords", "client_types"},
		"properties": map[string]any{
			"project_name": map[string]any{"type": "string", "description": "Human-readable project name"},
			"mobile_capabilities": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string", "enum": []string{"camera", "local_notifications", "push_notifications", "biometric_auth", "identity_verification"}},
				"description": "Native capabilities explicitly required by a mobile project. Use only listed enum values; identity_verification also requires camera. Empty [] for non-mobile projects.",
			},
			"client_types": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Access persona names inferred silently from the project domain and workflows. Each entry creates a separate client_type + role record. admin_panel, webapp, and mobile: always include \"Administrator\" first and add 1-4 sensible domain-specific names when useful. Do not derive these from platform questionnaire choices. landing/web: empty [].",
			},
			"image_keywords": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "2–4 Unsplash search terms that visually represent this project's real-world domain. Be specific and physical: ['freight truck highway','warehouse forklift','shipping containers'] for logistics; ['espresso barista','cafe interior'] for coffee; ['doctor patient','clinic'] for healthcare. NEVER use generic terms like 'business','technology','office','app'.",
			},
			"project_type": map[string]any{
				"type":        "string",
				"enum":        []string{"admin_panel", "landing", "web", "webapp", "mobile"},
				"description": "Detected project type. 'mobile' = installable React/Vite app packaged with Capacitor for iOS/Android. 'webapp' = responsive web app/PWA delivered by URL. A bare 'mobile app' must use the platform-types answer; if unresolved, prefer 'mobile'. 'landing' = single-page promotional site. 'web' = multi-page marketing/content website. 'admin_panel' = internal back-office tool.",
			},
			"tables": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"slug", "label", "fields", "mock_data"},
					"properties": map[string]any{
						"slug":           map[string]any{"type": "string"},
						"label":          map[string]any{"type": "string"},
						"is_login_table": map[string]any{"type": "boolean"},
						"login_strategy": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
						"fields": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"slug", "label", "type"},
								"properties": map[string]any{
									"slug":  map[string]any{"type": "string"},
									"label": map[string]any{"type": "string"},
									"type":  map[string]any{"type": "string"},
								},
							},
						},
						"mock_data": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object"},
						},
					},
				},
			},
			"relations": map[string]any{
				"type":        "array",
				"description": "Every foreign-key relationship between tables. Only Many2One is supported. The FK column {table_to}_id is auto-created on table_from.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"table_from", "table_to", "type"},
					"properties": map[string]any{
						"table_from": map[string]any{"type": "string"},
						"table_to":   map[string]any{"type": "string"},
						"type": map[string]any{
							"type": "string",
							"enum": []string{"Many2One"},
						},
					},
				},
			},
			"ui_structure": map[string]any{
				"type":        "string",
				"description": "Rich, detailed description of pages, layout, features and visual structure for the frontend developer",
			},
			"design": map[string]any{
				"type":        "object",
				"description": "Complete design system tokens. The code generator uses these exact values — fill every field.",
				"required": []string{
					"primary_color", "primary_hsl", "background_color", "background_hsl",
					"surface_color", "surface_hsl", "sidebar_background", "sidebar_background_hsl", "sidebar_style",
					"text_color", "text_muted_color", "border_color",
					"accent_color", "accent_hsl", "font_family", "body_font",
					"border_radius", "design_inspiration",
				},
				"properties": map[string]any{
					"primary_color":          map[string]any{"type": "string"},
					"primary_hsl":            map[string]any{"type": "string"},
					"background_color":       map[string]any{"type": "string"},
					"background_hsl":         map[string]any{"type": "string"},
					"surface_color":          map[string]any{"type": "string"},
					"surface_hsl":            map[string]any{"type": "string"},
					"sidebar_background":     map[string]any{"type": "string"},
					"sidebar_background_hsl": map[string]any{"type": "string"},
					"sidebar_foreground":     map[string]any{"type": "string"},
					"sidebar_style":          map[string]any{"type": "string", "enum": []string{"light", "medium", "dark", "colored"}},
					"text_color":             map[string]any{"type": "string"},
					"text_muted_color":       map[string]any{"type": "string"},
					"border_color":           map[string]any{"type": "string"},
					"accent_color":           map[string]any{"type": "string"},
					"accent_hsl":             map[string]any{"type": "string"},
					"font_family":            map[string]any{"type": "string"},
					"body_font":              map[string]any{"type": "string"},
					"border_radius":          map[string]any{"type": "string"},
					"design_inspiration":     map[string]any{"type": "string"},
				},
			},
		},
	},
}

var toolPlanChanges = funcDeclaration{
	Name:        "plan_changes",
	Description: "List every file that needs to be created or modified to fulfil the requested code change. Do not include file contents — only paths and one-sentence descriptions.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"files_to_change", "files_to_create", "summary"},
		"properties": map[string]any{
			"files_to_change": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "description"},
					"properties": map[string]any{
						"path":        map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
				},
			},
			"files_to_create": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "description"},
					"properties": map[string]any{
						"path":        map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
				},
			},
			"summary": map[string]any{"type": "string"},
		},
	},
}

var toolEmitProject = funcDeclaration{
	Name:        "emit_project",
	Description: "Return the complete set of generated project files. Include every file needed to run the project. File contents must be complete — never truncate.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"project_name", "files", "env"},
		"properties": map[string]any{
			"project_name": map[string]any{"type": "string"},
			"env": map[string]any{
				"type":        "object",
				"description": "All VITE_* environment variables with their real values",
			},
			"files": map[string]any{
				"type":        "array",
				"description": "CRITICAL: Must be a JSON array value — never a JSON-encoded string. Each element is an object with path and content.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "content"},
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
					},
				},
			},
		},
	},
}

var toolEmitVisualEdit = funcDeclaration{
	Name:        "emit_visual_edit",
	Description: "Return the surgically edited files and a one-sentence summary of what was changed. Only include files that actually changed.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"files", "change_summary"},
		"properties": map[string]any{
			"files": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "content"},
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
					},
				},
			},
			"change_summary": map[string]any{"type": "string"},
		},
	},
}

var toolEmitManifest = funcDeclaration{
	Name:        "emit_manifest",
	Description: "Return the complete file manifest for a React admin panel, grouped by dependency level. Group 0 = foundation (generated first). Groups 1..N = features (generated in parallel after foundation).",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"groups"},
		"properties": map[string]any{
			"groups": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "name", "files"},
					"properties": map[string]any{
						"id":   map[string]any{"type": "integer"},
						"name": map[string]any{"type": "string"},
						"files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"path", "exports"},
								"properties": map[string]any{
									"path": map[string]any{"type": "string"},
									"exports": map[string]any{
										"type":  "array",
										"items": map[string]any{"type": "string"},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var toolIntegrateAgent = funcDeclaration{
	Name:        "integrate_agent",
	Description: "Return the files that wire the AI agent into the frontend (the new widget/component plus the app-shell file it is mounted in) and a one-sentence summary. Only include files you create or change — never the provided agentClient.ts/useAgent.ts.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"files", "change_summary"},
		"properties": map[string]any{
			"files": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "content"},
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
					},
				},
			},
			"change_summary": map[string]any{"type": "string"},
		},
	},
}

var toolRepairFile = funcDeclaration{
	Name:        "repair_file",
	Description: "Return the corrected content of a single TypeScript/TSX file. Fix all import errors listed in the prompt. Output the complete file — never truncate.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"content"},
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "Full corrected file content",
			},
		},
	},
}

var toolBuildAgentSpec = funcDeclaration{
	Name:        "build_agent",
	Description: "Return the complete definition of a reusable AI agent for the application's end-users: a short name, a one-sentence description, a full system-prompt instruction, the minimal per-table data permissions it needs, and a short confirmation reply for the builder.",
	Parameters: map[string]any{
		"type":     "object",
		"required": []string{"name", "description", "instruction", "permissions", "reply"},
		"properties": map[string]any{
			"name":        map[string]any{"type": "string", "description": "Short, human-readable agent name, e.g. 'Order Assistant'."},
			"description": map[string]any{"type": "string", "description": "One-sentence summary of what the agent does, for the builder's reference."},
			"instruction": map[string]any{"type": "string", "description": "The agent's complete system prompt: its role, personality, what it helps end-users with, and how it should behave. Write it in the same language as the builder's request. Do NOT mention tool names or internal table slugs."},
			"reply":       map[string]any{"type": "string", "description": "A short, friendly confirmation message for the builder, in the builder's language, summarizing the agent you created."},
			"permissions": map[string]any{
				"type":        "array",
				"description": "The minimal set of table permissions the agent needs. Grant only what is necessary. Each table_slug MUST be one of the slugs from the provided project schema — never invent slugs.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"table_slug"},
					"properties": map[string]any{
						"table_slug": map[string]any{"type": "string", "description": "Slug of a table from the project schema."},
						"can_create": map[string]any{"type": "boolean", "description": "Allow creating records."},
						"can_read":   map[string]any{"type": "boolean", "description": "Allow fetching a single record by id."},
						"can_update": map[string]any{"type": "boolean", "description": "Allow updating records."},
						"can_delete": map[string]any{"type": "boolean", "description": "Allow deleting records."},
						"can_list":   map[string]any{"type": "boolean", "description": "Allow listing/searching records."},
					},
				},
			},
		},
	},
}
