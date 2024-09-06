// Idea:
// Collect searchparameters from url
// Haal elementen relevant voor conceptmapping eruit en map naar searchparameter struct (R4)
// Valueset is belangrijk omdat die momenteel nog niet uit structure definition kan wroden gehaald;
// want soms heeft Pacmed(?) een eigen valueset i.p.v. de R4 valueset
// Ook belangrijp om fhir path te hebben om te weten waar een conceptmapping op van toepassing is
// Plaats van conceptmapping is voor applyFilter, maar kan in geval van een code ook direct bij vullen al
// Ergens in de conceptmap komt ook te staan voor welke ziekenhuis de conceptmap gledt, bijv.
// version: organization=mst&versionNumber=1.0
// Maar hoe relateert dat aan deels generieke, deels ziekenhuis specifieke conceptmaps?

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/fenix/models/fhir" // for use of the conceptMap.go file
)

// Mapping van FHIRPath naar ValueSet
// TODO: fill from resource structure definition (R4) or from searchparameter (url)
// (Maybe default from R4 and override with searchparameter)
// Of course this is only releavant for fhir pahts where a valueset is relevant, so for codes, coding.codes, etc.
// Look up what is the entire list of fhir paths that can have a valueset
var fhirPathToValueset = map[string]string{
	"Patient.gender": "http://hl7.org/fhir/ValueSet/administrative-gender",
}

// Mapping van Valueset naar ConceptMap (gebaseerd op targetUri)
// TODO: fill from reading all ConceptMap resources
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
var valueSetToConceptMap = map[string]string{
	"http://hl7.org/fhir/ValueSet/administrative-gender": "http://example.org/fhir/ConceptMap/gender-to-another-system",
}

// Functie om ConceptMap op te halen voor een gegeven FHIRPath
func getConceptMapForFhirPath(fhirPath string) (string, error) {
	valueSet, exists := fhirPathToValueset[fhirPath]
	if !exists {
		return "", fmt.Errorf("no valueset found voor FHIRPath: %s", fhirPath)
	}

	conceptMap, exists := valueSetToConceptMap[valueSet]
	if !exists {
		return "", fmt.Errorf("no ConceptMap found voor ValueSet: %s", valueSet)
	}

	return conceptMap, nil
}

func ExampleDetermineConceptMapforPath() {
	// Voorbeeldgebruik
	fhirPath := "Patient.gender"
	conceptMap, err := getConceptMapForFhirPath(fhirPath)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("De ConceptMap voor", fhirPath, "is:", conceptMap)
	}
}

// FetchConceptMap fetches a ConceptMap from a given URL and parses it into a Go struct
// TODO: see if this can be done with the fhir package and code in datasource.go
func FetchConceptMap(url string) (fhir.ConceptMap, error) {
	resp, err := http.Get(url)
	if err != nil {
		return fhir.ConceptMap{}, err
	}
	defer resp.Body.Close()

	var conceptMap fhir.ConceptMap
	err = json.NewDecoder(resp.Body).Decode(&conceptMap)
	if err != nil {
		return fhir.ConceptMap{}, err
	}

	return conceptMap, nil
}

func TranslateCode(conceptMap fhir.ConceptMap, sourceCode *string) (*string, *string, error) {
	for _, group := range conceptMap.Group {
		if group.Source == conceptMap.SourceUri {
			for _, element := range group.Element {
				if element.Code == sourceCode {
					for _, target := range element.Target {
						return target.Code, target.Display, nil
					}
				}
			}
		}
	}
	return nil, nil, fmt.Errorf("code not found in ConceptMap")
}

// TODO: implement within datasource.go before ApplyFilter
// Do we also translate back to the source data?
func ExampleApplyConceptmap() {
	// Voorbeeld ConceptMap URL
	conceptMapURL := "http://example.org/fhir/ConceptMap/gender-to-another-system"

	// Fetch en parse de ConceptMap
	conceptMap, err := FetchConceptMap(conceptMapURL)
	if err != nil {
		fmt.Println("Error fetching ConceptMap:", err)
		return
	}

	// Vertaal een code
	sourceCode := "male"
	targetCode, targetDisplay, err := TranslateCode(conceptMap, &sourceCode)
	if err != nil {
		fmt.Println("Error translating code:", err)
		return
	}

	fmt.Printf("Source Code: %s\nTarget Code: %s\nTarget Display: %s\n", sourceCode, *targetCode, *targetDisplay)
}
