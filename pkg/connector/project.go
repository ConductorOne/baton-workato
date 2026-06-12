package connector

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

var _ connectorbuilder.ResourceSyncerV2 = (*projectBuilder)(nil)

type projectBuilder struct {
	client *client.WorkatoClient
}

func (o *projectBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return projectResourceType
}

// List returns all the projects.
func (o *projectBuilder) List(ctx context.Context, _ *v2.ResourceId, attr rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	projects, nextToken, annos, err := o.client.GetProjects(ctx, attr.PageToken.Token)
	if err != nil {
		return nil, &rs.SyncOpResults{Annotations: annos}, err
	}

	rv := make([]*v2.Resource, len(projects))

	for i, project := range projects {
		us, err := projectResource(project)
		if err != nil {
			return nil, &rs.SyncOpResults{Annotations: annos}, err
		}
		rv[i] = us
	}

	return rv, &rs.SyncOpResults{NextPageToken: nextToken, Annotations: annos}, nil
}

// Entitlements returns an empty slice since projects are not assignable.
func (o *projectBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants returns an empty slice since projects are not grantable.
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
