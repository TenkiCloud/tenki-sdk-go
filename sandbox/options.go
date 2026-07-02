package sandbox

import (
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
)

const (
	defaultBaseURL               = "https://api.tenki.cloud"
	defaultClientTimeout         = 30 * time.Second
	defaultDataPlaneReadyTimeout = 60 * time.Second

	// EnvAPIEndpoint overrides the default base URL when set.
	EnvAPIEndpoint = "TENKI_API_ENDPOINT"
	// EnvAPIURL overrides the default base URL when set.
	EnvAPIURL = "TENKI_API_URL"
	// EnvGatewayURL overrides the default gateway base URL when set.
	EnvGatewayURL = "TENKI_SANDBOX_GATEWAY_URL"
	// EnvAuthToken provides the auth token when not passed via WithAuthToken.
	EnvAuthToken = "TENKI_AUTH_TOKEN"
	// EnvAPIKey provides the auth token when not passed via WithAuthToken.
	EnvAPIKey             = "TENKI_API_KEY"
	defaultCPUCores       = int32(2)
	defaultMemoryMB       = int32(4096)
	defaultDiskSizeGB     = int32(5)
	openCodeAPIKeyEnvVar  = "OPENCODE_API_KEY"
	openCodeNpmEnvVar     = "OPENCODE_PROVIDER_NPM"
	openCodeBaseURLEnvVar = "OPENCODE_PROVIDER_BASE_URL"
	githubTokenEnvVar     = "GH_TOKEN"
	gitTokenEnvVar        = "GIT_TOKEN"
)

const (
	DefaultSessionCreateTimeout  = 3 * time.Minute
	DefaultSnapshotCreateTimeout = 5 * time.Minute
	DefaultRestoreTimeout        = 5 * time.Minute
	DefaultExecTimeout           = 30 * time.Second
	DefaultVolumeDetachTimeout   = 2 * time.Minute
)

type clientConfig struct {
	baseURL               string
	gatewayAddress        string
	authToken             string
	cookieName            string
	httpClient            *http.Client
	httpTimeout           time.Duration
	dataPlaneReadyTimeout time.Duration
	connectOpts           []connect.ClientOption
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		baseURL:               defaultBaseURL,
		httpTimeout:           defaultClientTimeout,
		dataPlaneReadyTimeout: defaultDataPlaneReadyTimeout,
	}
}

// Option configures Client behavior.
type Option interface {
	apply(*clientConfig)
}

type optionFunc func(*clientConfig)

func (f optionFunc) apply(cfg *clientConfig) {
	f(cfg)
}

// WithBaseURL sets sandbox service base URL.
func WithBaseURL(baseURL string) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.baseURL = baseURL
	})
}

// WithGatewayAddress sets the sandbox gateway base URL used for SSH websocket transport.
func WithGatewayAddress(addr string) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.gatewayAddress = addr
	})
}

// WithAuthToken sets the API authentication token.
func WithAuthToken(token string) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.authToken = token
	})
}

// WithHTTPClient sets custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.httpClient = httpClient
	})
}

// WithHTTPTimeout sets HTTP timeout for default HTTP client.
func WithHTTPTimeout(timeout time.Duration) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.httpTimeout = timeout
	})
}

// WithDataPlaneReadyTimeout sets the wall-clock budget used to wait for the
// data-plane edge route to become serving.
func WithDataPlaneReadyTimeout(timeout time.Duration) Option {
	return optionFunc(func(cfg *clientConfig) {
		if timeout > 0 {
			cfg.dataPlaneReadyTimeout = timeout
		}
	})
}

// WithCookieName overrides the default session cookie name ("ory_kratos_session").
func WithCookieName(name string) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.cookieName = name
	})
}

// WithConnectClientOptions appends connect client options.
func WithConnectClientOptions(opts ...connect.ClientOption) Option {
	return optionFunc(func(cfg *clientConfig) {
		cfg.connectOpts = append(cfg.connectOpts, opts...)
	})
}

// DetachVolumeOption configures Session.DetachVolume behavior.
type DetachVolumeOption interface {
	applyDetachVolume(*detachVolumeConfig)
}

