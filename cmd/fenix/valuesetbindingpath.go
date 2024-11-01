package main

import (
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

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

// RegeneratePath filters valuesetbindingPath by keeping only elements
// found in fieldTypeString, then capitalizes the first character of the final result.
func RegeneratePath(valuesetbindingPath string, fieldTypeString string) string {
	// Convert fieldTypeString to lowercase for case-insensitive comparison
	lowerFieldTypeString := strings.ToLower(fieldTypeString)

	// Split the valuesetbindingPath into elements
	pathElements := strings.Split(valuesetbindingPath, ".")
	var resultElements []string

	// Iterate over each element, include it in the result if it's in fieldTypeString
	for _, element := range pathElements {
		lowerElement := strings.ToLower(element) // Make element lowercase for comparison
		if strings.Contains(lowerFieldTypeString, lowerElement) {
			resultElements = append(resultElements, element)
		}
	}

	// Join the filtered elements with dots to form the new path
	result := strings.Join(resultElements, ".")

	// Capitalize only the first character of the final result string
	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}

	return result
}

// TransformFieldType takes a fieldType string like "fhir.AdministrativeGender"
// and converts it to "Administrative.gender" by removing "fhir.", splitting
// on capital letters, and formatting as required.
func AlternativePath(fieldType string) string {
	// Step 1: Remove "fhir." prefix if it exists
	if strings.HasPrefix(fieldType, "fhir.") {
		fieldType = strings.TrimPrefix(fieldType, "fhir.")
	}

	// Step 2: Split based on capitalized parts using regular expression
	// This will match each word part that starts with an uppercase letter
	re := regexp.MustCompile(`[A-Z][a-z]*`)
	parts := re.FindAllString(fieldType, -1)

	// Step 3: Format each part: first part keeps its case, others lowercase
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToLower(parts[i])
	}

	// Step 4: Join parts with dots and return the result
	return strings.Join(parts, ".")
}

// AlternativePathTwo takes structValue and fieldName as inputs,
// removes "fhir." from structValue, and combines them
// in the format "StructValue.fieldname" with proper casing.
func AlternativePathTwo(structValue string, fieldName string) string {
	// Step 1: Remove "fhir." prefix if it exists
	if strings.HasPrefix(structValue, "*fhir.") {
		structValue = strings.TrimPrefix(structValue, "*fhir.")
	}

	// Step 2: Format structValue to lowercase and split into parts
	structParts := strings.Split(structValue, ".")
	for i := 0; i < len(structParts); i++ {
		structParts[i] = strings.ToLower(structParts[i])
	}

	// Step 3: Lowercase the fieldName
	fieldName = strings.ToLower(fieldName)

	// Step 4: Combine structParts and fieldName with a dot
	combined := strings.Join(structParts, ".") + "." + fieldName

	// Step 5: Capitalize the first character of the result
	if len(combined) > 0 {
		combined = strings.ToUpper(combined[:1]) + combined[1:]
	}

	return combined
}
