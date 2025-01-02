package connector

import (
	"context"
	"fmt"

	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/pagination"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
	"github.com/conductorone/baton-sdk/pkg/types/grant"
)

const (
	assignedEntitlement = "assigned"
)

type privilegeBuilder struct {
	client *client.WorkatoClient
	cache  *collaboratorCache
}

func (o *privilegeBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return privilegeResourceType
}

// List returns all the users from the database as resource objects.
// Users include a UserTrait because they are the 'shape' of a standard user.
func (o *privilegeBuilder) List(ctx context.Context, parentResourceID *v2.ResourceId, pToken *pagination.Token) ([]*v2.Resource, string, annotations.Annotations, error) {
	l := ctxzap.Extract(ctx)

	if pToken == nil || pToken.Token == "" {
		err := o.cache.buildCache(ctx)
		if err != nil {
			l.Error("Error building cache", zap.Error(err))
			return nil, "", nil, err
		}
	}

	privileges := workato.AllCompoundPrivileges()

	rv := make([]*v2.Resource, len(privileges))

	for i, privilege := range privileges {
		us, err := privilegeResource(&privilege)
		if err != nil {
			return nil, "", nil, err
		}
		rv[i] = us
	}

	return rv, "", nil, nil
}

// Entitlements always returns an empty slice for users.
func (o *privilegeBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ *pagination.Token) ([]*v2.Entitlement, string, annotations.Annotations, error) {
	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("Assigned %s to scopes", collaboratorResourceType.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s have %s`", collaboratorResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, assignedEntitlement, assigmentOptions...))

	return rv, "", nil, nil
}

// Grants always returns an empty slice for users since they don't have any entitlements.
func (o *privilegeBuilder) Grants(ctx context.Context, resource *v2.Resource, pToken *pagination.Token) ([]*v2.Grant, string, annotations.Annotations, error) {
	privilegeId := resource.Id.Resource

	users := o.cache.getUsersByPrivilege(privilegeId)

	var rv []*v2.Grant

	for _, user := range users {
		collaboratorId, err := rs.NewResourceID(collaboratorResourceType, user.User.Id)
		if err != nil {
			return nil, "", nil, err
		}

		// Collaborator only have privileges if a role is assigned to them
		// To update collaborator privileges, the role must be updated
		grantToCollaborator := grant.NewGrant(
			resource,
			assignedEntitlement,
			collaboratorId,
			grant.WithAnnotation(&v2.GrantImmutable{}),
		)

		rv = append(rv, grantToCollaborator)
	}

	return rv, "", nil, nil
}

func newPrivilegeBuilder(client *client.WorkatoClient) *privilegeBuilder {
	return &privilegeBuilder{
		client: client,
		cache:  newCollaboratorCache(client),
	}
}

func privilegeResource(privilege *workato.CompoundPrivilege) (*v2.Resource, error) {
	profile := map[string]interface{}{
		"resource":    privilege.Resource,
		"permission":  privilege.Privilege.Id,
		"description": privilege.Privilege.Description,
	}

	traits := []rs.RoleTraitOption{
		rs.WithRoleProfile(profile),
	}

	ret, err := rs.NewRoleResource(
		privilege.Resource+"-"+privilege.Privilege.Id,
		privilegeResourceType,
		privilege.Id(),
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
