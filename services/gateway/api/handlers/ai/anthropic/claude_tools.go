package anthropic

// ============================================================================
// Claude Function Tool Schemas
//
// Each variable here corresponds to one Anthropic tool-use tool.
// When a tool is passed with tool_choice={type:"tool", name:"<name>"}, Claude
// MUST populate the tool's input_schema exactly — no text, no markdown, no JSON
// escaping bugs.  The output is decoded directly into the matching Go struct.
//
// Tool ↔ Go struct mapping:
//   ToolArchitectPlan  → models.ArchitectPlan
//   ToolPlanChanges    → models.SonnetPlanResult
//   ToolEmitProject    → models.GeneratedProject
//   ToolEmitDiagrams   → models.HaikuPlan
//   ToolEmitVisualEdit → VisualEditOutput  (defined in ai_messging.go)
// ============================================================================

var ToolArchitectPlan = claudeFunctionTool{
	Name:        "plan_architecture",
	Description: "Return the complete project architecture: database tables with fields and mock data, all relations between tables, a rich UI structure description, and a complete design system for the frontend developer.",
	InputSchema: map[string]any{
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
				"description": "Detected project type. ALWAYS choose one of these five. KEYWORD OVERRIDE (highest priority unless it is explicitly an internal staff/admin tool): if the user calls it a 'mobile app' or 'mobile application' → choose 'mobile'; if the user calls it an 'app', 'web app', 'webapp', or 'web application' without an installable-mobile signal → choose 'webapp'. Otherwise: 'landing' = strict single-page promotional site. 'web' = multi-page marketing/content website. 'admin_panel' = internal back-office tool for staff to manage data. 'webapp' = a product SaaS workspace used by end-users (Linear/Notion/Trello/Slack/Asana-like), NOT a marketing site and NOT an internal admin dashboard. 'mobile' = installable React/Vite app packaged with Capacitor for iOS/Android.",
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
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": true,
							},
						},
					},
				},
			},
			"relations": map[string]any{
				"type":        "array",
				"description": "Every foreign-key relationship between tables. Only Many2One is supported. The FK column {table_to}_id is auto-created on table_from (e.g. orders→customers creates column 'customers_id' on orders).",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"table_from", "table_to", "type"},
					"properties": map[string]any{
						"table_from": map[string]any{"type": "string", "description": "Source table slug — the 'many' side (e.g. 'orders' when many orders belong to one customer)"},
						"table_to":   map[string]any{"type": "string", "description": "Target table slug — the 'one' side (e.g. 'customers')"},
						"type": map[string]any{
							"type":        "string",
							"enum":        []string{"Many2One"},
							"description": "Always Many2One. Creates FK column {table_to}_id on table_from.",
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
					"primary_color":          map[string]any{"type": "string", "description": "Hex color, e.g. #6366f1"},
					"primary_hsl":            map[string]any{"type": "string", "description": "HSL without hsl(), e.g. 239 84% 67%"},
					"background_color":       map[string]any{"type": "string", "description": "Page background hex"},
					"background_hsl":         map[string]any{"type": "string", "description": "Page background HSL"},
					"surface_color":          map[string]any{"type": "string", "description": "Card/panel surface hex"},
					"surface_hsl":            map[string]any{"type": "string", "description": "Card/panel surface HSL"},
					"sidebar_background":     map[string]any{"type": "string", "description": "Sidebar bg hex"},
					"sidebar_background_hsl": map[string]any{"type": "string"},
					"sidebar_foreground":     map[string]any{"type": "string", "description": "Sidebar text hex"},
					"sidebar_style":          map[string]any{"type": "string", "enum": []string{"light", "medium", "dark", "colored"}, "description": "Sidebar visual weight"},
					"text_color":             map[string]any{"type": "string", "description": "Primary text hex"},
					"text_muted_color":       map[string]any{"type": "string", "description": "Secondary/muted text hex"},
					"border_color":           map[string]any{"type": "string", "description": "Border hex"},
					"accent_color":           map[string]any{"type": "string", "description": "Accent/highlight hex"},
					"accent_hsl":             map[string]any{"type": "string", "description": "Accent HSL"},
					"font_family":            map[string]any{"type": "string", "description": "Heading font name, e.g. Syne or Inter"},
					"body_font":              map[string]any{"type": "string", "description": "Body font name, e.g. DM Sans or Inter"},
					"border_radius":          map[string]any{"type": "string", "description": "Base border radius, e.g. 8px"},
					"design_inspiration":     map[string]any{"type": "string", "description": "Archetype name or reference, e.g. Obsidian Cinematic or TMS Domain"},
				},
			},
		},
	},
}

