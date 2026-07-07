package connector

import (
	"context"
	"fmt"
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
	syncEnvironmentRoles   bool
}

// ResourceSyncers returns a ResourceSyncer for each resource type that should be synced from the upstream service.
func (d *Connector) ResourceSyncers(ctx context.Context) []connectorbuilder.ResourceSyncerV2 {
	return []connectorbuilder.ResourceSyncerV2{
		newEnvironmentRoleBuilder(d.client, d.env),
		newCollaboratorBuilder(d.client, d.env, d.disableCustomRolesSync, d.syncEnvironmentRoles),
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
		AccountCreationSchema: &v2.ConnectorAccountCreationSchema{
			FieldMap: map[string]*v2.ConnectorAccountCreationSchema_Field{
				"email": {
					DisplayName: "Email",
					Required:    true,
					Description: "Email address of the collaborator to invite.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "jane.doe@example.com",
					Order:       1,
				},
				"name": {
					DisplayName: "Name",
					Required:    true,
					Description: "Full name of the collaborator to invite.",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "Jane Doe",
					Order:       2,
				},
				"env_roles": {
					DisplayName: "Environment Roles",
					Required:    true,
					Description: "List of environment + role assignments in \"env:role\" format " +
						"(e.g. \"dev:Admin\", \"prod:Analyst\"). Allowed environments: dev, test, prod. " +
						"At least one entry is required. The role must exist in the target environment — " +
						"a non-provisioned environment fails at invite time with \"Environment <env> not found\".",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringListField{
						StringListField: &v2.ConnectorAccountCreationSchema_StringListField{},
					},
					Order: 3,
				},
				"user_group_ids": {
					DisplayName: "User Group IDs",
					Required:    false,
					Description: "Comma-separated list of Workato user group string IDs to add the collaborator to, e.g. \"am-WxEKCibh-dTXBtz\".",
					Field: &v2.ConnectorAccountCreationSchema_Field_StringField{
						StringField: &v2.ConnectorAccountCreationSchema_StringField{},
					},
					Placeholder: "am-WxEKCibh-dTXBtz,am-AbCdEfGh-iJkLmNo",
					Order:       4,
				},
			},
		},
	}, nil
}

// Validate is called to ensure that the connector is properly configured. It should exercise any API credentials
// to be sure that they are valid.
func (d *Connector) Validate(ctx context.Context) (annotations.Annotations, error) {
	_, annos, err := d.client.GetCollaborators(ctx)
	if err != nil {
		return nil, fmt.Errorf("baton-workato: failed to validate credentials: %w", err)
	}
	return annos, nil
}

// New returns a new instance of the connector.
func NewConnector(ctx context.Context, workatoClient *client.WorkatoClient, env workato.Environment, disableCustomRolesSync bool, syncEnvironmentRoles bool) (*Connector, error) {
	return &Connector{
		client:                 workatoClient,
		env:                    env,
		disableCustomRolesSync: disableCustomRolesSync,
		syncEnvironmentRoles:   syncEnvironmentRoles,
	}, nil
}

// New returns the Workato connector configured to sync against the instance URL.
func New(ctx context.Context, config *cfg.Workato, connectorOpts *cli.ConnectorOpts) (connectorbuilder.ConnectorBuilderV2, []connectorbuilder.Opt, error) {
	l := ctxzap.Extract(ctx)
	err := field.Validate(cfg.Config, config)
	if err != nil {
		return nil, nil, err
	}

	dataCenterUrl := client.WorkatoDataCenters[config.WorkatoDataCenter]
	// base-url override: empty in production (use the data center), set only for
	// self-hosted proxies or integration tests against cmd/test-server.
	if config.WorkatoBaseUrl != "" {
		dataCenterUrl = config.WorkatoBaseUrl
	}

	env, err := workato.EnvFromString(config.WorkatoEnv)
	if err != nil {
		return nil, nil, err
	}

	workatoClient, err := client.NewWorkatoClient(ctx, config.WorkatoApiKey, dataCenterUrl)
	if err != nil {
		return nil, nil, err
	}

	syncEnvRoles := connectorOpts != nil && connectorOpts.WillSyncResourceType(environmentRoleResourceType.Id)
	cb, err := NewConnector(ctx, workatoClient, env, config.DisableCustomRolesSync, syncEnvRoles)
	if err != nil {
		l.Error("error creating connector", zap.Error(err))
		return nil, nil, err
	}
	return cb, nil, nil
}
