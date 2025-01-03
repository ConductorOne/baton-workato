package workato

import "testing"

func TestBaseRoles(t *testing.T) {
	_, err := FindRelatedPrivilegesErr(analystBaseRoles)
	if err != nil {
		t.Errorf("Error: %v", err)
	}

	_, err = FindRelatedPrivilegesErr(operatorBaseRoles)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
}
