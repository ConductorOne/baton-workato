package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	ApiKeyField = field.StringField(
		"workato-api-key",
		field.WithRequired(true),
		field.WithDisplayName("API key"),
		field.WithDescription("Your workato API key"),
		field.WithIsSecret(true),
	)

	WorkatoDataCenterFiekd = field.SelectField(
		"workato-data-center",
		[]string{"us", "eu", "jp", "sg", "au", "il", "sandbox"},
		field.WithDisplayName("Data center"),
		field.WithDescription("Your workato data center (us, eu, jp, sg, au, il, sandbox). See more on https://docs.workato.com/workato-api.html#base-url"),
		field.WithDefaultValue("us"),
	)

	// BaseUrlField overrides the data-center-derived base URL. It is empty in
	// production (the data center selection is used) and only set for integration
	// tests that point the connector at the local cmd/test-server mock.
	BaseUrlField = field.StringField(
		"workato-base-url",
		field.WithDisplayName("Base URL override"),
		field.WithDescription("Override the API base URL. Leave empty to use the selected data center; set only for self-hosted proxies or integration tests against the local test server."),
		field.WithExportTarget(field.ExportTargetCLIOnly),
	)

	WorkatoEnv = field.SelectField(
		"workato-env",
		[]string{"dev", "test", "prod", "all"},
		field.WithDisplayName("Environment"),
		field.WithDescription("Your workato environment (dev, test, prod, all)"),
		field.WithDefaultValue("dev"),
	)

	DisableCustomRolesSync = field.BoolField(
		"disable-custom-roles-sync",
		field.WithDisplayName("Disable custom roles sync"),
		field.WithDescription("Whether to disable custom roles sync or not"),
		field.WithDefaultValue(false),
	)

	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		ApiKeyField,
		WorkatoDataCenterFiekd,
		WorkatoEnv,
		DisableCustomRolesSync,
		BaseUrlField,
	}

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated. For example, a
	// username and password can be required together, or an access token can be
	// marked as mutually exclusive from the username password pair.
	FieldRelationships = []field.SchemaFieldRelationship{}
)

//go:generate go run ./gen
var Config = field.NewConfiguration(
	ConfigurationFields,
	field.WithConstraints(FieldRelationships...),
	field.WithConnectorDisplayName("Workato"),
	field.WithHelpUrl("/docs/baton/workato"),
	field.WithIconUrl("/static/app-icons/workato.svg"),
)
