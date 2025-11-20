package connector

import (
	"context"
	"io"

	"github.com/conductorone/baton-workato/pkg/connector/workato"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"

	"github.com/conductorone/baton-workato/pkg/connector/client"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
	"github.com/conductorone/baton-sdk/pkg/cli"
	"github.com/conductorone/baton-sdk/pkg/connectorbuilder"
	"github.com/conductorone/baton-sdk/pkg/field"
	cfg "github.com/conductorone/baton-workato/pkg/config"
)

type Connector struct {
	client                 *client.WorkatoClient
	env                    workato.Environment
	disableCustomRolesSync bool
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncerV2 {
	return []connectorbuilder.ResourceSyncerV2{
		newCollaboratorBuilder(d.client, d.env),
		newPrivilegeBuilder(d.client),
		newRoleBuilder(d.client, d.env, d.disableCustomRolesSync),
		newFolderBuilder(d.client, d.disableCustomRolesSync),
		newProjectBuilder(d.client),
	}
}

// Asset takes an input AssetRef and attempts to fetch it using the connector's authenticated http client
// It streams a response, always starting with a metadata object, following by chunked payloads for the asset.
func (d *Connector) Asset(ctx context.Context, asset *v2.AssetRef) (string, io.ReadCloser, error) {
	return "", nil, nil
}

// Metadata returns metadata about the connector.
func (d *Connector) Metadata(ctx context.Context) (*v2.ConnectorMetadata, error) {
	return &v2.ConnectorMetadata{
		DisplayName: "Workato",
		Description: "Connector to sync collaborators, project, folders, roles and privileges from Workato.",
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials
// to be sure that they are valid.
func (d *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	return nil, nil
}

// New returns a new instance of the connector.
func NewConnector(ctx context.Context, workatoClient *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool) (*Connector, error) {
	return &Connector{
		client:                 workatoClient,
		env:                    env,
		disableCustomRolesSync: disableCustomRolesSync,
	}, nil
}

// New returns the Workato connector configured to sync against the instance URL.
func New(ctx context.Context, config *cfg.Workato, opts *cli.ConnectorOpts) (connectorbuilder.ConnectorBuilderV2, []connectorbuilder.Opt, error) {
	l := ctxzap.Extract(ctx)
	err := field.Validate(cfg.Config, config)
	if err != nil {
		return nil, nil, err
	}

	dataCenterUrl := client.WorkatoDataCenters[config.WorkatoDataCenter]

	env, err := workato.EnvFromString(config.WorkatoEnv)
	if err != nil {
		return nil, nil, err
	}

	workatoClient, err := client.NewWorkatoClient(ctx, config.WorkatoApiKey, dataCenterUrl)
	if err != nil {
		return nil, nil, err
	}

	cb, err := NewConnector(ctx, workatoClient, env, config.DisableCustomRolesSync)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, nil, err
	}
	return cb, nil, nil
}