type detachVolumeOptionFunc func(*detachVolumeConfig)

func (f detachVolumeOptionFunc) applyDetachVolume(cfg *detachVolumeConfig) {
	f(cfg)
}

// WithForceDetach bypasses stuck SYNC_PENDING cleanup and marks the attachment detached immediately.
func WithForceDetach() DetachVolumeOption {
	return detachVolumeOptionFunc(func(cfg *detachVolumeConfig) {
		cfg.force = true
	})
}

// WithDetachWaitTimeout overrides how long Session.DetachVolume waits for the attachment to leave the session.
// RW volumes may transition to `SYNC_PENDING` before later becoming `DETACHED` once sync-back completes.
// Use `0` to return immediately after the detach RPC succeeds.
func WithDetachWaitTimeout(timeout time.Duration) DetachVolumeOption {
	return detachVolumeOptionFunc(func(cfg *detachVolumeConfig) {
		cfg.waitTimeout = timeout
	})
}

type createConfig struct {
	allowInbound   bool
	allowOutbound  bool
	maxDuration    *time.Duration
	idleTimeout    *time.Duration
	pauseRetention *time.Duration
	cpuCores       *int32
	cpuCoresSet    bool
	memoryMB       *int32
	memoryMBSet    bool
	diskSizeGB     *int32
	diskSizeGBSet  bool
	name           string
	snapshotID     string
	image          string
	workspaceID    string
	projectID      string
	metadata       map[string]string
	tags           []string
	env            map[string]string
	sshKeys        []string
	enableOpenCode bool
	cloneRepoURL   string
	volumes        []*VolumeMount
	sticky         bool
}

type createVolumeConfig struct {
	workspaceID string
	projectID   string
	name        string
	sizeBytes   int64
}

type templateConfig struct {
	workspaceID      string
	workspaceIDSet   bool
	projectID        string
	projectIDSet     bool
	name             string
	nameSet          bool
	baseImageID      string
	baseImageIDSet   bool
	setupScript      string
	setupScriptSet   bool
	startCmd         *string
	startCmdSet      bool
	env              map[string]string
	envSet           bool
	tags             []string
	tagsSet          bool
	resources        *TemplateResources
	resourcesSet     bool
	parentTemplateID string
	parentImage      string
}

type listConfig struct {
	tags   []string
	sticky *bool
}

type volumeConfig struct {
	readonly bool
}

type detachVolumeConfig struct {
	force       bool
	waitTimeout time.Duration
}

type exposeConfig struct {
	expiresAt *time.Time
	slug      string
}

type updateSessionConfig struct {
	name    *string
	tags    []string
	tagsSet bool
	sticky  *bool
}

type updateSnapshotConfig struct {
	name         *string
	expiresAt    *time.Time
	expiresAtSet bool
	tags         []string
	tagsSet      bool
}

type updateVolumeConfig struct {
	name    *string
	tags    []string
	tagsSet bool
}

// VolumeMount configures one session volume attachment.
type VolumeMount struct {
	VolumeID  string
	MountPath string
	ReadOnly  bool
	State     string
}

type nameOption struct {
	name string
}

func (o nameOption) applyCreate(cfg *createConfig) {
	cfg.name = strings.TrimSpace(o.name)
}

func (o nameOption) applyUpdateSession(cfg *updateSessionConfig) {
	name := strings.TrimSpace(o.name)
	cfg.name = &name
}

func (o nameOption) applyUpdateSnapshot(cfg *updateSnapshotConfig) {
	name := strings.TrimSpace(o.name)
	cfg.name = &name
}

func (o nameOption) applyUpdateVolume(cfg *updateVolumeConfig) {
	name := strings.TrimSpace(o.name)
	cfg.name = &name
}

// WithName sets a human-readable name on create and update requests.
func WithName(name string) interface {
	CreateOption
	UpdateSessionOption
	UpdateSnapshotOption
	UpdateVolumeOption
} {
	return nameOption{name: name}
}

// WithSnapshot restores a new session from one snapshot instead of cold-booting from a base image.
func WithSnapshot(snapshotID string) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.snapshotID = strings.TrimSpace(snapshotID)
		if cfg.snapshotID != "" {
			cfg.image = ""
		}
	})
}

