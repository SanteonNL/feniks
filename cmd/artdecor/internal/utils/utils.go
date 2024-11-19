// File: internal/utils/utils.go
package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/SanteonNL/fenix/cmd/artdecor/types"

	"golang.org/x/exp/slices"
)

// ParseQueryParams converts a map of query parameters to a URL query string
func ParseQueryParams(queryParams any, validParams []string) (string, error) {
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

// validQueryParam checks if a parameter is in the list of valid parameters
func validQueryParam(param string, validParams []string) bool {
	return slices.Contains(validParams, param)
}

// CodeSystemNameToValueSetName converts a code system name to a value set name
func CodeSystemNameToValueSetName(s string) string {
	return strings.TrimSuffix(s, "CodeSystem") + "Codelijst"
}

// FilterConceptMaps filters concept maps based on a prefix
func FilterConceptMaps(cms *[]types.DECORConceptMap, prefix string) *[]types.DECORConceptMap {
	if cms == nil {
		return nil
	}
	filtered := slices.DeleteFunc(*cms, func(cm types.DECORConceptMap) bool {
		return !strings.HasPrefix(cm.DisplayName, prefix)
	})
	return &filtered
}

// DownloadFile downloads a file from a URL to a local path
func DownloadFile(path string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
