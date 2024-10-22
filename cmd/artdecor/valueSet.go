package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/SanteonNL/fenix/cmd/artdecor/body"
)

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
			Id:             *codeSystem,
			IdentifierName: *codeSystemName,
		},
	)

	vs.PublishingAuthority = cs.PublishingAuthority

	var items []*ValueSetElement
	// TODO: handle multiple ConceptList
	for _, concept := range cs.ConceptList[0].CodedConcept {
		var code = concept.Code

		var item = new(ValueSetElement)
		item.Is = "concept"
		item.Code = &code
		item.CodeSystem = codeSystem
		item.CodeSystemName = codeSystemName
		// TODO: handle multiple Designation
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

func (c *ArtDecorApiClient) ValueSet(id, effectiveDate string, queryParams any) (*DECORValueSet, error) {
	var endpoint string = "/valueset"
	var validParams = []string{"language", "prefix", "release"}

	if !oidRegex.MatchString(id) {
		return nil, fmt.Errorf("invalid OID %q", id)
	}
	endpoint += "/" + id

	if effectiveDate != "" {
		if !dateRegex.MatchString(effectiveDate) {
			return nil, fmt.Errorf("invalid date %q", effectiveDate)
		}
		endpoint += "/" + effectiveDate
	}

	query, err := parseQueryParams(queryParams, validParams)
	if err != nil {
		return nil, err
	}
	if query != "" {
		endpoint += "?" + strings.TrimSuffix(query, "&")
	}

	resp := new(DECORValueSet)
	if err := c.get(endpoint, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *ArtDecorApiClient) CreateValueSet(valueSet DECORValueSet, queryParams any) error {
	var endpoint string = "/valueset"
	var validParams = []string{
		"baseId", "keepIds", "refOnly", "sourceEffectiveDate", "sourceId",
		"targetDate", "prefix",
	}

	query, err := parseQueryParams(queryParams, validParams)
	if err != nil {
		return err
	}
	if query != "" {
		endpoint += "?" + strings.TrimSuffix(query, "&")
	}
	log.Default().Println(endpoint)

	resp := new(body.ErrorResponse)
	if err := c.post(endpoint, valueSet, resp); err != nil {
		return err
	}
	log.Default().Printf("Response %+v", resp)
	return nil
}
