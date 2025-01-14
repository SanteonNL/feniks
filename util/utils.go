package util

import (
	"log"
	"os"
	"path/filepath"
)

func GetAbsolutePath(relativePath string) string {
	// Get the current working directory
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Join the current working directory with the relative path
	absolutePath := filepath.Join(root, relativePath)

	return absolutePath
}

// TODO: check of this function can be removed and use StringPtrOrNil instead that is now definied in populate.go
func StringPtr(s string) *string {
	return &s
}

func IntPtr(i int) *int {
	return &i
}
