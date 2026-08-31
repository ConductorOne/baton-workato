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
		// Add test cases here.
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
