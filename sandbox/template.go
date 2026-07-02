package sandbox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	sandboxv1 "github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1"
)

var templateBuildPollInterval = 2 * time.Second

// TemplateBuildState mirrors template build lifecycle states from the service contract.
type TemplateBuildState string

const (
	TemplateBuildStateUnspecified TemplateBuildState = "UNSPECIFIED"
	TemplateBuildStatePending     TemplateBuildState = "PENDING"
	TemplateBuildStateBuilding    TemplateBuildState = "BUILDING"
	TemplateBuildStateReady       TemplateBuildState = "READY"
	TemplateBuildStateFailed      TemplateBuildState = "FAILED"
)

// TemplateVisibility mirrors template visibility states from the service contract.
type TemplateVisibility string

const (
	TemplateVisibilityUnspecified TemplateVisibility = "UNSPECIFIED"
	TemplateVisibilityPrivate     TemplateVisibility = "PRIVATE"
	TemplateVisibilityPublic      TemplateVisibility = "PUBLIC"
)

// TemplateResources is the SDK view of template CPU/memory defaults.
type TemplateResources struct {
	CPUCores   int32
	MemoryMB   int32
	DiskSizeGB int32
}

// TemplateBuild is the SDK view of one template build.
type TemplateBuild struct {
	ID                 string
	TemplateID         string
	State              TemplateBuildState
	Version            int32
	Error              string
	BuildLogTail       string
	BuildLogTruncated  bool
	BuildLogArtifactID string
	StartedAt          time.Time
	CompletedAt        time.Time
	SnapshotID         string
	SessionID          string
}

// Template is the SDK view of one sandbox template.
type Template struct {
	ID                string
	WorkspaceID       string
	ProjectID         string
	OwnerType         string
	OwnerID           string
	Name              string
	BaseImageID       string
	SetupScript       string
	StartCmd          string
	EnvVars           map[string]string
	Tags              []string
	Resources         *TemplateResources
	LatestBuild       *TemplateBuild
	Visibility        TemplateVisibility
	ParentWorkspaceID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// CreateTemplate creates one sandbox template.
func (c *Client) CreateTemplate(ctx context.Context, opts ...TemplateOption) (*Template, error) {
	cfg := defaultTemplateConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyTemplate(&cfg)
	}
	if cfg.resourcesSet {
		if err := validateTemplateResources(cfg.resources); err != nil {
			return nil, err
		}
	}

	req := &sandboxv1.CreateTemplateRequest{
		WorkspaceId: cfg.workspaceID,
		Name:        cfg.name,
		BaseImageId: cfg.baseImageID,
		SetupScript: cfg.setupScript,
		EnvVars:     cloneStringMap(cfg.env),
		Tags:        append([]string(nil), cfg.tags...),
	}
	if pID := strings.TrimSpace(cfg.projectID); pID != "" {
		req.ProjectId = &pID
	}
	if parentTemplateID := strings.TrimSpace(cfg.parentTemplateID); parentTemplateID != "" {
		req.ParentTemplateId = &parentTemplateID
	}
	if parentImage := strings.TrimSpace(cfg.parentImage); parentImage != "" {
		req.ParentImage = &parentImage
	}
	if cfg.startCmdSet {
		req.StartCmd = cfg.startCmd
	}
	if cfg.resourcesSet {
		req.Resources = templateResourcesToProto(cfg.resources)
	}

	resp, err := c.sandbox.CreateTemplate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, mapError(err)
	}
	return templateFromProto(resp.Msg.Template), nil
}

// BuildTemplate starts one template build.
func (c *Client) BuildTemplate(ctx context.Context, templateID string, opts ...TemplateBuildOption) (*TemplateBuild, error) {
	var cfg templateBuildConfig
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyTemplateBuild(&cfg)
	}

	req := &sandboxv1.BuildTemplateRequest{
		TemplateId:      strings.TrimSpace(templateID),
		PublishRawImage: cfg.publishRawImage,
	}

	resp, err := c.sandbox.BuildTemplate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, mapError(err)
	}
	return templateBuildFromProto(resp.Msg.Build), nil
}

// WaitForTemplateBuild polls until one template build reaches READY or FAILED.
func (c *Client) WaitForTemplateBuild(ctx context.Context, buildID string) (*TemplateBuild, error) {
	for {
		build, err := c.GetTemplateBuild(ctx, buildID)
		if err != nil {
			return nil, err
		}
		switch build.State {
		case TemplateBuildStateReady:
			return build, nil
		case TemplateBuildStateFailed:
			if build.Error != "" {
				return build, fmt.Errorf("%w: %s", ErrTemplateBuildFailed, build.Error)
			}
			return build, ErrTemplateBuildFailed
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(templateBuildPollInterval):
		}
	}
}

