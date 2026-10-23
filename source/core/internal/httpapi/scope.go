package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"docssearch/core/internal/texts"
)

func read_ListScope(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch raw {
	case "", texts.ScopeGlobal, texts.ScopePersonal, texts.ScopeAll:
		return raw, true
	default:
		return "", false
	}
}

func read_UploadScope(raw string, accessLevel int) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		if accessLevel >= adminAccessLevel {
			return texts.ScopeGlobal, true
		}
		return texts.ScopePersonal, true
	}
	if raw == texts.ScopeGlobal || raw == texts.ScopePersonal {
		return raw, true
	}
	return "", false
}

func check_CanUploadGlobal(accessLevel int) bool {
	return accessLevel >= adminAccessLevel
}

func check_CanDeleteDocument(viewerID string, viewerLevel int, scope, ownerID string) bool {
	if scope == texts.ScopePersonal {
		return viewerID == ownerID || viewerLevel >= adminAccessLevel
	}
	return viewerLevel >= adminAccessLevel
}

func document_MatchesList(scopeFilter, documentScope, ownerID, viewerID string, documentLevel, viewerLevel int) bool {
	switch scopeFilter {
	case texts.ScopeAll:
		return true
	case texts.ScopeGlobal:
		return documentScope == texts.ScopeGlobal && documentLevel <= viewerLevel
	case texts.ScopePersonal:
		return documentScope == texts.ScopePersonal && ownerID == viewerID
	default:
		if documentScope == texts.ScopePersonal {
			return ownerID == viewerID
		}
		return documentScope == texts.ScopeGlobal && documentLevel <= viewerLevel
	}
}

// Чужой личный документ для сотрудника выглядит как отсутствующий.
func document_ReadStatus(viewerID string, viewerLevel int, scope, ownerID string, documentLevel int) int {
	if scope == texts.ScopePersonal {
		if viewerID == ownerID || viewerLevel >= adminAccessLevel {
			return http.StatusOK
		}
		return http.StatusNotFound
	}
	if viewerLevel < documentLevel {
		return http.StatusForbidden
	}
	return http.StatusOK
}

func read_AccessLevel(raw string, fallback int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
