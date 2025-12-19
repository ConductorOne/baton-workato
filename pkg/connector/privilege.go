package connector

import (
	"context"
	"fmt"

	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/types/entitlement"
)

const (
	assignedEntitlement = "assigned"
)

var _ connectorbuilder.ResourceSyncerV2 = (*privilegeBuilder)(nil)

type privilegeBuilder struct {
	client *client.WorkatoClient
	cache  *collaboratorCache
}

func (o *privilegeBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return privilegeResourceType
}

// List returns all the privileges.
func (o *privilegeBuilder) List(ctx context.Context, _ *v2.ResourceId, _ rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("Listing privileges")

	privileges := workato.AllCompoundPrivileges()

	rv := make([]*v2.Resource, 0)

	for _, privilege := range privileges {
		us, err := privilegeResource(&privilege)
		if err != nil {
			return nil, nil, err
		}
		rv = append(rv, us)
	}

	return rv, nil, nil
}

// Entitlements returns an entitlement for the privilege to be assigned to a collaborator.
func (o *privilegeBuilder) Entitlements(_ context.Context, resource *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	var rv []*v2.Entitlement
	assigmentOptions := []entitlement.EntitlementOption{
		entitlement.WithGrantableTo(collaboratorResourceType),
		entitlement.WithDescription(fmt.Sprintf("Assigned %s to scopes", collaboratorResourceType.DisplayName)),
		entitlement.WithDisplayName(fmt.Sprintf("%s have %s`", collaboratorResourceType.DisplayName, resource.DisplayName)),
	}
	rv = append(rv, entitlement.NewAssignmentEntitlement(resource, assignedEntitlement, assigmentOptions...))

	return rv, nil, nil
}

// Grants returns an empty slice. Grants for privileges are emitted when listing collaborator grants.
func (o *privilegeBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
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