// GetTemplate fetches one template by ID.
func (c *Client) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	resp, err := c.sandbox.GetTemplate(ctx, connect.NewRequest(&sandboxv1.GetTemplateRequest{
		TemplateId: strings.TrimSpace(templateID),
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return templateFromProto(resp.Msg.Template), nil
}

// ListTemplates lists templates for one workspace.
func (c *Client) ListTemplates(ctx context.Context, workspaceID string, opts ...ListOption) ([]*Template, error) {
	cfg := defaultListConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyList(&cfg)
	}
	resp, err := c.sandbox.ListTemplates(ctx, connect.NewRequest(&sandboxv1.ListTemplatesRequest{
		WorkspaceId: strings.TrimSpace(workspaceID),
		PageSize:    100,
		Tags:        append([]string(nil), cfg.tags...),
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return templatesFromProto(resp.Msg.Templates), nil
}

// ListProjectTemplates lists templates for one project.
func (c *Client) ListProjectTemplates(ctx context.Context, projectID string, opts ...ListOption) ([]*Template, error) {
	cfg := defaultListConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyList(&cfg)
	}
	resp, err := c.sandbox.ListProjectTemplates(ctx, connect.NewRequest(&sandboxv1.ListProjectTemplatesRequest{
		ProjectId: strings.TrimSpace(projectID),
		PageSize:  100,
		Tags:      append([]string(nil), cfg.tags...),
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return templatesFromProto(resp.Msg.Templates), nil
}

// UpdateTemplate updates one template.
func (c *Client) UpdateTemplate(ctx context.Context, templateID string, opts ...TemplateOption) (*Template, error) {
	cfg := defaultTemplateConfig()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyTemplate(&cfg)
	}
	if cfg.resourcesSet {
		if err := validateTemplateResources(cfg.resources); err != nil {
			return nil, err
		}
	}

	req := &sandboxv1.UpdateTemplateRequest{
		TemplateId: strings.TrimSpace(templateID),
	}
	if cfg.nameSet {
		req.Name = &cfg.name
	}
	if cfg.baseImageIDSet {
		req.BaseImageId = &cfg.baseImageID
	}
	if cfg.setupScriptSet {
		req.SetupScript = &cfg.setupScript
	}
	if cfg.startCmdSet {
		req.StartCmd = cfg.startCmd
	}
	if cfg.envSet {
		req.EnvVars = cloneStringMap(cfg.env)
	}
	if cfg.tagsSet {
		if len(cfg.tags) == 0 {
			req.ClearTags = true
		} else {
			req.Tags = append([]string(nil), cfg.tags...)
		}
	}
	if cfg.resourcesSet {
		req.Resources = templateResourcesToProto(cfg.resources)
	}

	resp, err := c.sandbox.UpdateTemplate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, mapError(err)
	}
	return templateFromProto(resp.Msg.Template), nil
}

// DeleteTemplate deletes one template.
func (c *Client) DeleteTemplate(ctx context.Context, templateID string) (*Template, error) {
	resp, err := c.sandbox.DeleteTemplate(ctx, connect.NewRequest(&sandboxv1.DeleteTemplateRequest{
		TemplateId: strings.TrimSpace(templateID),
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return templateFromProto(resp.Msg.Template), nil
}

// GetTemplateBuild fetches one template build by ID.
func (c *Client) GetTemplateBuild(ctx context.Context, buildID string) (*TemplateBuild, error) {
	resp, err := c.sandbox.GetTemplateBuild(ctx, connect.NewRequest(&sandboxv1.GetTemplateBuildRequest{
		BuildId: strings.TrimSpace(buildID),
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return templateBuildFromProto(resp.Msg.Build), nil
}

// IsReady returns true if the build snapshot is ready for restore.
func (s TemplateBuildState) IsReady() bool {
	return s == TemplateBuildStateReady
}

// IsTerminal returns true if the build has reached a final state.
func (s TemplateBuildState) IsTerminal() bool {
	return s == TemplateBuildStateReady || s == TemplateBuildStateFailed
}

func templateResourcesToProto(resources *TemplateResources) *sandboxv1.TemplateResources {
	if resources == nil {
		return nil
	}
	return &sandboxv1.TemplateResources{
		CpuCores:   resources.CPUCores,
		MemoryMb:   resources.MemoryMB,
		DiskSizeGb: resources.DiskSizeGB,
	}
}

func templatesFromProto(protoTemplates []*sandboxv1.Template) []*Template {
	templates := make([]*Template, 0, len(protoTemplates))
	for _, protoTemplate := range protoTemplates {
		templates = append(templates, templateFromProto(protoTemplate))
	}
	return templates
}

func templateFromProto(protoTemplate *sandboxv1.Template) *Template {
	if protoTemplate == nil {
		return nil
	}

	template := &Template{
		ID:          protoTemplate.Id,
		WorkspaceID: protoTemplate.WorkspaceId,
		ProjectID:   protoTemplate.ProjectId,
		OwnerType:   protoTemplate.OwnerType,
		OwnerID:     protoTemplate.OwnerId,
		Name:        protoTemplate.Name,
		BaseImageID: protoTemplate.BaseImageId,
		SetupScript: protoTemplate.SetupScript,
		Tags:        append([]string(nil), protoTemplate.Tags...),
		EnvVars:     cloneStringMap(protoTemplate.EnvVars),
		LatestBuild: templateBuildFromProto(protoTemplate.LatestBuild),
		Visibility:  templateVisibilityFromProto(protoTemplate.Visibility),
	}
	if protoTemplate.ParentWorkspaceId != nil {
		template.ParentWorkspaceID = protoTemplate.GetParentWorkspaceId()
	}
	if protoTemplate.StartCmd != nil {
		template.StartCmd = protoTemplate.GetStartCmd()
	}
	if protoTemplate.Resources != nil {
		template.Resources = &TemplateResources{
			CPUCores:   protoTemplate.Resources.CpuCores,
			MemoryMB:   protoTemplate.Resources.MemoryMb,
			DiskSizeGB: protoTemplate.Resources.DiskSizeGb,
		}
	}
	if protoTemplate.CreatedAt != nil {
		template.CreatedAt = protoTemplate.CreatedAt.AsTime()
	}
	if protoTemplate.UpdatedAt != nil {
		template.UpdatedAt = protoTemplate.UpdatedAt.AsTime()
	}
	return template
}

func templateBuildFromProto(protoBuild *sandboxv1.TemplateBuild) *TemplateBuild {
	if protoBuild == nil {
		return nil
	}

	build := &TemplateBuild{
		ID:                protoBuild.Id,
		TemplateID:        protoBuild.TemplateId,
		State:             templateBuildStateFromProto(protoBuild.State),
		Version:           protoBuild.Version,
		BuildLogTruncated: protoBuild.BuildLogTruncated,
	}
	if protoBuild.Error != nil {
		build.Error = protoBuild.GetError()
	}
	if protoBuild.BuildLogTail != nil {
		build.BuildLogTail = protoBuild.GetBuildLogTail()
	}
	if protoBuild.BuildLogArtifactId != nil {
		build.BuildLogArtifactID = protoBuild.GetBuildLogArtifactId()
	}
	if protoBuild.StartedAt != nil {
		build.StartedAt = protoBuild.StartedAt.AsTime()
	}
	if protoBuild.CompletedAt != nil {
		build.CompletedAt = protoBuild.CompletedAt.AsTime()
	}
	if protoBuild.SnapshotId != nil {
		build.SnapshotID = protoBuild.GetSnapshotId()
	}
	if protoBuild.SessionId != nil {
		build.SessionID = protoBuild.GetSessionId()
	}
	return build
}

func templateBuildStateFromProto(state sandboxv1.TemplateBuildState) TemplateBuildState {
	switch state {
	case sandboxv1.TemplateBuildState_TEMPLATE_BUILD_STATE_PENDING:
		return TemplateBuildStatePending
	case sandboxv1.TemplateBuildState_TEMPLATE_BUILD_STATE_BUILDING:
		return TemplateBuildStateBuilding
	case sandboxv1.TemplateBuildState_TEMPLATE_BUILD_STATE_READY:
		return TemplateBuildStateReady
	case sandboxv1.TemplateBuildState_TEMPLATE_BUILD_STATE_FAILED:
		return TemplateBuildStateFailed
	default:
		return TemplateBuildStateUnspecified
	}
}

func templateVisibilityFromProto(visibility sandboxv1.TemplateVisibility) TemplateVisibility {
	switch visibility {
	case sandboxv1.TemplateVisibility_TEMPLATE_VISIBILITY_PRIVATE:
		return TemplateVisibilityPrivate
	case sandboxv1.TemplateVisibility_TEMPLATE_VISIBILITY_PUBLIC:
		return TemplateVisibilityPublic
	default:
		return TemplateVisibilityUnspecified
	}
}
