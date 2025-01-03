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

func TestName(t *testing.T) {
	roles := []int{}

	roles = append(roles[:0], roles[0+1:]...)

	if len(roles) != 4 {
		t.Errorf("Error: %v", roles)
	}
}
