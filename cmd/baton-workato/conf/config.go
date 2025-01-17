package conf

import (
	"errors"

	"github.com/conductorone/baton-workato/pkg/connector/workato"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/spf13/viper"
)

var (
	ApiKeyField = field.StringField(
		"workato-api-key",
		field.WithRequired(true),
		field.WithDescription("Your workato API key"),
	)

	WorkatoDataCenterFiekd = field.StringField(
		"workato-data-center",
		field.WithDescription("Your workato data center (us, eu, jp, sg, au) default is 'us' see more on https://docs.workato.com/workato-api.html#base-url"),
		field.WithDefaultValue("us"),
	)

	WorkatoEnv = field.StringField(
		"workato-env",
		field.WithDescription("Your workato environment (dev, test, prod) default is 'dev'"),
		field.WithDefaultValue("dev"),
	)

	// ConfigurationFields defines the external configuration required for the
	// connector to run. Note: these fields can be marked as optional or
	// required.
	ConfigurationFields = []field.SchemaField{
		ApiKeyField,
		WorkatoDataCenterFiekd,
		WorkatoEnv,
	}

	// FieldRelationships defines relationships between the fields listed in
	// ConfigurationFields that can be automatically validated. For example, a
	// username and password can be required together, or an access token can be
	// marked as mutually exclusive from the username password pair.
	FieldRelationships = []field.SchemaFieldRelationship{}
)

// ValidateConfig is run after the configuration is loaded, and should return an
// error if it isn't valid. Implementing this function is optional, it only
// needs to perform extra validations that cannot be encoded with configuration
// parameters.
func ValidateConfig(v *viper.Viper) error {
	if _, ok := client.WorkatoDataCenters[v.GetString(WorkatoDataCenterFiekd.FieldName)]; !ok {
		return errors.New("invalid workato data center")
	}

	_, err := workato.EnvFromString(v.GetString(WorkatoEnv.FieldName))
	if err != nil {
		return err
	}

	return nil
}
