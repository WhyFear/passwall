package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"passwall/internal/model"
	"passwall/internal/repository"
)

var errInvalidNodeFilter = errors.New("invalid node filter")

func parseNodeFilter(statusText string, typeText string, countryCodeText string, riskLevelText string, appUnlockText string) (*repository.NodeFilter, error) {
	filter := &repository.NodeFilter{}
	hasFilter := false

	if strings.TrimSpace(statusText) != "" {
		statusParts := splitFilterValues(statusText)
		statuses := make([]model.ProxyStatus, 0, len(statusParts))
		for _, statusPart := range statusParts {
			status, err := strconv.Atoi(statusPart)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid status %q", errInvalidNodeFilter, statusPart)
			}
			statuses = append(statuses, model.ProxyStatus(status))
		}
		if len(statuses) > 0 {
			filter.Status = statuses
			hasFilter = true
		}
	}
	if strings.TrimSpace(typeText) != "" {
		typeParts := splitFilterValues(typeText)
		types := make([]model.ProxyType, 0, len(typeParts))
		for _, proxyType := range typeParts {
			types = append(types, model.ProxyType(proxyType))
		}
		if len(types) > 0 {
			filter.Types = types
			hasFilter = true
		}
	}
	if strings.TrimSpace(countryCodeText) != "" {
		filter.CountryCode = splitFilterValues(countryCodeText)
		hasFilter = hasFilter || len(filter.CountryCode) > 0
	}
	if strings.TrimSpace(riskLevelText) != "" {
		filter.RiskLevel = splitFilterValues(riskLevelText)
		hasFilter = hasFilter || len(filter.RiskLevel) > 0
	}
	if strings.TrimSpace(appUnlockText) != "" {
		filter.AppUnlock = splitFilterValues(appUnlockText)
		hasFilter = hasFilter || len(filter.AppUnlock) > 0
	}

	if !hasFilter {
		return nil, nil
	}
	return filter, nil
}

func splitFilterValues(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		result = append(result, part)
	}
	return result
}