var ToolPlanChanges = claudeFunctionTool{
	Name:        "plan_changes",
	Description: "List every file that needs to be created or modified to fulfil the requested code change. Do not include file contents — only paths and one-sentence descriptions.",
	InputSchema: map[string]any{
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
			"summary": map[string]any{"type": "string", "description": "One sentence summary of what will change"},
		},
	},
}

var ToolEmitProject = claudeFunctionTool{
	Name:        "emit_project",
	Description: "Return the complete set of generated project files. Include every file needed to run the project. File contents must be complete — never truncate.",
	InputSchema: map[string]any{
		"type":     "object",
		"required": []string{"project_name", "files", "env"},
		"properties": map[string]any{
			"project_name": map[string]any{"type": "string"},
			"env": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"description":          "All VITE_* environment variables with their real values",
			},
			"files": map[string]any{
				"type":        "array",
				"description": "CRITICAL: Must be a JSON array value — never a JSON-encoded string. Each element is an object with path and content.",
				"minItems":    1,
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "content"},
					"properties": map[string]any{
						"path":    map[string]any{"type": "string", "description": "Relative file path e.g. src/App.tsx"},
						"content": map[string]any{"type": "string", "description": "Complete file content as a plain string — no extra JSON encoding"},
					},
				},
			},
		},
	},
}

var ToolEmitDiagrams = claudeFunctionTool{
	Name:        "emit_diagrams",
	Description: "Return the BPMN 2.0 process diagram and the infrastructure dependency diagram for the project.",
	InputSchema: map[string]any{
		"type":     "object",
		"required": []string{"bpmn_xml", "infra_diagram"},
		"properties": map[string]any{
			"bpmn_xml": map[string]any{
				"type":        "string",
				"description": "Full BPMN 2.0 XML. Newlines must be literal \\n inside the string value.",
			},
			"infra_diagram": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"from", "to", "label"},
					"properties": map[string]any{
						"from":  map[string]any{"type": "string"},
						"to":    map[string]any{"type": "string"},
						"label": map[string]any{"type": "string"},
					},
				},
			},
		},
	},
}

var ToolEmitVisualEdit = claudeFunctionTool{
	Name:        "emit_visual_edit",
	Description: "Return the surgically edited files and a one-sentence summary of what was changed. Only include files that actually changed.",
	InputSchema: map[string]any{
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
						"content": map[string]any{"type": "string", "description": "Complete updated file content"},
					},
				},
			},
			"change_summary": map[string]any{"type": "string", "description": "One sentence describing what was changed and why"},
		},
	},
}

var ToolIntegrateAgent = claudeFunctionTool{
	Name:        "integrate_agent",
	Description: "Return the files that wire the AI agent into the frontend (the new widget/component plus the app-shell file it is mounted in) and a one-sentence summary. Only include files you create or change — never the provided agentClient.ts/useAgent.ts.",
	InputSchema: map[string]any{
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
						"content": map[string]any{"type": "string", "description": "Complete file content"},
					},
				},
			},
			"change_summary": map[string]any{"type": "string", "description": "One sentence describing what was added and where the agent was mounted"},
		},
	},
}

