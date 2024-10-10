package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

func filterConceptMaps(cms *[]DECORConceptMap, prefix string) *[]DECORConceptMap {
	filtered := slices.DeleteFunc(*cms, func(cm DECORConceptMap) bool {
		return !strings.HasPrefix(cm.DisplayName, prefix)
	})
	return &filtered
}

func downloadFile(path string, url string) error {
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
