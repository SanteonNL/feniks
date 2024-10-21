package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/SanteonNL/fenix/models/fhir" // for use of the conceptMap.go file
	"github.com/rs/zerolog"
)

// Per resource:
// - Mapping van FHIRPath naar Valueset
// - Mapping van Valueset naar ConceptMap

// Mapping van FHIRPath naar ValueSet
// TODO: Add possibility to overrirde Valueset from resource structure definition (R4) with Valueset from searchparameter (url)

var FhirPathToValueset = make(map[string]string)

// Mapping van Valueset naar ConceptMap (gebaseerd op targetUri)
// TODO: Check if targetCanonical should also work or make an agreement taht we always use the more generic targetUri
// TODO: Add the possibitlity to read all ConceptMaps in the case a code is requested without a valueset?
// (Maybe read the entire ConceptMap resource and filter on targetUri, depending on how memory intensive that is)
// The ConceptMaps have to be read in anyways to get the mappings, but the complete conceptmaps that are needed
// might be limited to the ones that are relevant for the searchparameters.
// So maybe best to do it in 2 stesps: first make a map with name and targetUri, then read the complete ConceptMaps
// that are relevant for the searchparameters.
// Per Conceptmap there is only one targetUri (the valueset uri), so that is the key to use.
// How to handle the fact that multiple conceptmaps can be used for the same valueset?
// Solution: make a map with valueset uri as key and conceptmap as value
// Then there is also some other data needed to know which conceptmap to use, like the organization and version
// Maybe that has to be collected already in the first step, so that the conceptmaps can be filtered on that
var ValueSetToConceptMap = make(map[string]*fhir.ConceptMap)

// Functie om ConceptMap op te halen voor een gegeven FHIRPath
// This is the fhirPath that in structureDefinition is used to define the valueset, nl. element.binding.valueSet
func getConceptMapForFhirPath(fhirPath string, log zerolog.Logger) (fhir.ConceptMap, error) {
	valueSet, exists := FhirPathToValueset[fhirPath]
	if !exists {
		return fhir.ConceptMap{}, fmt.Errorf("no valueset found voor FHIRPath: %s", fhirPath)
	}
	log.Debug().Msgf("valueSet for fhirPath %s: %s", fhirPath, valueSet)

	conceptMap, exists := ValueSetToConceptMap[valueSet]
	if !exists {
		return fhir.ConceptMap{}, fmt.Errorf("no ConceptMap found voor ValueSet: %s", valueSet)
	}
	log.Debug().Msgf("ConceptMap for valueset %s: %s", valueSet, *conceptMap.Id)

	return *conceptMap, nil
}

func TranslateCode(conceptMap fhir.ConceptMap, sourceCode *string, log zerolog.Logger) (*string, *string, error) {
	log.Debug().Msgf("sourceCode: %v", *sourceCode)
	for _, group := range conceptMap.Group {
		log.Debug().Msgf("Group: %v", group)
		log.Debug().Msgf("Group.Source: %v", *group.Source)
		log.Debug().Msgf("Source: %v", *conceptMap.SourceUri)
		//if group.Source == conceptMap.SourceUri {
		for _, element := range group.Element {
			log.Debug().Msgf("Element: %v", element)
			log.Debug().Msgf("Element.Code: %v", *element.Code)
			log.Debug().Msgf("Element.Code.targetCode: %v", *element.Target[0].Code)
			if *element.Code == *sourceCode {
				for _, target := range element.Target {
					log.Debug().Msgf("Returning targetCode: %s, targetDisplay: %s", *target.Code, *target.Display)
					return target.Code, target.Display, nil
				}
			}
		}
		//}
	}
	return nil, nil, fmt.Errorf("code not found in ConceptMap")
}

// PrintElementsWithCodeType prints elements from the StructureDefinition
// where the type is either 'code' or 'CodeableConcept', including any value set bindings.
func PrintElementsWithCodeType(structureDefinition *fhir.StructureDefinition) {
	// Iterate through the elements in the Snapshot (you can also use Differential if needed)
	for _, element := range structureDefinition.Snapshot.Element {
		for _, t := range element.Type { // Choice based on https://www.hl7.org/fhir/search.html#token, CodeableReference is excluded because it is R5
			if t.Code == "code" || t.Code == "Coding" || t.Code == "CodeableConcept" || t.Code == "Quantity" {
				// Print basic details
				fmt.Printf("Path: %s, Type: %s, Definition: %s\n", element.Path, t.Code, *element.Definition)

				// Print value set binding information if available
				if element.Binding != nil {
					fmt.Printf("  Binding Strength: %s\n", element.Binding.Strength)
					fmt.Printf("  Value Set URL: %s\n", *element.Binding.ValueSet)
					FhirPathToValueset[element.Path] = *element.Binding.ValueSet
				} else {
					fmt.Println("  No binding information available.")
				}
				break
			}
		}
	}
}

