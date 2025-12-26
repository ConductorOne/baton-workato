package connector

import (
	"context"

	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	rs "github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
)

var _ connectorbuilder.ResourceSyncerV2 = (*environmentBuilder)(nil)

type environmentBuilder struct {
	env workato.Environment
}

func (o *environmentBuilder) ResourceType(ctx context.Context) *v2.ResourceType {
	return environmentResourceType
}

// List returns environment resources based on the configured environment.
// If env is "all", it returns dev, test, and prod environments.
// Otherwise, it returns only the specific environment.
func (o *environmentBuilder) List(ctx context.Context, _ *v2.ResourceId, _ rs.SyncOpAttrs) ([]*v2.Resource, *rs.SyncOpResults, error) {
	l := ctxzap.Extract(ctx)
	l.Debug("Listing environments")

	rv := make([]*v2.Resource, 0)

	if o.env == workato.All {
		for _, env := range workato.AllEnvironments() {
			resource, err := environmentResource(env)
			if err != nil {
				return nil, nil, err
			}
			rv = append(rv, resource)
		}
		return rv, nil, nil
	}

	resource, err := environmentResource(o.env)
	if err != nil {
		return nil, nil, err
	}
	rv = append(rv, resource)

	return rv, nil, nil
}

// Entitlements returns an empty slice since environments are not assignable.
func (o *environmentBuilder) Entitlements(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Entitlement, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

// Grants returns an empty slice since environments are not grantable.
func (o *environmentBuilder) Grants(_ context.Context, _ *v2.Resource, _ rs.SyncOpAttrs) ([]*v2.Grant, *rs.SyncOpResults, error) {
	return nil, nil, nil
}

func newEnvironmentBuilder(env workato.Environment) *environmentBuilder {
	return &environmentBuilder{
		env: env,
	}
}

func environmentResource(env workato.Environment) (*v2.Resource, error) {
	displayName, err := workato.EnvironmentDisplayName(env)
	if err != nil {
		return nil, err
	}

	profile := map[string]interface{}{
		"environment": env.String(),
		"name":        displayName,
	}

	traits := []rs.GroupTraitOption{
		rs.WithGroupProfile(profile),
	}

	ret, err := rs.NewGroupResource(
		displayName,
		environmentResourceType,
		env.String(),
		traits,
	)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
