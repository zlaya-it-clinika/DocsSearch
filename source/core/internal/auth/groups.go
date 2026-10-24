package auth

import (
	"strings"

	"docssearch/core/internal/texts"
)

type GroupRole struct {
	RoleID string
	Group  string
}

func CommonName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if len(part) > 3 && strings.EqualFold(part[:3], "CN=") {
			return strings.TrimSpace(part[3:])
		}
	}
	return value
}

func Group_Names(values []string) []string {
	names := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		name := CommonName(value)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names
}

// Совпавшие роли заменяют набор. Главный администратор остается, даже если его группы нет в списке.
// Пустой список совпадений возвращает роль по умолчанию.
func RoleIDs_ForGroups(groups []string, mapped []GroupRole, current []string, defaultRoleID string) []string {
	known := map[string]struct{}{}
	for _, group := range Group_Names(groups) {
		known[strings.ToLower(group)] = struct{}{}
	}
	next := []string{}
	for _, role := range mapped {
		name := strings.ToLower(strings.TrimSpace(role.Group))
		if name == "" {
			continue
		}
		if _, found := known[name]; !found {
			continue
		}
		if !contains_RoleID(next, role.RoleID) {
			next = append(next, role.RoleID)
		}
	}
	if len(next) == 0 && defaultRoleID != "" {
		next = append(next, defaultRoleID)
	}
	if contains_RoleID(current, texts.RoleIDChief) && !contains_RoleID(next, texts.RoleIDChief) {
		next = append(next, texts.RoleIDChief)
	}
	return next
}

func contains_RoleID(ids []string, wanted string) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}
