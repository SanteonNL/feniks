// File: types/valueset.go
package types

import (
	"os"
	"strings"
)

// DECORValueSet represents a value set in the ART-DECOR system
type DECORValueSet struct {
	Uuid                *string                       `json:"uuid,omitempty"`
	Id                  *oid                          `json:"id,omitempty"`
	EffectiveDate       *date                         `json:"effectiveDate,omitempty"`
	StatusCode          *string                       `json:"statusCode,omitempty"`
	Type                *string                       `json:"type,omitempty"`
	ExpirtationDate     *date                         `json:"expirationDate,omitempty"`
	OfficialReleaseDate *date                         `json:"officialReleaseDate,omitempty"`
	LastModifiedDate    *date                         `json:"lastModifiedDate,omitempty"`
	CanonicalUri        *string                       `json:"canonicalUri,omitempty"`
	VersionLabel        *string                       `json:"versionLabel,omitempty"`
	Ref                 *oid                          `json:"ref,omitempty"`
	Flexibility         *date                         `json:"flexibility,omitempty"`
	Name                string                        `json:"name"`
	DisplayName         string                        `json:"displayName"`
	Experimental        *bool                         `json:"experimental,omitempty"`
	Url                 *string                       `json:"url,omitempty"`
	Ident               *string                       `json:"ident,omitempty"`
	Desc                []*FreeFormMarkupWithLanguage `json:"desc,omitempty"`
	SourceCodeSystem    []*SourceCodeSystem           `json:"sourceCodeSystem,omitempty"`
	PublishingAuthority []*AuthorityType              `json:"publishingAuthority,omitempty"`
	Purpose             []*FreeFormMarkupWithLanguage `json:"purpose,omitempty"`
	Copyright           []*CopyrightText              `json:"copyright,omitempty"`
	Items               []*ValueSetElement            `json:"items,omitempty"`
}

// DECORCodeSystem represents a code system in the ART-DECOR system
type DECORCodeSystem struct {
	Id                  oid                     `json:"id"`
	Name                string                  `json:"name"`
	DisplayName         string                  `json:"displayName"`
	PublishingAuthority []*AuthorityType        `json:"publishingAuthority,omitempty"`
	ConceptList         []CodeSystemConceptList `json:"conceptList,omitempty"`
}

// CodeSystemConceptList represents a list of coded concepts
type CodeSystemConceptList struct {
	CodedConcept []CodedConcept `json:"codedConcept"`
}

// SourceCodeSystem represents a source code system reference
type SourceCodeSystem struct {
	Id             oid     `json:"id"`
	IdentifierName string  `json:"identifierName"`
	CanonicalUri   *string `json:"canonicalUri,omitempty"`
	Context        *string `json:"context,omitempty"`
}

// ValueSetElement represents an item in a value set
type ValueSetElement struct {
	Is                string                        `json:"is"`
	Ref               *oid                          `json:"ref,omitempty"`
	Flexibility       *date                         `json:"flexibility,omitempty"`
	Op                *string                       `json:"op,omitempty"`
	Code              *string                       `json:"code,omitempty"`
	CodeSystem        *oid                          `json:"codeSystem,omitempty"`
	CodeSystemName    *string                       `json:"codeSystemName,omitempty"`
	CodeSystemVersion *string                       `json:"codeSystemVersion,omitempty"`
	DisplayName       *string                       `json:"displayName,omitempty"`
	Ordinal           *string                       `json:"ordinal,omitempty"`
	Level             *string                       `json:"level,omitempty"`
	Type              *string                       `json:"type,omitempty"`
	Exception         *bool                         `json:"exception,omitempty"`
	Designation       []*Designation                `json:"designation,omitempty"`
	Desc              []*FreeFormMarkupWithLanguage `json:"desc,omitempty"`
}

// FromCodeSystem converts a code system to a value set
func (vs *DECORValueSet) FromCodeSystem(cs *DECORCodeSystem) {
	var ident = os.Getenv("ART_PROJECT")
	var codeSystem = cs.Id
	var codeSystemName = &cs.Name

	vs.Name = codeSystemNameToValueSetName(*codeSystemName)
	vs.DisplayName = codeSystemNameToValueSetName(cs.DisplayName)
	vs.Ident = &ident

	vs.SourceCodeSystem = append(
		vs.SourceCodeSystem,
		&SourceCodeSystem{
			Id:             codeSystem,
			IdentifierName: *codeSystemName,
		},
	)

	vs.PublishingAuthority = cs.PublishingAuthority

	var items []*ValueSetElement
	// Handle multiple ConceptList
	for _, concept := range cs.ConceptList[0].CodedConcept {
		var code = concept.Code

		var item = new(ValueSetElement)
		item.Is = "concept"
		item.Code = &code
		item.CodeSystem = &codeSystem
		item.CodeSystemName = codeSystemName
		// Handle multiple Designation
		if len(concept.Designation) > 0 {
			item.DisplayName = &concept.Designation[0].DisplayName
		}
		item.Level = &concept.Level
		item.Type = &concept.Type
		item.Designation = concept.Designation
		item.Desc = concept.Desc

		items = append(items, item)
	}
	vs.Items = items
}

// Helper function to convert a code system name to a value set name
func codeSystemNameToValueSetName(name string) string {
	return strings.ReplaceAll(name, "CodeSystem", "ValueSet")
}