// WithImage restores a new session from a registry image ref.
func WithImage(image string) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.image = strings.TrimSpace(image)
		if cfg.image != "" {
			cfg.snapshotID = ""
		}
	})
}

func defaultCreateConfig(client *Client) createConfig {
	_ = client
	cpuCores := defaultCPUCores
	memoryMB := defaultMemoryMB
	diskSizeGB := defaultDiskSizeGB
	return createConfig{
		allowInbound:  true,
		allowOutbound: true,
		cpuCores:      &cpuCores,
		memoryMB:      &memoryMB,
		diskSizeGB:    &diskSizeGB,
		metadata:      map[string]string{},
		env:           map[string]string{},
		sshKeys:       []string{},
		volumes:       []*VolumeMount{},
	}
}

func defaultCreateVolumeConfig() createVolumeConfig {
	return createVolumeConfig{}
}

func defaultTemplateConfig() templateConfig {
	return templateConfig{}
}

func defaultListConfig() listConfig {
	return listConfig{}
}

func defaultDetachVolumeConfig() detachVolumeConfig {
	return detachVolumeConfig{
		waitTimeout: DefaultVolumeDetachTimeout,
	}
}

func defaultExposeConfig() exposeConfig {
	return exposeConfig{}
}

// ExposePortOption configures Session.ExposePort behavior.
type ExposePortOption interface {
	applyExpose(*exposeConfig)
}

type exposePortOptionFunc func(*exposeConfig)

func (f exposePortOptionFunc) applyExpose(cfg *exposeConfig) {
	f(cfg)
}

// WithExposureTTL sets a relative TTL for one exposed port.
func WithExposureTTL(ttl time.Duration) ExposePortOption {
	return exposePortOptionFunc(func(cfg *exposeConfig) {
		if ttl <= 0 {
			cfg.expiresAt = nil
			return
		}
		expiresAt := time.Now().Add(ttl)
		cfg.expiresAt = &expiresAt
	})
}

func WithSlug(slug string) ExposePortOption {
	return exposePortOptionFunc(func(cfg *exposeConfig) {
		cfg.slug = strings.TrimSpace(slug)
	})
}

// WithOpenCode toggles eager opencode startup during session provisioning.
func WithOpenCode(enabled bool) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.enableOpenCode = enabled
	})
}

// WithCloneRepo configures repo clone during provisioning and enables opencode.
func WithCloneRepo(repoURL string) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.cloneRepoURL = strings.TrimSpace(repoURL)
		if cfg.cloneRepoURL != "" {
			cfg.enableOpenCode = true
		}
	})
}

// CreateOption configures Create behavior.
type CreateOption interface {
	applyCreate(*createConfig)
}

type createOptionFunc func(*createConfig)

func (f createOptionFunc) applyCreate(cfg *createConfig) {
	f(cfg)
}

// CreateVolumeOption configures volume creation behavior.
type CreateVolumeOption interface {
	applyCreateVolume(*createVolumeConfig)
}

type createVolumeOptionFunc func(*createVolumeConfig)

func (f createVolumeOptionFunc) applyCreateVolume(cfg *createVolumeConfig) {
	f(cfg)
}

// TemplateOption configures template create/update behavior.
type TemplateOption interface {
	applyTemplate(*templateConfig)
}

type templateOptionFunc func(*templateConfig)

func (f templateOptionFunc) applyTemplate(cfg *templateConfig) {
	f(cfg)
}

type templateBuildConfig struct {
	publishRawImage *bool
}

// TemplateBuildOption configures one template build.
type TemplateBuildOption interface {
	applyTemplateBuild(*templateBuildConfig)
}

type templateBuildOptionFunc func(*templateBuildConfig)

func (f templateBuildOptionFunc) applyTemplateBuild(cfg *templateBuildConfig) {
	f(cfg)
}

// WithBuildPublishRawImage overrides whether this build publishes the raw
// rootfs disk image alongside the build snapshot. Unset defers to server
// configuration.
func WithBuildPublishRawImage(publish bool) TemplateBuildOption {
	return templateBuildOptionFunc(func(cfg *templateBuildConfig) {
		cfg.publishRawImage = &publish
	})
}

