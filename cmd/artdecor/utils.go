package main

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

func parseQueryParams(queryParams any, validParams []string) (string, error) {
	switch q := queryParams.(type) {
	case map[string]string:
		var query string
		for k, v := range q {
			if !validQueryParam(k, validParams) {
				return "", fmt.Errorf("invalid query parameter %q", k)
			}
			query += k + "=" + v + "&"
		}
		return strings.TrimSuffix(query, "&"), nil
	}
	return "", nil
}

func validQueryParam(param string, validParams []string) bool {
	return slices.Contains(validParams, param)
}

func codeSystemNameToValueSetName(s string) string {
	return strings.TrimSuffix(s, "CodeSystem") + "Codelijst"
}
