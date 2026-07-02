package sandbox

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect"
	sandboxv1 "github.com/TenkiCloud/tenki-sdk-go/sandbox/internal/proto/tenki/sandbox/v1"
)

type PreviewURL struct {
	ID             string
	ProjectID      string
	WorkspaceID    string
	OwnerID        string
	Slug           string
	Token          string
	PreviewURL     string
	SessionID      string
	Port           *int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
	LastAccessedAt *time.Time
}

func previewURLFromProto(protoPreviewURL *sandboxv1.PreviewUrl) *PreviewURL {
	if protoPreviewURL == nil {
		return nil
	}
	result := &PreviewURL{
		ID:          protoPreviewURL.Id,
		ProjectID:   protoPreviewURL.ProjectId,
		WorkspaceID: protoPreviewURL.WorkspaceId,
		OwnerID:     protoPreviewURL.OwnerId,
		Slug:        protoPreviewURL.Slug,
		Token:       protoPreviewURL.Token,
		PreviewURL:  protoPreviewURL.PreviewUrl,
		SessionID:   protoPreviewURL.GetSessionId(),
	}
	if protoPreviewURL.CreatedAt != nil {
		result.CreatedAt = protoPreviewURL.CreatedAt.AsTime()
	}
	if protoPreviewURL.UpdatedAt != nil {
		result.UpdatedAt = protoPreviewURL.UpdatedAt.AsTime()
	}
	if protoPreviewURL.Port != nil {
		port := protoPreviewURL.GetPort()
		result.Port = &port
	}
	result.LastAccessedAt = protoTimestampPtr(protoPreviewURL.LastAccessedAt)
	return result
}

func (c *Client) CreatePreviewURL(ctx context.Context, projectID string, slug string, sessionID *string, port *int32) (*PreviewURL, error) {
	req := &sandboxv1.CreatePreviewUrlRequest{
		ProjectId: strings.TrimSpace(projectID),
		Slug:      strings.TrimSpace(slug),
	}
	if sessionID != nil {
		req.SessionId = sessionID
	}
	if port != nil {
		req.Port = port
	}
	resp, err := c.sandbox.CreatePreviewUrl(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, mapError(err)
	}
	return previewURLFromProto(resp.Msg.PreviewUrl), nil
}

func (c *Client) DeletePreviewURL(ctx context.Context, previewURLID string) error {
	_, err := c.sandbox.DeletePreviewUrl(ctx, connect.NewRequest(&sandboxv1.DeletePreviewUrlRequest{
		PreviewUrlId: strings.TrimSpace(previewURLID),
	}))
	return mapError(err)
}

func (c *Client) BindPreviewURL(ctx context.Context, previewURLID string, sessionID string, port int32) (*PreviewURL, error) {
	resp, err := c.sandbox.BindPreviewUrl(ctx, connect.NewRequest(&sandboxv1.BindPreviewUrlRequest{
		PreviewUrlId: strings.TrimSpace(previewURLID),
		SessionId:    strings.TrimSpace(sessionID),
		Port:         port,
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return previewURLFromProto(resp.Msg.PreviewUrl), nil
}

func (c *Client) UnbindPreviewURL(ctx context.Context, previewURLID string) (*PreviewURL, error) {
	resp, err := c.sandbox.UnbindPreviewUrl(ctx, connect.NewRequest(&sandboxv1.UnbindPreviewUrlRequest{
		PreviewUrlId: strings.TrimSpace(previewURLID),
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return previewURLFromProto(resp.Msg.PreviewUrl), nil
}

func (c *Client) ListPreviewURLs(ctx context.Context, projectID string, pageSize int32, pageToken string) ([]*PreviewURL, string, error) {
	resp, err := c.sandbox.ListPreviewUrls(ctx, connect.NewRequest(&sandboxv1.ListPreviewUrlsRequest{
		ProjectId: strings.TrimSpace(projectID),
		PageSize:  pageSize,
		PageToken: strings.TrimSpace(pageToken),
	}))
	if err != nil {
		return nil, "", mapError(err)
	}
	items := make([]*PreviewURL, 0, len(resp.Msg.PreviewUrls))
	for _, item := range resp.Msg.PreviewUrls {
		items = append(items, previewURLFromProto(item))
	}
	return items, resp.Msg.NextPageToken, nil
}

func (c *Client) GetPreviewURL(ctx context.Context, previewURLID string) (*PreviewURL, error) {
	resp, err := c.sandbox.GetPreviewUrl(ctx, connect.NewRequest(&sandboxv1.GetPreviewUrlRequest{
		Lookup: &sandboxv1.GetPreviewUrlRequest_PreviewUrlId{PreviewUrlId: strings.TrimSpace(previewURLID)},
	}))
	if err != nil {
		return nil, mapError(err)
	}
	return previewURLFromProto(resp.Msg.PreviewUrl), nil
}
