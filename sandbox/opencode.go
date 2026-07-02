package sandbox

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"connectrpc.com/connect"
	sandboxv1 "github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1"
)

// Event is one streamed OpenCode event.
type Event struct {
	Type string
	Data string
}

// RunResult is the final OpenCode run output.
type RunResult struct {
	RawOutput        string
	Model            string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	TotalCost        float64
	CostBreakdown    []CostBreakdownEntry
	Duration         time.Duration
}

type CostBreakdownEntry struct {
	Agent            string
	Model            string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	TotalCost        float64
}

// OpenCodeRunError is returned when OpenCode run completes with success=false.
type OpenCodeRunError struct {
	Code      string
	Message   string
	RawOutput string
	// Provider/Model identify the LLM call that failed, when the error
	// originated at a provider. Best-effort — empty when unattributable
	// (e.g. sandbox boot failures).
	Provider string
	Model    string
}

func (e *OpenCodeRunError) Error() string {
	if strings.TrimSpace(e.Code) != "" {
		return fmt.Sprintf("sandbox: opencode failed (%s): %s", e.Code, e.Message)
	}
	return fmt.Sprintf("sandbox: opencode failed: %s", e.Message)
}

// OpenCode wraps OpenCode operations for one sandbox session.
type OpenCode struct {
	session *Session
}

type OpenCodeInstance struct {
	WorkDir string
	session *Session
}

// InstanceProviderConfig holds non-secret provider configuration.
type InstanceProviderConfig struct {
	BaseURL string
	Npm     string
}

type instanceConfig struct {
	workDir         string
	model           string
	env             map[string]string
	agentModels     map[string]string
	providerConfigs map[string]InstanceProviderConfig
}

func defaultInstanceConfig() instanceConfig {
	return instanceConfig{env: map[string]string{}}
}

type InstanceOption interface {
	applyInstance(*instanceConfig)
}

type instanceOptionFunc func(*instanceConfig)

func (f instanceOptionFunc) applyInstance(cfg *instanceConfig) {
	f(cfg)
}

func WithInstanceWorkDir(dir string) InstanceOption {
	return instanceOptionFunc(func(cfg *instanceConfig) {
		cfg.workDir = strings.TrimSpace(dir)
	})
}

func WithInstanceModel(model string) InstanceOption {
	return instanceOptionFunc(func(cfg *instanceConfig) {
		cfg.model = strings.TrimSpace(model)
	})
}

func WithInstanceEnv(env map[string]string) InstanceOption {
	return instanceOptionFunc(func(cfg *instanceConfig) {
		cfg.env = cloneStringMap(env)
	})
}

// WithInstanceAgentModels sets per-agent model overrides (e.g. "code-quality" → "zai/glm-4.6").
func WithInstanceAgentModels(models map[string]string) InstanceOption {
	return instanceOptionFunc(func(cfg *instanceConfig) {
		cfg.agentModels = cloneStringMap(models)
	})
}

// WithInstanceProviderConfigs sets non-secret provider configs (npm, baseURL) per provider.
func WithInstanceProviderConfigs(configs map[string]InstanceProviderConfig) InstanceOption {
	return instanceOptionFunc(func(cfg *instanceConfig) {
		if len(configs) == 0 {
			return
		}
		cfg.providerConfigs = make(map[string]InstanceProviderConfig, len(configs))
		for k, v := range configs {
			cfg.providerConfigs[k] = v
		}
	})
}

func (oc *OpenCode) StartInstance(ctx context.Context, opts ...InstanceOption) (*OpenCodeInstance, error) {
	if oc == nil || oc.session == nil || oc.session.client == nil {
		return nil, errors.New("sandbox: opencode session not initialized")
	}

	cfg := defaultInstanceConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyInstance(&cfg)
	}
	if cfg.workDir == "" {
		return nil, errors.New("sandbox: opencode instance work dir is required")
	}

	var protoProviderConfigs map[string]*sandboxv1.OpenCodeProviderConfig
	if len(cfg.providerConfigs) > 0 {
		protoProviderConfigs = make(map[string]*sandboxv1.OpenCodeProviderConfig, len(cfg.providerConfigs))
		for k, v := range cfg.providerConfigs {
			protoProviderConfigs[k] = &sandboxv1.OpenCodeProviderConfig{
				BaseUrl: v.BaseURL,
				Npm:     v.Npm,
			}
		}
	}
	if _, err := oc.session.client.sandbox.StartOpenCodeInstance(ctx, connect.NewRequest(&sandboxv1.StartOpenCodeInstanceRequest{
		SessionId:       oc.session.ID,
		WorkDir:         cfg.workDir,
		Model:           cfg.model,
		Env:             cloneStringMap(cfg.env),
		AgentModels:     cloneStringMap(cfg.agentModels),
		ProviderConfigs: protoProviderConfigs,
	})); err != nil {
		return nil, mapError(err)
	}

	return &OpenCodeInstance{
		WorkDir: cfg.workDir,
		session: oc.session,
	}, nil
}

