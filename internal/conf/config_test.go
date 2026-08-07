package conf

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestOTelConfigYamlUnmarshaling(t *testing.T) {
	yamlData := `
source:
  otel:
    enable: true
    endpoint: "localhost:4317"
    sample_rate: 1
    std_out: true
    timeout: 5s
  nocli:
    exec_timeout: 5m
    default_tool_timeout: 60s
    tool_timeouts:
      terminal: 5m
    agents:
      main:
        max_iterations: 20
        timeout: 5m
  mcp:
    default_timeout: 30s
  rag:
    timeout: 60s
  openai:
    context_compress:
      timeout: 30s
`

	var cfg Config
	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal yaml: %v", err)
	}

	if cfg.Source.OTel == nil {
		t.Fatal("expected cfg.Source.OTel to be non-nil")
	}

	otel := cfg.Source.OTel
	if !otel.Enable {
		t.Errorf("expected otel.Enable to be true, got %v", otel.Enable)
	}

	if otel.Endpoint != "localhost:4317" {
		t.Errorf("expected otel.Endpoint to be 'localhost:4317', got '%s'", otel.Endpoint)
	}

	if otel.SampleRate != 1.0 {
		t.Errorf("expected otel.SampleRate to be 1.0, got %f", otel.SampleRate)
	}

	if !otel.StdOut {
		t.Errorf("expected otel.StdOut to be true, got %v", otel.StdOut)
	}

	if otel.Timeout.Duration != 5*time.Second {
		t.Errorf("expected otel.Timeout.Duration to be 5s, got %v", otel.Timeout.Duration)
	}

	if cfg.Source.Nocli.ExecTimeout.Duration != 5*time.Minute {
		t.Errorf("expected nocli.exec_timeout to be 5m, got %v", cfg.Source.Nocli.ExecTimeout.Duration)
	}

	if cfg.Source.Nocli.DefaultToolTimeout.Duration != 60*time.Second {
		t.Errorf("expected nocli.default_tool_timeout to be 60s, got %v", cfg.Source.Nocli.DefaultToolTimeout.Duration)
	}

	if cfg.Source.Nocli.ToolTimeouts["terminal"].Duration != 5*time.Minute {
		t.Errorf("expected nocli.tool_timeouts.terminal to be 5m, got %v", cfg.Source.Nocli.ToolTimeouts["terminal"].Duration)
	}

	if cfg.Source.Nocli.Agents["main"].Timeout.Duration != 5*time.Minute {
		t.Errorf("expected nocli.agents.main.timeout to be 5m, got %v", cfg.Source.Nocli.Agents["main"].Timeout.Duration)
	}

	if cfg.Source.MCP.DefaultTimeout.Duration != 30*time.Second {
		t.Errorf("expected mcp.default_timeout to be 30s, got %v", cfg.Source.MCP.DefaultTimeout.Duration)
	}

	if cfg.Source.RAG.Timeout.Duration != 60*time.Second {
		t.Errorf("expected rag.timeout to be 60s, got %v", cfg.Source.RAG.Timeout.Duration)
	}

	if cfg.Source.OpenAI.ContextCompress.Timeout.Duration != 30*time.Second {
		t.Errorf("expected openai.context_compress.timeout to be 30s, got %v", cfg.Source.OpenAI.ContextCompress.Timeout.Duration)
	}
}
