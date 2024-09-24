package main

import (
	"encoding/csv"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/SanteonNL/fenix/cmd/artdecor/body"
)

type DECORConceptMap struct {
	Uuid                *string                       `json:"uuid,omitempty"`
	Id                  *oid                          `json:"id,omitempty"`
	EffectiveDate       *date                         `json:"effectiveDate,omitempty"`
	StatusCode          *string                       `json:"statusCode,omitempty"`
	ExpirationDate      *date                         `json:"expirationDate,omitempty"`
	OfficialReleaseDate *date                         `json:"officialReleaseDate,omitempty"`
	LastModifiedDate    *date                         `json:"lastModifiedDate,omitempty"`
	CanonicalUri        *string                       `json:"canonicalUri,omitempty"`
	VersionLabel        *string                       `json:"versionLabel,omitempty"`
	Ref                 *oid                          `json:"ref,omitempty"`
	Flexibility         *date                         `json:"flexibility,omitempty"`
	DisplayName         string                        `json:"displayName"`
	Url                 *string                       `json:"url,omitempty"`
	Ident               *string                       `json:"ident,omitempty"`
	Desc                []*FreeFormMarkupWithLanguage `json:"desc,omitempty"`
	PublishingAuthority []*AuthorityType              `json:"publishingAuthority,omitempty"`
	EndorsingAuthority  []*AuthorityType              `json:"endorsingAuthority,omitempty"`
	Purpose             []*FreeFormMarkupWithLanguage `json:"purpose,omitempty"`
	Copyright           []*CopyrightText              `json:"copyright,omitempty"`
	Jurisdiction        []*ValueCodingType            `json:"jurisdiction,omitempty"`
	SourceScope         []*SourceOrTargetValueSet     `json:"sourceScope,omitempty"`
	TargetScope         []*SourceOrTargetValueSet     `json:"targetScope,omitempty"`
	Group               []*ConceptMapGroupDefinition  `json:"group,omitempty"`
}

func (cm *DECORConceptMap) FromSanteonCSV(source, name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	name = filepath.Base(name)
	name = strings.TrimPrefix(name, "conceptmap_")
	name = strings.TrimSuffix(name, filepath.Ext(name))

	r := csv.NewReader(f)
	r.Comma = ';'

	records, err := r.ReadAll()
	if err != nil {
		return err
	}

	var ident = os.Getenv("ART_PROJECT")

	cm.DisplayName = os.Getenv("ORGANIZATION") + "_" + source + "_" + name
	cm.Ident = &ident

	cm.PublishingAuthority = append(
		cm.PublishingAuthority,
		&AuthorityType{
			Name: "",
		},
	)

	var groupKey string
	var groups = make(map[string]*ConceptMapGroupDefinition)
	for _, record := range records[1:] {
		log.Default().Println("RECORD ", record)

		var sourceCode = record[2]
		var sourceCodeSystem = record[1]
		var sourceDisplay = record[3]

		var targetCode = record[5]
		var targetCodeSystem = record[4]
		var targetDisplay = record[6]

		groupKey = sourceCodeSystem + "|" + targetCodeSystem

		var group = groups[groupKey]
		if group == nil {
			group = new(ConceptMapGroupDefinition)
		}

		var elem = ConceptMapElement{}
		elem.Code = &sourceCode
		if sourceDisplay != "" {
			elem.DisplayName = &sourceDisplay
		}

		var target = new(ConceptMapTarget)
		target.Code = &targetCode
		if targetDisplay != "" {
			target.DisplayName = &targetDisplay
		}
		var relationship = "equal"
		target.Relationship = &relationship

		elem.Target = append(elem.Target, target)

		group.Element = append(group.Element, elem)

		groups[groupKey] = group
	}
	// cm.Group = append(cm.Group, group)
	if len(groups) > 1 {
		return errors.New("more than one (source or target) code system")
	}

	codeSystems := strings.Split(groupKey, "|")
	sourceCodeSystem := codeSystems[0]
	targetCodeSystem := codeSystems[1]

	// if sourceCodeSystem == "" {
	// 	sourceCodeSystem = "http://UNKNOWN.INFO"
	// }
	// if targetCodeSystem == "" {
	// 	targetCodeSystem = "http://UNKNOWN.INFO"
	// }

	cm.SourceScope = append(
		cm.SourceScope,
		&SourceOrTargetValueSet{
			CanonicalUri: &sourceCodeSystem,
		},
	)

	cm.TargetScope = append(
		cm.TargetScope,
		&SourceOrTargetValueSet{
			CanonicalUri: &targetCodeSystem,
		},
	)

	cm.Group = append(cm.Group, groups[groupKey])
	cm.Group[0].Source = append(
		cm.Group[0].Source,
		SourceTargetElement{
			CanoncialUri: &sourceCodeSystem,
		},
	)
	cm.Group[0].Target = append(
		cm.Group[0].Target,
		SourceTargetElement{
			CanoncialUri: &targetCodeSystem,
		},
	)

	return nil
}

func (c *ArtDecorApiClient) CreateConceptMap(conceptMap DECORConceptMap, queryParams any) error {
	var endpoint string = "/conceptmap"
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
	if err := c.post(endpoint, conceptMap, resp); err != nil {
		return err
	}
	log.Default().Printf("Response %+v", resp)
	return nil
}

func (c *ArtDecorApiClient) ReadConceptMap(queryParams any) (*DECORConceptMap, error) {
	var endpoint string = "/conceptmap"
	var validParams = []string{
		"codeSystemEffectiveDate", "codeSystemEffectiveDate:source", "codeSystemEffectiveDate:target",
		"codeSystemId", "codeSystemId:source", "codeSystemId:target", "governanceGroupId", "includebbr",
		"max", "prefix", "resolve", "search", "sort", "sortorder", "status",
		"valueSetEffectiveDate", "valueSetEffectiveDate:source", "valueSetEffectiveDate:target",
		"valueSetId", "valueSetId:source", "valueSetId:target",
	}

	query, err := parseQueryParams(queryParams, validParams)
	if err != nil {
		return nil, err
	}
	if query != "" {
		endpoint += "?" + strings.TrimSuffix(query, "&")
	}
	log.Default().Println(endpoint)

	resp := new(any)
	if err := c.get(endpoint, resp); err != nil {
		return nil, err
	}
	log.Default().Printf("Response %+v", *resp)
	// respMap := (*resp).(map[string]interface{})
	// log.Default().Printf("ResponseMAP %+v", respMap["conceptMap"])

	var cm = new(DECORConceptMap)
	return cm, nil
}
