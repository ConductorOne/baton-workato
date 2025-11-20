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

	WorkatoDataCenterFiekd = field.StringField(
		"workato-data-center",
		field.WithDisplayName("Data center"),
		field.WithDescription("Your workato data center (us, eu, jp, sg, au) default is 'us' see more on https://docs.workato.com/workato-api.html#base-url"),
		field.WithDefaultValue("us"),
	)

	WorkatoEnv = field.StringField(
		"workato-env",
		field.WithDisplayName("Environment"),
		field.WithDescription("Your workato environment (dev, test, prod) default is 'dev'"),
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
	field.WithConnectorDisplayName("Wrokato"),
	field.WithHelpUrl("/docs/baton/workato"),
	field.WithIconUrl("/static/app-icons/workato.svg"),
)
