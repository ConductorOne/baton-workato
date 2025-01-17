package workato

import (
	"reflect"
	"testing"
)

func TestAllCompoundPrivileges(t *testing.T) {
	result := AllCompoundPrivileges()

	if len(result) != 80 {
		t.Errorf("Expected 80, got %d", len(result))
	}
}

func TestFindRelatedPrivileges(t *testing.T) {
	cases := []struct {
		name string

		input    map[string][]string
		expected []CompoundPrivilege
	}{
		{
			name:     "empty",
			input:    map[string][]string{},
			expected: []CompoundPrivilege{},
		},
		{
			name: "two",
			input: map[string][]string{
				"Recipes": {"read", "create"},
				"Folders": {"read", "create"},
			},
			expected: []CompoundPrivilege{
				{
					Resource: "Recipes",
					Privilege: Privilege{
						Id:          "read",
						Description: "View recipes in a workspace.",
					},
				},
				{
					Resource: "Recipes",
					Privilege: Privilege{
						Id:          "create",
						Description: "Create recipes in a workspace.",
					},
				},
				{
					Resource: "Folders",
					Privilege: Privilege{
						Id:          "read",
						Description: "View folders and sub-folders in a workspace.",
					},
				},
				{
					Resource: "Folders",
					Privilege: Privilege{
						Id:          "create",
						Description: "Create folders and sub-folders in a workspace.",
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := FindRelatedPrivileges(c.input)

			if len(result) != len(c.expected) {
				t.Errorf("Expected %d, got %d", len(c.expected), len(result))
			}

			if !reflect.DeepEqual(result, c.expected) {
				t.Errorf("Expected %v, got %v", c.expected, result)
			}
		})
	}
}
