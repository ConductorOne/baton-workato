package connector

import (
	"context"

	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type projectBuilder struct {
	client *client.WorkatoClient
}

func (o *projectBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return projectResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *projectBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	projects, nextToken, err := o.client.GetProjects(ctx, pToken.Token)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, len(projects))

	for i, project := range projects {
		us, err := projectResource(&project)
		if err != nil {
			return nil, "", nil, err
		}
		rv[i] = us
	}

	return rv, nextToken, nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *projectBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *projectBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
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