// ListOption configures list filtering behavior.
type ListOption interface {
	applyList(*listConfig)
}

type listOptionFunc func(*listConfig)

func (f listOptionFunc) applyList(cfg *listConfig) {
	f(cfg)
}

// UpdateSessionOption configures Session.Update behavior.
type UpdateSessionOption interface {
	applyUpdateSession(*updateSessionConfig)
}

type updateSessionOptionFunc func(*updateSessionConfig)

func (f updateSessionOptionFunc) applyUpdateSession(cfg *updateSessionConfig) {
	f(cfg)
}

// UpdateSnapshotOption configures Client.UpdateSnapshot behavior.
type UpdateSnapshotOption interface {
	applyUpdateSnapshot(*updateSnapshotConfig)
}

type updateSnapshotOptionFunc func(*updateSnapshotConfig)

func (f updateSnapshotOptionFunc) applyUpdateSnapshot(cfg *updateSnapshotConfig) {
	f(cfg)
}

// UpdateVolumeOption configures Client.UpdateVolume behavior.
type UpdateVolumeOption interface {
	applyUpdateVolume(*updateVolumeConfig)
}

type updateVolumeOptionFunc func(*updateVolumeConfig)

func (f updateVolumeOptionFunc) applyUpdateVolume(cfg *updateVolumeConfig) {
	f(cfg)
}

type workspaceIDOption struct {
	workspaceID string
}

func (o workspaceIDOption) applyCreate(cfg *createConfig) {
	cfg.workspaceID = o.workspaceID
}

func (o workspaceIDOption) applyCreateVolume(cfg *createVolumeConfig) {
	cfg.workspaceID = o.workspaceID
}

func (o workspaceIDOption) applyTemplate(cfg *templateConfig) {
	cfg.workspaceID = o.workspaceID
	cfg.workspaceIDSet = true
}

// WithVolumeName sets the volume name for CreateVolume.
func WithVolumeName(name string) CreateVolumeOption {
	return createVolumeOptionFunc(func(cfg *createVolumeConfig) {
		cfg.name = strings.TrimSpace(name)
	})
}

// WithVolumeSize sets the volume size in bytes for CreateVolume. Prefer helpers like GB or GiB.
func WithVolumeSize(sizeBytes int64) CreateVolumeOption {
	return createVolumeOptionFunc(func(cfg *createVolumeConfig) {
		cfg.sizeBytes = sizeBytes
	})
}

// WithWorkspaceID scopes Create/CreateVolume/ListVolumes to one workspace.
func WithWorkspaceID(workspaceID string) interface {
	CreateOption
	CreateVolumeOption
	TemplateOption
} {
	return workspaceIDOption{workspaceID: strings.TrimSpace(workspaceID)}
}

type projectIDOption struct {
	projectID string
}

func (o projectIDOption) applyCreate(cfg *createConfig) {
	cfg.projectID = o.projectID
}

func (o projectIDOption) applyCreateVolume(cfg *createVolumeConfig) {
	cfg.projectID = o.projectID
}

func (o projectIDOption) applyTemplate(cfg *templateConfig) {
	cfg.projectID = o.projectID
	cfg.projectIDSet = true
}

// WithProjectID scopes Create/CreateVolume/CreateTemplate to one project.
func WithProjectID(projectID string) interface {
	CreateOption
	CreateVolumeOption
	TemplateOption
} {
	return projectIDOption{projectID: strings.TrimSpace(projectID)}
}

// VolumeOption configures per-volume attachment behavior.
type VolumeOption interface {
	applyVolume(*volumeConfig)
}

type volumeOptionFunc func(*volumeConfig)

func (f volumeOptionFunc) applyVolume(cfg *volumeConfig) {
	f(cfg)
}

// WithReadOnly mounts a volume read-only.
func WithReadOnly() VolumeOption {
	return volumeOptionFunc(func(cfg *volumeConfig) {
		cfg.readonly = true
	})
}

