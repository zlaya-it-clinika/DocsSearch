package auth

import "testing"

func TestGroup_NamesReadsCommonName(t *testing.T) {
	names := Group_Names([]string{
		"CN=Архив,OU=Groups,DC=corp,DC=local",
		"cn=архив,OU=Other,DC=corp,DC=local",
		"Docs",
	})
	if len(names) != 2 || names[0] != "Архив" || names[1] != "Docs" {
		t.Fatalf("%v", names)
	}
}

func TestRoleIDs_ForGroupsKeepsChief(t *testing.T) {
	mapped := []GroupRole{{RoleID: "role-1", Group: "Архив"}}
	matched := RoleIDs_ForGroups([]string{"CN=Архив,OU=Groups,DC=corp,DC=local"}, mapped, []string{"role-chief"}, "role-user")
	if len(matched) != 2 || matched[0] != "role-1" || matched[1] != "role-chief" {
		t.Fatalf("matched %v", matched)
	}
	fallback := RoleIDs_ForGroups(nil, mapped, []string{"role-user"}, "role-user")
	if len(fallback) != 1 || fallback[0] != "role-user" {
		t.Fatalf("fallback %v", fallback)
	}
	chiefFallback := RoleIDs_ForGroups(nil, mapped, []string{"role-chief"}, "role-user")
	if len(chiefFallback) != 2 || chiefFallback[0] != "role-user" || chiefFallback[1] != "role-chief" {
		t.Fatalf("chief fallback %v", chiefFallback)
	}
}
