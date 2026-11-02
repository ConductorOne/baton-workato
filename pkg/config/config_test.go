package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/conductorone/baton-workato/pkg/connector/client"
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

// TestDataCenterSelectURLLockstep binds the workato-data-center select options
// to the WorkatoDataCenters URL map in BOTH directions, closing the gap where a
// future select option without a map entry would pass validation and resolve to
// an empty base URL at the direct index in pkg/connector/connector.go.
//
// The in-list is derived hermetically from the in-memory marshal of the
// connector's declared field.Configuration (json.Marshal(&Config) →
// Configuration.MarshalJSON → protojson, the same shape the CLI writes to
// config_schema.json), so a config.go-only option addition is caught at PR
// time — not only post-merge when the regen bot commits a fresh schema.
// A second assertion keeps the committed config_schema.json pinned to that
// in-memory list, so post-merge file freshness stays enforced too.
func TestDataCenterSelectURLLockstep(t *testing.T) {
	inList := parseDataCenterInList(t, jsonMarshalConfigSchema(t))

	// Direction 1: every select option must have a URL map entry.
	for _, opt := range inList {
		u, ok := client.WorkatoDataCenters[opt]
		require.True(t, ok, "select option %q has no WorkatoDataCenters entry; it would resolve to an empty base URL", opt)
		require.NotEmpty(t, u, "WorkatoDataCenters[%q] must not be empty", opt)
	}

	// Direction 2: every URL map key must be an accepted select option.
	accepted := make(map[string]struct{}, len(inList))
	for _, opt := range inList {
		accepted[opt] = struct{}{}
	}
	for region := range client.WorkatoDataCenters {
		require.Contains(t, accepted, region, "WorkatoDataCenters key %q is not an accepted select option", region)
	}

	// Freshness pin: the committed config_schema.json must equal the in-memory
	// marshal, so the file never drifts from the declared configuration.
	committed := parseDataCenterInList(t, readCommittedSchema(t))
	require.Equal(t, inList, committed,
		"committed config_schema.json is stale: its workato-data-center in-list differs from the in-memory field.Configuration marshal")
}

// jsonMarshalConfigSchema marshals the in-memory configuration the same way the
// CLI does when writing config_schema.json.
func jsonMarshalConfigSchema(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(&Config)
	require.NoError(t, err)
	return b
}

func readCommittedSchema(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "config_schema.json"))
	require.NoError(t, err, "committed config_schema.json must be readable from the test")
	return b
}

// parseDataCenterInList extracts the workato-data-center select in-list from a
// marshaled config-schema document.
func parseDataCenterInList(t *testing.T, schemaBytes []byte) []string {
	t.Helper()
	var schema struct {
		Fields []struct {
			Name        string `json:"name"`
			StringField *struct {
				Rules struct {
					In []string `json:"in"`
				} `json:"rules"`
			} `json:"stringField"`
		} `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(schemaBytes, &schema))

	var selectOptions []string
	for _, f := range schema.Fields {
		if f.Name == "workato-data-center" && f.StringField != nil {
			selectOptions = f.StringField.Rules.In
			break
		}
	}
	require.NotEmpty(t, selectOptions, "workato-data-center property with stringField.rules.in not found")
	return selectOptions
}
