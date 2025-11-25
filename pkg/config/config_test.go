package config

import (
	"testing"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/conductorone/baton-sdk/pkg/test"
	"github.com/spf13/viper"
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
