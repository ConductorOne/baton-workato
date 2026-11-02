package config

import (
	"testing"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigs(t *testing.T) {
	configurationSchema := field.NewConfiguration(
		ConfigurationFields,
		field.WithConstraints(FieldRelationships...),
	)

	testCases := []test.TestCase{
		{
			Configs: map[string]string{
				"workato-api-key":     "key",
				"workato-data-center": "kr",
			},
			IsValid: true,
			Message: "kr data center is valid",
		},
		{
			Configs: map[string]string{
				"workato-api-key":     "key",
				"workato-data-center": "cn",
			},
			IsValid: true,
			Message: "cn data center is valid",
		},
		{
			Configs: map[string]string{
				"workato-api-key":     "key",
				"workato-data-center": "us",
			},
			IsValid: true,
			Message: "us data center is valid",
		},
		{
			Configs: map[string]string{
				"workato-api-key":     "key",
				"workato-data-center": "xx",
			},
			IsValid: false,
			Message: "unknown data center is rejected",
		},
		{
			Configs: map[string]string{
				"workato-data-center": "us",
			},
			IsValid: false,
			Message: "missing workato-api-key is rejected",
		},
	}

	validateConfig := func(v *viper.Viper) error {
		return nil
	}

	test.ExerciseTestCases(t, configurationSchema, validateConfig, testCases)
}

// TestDisableCustomRolesSyncDefault pins the resolved default for
// disable-custom-roles-sync so an accidental flip in either direction fails
// CI instead of silently changing sync behavior for installs that don't set
// the flag explicitly.
func TestDisableCustomRolesSyncDefault(t *testing.T) {
	defaultValue, err := field.GetDefaultValue[bool](DisableCustomRolesSync)
	require.NoError(t, err)
	require.NotNil(t, defaultValue)
	require.True(t, *defaultValue, "disable-custom-roles-sync should default to true")
}

// TestDataCenterDefault pins the resolved default for workato-data-center so
// an accidental change to the default data center fails CI instead of
// silently redirecting installs to a different Workato deployment.
func TestDataCenterDefault(t *testing.T) {
	defaultValue, err := field.GetDefaultValue[string](WorkatoDataCenterFiekd)
	require.NoError(t, err)
	require.NotNil(t, defaultValue)
	require.Equal(t, "us", *defaultValue, "workato-data-center should default to us")
}
