package connector

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
)

type collaboratorBuilder struct {
	client *client.WorkatoClient
}

func (o *collaboratorBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return collaboratorResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *collaboratorBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	collaborators, err := o.client.GetCollaborators(ctx)
	if err != nil {
		return nil, "", nil, err
	}

	rv := make([]*v2.Resource, len(collaborators))

	for i, collaborator := range collaborators {
		us, err := collaboratorResource(&collaborator)
		if err != nil {
			return nil, "", nil, err
		}
		rv[i] = us
	}

	return rv, "", nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *collaboratorBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *collaboratorBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	return nil, "", nil, nil
}

func newCollaboratorBuilder(client *client.WorkatoClient) *collaboratorBuilder {
	return &collaboratorBuilder{
		client: client,
	}
}

func collaboratorResource(collaborator *client.Collaborator) (*v2.Resource, error) {
	var userStatus = v2.UserTrait_Status_STATUS_ENABLED

	profile := map[string]interface{}{
		"id":         collaborator.Id,
		"email":      collaborator.Email,
		"name":       collaborator.Name,
		"externalId": collaborator.ExternalId,
		"createdAt":  collaborator.CreatedAt.String(),
		"grantType":  collaborator.GrantType,
		"timeZone":   collaborator.TimeZone,
	}

	traits := []resource.UserTraitOption{
		resource.WithUserProfile(profile),
		resource.WithStatus(userStatus),
		resource.WithEmail(collaborator.Email, true),
		resource.WithUserLogin(collaborator.Email),
	}

	ret, err := resource.NewUserResource(
		collaborator.Name,
		collaboratorResourceType,
		collaborator.Id,
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
