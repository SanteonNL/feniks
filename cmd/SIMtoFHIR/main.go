package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type FHIRMapper struct {
	mappings map[string]map[string]string
}

func NewFHIRMapper() *FHIRMapper {
	return &FHIRMapper{
		mappings: make(map[string]map[string]string),
	}
}

func (m *FHIRMapper) AddMapping(csvName, csvField, fhirPath, resourceType string) {
	key := fmt.Sprintf("%s:%s", csvName, csvField)
	if _, ok := m.mappings[resourceType]; !ok {
		m.mappings[resourceType] = make(map[string]string)
	}
	m.mappings[resourceType][key] = fhirPath
}

func (m *FHIRMapper) MapCSVToFHIR(csvName string, csvData map[string]interface{}, resourceType string) map[string]interface{} {
	fhirResource := map[string]interface{}{
		"resourceType": resourceType,
	}

	if resourceMapping, ok := m.mappings[resourceType]; ok {
		for csvField, value := range csvData {
			key := fmt.Sprintf("%s:%s", csvName, csvField)
			if fhirPath, ok := resourceMapping[key]; ok {
				setNestedValue(fhirResource, strings.Split(fhirPath, "."), value)
			}
		}
	}

	return fhirResource
}

func (m *FHIRMapper) AddReference(fhirResource map[string]interface{}, referencePath, referencedResourceType, referencedResourceID string) {
	reference := map[string]interface{}{
		"reference": fmt.Sprintf("%s/%s", referencedResourceType, referencedResourceID),
	}
	setNestedValue(fhirResource, strings.Split(referencePath, "."), reference)
}

func setNestedValue(nestedMap map[string]interface{}, keys []string, value interface{}) {
	for i, key := range keys {
		if i == len(keys)-1 {
			nestedMap[key] = value
		} else {
			if _, ok := nestedMap[key]; !ok {
				nestedMap[key] = make(map[string]interface{})
			}
			nestedMap = nestedMap[key].(map[string]interface{})
		}
	}
}

func main() {
	mapper := NewFHIRMapper()

	// Add mappings for Encounter resource
	mapper.AddMapping("contact", "date", "period.start", "Encounter")
	mapper.AddMapping("contact", "type", "type.0.coding.0.code", "Encounter")
	mapper.AddMapping("contact", "id", "id", "Encounter")

	// Add mappings for Location resource
	mapper.AddMapping("contactdetail", "address", "address.text", "Location")
	mapper.AddMapping("contactdetail", "name", "name", "Location")
	mapper.AddMapping("contactdetail", "contact_id", "id", "Location")

	// Sample CSV data with multiple contacts and contact details
	contactsData := []map[string]interface{}{
		{
			"id":   "enc1",
			"date": "2023-07-23",
			"type": "outpatient",
		},
		{
			"id":   "enc2",
			"date": "2023-07-24",
			"type": "inpatient",
		},
	}

	contactDetailsData := []map[string]interface{}{
		{
			"contact_id": "enc1",
			"address":    "123 Main St, Anytown, USA",
			"name":       "City Hospital",
		},
		{
			"contact_id": "enc1",
			"address":    "456 Oak Rd, Othertown, USA",
			"name":       "County Clinic",
		},
		{
			"contact_id": "enc2",
			"address":    "789 Pine Ave, Somewhere, USA",
			"name":       "State Medical Center",
		},
	}

	// Map CSV data to FHIR resources
	encounters := make(map[string]map[string]interface{})
	locations := make([]map[string]interface{}, len(contactDetailsData))

	// Create Encounter resources
	for _, contactData := range contactsData {
		encounter := mapper.MapCSVToFHIR("contact", contactData, "Encounter")
		encounters[contactData["id"].(string)] = encounter
	}

	// Create Location resources and link them to Encounters
	for i, contactDetailData := range contactDetailsData {
		location := mapper.MapCSVToFHIR("contactdetail", contactDetailData, "Location")
		locations[i] = location

		// Find the corresponding Encounter and add a reference to this Location
		if encounter, ok := encounters[contactDetailData["contact_id"].(string)]; ok {
			locationReference := map[string]interface{}{
				"location": map[string]interface{}{
					"reference": fmt.Sprintf("Location/%s", location["id"]),
				},
			}

			// Initialize the location array if it doesn't exist
			if _, exists := encounter["location"]; !exists {
				encounter["location"] = make([]interface{}, 0)
			}

			// Append the new location reference
			encounter["location"] = append(encounter["location"].([]interface{}), locationReference)
		}
	}

	// Print the resulting FHIR resources in JSON format
	fmt.Println("Encounters:")
	for id, encounter := range encounters {
		jsonData, err := json.MarshalIndent(encounter, "", "  ")
		if err != nil {
			fmt.Printf("Error marshalling Encounter %s: %v\n", id, err)
			continue
		}
		fmt.Printf("Encounter %s:\n%s\n\n", id, string(jsonData))
	}

	fmt.Println("Locations:")
	for i, location := range locations {
		jsonData, err := json.MarshalIndent(location, "", "  ")
		if err != nil {
			fmt.Printf("Error marshalling Location %d: %v\n", i+1, err)
			continue
		}
		fmt.Printf("Location %d:\n%s\n\n", i+1, string(jsonData))
	}
}