// LoadStructureDefinitions loads all StructureDefinitions into a global map.
func LoadConceptMaps(log zerolog.Logger) error {
	files, err := os.ReadDir("config/conceptmaps")
	if err != nil {
		return fmt.Errorf("failed to read conceptmaps directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			filePath := filepath.Join("config/conceptmaps", file.Name())
			conceptMap, err := ReadFHIRResource(filePath, fhir.UnmarshalConceptMap)
			if err != nil {
				return fmt.Errorf("failed to read Conceptmap from file: %v", err)
			}
			log.Debug().Str("conceptMap", file.Name()).Msg("Loaded conceptMap")
			printConceptMapDetails(conceptMap)
		}
	}

	return nil
}

func printConceptMapDetails(conceptMap *fhir.ConceptMap) {
	// Print ConceptMap details
	/*fmt.Printf("ConceptMap ID: %s\n", *conceptMap.Id)
	fmt.Printf("ConceptMap SourceUri (source valuesetname): %s\n", *conceptMap.SourceUri)
	fmt.Printf("ConceptMap TargetUri (target valuesetname): %s\n", *conceptMap.TargetUri)
	*/
	ValueSetToConceptMap[*conceptMap.TargetUri] = conceptMap

	/*
		// Iterate through groups and display elements
		for _, group := range conceptMap.Group {
			fmt.Printf("Source (system_source): %s, Target (system_target): %s\n", *group.Source, *group.Target)
			for _, element := range group.Element {
				// Ensure there is at least one target to avoid panic
				if len(element.Target) > 0 {
					fmt.Printf("Code: %s, Target: %v\n", *element.Code, *element.Target[0].Code)
				}
			}
		}*/
}

func applyConceptMapping(structPath string, structType reflect.Type, structPointer interface{}, log zerolog.Logger) error {
	// Get the FHIR path for ValueSet binding
	fhirPath := getValueSetBindingPath(structPath, structType.Name())
	log.Debug().Msgf("FHIR Path to determine ValueSet: %s", fhirPath)

	_, exists := FhirPathToValueset[fhirPath]
	if exists {
		log.Debug().Msgf("FHIR Path %s is in FhirPathToValueset", fhirPath)

		// Collect the ConceptMap for the FHIR path
		conceptMap, err := getConceptMapForFhirPath(fhirPath, log)
		if err != nil {
			log.Debug().Msgf("Failed to get ConceptMap for FHIR Path: %v", err)
			return err
		}

		log.Debug().Msgf("ConceptMap for FHIR Path %s: %v", fhirPath, *conceptMap.Id)

		// Check for system, code, or display fields in the struct
		structValue := reflect.ValueOf(structPointer).Elem()
		for i := 0; i < structType.NumField(); i++ {
			fieldName := structType.Field(i).Name
			fieldNameLower := strings.ToLower(fieldName)

			if fieldNameLower == "system" || fieldNameLower == "code" || fieldNameLower == "display" {
				log.Debug().Msgf("This field might need concept mapping: %s", fieldNameLower)
			}
		}

		// Set the code field if it exists in the struct
		codeField := structValue.FieldByName("Code")
		codeFieldValue := getStringValue(codeField.Elem())
		log.Debug().Msgf("Current code field value: %s", codeFieldValue)

		// Perform concept mapping
		targetCode, targetDisplay, err := TranslateCode(conceptMap, &codeFieldValue, log)
		if err != nil {
			return fmt.Errorf("Failed to map concept code: %v", err)
		}
		log.Debug().Msgf("Mapped concept code: %v", *targetCode)

		// Set the mapped code back to the struct's Code field
		if codeField.IsValid() && codeField.CanSet() {
			if err := SetField(structPath, structPointer, "Code", *targetCode, log); err != nil {
				return err
			}
		}

		log.Debug().Msgf("Target display could be used for display: %s", *targetDisplay)
	} else {
		log.Debug().Msgf("FHIR Path %s is not in FhirPathToValueset", fhirPath)
	}

	return nil
}