var ToolEmitManifest = claudeFunctionTool{
	Name:        "emit_manifest",
	Description: "Return the complete file manifest for a React admin panel, grouped by dependency level. Group 0 = foundation (generated first). Groups 1..N = features (generated in parallel after foundation). Also emit top-level `routes` (path↔page↔file map) and `entity_types` (TypeScript interfaces src/types.ts must export) — features use them as a hard contract.",
	InputSchema: map[string]any{
		"type":     "object",
		"required": []string{"groups"},
		"properties": map[string]any{
			"export_style": map[string]any{
				"type":        "string",
				"enum":        []string{"named-lazy", "default-export"},
				"description": "Page export convention. ALWAYS set to 'named-lazy' for new projects: pages use `export function PageName()`, App.tsx loads them with `lazy(() => import('@/pages/PageName').then(m => ({ default: m.PageName })))`. 'default-export' is reserved for legacy regeneration.",
			},
			"groups": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "name", "files"},
					"properties": map[string]any{
						"id":   map[string]any{"type": "integer", "description": "0 for foundation, 1..N for feature groups"},
						"name": map[string]any{"type": "string", "description": "Group name, e.g. 'foundation', 'users', 'orders'"},
						"files": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"path", "exports"},
								"properties": map[string]any{
									"path": map[string]any{"type": "string", "description": "Relative file path, e.g. src/pages/UsersPage.tsx"},
									"exports": map[string]any{
										"type":        "array",
										"items":       map[string]any{"type": "string"},
										"description": "All NAMED exports from this file. For pages this is exactly [PageName] — same as the route's page_name.",
									},
									"kind": map[string]any{
										"type":        "string",
										"enum":        []string{"page", "ui", "shared", "layout", "types", "hook", "app", "feature"},
										"description": "Role of this file: 'page' for routed pages, 'ui' for primitive UI kit, 'shared' for DataTable/FormModal/etc., 'layout' for Layout/Navbar/Footer, 'types' for src/types.ts, 'hook' for src/hooks/*, 'app' for src/App.tsx, 'feature' for non-page feature components.",
									},
									"route": map[string]any{
										"type":        "string",
										"description": "Canonical URL path for pages (e.g. '/' or '/users/:id'). Omit for non-pages.",
									},
									"props_interface": map[string]any{
										"type":        "string",
										"description": "For ui-kit components, the TS interface name (e.g. 'ButtonProps'). Omit otherwise.",
									},
									"variants": map[string]any{
										"type":        "object",
										"description": "For ui-kit components built with cva, the variant axes and allowed values, e.g. {variant: ['default','outline'], size: ['sm','md','lg']}. Omit otherwise.",
										"additionalProperties": map[string]any{
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
			"routes": map[string]any{
				"type":        "array",
				"description": "Global route map: every page that App.tsx must register. One entry per page file. Order is the order routes appear in App.tsx.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"path", "page_name", "file_path"},
					"properties": map[string]any{
						"path":      map[string]any{"type": "string", "description": "URL path, e.g. '/' or '/users/:id'."},
						"page_name": map[string]any{"type": "string", "description": "Named export from the file (e.g. 'HomePage'). MUST match exports[0] of the page file."},
						"file_path": map[string]any{"type": "string", "description": "Absolute project path, e.g. 'src/pages/HomePage.tsx'."},
					},
				},
			},
			"entity_types": map[string]any{
				"type":        "array",
				"description": "TypeScript interfaces that src/types.ts MUST export. One entry per table; field list is exhaustive. Foundation reads this as a hard contract.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"name", "fields"},
					"properties": map[string]any{
						"name": map[string]any{"type": "string", "description": "PascalCase singular of the table label, e.g. 'Client' for table 'clients'."},
						"fields": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"name", "ts_type"},
								"properties": map[string]any{
									"name":     map[string]any{"type": "string"},
									"ts_type":  map[string]any{"type": "string", "description": "TS type: 'string' | 'number' | 'boolean' | 'Date' | 'string[]' | etc."},
									"optional": map[string]any{"type": "boolean"},
								},
							},
						},
					},
				},
			},
		},
	},
}

var ToolRepairFile = claudeFunctionTool{
	Name:        "repair_file",
	Description: "Return the corrected content of a single TypeScript/TSX file. Fix all import errors listed in the prompt. Output the complete file — never truncate.",
	InputSchema: map[string]any{
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

var ToolBuildAgentSpec = claudeFunctionTool{
	Name:        "build_agent",
	Description: "Return the complete definition of a reusable AI agent for the application's end-users: a short name, a one-sentence description, a full system-prompt instruction, the minimal per-table data permissions it needs, and a short confirmation reply for the builder.",
	InputSchema: map[string]any{
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

func ForcedTool(toolName string) *toolChoice {
	return &toolChoice{Type: "tool", Name: toolName}
}