// WithVolume attaches a persistent volume during Create.
func WithVolume(volumeID, mountPath string, opts ...VolumeOption) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		volumeCfg := volumeConfig{}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			opt.applyVolume(&volumeCfg)
		}
		cfg.volumes = append(cfg.volumes, &VolumeMount{
			VolumeID:  strings.TrimSpace(volumeID),
			MountPath: strings.TrimSpace(mountPath),
			ReadOnly:  volumeCfg.readonly,
		})
	})
}

// OpenCodeProviderConfig configures OpenCode provider env vars for Create.
type OpenCodeProviderConfig struct {
	APIKey  string
	BaseURL string
	Npm     string
}

// WithOpenCodeProvider sets OpenCode provider env vars for Create.
func WithOpenCodeProvider(provider OpenCodeProviderConfig) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		if cfg.env == nil {
			cfg.env = map[string]string{}
		}
		if apiKey := strings.TrimSpace(provider.APIKey); apiKey != "" {
			cfg.env[openCodeAPIKeyEnvVar] = apiKey
		}
		if baseURL := strings.TrimSpace(provider.BaseURL); baseURL != "" {
			cfg.env[openCodeBaseURLEnvVar] = baseURL
		}
		if npm := strings.TrimSpace(provider.Npm); npm != "" {
			cfg.env[openCodeNpmEnvVar] = npm
		}
	})
}

// WithGitHubToken sets both GH_TOKEN and GIT_TOKEN for Create.
func WithGitHubToken(token string) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			return
		}
		if cfg.env == nil {
			cfg.env = map[string]string{}
		}
		cfg.env[githubTokenEnvVar] = trimmed
		cfg.env[gitTokenEnvVar] = trimmed
	})
}

// WithAllowInbound sets inbound network policy on Create requests.
func WithAllowInbound(allowInbound bool) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.allowInbound = allowInbound
	})
}

// WithAllowOutbound sets outbound network policy on Create requests.
func WithAllowOutbound(allowOutbound bool) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.allowOutbound = allowOutbound
	})
}

// WithMaxDuration sets max session duration on Create requests.
func WithMaxDuration(maxDuration time.Duration) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.maxDuration = &maxDuration
	})
}

// WithIdleTimeout sets the inactivity window after which a session is auto-paused.
func WithIdleTimeout(idleTimeout time.Duration) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.idleTimeout = &idleTimeout
	})
}

// WithPauseRetention sets how long paused state is retained for the session.
func WithPauseRetention(retention time.Duration) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.pauseRetention = &retention
	})
}

// WithCPUCores sets CPU cores on Create requests.
func WithCPUCores(cpuCores int32) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.cpuCores = &cpuCores
		cfg.cpuCoresSet = true
	})
}

// WithMemoryMB sets memory on Create requests.
func WithMemoryMB(memoryMB int32) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.memoryMB = &memoryMB
		cfg.memoryMBSet = true
	})
}

// WithDiskSizeGB sets ephemeral root disk size on Create requests.
func WithDiskSizeGB(gb int) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		value := int32(gb)
		cfg.diskSizeGB = &value
		cfg.diskSizeGBSet = true
	})
}

// WithMetadata sets metadata on Create requests.
func WithMetadata(metadata map[string]string) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.metadata = cloneStringMap(metadata)
	})
}

// WithTags sets session or template tags on create/update requests.
func WithTags(tags ...string) interface {
	CreateOption
	TemplateOption
	UpdateSessionOption
	UpdateSnapshotOption
	UpdateVolumeOption
} {
	normalized := normalizeTagList(tags)
	return struct {
		createOptionFunc
		templateOptionFunc
		updateSessionOptionFunc
		updateSnapshotOptionFunc
		updateVolumeOptionFunc
	}{
		createOptionFunc(func(cfg *createConfig) {
			cfg.tags = append(cfg.tags[:0], normalized...)
		}),
		templateOptionFunc(func(cfg *templateConfig) {
			cfg.tags = append(cfg.tags[:0], normalized...)
			cfg.tagsSet = true
		}),
		updateSessionOptionFunc(func(cfg *updateSessionConfig) {
			cfg.tags = append(cfg.tags[:0], normalized...)
			cfg.tagsSet = true
		}),
		updateSnapshotOptionFunc(func(cfg *updateSnapshotConfig) {
			cfg.tags = append(cfg.tags[:0], normalized...)
			cfg.tagsSet = true
		}),
		updateVolumeOptionFunc(func(cfg *updateVolumeConfig) {
			cfg.tags = append(cfg.tags[:0], normalized...)
			cfg.tagsSet = true
		}),
	}
}

