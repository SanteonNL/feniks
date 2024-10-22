package main

import (
	"fmt"
	"strings"
)

type DECORCodeSystem struct {
	Id                  *oid                            `json:"id,omitempty"`
	EffectiveDate       *date                           `json:"effectiveDate,omitempty"`
	StatusCode          *string                         `json:"statusCode,omitempty"`
	Type                *string                         `json:"type,omitempty"`
	ExpirtationDate     *date                           `json:"expirationDate,omitempty"`
	OfficialReleaseDate *date                           `json:"officialReleaseDate,omitempty"`
	LastModifiedDate    *date                           `json:"lastModifiedDate,omitempty"`
	CanonicalUri        *string                         `json:"canonicalUri,omitempty"`
	VersionLabel        *string                         `json:"versionLabel,omitempty"`
	Ref                 *oid                            `json:"ref,omitempty"`
	Flexibility         *date                           `json:"flexibility,omitempty"`
	Name                string                          `json:"name"`
	DisplayName         string                          `json:"displayName"`
	Experimental        *bool                           `json:"experimental,omitempty"`
	CaseSensitive       *bool                           `json:"caseSensitive,omitempty"`
	Url                 *string                         `json:"url,omitempty"`
	Ident               *string                         `json:"ident,omitempty"`
	Desc                []*FreeFormMarkupWithLanguage   `json:"desc,omitempty"`
	PublishingAuthority []*AuthorityType                `json:"publishingAuthority,omitempty"`
	Purpose             []*FreeFormMarkupWithLanguage   `json:"purpose,omitempty"`
	Copyright           []*CopyrightText                `json:"copyright,omitempty"`
	Property            []*CodeSystemPropertyDefinition `json:"property,omitempty"`
	ConceptList         []*CodeSystemConceptList        `json:"conceptList,omitempty"`
}

func (c *ArtDecorApiClient) CodeSystem(id, effectiveDate string, queryParams any) (*DECORCodeSystem, error) {
	var endpoint string = "/codesystem"
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

	resp := new(DECORCodeSystem)
	if err := c.get(endpoint, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