func (i *OpenCodeInstance) Run(ctx context.Context, prompt string, opts ...RunOption) (*RunResult, error) {
	if i == nil || i.session == nil || i.session.OpenCode == nil {
		return nil, errors.New("sandbox: opencode instance not initialized")
	}

	runOpts := make([]RunOption, 0, len(opts)+1)
	runOpts = append(runOpts, runWithDirectory(i.WorkDir))
	runOpts = append(runOpts, opts...)

	return i.session.OpenCode.Run(ctx, prompt, runOpts...)
}

func (oc *OpenCode) Run(ctx context.Context, prompt string, opts ...RunOption) (*RunResult, error) {
	if oc == nil || oc.session == nil || oc.session.client == nil {
		return nil, errors.New("sandbox: opencode session not initialized")
	}

	cfg := defaultRunConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyRun(&cfg)
	}

	req := &sandboxv1.OpenCodeRunRequest{
		SessionId: oc.session.ID,
		Model:     cfg.model,
		Prompt:    prompt,
		Directory: cfg.directory,
	}
	if cfg.sessionTitle != nil {
		req.SessionTitle = *cfg.sessionTitle
	}
	if cfg.agent != nil {
		req.Agent = *cfg.agent
	}
	if cfg.timeout != nil && *cfg.timeout > 0 {
		req.TimeoutSeconds = int32(math.Ceil(cfg.timeout.Seconds()))
	}

	started := time.Now()
	stream, err := oc.session.client.sandbox.OpenCodeRun(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, mapError(err)
	}

	for stream.Receive() {
		msg := stream.Msg()
		if msg == nil {
			continue
		}

		switch payload := msg.Payload.(type) {
		case *sandboxv1.OpenCodeRunResponse_Event:
			if cfg.onEvent != nil && payload.Event != nil {
				cfg.onEvent(Event{
					Type: payload.Event.EventType,
					Data: payload.Event.Data,
				})
			}
		case *sandboxv1.OpenCodeRunResponse_Result:
			if payload.Result == nil {
				return nil, errors.New("sandbox: missing opencode result")
			}
			if !payload.Result.Success {
				return nil, &OpenCodeRunError{
					Code:      payload.Result.ErrorCode,
					Message:   payload.Result.ErrorMessage,
					RawOutput: payload.Result.RawOutput,
					Provider:  payload.Result.ErrorProvider,
					Model:     payload.Result.ErrorModel,
				}
			}
			costBreakdown := make([]CostBreakdownEntry, 0, len(payload.Result.CostBreakdown))
			for _, entry := range payload.Result.CostBreakdown {
				if entry == nil {
					continue
				}
				costBreakdown = append(costBreakdown, CostBreakdownEntry{
					Agent:            entry.Agent,
					Model:            entry.Model,
					InputTokens:      int(entry.InputTokens),
					OutputTokens:     int(entry.OutputTokens),
					CacheReadTokens:  int(entry.CacheReadTokens),
					CacheWriteTokens: int(entry.CacheWriteTokens),
					ReasoningTokens:  int(entry.ReasoningTokens),
					TotalCost:        entry.TotalCost,
				})
			}

			return &RunResult{
				RawOutput:        payload.Result.RawOutput,
				Model:            cfg.model,
				InputTokens:      int(payload.Result.InputTokens),
				OutputTokens:     int(payload.Result.OutputTokens),
				CacheReadTokens:  int(payload.Result.CacheReadTokens),
				CacheWriteTokens: int(payload.Result.CacheWriteTokens),
				ReasoningTokens:  int(payload.Result.ReasoningTokens),
				TotalCost:        payload.Result.TotalCost,
				CostBreakdown:    costBreakdown,
				Duration:         time.Since(started),
			}, nil
		}
	}

	if err := stream.Err(); err != nil {
		return nil, mapError(err)
	}
	return nil, ErrStreamClosed
}

func (oc *OpenCode) Abort(ctx context.Context) error {
	if oc == nil || oc.session == nil {
		return errors.New("sandbox: opencode session not initialized")
	}

	res, err := oc.session.Exec(ctx, "curl",
		WithArgs("-sS", "-X", "POST", "http://localhost:41230/session/abort"),
	)
	if err != nil {
		return err
	}
	if res.Status != CommandStatusSucceeded {
		return errors.New("sandbox: failed to abort opencode session")
	}
	return nil
}
