package config

import "time"

type (
	AgentConfig struct {
		Model     string
		MaxTokens int
		Timeout   time.Duration
	}

	AIAgents struct {
		// Router classifies user intent on every message (fast + cheap).
		Router AgentConfig

		// Architect plans project structure: tables, design, relations.
		Architect AgentConfig

		// Coder generates code for admin_panel and web projects (chunked).
		Coder AgentConfig

		// LandingCoder generates code for landing pages and single-file paths.
		LandingCoder AgentConfig

		// Planner decides which files to change in the edit flow.
		Planner AgentConfig

		// Inspector answers questions about existing code and validates output.
		Inspector AgentConfig

		// DatabaseAssistant generates and executes database queries.
		DatabaseAssistant AgentConfig

		// AgentBuilder generates reusable end-user agent definitions (system
		// prompt + minimal table permissions) from a builder's description.
		AgentBuilder AgentConfig
	}
)

func loadOpenAIAgents() AIAgents {
	const (
		nano = "gpt-5.4-nano"
		mini = "gpt-5.4-mini"
		mid  = "gpt-5.4"
		top  = "gpt-5.5"
	)

	return AIAgents{
		Router: AgentConfig{
			Model:     nano,
			MaxTokens: 2000,
			Timeout:   90 * time.Second,
		},
		Architect: AgentConfig{
			Model:     top,
			MaxTokens: 32000,
			Timeout:   600 * time.Second,
		},
		Coder: AgentConfig{
			Model:     mid,
			MaxTokens: 64000,
			Timeout:   900 * time.Second,
		},
		LandingCoder: AgentConfig{
			Model:     mid,
			MaxTokens: 64000,
			Timeout:   900 * time.Second,
		},
		Planner: AgentConfig{
			Model:     mini,
			MaxTokens: 16000,
			Timeout:   300 * time.Second,
		},
		Inspector: AgentConfig{
			Model:     mini,
			MaxTokens: 8000,
			Timeout:   180 * time.Second,
		},
		DatabaseAssistant: AgentConfig{
			Model:     nano,
			MaxTokens: 4000,
			Timeout:   120 * time.Second,
		},
		AgentBuilder: AgentConfig{
			Model:     mid,
			MaxTokens: 8000,
			Timeout:   180 * time.Second,
		},
	}
}

func loadGeminiAgents() AIAgents {
	const flash = "gemini-2.5-flash"

	return AIAgents{
		Router: AgentConfig{
			Model:     flash,
			MaxTokens: 2000,
			Timeout:   60 * time.Second,
		},
		Architect: AgentConfig{
			Model:     flash,
			MaxTokens: 16000,
			Timeout:   180 * time.Second,
		},
		Coder: AgentConfig{
			Model:     flash,
			MaxTokens: 65536,
			Timeout:   900 * time.Second,
		},
		LandingCoder: AgentConfig{
			Model:     flash,
			MaxTokens: 65536,
			Timeout:   900 * time.Second,
		},
		Planner: AgentConfig{
			Model:     flash,
			MaxTokens: 16000,
			Timeout:   180 * time.Second,
		},
		Inspector: AgentConfig{
			Model:     flash,
			MaxTokens: 8000,
			Timeout:   120 * time.Second,
		},
		DatabaseAssistant: AgentConfig{
			Model:     flash,
			MaxTokens: 4000,
			Timeout:   60 * time.Second,
		},
		AgentBuilder: AgentConfig{
			Model:     flash,
			MaxTokens: 8000,
			Timeout:   120 * time.Second,
		},
	}
}

func loadAIAgents() AIAgents {
	const (
		haiku  = "claude-haiku-4-5-20251001"
		sonnet = "claude-sonnet-4-6"
		opus   = "claude-opus-4-7"
	)

	return AIAgents{
		Router: AgentConfig{
			Model:     haiku,
			MaxTokens: 2000,
			Timeout:   90 * time.Second,
		},
		Architect: AgentConfig{
			Model:     opus,
			MaxTokens: 32000,
			Timeout:   600 * time.Second,
		},
		Coder: AgentConfig{
			Model:     sonnet,
			MaxTokens: 64000,
			Timeout:   900 * time.Second,
		},
		LandingCoder: AgentConfig{
			Model:     sonnet,
			MaxTokens: 64000,
			Timeout:   900 * time.Second,
		},
		Planner: AgentConfig{
			Model:     sonnet,
			MaxTokens: 16000,
			Timeout:   300 * time.Second,
		},
		Inspector: AgentConfig{
			Model:     haiku,
			MaxTokens: 8000,
			Timeout:   180 * time.Second,
		},
		DatabaseAssistant: AgentConfig{
			Model:     sonnet,
			MaxTokens: 4000,
			Timeout:   120 * time.Second,
		},
		AgentBuilder: AgentConfig{
			Model:     sonnet,
			MaxTokens: 8000,
			Timeout:   180 * time.Second,
		},
	}
}