// WithExpiresAt sets snapshot expiration on update requests.
func WithExpiresAt(expiresAt time.Time) UpdateSnapshotOption {
	return updateSnapshotOptionFunc(func(cfg *updateSnapshotConfig) {
		value := expiresAt
		cfg.expiresAt = &value
		cfg.expiresAtSet = true
	})
}

// WithoutExpiresAt clears snapshot expiration on update requests.
func WithoutExpiresAt() UpdateSnapshotOption {
	return updateSnapshotOptionFunc(func(cfg *updateSnapshotConfig) {
		cfg.expiresAt = nil
		cfg.expiresAtSet = true
	})
}

// WithTagFilter sets AND-tag filtering on list requests.
func WithTagFilter(tags ...string) ListOption {
	normalized := normalizeTagList(tags)
	return listOptionFunc(func(cfg *listConfig) {
		cfg.tags = append(cfg.tags[:0], normalized...)
	})
}

// WithSticky marks a session as sticky on Create.
func WithSticky() CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.sticky = true
	})
}

// WithSetSticky toggles the sticky flag on Update.
func WithSetSticky(sticky bool) UpdateSessionOption {
	return updateSessionOptionFunc(func(cfg *updateSessionConfig) {
		cfg.sticky = &sticky
	})
}

// WithStickyFilter filters sessions by sticky flag on List.
func WithStickyFilter(sticky bool) ListOption {
	return listOptionFunc(func(cfg *listConfig) {
		cfg.sticky = &sticky
	})
}

// WithSSHKeys overrides session SSH authorized keys at creation time.
func WithSSHKeys(keys []string) CreateOption {
	return createOptionFunc(func(cfg *createConfig) {
		cfg.sshKeys = append([]string(nil), keys...)
	})
}

type execConfig struct {
	args     []string
	timeout  *time.Duration
	env      map[string]string
	onOutput func(Output)
}

func defaultExecConfig() execConfig {
	return execConfig{
		args: []string{},
		env:  map[string]string{},
	}
}

// ExecOption configures Exec behavior.
type ExecOption interface {
	applyExec(*execConfig)
}

type execOptionFunc func(*execConfig)

func (f execOptionFunc) applyExec(cfg *execConfig) {
	f(cfg)
}

// WithArgs sets command args for Exec.
func WithArgs(args ...string) ExecOption {
	return execOptionFunc(func(cfg *execConfig) {
		cfg.args = append([]string(nil), args...)
	})
}

type runConfig struct {
	model        string
	timeout      *time.Duration
	onEvent      func(Event)
	directory    string
	sessionTitle *string
	agent        *string
}

func defaultRunConfig() runConfig {
	return runConfig{}
}

// RunOption configures OpenCode run behavior.
type RunOption interface {
	applyRun(*runConfig)
}

type runOptionFunc func(*runConfig)

func (f runOptionFunc) applyRun(cfg *runConfig) {
	f(cfg)
}

type timeoutOption struct {
	timeout time.Duration
}

func (o timeoutOption) applyExec(cfg *execConfig) {
	cfg.timeout = &o.timeout
}

func (o timeoutOption) applyRun(cfg *runConfig) {
	cfg.timeout = &o.timeout
}

// WithTimeout sets timeout for Exec and OpenCode Run.
func WithTimeout(timeout time.Duration) interface {
	ExecOption
	RunOption
} {
	return timeoutOption{timeout: timeout}
}

// WithModel sets OpenCode model for Run.
func WithModel(model string) RunOption {
	return runOptionFunc(func(cfg *runConfig) {
		cfg.model = model
	})
}

