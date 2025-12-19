package connector

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

var _ connectorbuilder.ResourceSyncerV2 = (*projectBuilder)(nil)

type projectBuilder struct {
	client *client.WorkatoClient
}

func (o *projectBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return projectResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *projectBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("Listing projects")

	projects, nextToken, err := o.client.GetProjects(ctx, attr.PageToken.Token)
	if err != nil {
		return nil, nil, err
	}

	rv := make([]*v2.Resource, len(projects))

	for i, project := range projects {
		us, err := projectResource(&project)
		if err != nil {
			return nil, nil, err
		}
		rv[i] = us
	}

	return rv, &rs.SyncOpResults{
		NextPageToken: nextToken,
	}, nil
}

// Entitlements always returns an empty slice for users.
func (o *projectBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *projectBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func newProjectBuilder(client *client.WorkatoClient) *projectBuilder {
	return &projectBuilder{
		client: client,
	}
}

func projectResource(project *client.Project) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"id":          project.Id,
		"name":        project.Name,
		"description": project.Description,
		"folder_id":   project.FolderId,
	}

	traits := []rs.AppTraitOption{
		rs.WithAppProfile(profile),
	}

	ret, err := rs.NewAppResource(
		project.Name,
		projectResourceType,
		project.Id,
		traits,
		rs.WithAnnotation(
			&v2.ChildResourceType{
				ResourceTypeId: folderResourceType.Id,
			},
		),
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
