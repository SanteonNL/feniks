package main

import (
	"reflect"
	"strings"
	"unicode"
)

// TODO: cleanup functionst that are not used anymore

// Function to determine the relevant FHIRPath for Coding and Quantity types
func getValueSetBindingPath(fhirPath string, fhirType string) string {
	// Check for Coding type
	if fhirType == "Coding" {
		// Check if the FHIRPath contains `.coding`, in that case the coding is nested under a CodeableConcept
		if strings.Contains(fhirPath, ".coding") {
			// Remove ".coding" from the path
			pathBeforeCoding := strings.Split(fhirPath, ".coding")[0]
			// Split the remaining path and get the last two elements
			parts := strings.Split(pathBeforeCoding, ".")
			if len(parts) >= 2 {
				joined := parts[len(parts)-2] + "." + parts[len(parts)-1]
				fhirPath := capitalizeFirstLetter(joined)
				return fhirPath
			}
			return pathBeforeCoding // Return the remaining path if it's too short
		} else {
			// Split the path and get the last two elements directly
			parts := strings.Split(fhirPath, ".")
			if len(parts) >= 2 {
				joined := parts[len(parts)-2] + "." + parts[len(parts)-1]
				fhirPath := capitalizeFirstLetter(joined)
				return fhirPath
			}
			return fhirPath // Return the path if it's too short
		}
	}

	// Check for Quantity type
	if fhirType == "Quantity" {
		fhirPath = extractAndCapitalizeLastTwoParts(fhirPath)
		return fhirPath
	}

	return "" // Default return for other cases
}

// TODO this function needs adjustment as sometimes the bindingpath has more than two parts
// extractAndCapitalizeLastTwoParts extracts the last two parts of the FHIR path, joins them, and capitalizes the first letter
func extractAndCapitalizeLastTwoParts(fhirPath string) string {
	parts := strings.Split(fhirPath, ".")
	if len(parts) >= 2 {
		// Join the last two parts of the path
		joined := parts[len(parts)-2] + "." + parts[len(parts)-1]
		// Capitalize the first letter of the joined string
		fhirPath = capitalizeFirstLetter(joined)
		return fhirPath
	}
	// If the path has fewer than 2 parts, return the original path
	return fhirPath
}

// Function to check if a type has a Code method (using the type directly, not an instance)
// This can be used to determine if a type is likely a code type
// TODO: check if the code type can be determined with getValueSetBindingPath instead
func typeHasCodeMethod(t reflect.Type) bool {
	// Check if the method exists on the type
	_, ok := t.MethodByName("Code")
	return ok
}

// Helper function to capitalize the first letter and make the rest lowercase
func capitalizeFirstLetter(s string) string {
	if len(s) == 0 {
		return s
	}
	// Convert first rune to uppercase and the rest to lowercase
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

// AlternativePathTwo takes structValue and fieldName as inputs,
// removes "fhir." from structValue, and combines them
// in the format "StructValue.fieldname" with proper casing.
func AlternativePath(structValue string, fieldName string) string {

	if strings.HasPrefix(structValue, "*fhir.") {
		structValue = strings.TrimPrefix(structValue, "*fhir.")
	}
	combined := structValue + "." + fieldName

	return combined
}