// WithOnEvent sets event callback for OpenCode Run streaming events.
func WithOnEvent(onEvent func(Event)) RunOption {
	return runOptionFunc(func(cfg *runConfig) {
		cfg.onEvent = onEvent
	})
}

func runWithDirectory(dir string) RunOption {
	return runOptionFunc(func(cfg *runConfig) {
		cfg.directory = dir
	})
}

// WithSessionTitle sets the OpenCode session title for Run.
func WithSessionTitle(title string) RunOption {
	return runOptionFunc(func(cfg *runConfig) {
		cfg.sessionTitle = &title
	})
}

// WithAgent sets the OpenCode agent name for Run.
func WithAgent(agent string) RunOption {
	return runOptionFunc(func(cfg *runConfig) {
		cfg.agent = &agent
	})
}

type envOption struct {
	env map[string]string
}

func (o envOption) applyCreate(cfg *createConfig) {
	if cfg.env == nil {
		cfg.env = map[string]string{}
	}
	for k, v := range o.env {
		cfg.env[k] = v
	}
}

func (o envOption) applyExec(cfg *execConfig) {
	if cfg.env == nil {
		cfg.env = map[string]string{}
	}
	for k, v := range o.env {
		cfg.env[k] = v
	}
}

func (o envOption) applyTemplate(cfg *templateConfig) {
	cfg.env = cloneStringMap(o.env)
	cfg.envSet = true
}

// WithEnvs sets environment variables for Create (session defaults) or Exec (command overrides).
func WithEnvs(env map[string]string) interface {
	CreateOption
	ExecOption
	TemplateOption
} {
	return envOption{env: cloneStringMap(env)}
}

// WithOnOutput sets a callback invoked for each output chunk during Exec.
// The callback fires as chunks arrive from the server stream, before Exec returns.
// The full Result is still returned with aggregated Stdout/Stderr.
func WithOnOutput(fn func(Output)) ExecOption {
	return execOptionFunc(func(cfg *execConfig) {
		cfg.onOutput = fn
	})
}

// WithEnv sets a single command environment override.
func WithEnv(key, value string) ExecOption {
	return execOptionFunc(func(cfg *execConfig) {
		if cfg.env == nil {
			cfg.env = map[string]string{}
		}
		cfg.env[key] = value
	})
}

// WithTemplateName sets the template name for CreateTemplate/UpdateTemplate.
func WithTemplateName(name string) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.name = strings.TrimSpace(name)
		cfg.nameSet = true
	})
}

// WithSetupScript sets the template setup script for CreateTemplate/UpdateTemplate.
func WithSetupScript(script string) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.setupScript = strings.TrimSpace(script)
		cfg.setupScriptSet = true
	})
}

// WithStartCmd sets the template start command for CreateTemplate/UpdateTemplate.
func WithStartCmd(startCmd string) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.startCmdSet = true
		value := strings.TrimSpace(startCmd)
		cfg.startCmd = &value
	})
}

// WithTemplateResources sets template CPU and memory defaults in MB.
func WithTemplateResources(cpuCores, memoryMB int32) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.resources = &TemplateResources{
			CPUCores: cpuCores,
			MemoryMB: memoryMB,
		}
		cfg.resourcesSet = true
	})
}

// WithTemplateDiskSizeGB sets the template default ephemeral root disk size.
func WithTemplateDiskSizeGB(gb int32) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		if cfg.resources == nil {
			cfg.resources = &TemplateResources{}
		}
		cfg.resources.DiskSizeGB = gb
		cfg.resourcesSet = true
	})
}

// WithBaseImageID sets the template base image.
func WithBaseImageID(baseImageID string) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.baseImageID = strings.TrimSpace(baseImageID)
		cfg.baseImageIDSet = true
	})
}

// WithParentTemplateID builds a template on top of the current publication of another template.
func WithParentTemplateID(templateID string) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.parentTemplateID = strings.TrimSpace(templateID)
	})
}

// WithParentImage builds a template on top of a pinned registry image snapshot.
func WithParentImage(image string) TemplateOption {
	return templateOptionFunc(func(cfg *templateConfig) {
		cfg.parentImage = strings.TrimSpace(image)
	})
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeTagList(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(strings.ToLower(tag))
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
