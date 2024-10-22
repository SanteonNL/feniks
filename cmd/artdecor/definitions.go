package main

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// https://decor.nictiz.nl/exist/apps/api//modules/library/api-schema.json

type pattern string

func (p pattern) String() string {
	return string(p)
}

const (
	oidPattern  pattern = "^[0-2](\\.(0|[1-9][0-9]*))*$"
	datePattern pattern = "^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}$"
)

var (
	oidRegex  = regexp.MustCompile(oidPattern.String())
	dateRegex = regexp.MustCompile(datePattern.String())
)

type oid string

func (o oid) String() string {
	return string(o)
}

func (o *oid) UnmarshalText(text []byte) error {
	if !oidRegex.Match(text) {
		return fmt.Errorf("invalid OID %q", text)
	}
	*o = oid(text)
	return nil
}

func (o oid) MarshalText() ([]byte, error) {
	if !oidRegex.MatchString(o.String()) {
		return nil, fmt.Errorf("invalid OID %q", o)
	}
	return []byte(o), nil
}

func (o *oid) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if !oidRegex.MatchString(v) {
		return fmt.Errorf("invalid OID %q", v)
	}
	*o = oid(v)
	return nil
}

func (o oid) MarshalJSON() ([]byte, error) {
	if !oidRegex.MatchString(o.String()) {
		return nil, fmt.Errorf("invalid OID %q", o)
	}
	return json.Marshal(o.String())
}

type date string

func (d date) String() string {
	return string(d)
}

func (d *date) UnmarshalText(text []byte) error {
	if !dateRegex.Match(text) {
		return fmt.Errorf("invalid date %q", text)
	}
	*d = date(text)
	return nil
}

func (d date) MarshalText() ([]byte, error) {
	if !dateRegex.MatchString(d.String()) {
		return nil, fmt.Errorf("invalid date %q", d)
	}
	return []byte(d), nil
}

func (d *date) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if !dateRegex.MatchString(v) {
		return fmt.Errorf("invalid date %q", v)
	}
	*d = date(v)
	return nil
}

func (d date) MarshalJSON() ([]byte, error) {
	if !dateRegex.MatchString(d.String()) {
		return nil, fmt.Errorf("invalid date %q", d)
	}
	return json.Marshal(d.String())
}

type AddrLine struct {
	Type *string `json:"type,omitempty"`
	Text *string `json:"#text,omitempty"`
}

type AuthorityType struct {
	Id       *oid        `json:"id,omitempty"`
	Name     string      `json:"name"`
	AddrLine []*AddrLine `json:"addrLine,omitempty"`
	Text     *string     `json:"#text,omitempty"`
}

type CodedConcept struct {
	Code                string                        `json:"code"`
	StatusCode          *string                       `json:"statusCode,omitempty"`
	EffectiveDate       *date                         `json:"effectiveDate,omitempty"`
	ExpirtationDate     *date                         `json:"expirationDate,omitempty"`
	OfficialReleaseDate *date                         `json:"officialReleaseDate,omitempty"`
	LastModifiedDate    *date                         `json:"lastModifiedDate,omitempty"`
	Level               string                        `json:"level"`
	Type                string                        `json:"type"`
	Designation         []*Designation                `json:"designation,omitempty"`
	Desc                []*FreeFormMarkupWithLanguage `json:"desc,omitempty"`
	Property            []*any                        `json:"property,omitempty"`
	Parent              []*ParentOrChild              `json:"parent,omitempty"`
	Child               []*ParentOrChild              `json:"child,omitempty"`
}

type CodeSystemPropertyDefinition struct {
	Code        *string `json:"code,omitempty"`
	Uri         *string `json:"uri,omitempty"`
	Description *string `json:"description,omitempty"`
	Type        *string `json:"type,omitempty"`
}

type CodeSystemConceptList struct {
	CodedConcept []CodedConcept `json:"codedConcept"`
}

type ConceptMapElement struct {
	Code        *string             `json:"code,omitempty"`
	DisplayName *string             `json:"displayName,omitempty"`
	Target      []*ConceptMapTarget `json:"target,omitempty"`
}

type ConceptMapGroupDefinition struct {
	Source  []SourceTargetElement `json:"source"`
	Target  []SourceTargetElement `json:"target"`
	Element []ConceptMapElement   `json:"element"`
}

type ConceptMapTarget struct {
	Code         *string                       `json:"code,omitempty"`
	DisplayName  *string                       `json:"displayName,omitempty"`
	Relationship *string                       `json:"relationship,omitempty"`
	Comment      []*FreeFormMarkupWithLanguage `json:"comment,omitempty"`
}

type CopyrightText struct {
	Language       string  `json:"language"`
	LastTranslated *date   `json:"lastTranslated,omitempty"`
	MimeType       *string `json:"mimeType,omitempty"`
	Text           *string `json:"#text,omitempty"`
}

type Designation struct {
	Language       string  `json:"language"`
	LastTranslated *date   `json:"lastTranslated,omitempty"`
	MimeType       *string `json:"mimeType,omitempty"`
	Type           *string `json:"type,omitempty"`
	DisplayName    string  `json:"displayName"`
	Text           string  `json:"#text"`
}

type FreeFormMarkupWithLanguage struct {
	Language       string  `json:"language"`
	LastTranslated *date   `json:"lastTranslated,omitempty"`
	MimeType       *string `json:"mimeType,omitempty"`
	Text           *string `json:"#text,omitempty"`
}

type ParentOrChild struct {
	Code *string `json:"code,omitempty"`
}

type SourceCodeSystem struct {
	Id             oid     `json:"id"`
	IdentifierName string  `json:"identifierName"`
	CanonicalUri   *string `json:"canonicalUri,omitempty"`
	Context        *string `json:"context,omitempty"`
}

type SourceOrTargetValueSet struct {
	Ref          *oid    `json:"ref,omitempty"`
	Flexibility  *date   `json:"flexibility,omitempty"`
	DisplayName  *string `json:"displayName,omitempty"`
	CanonicalUri *string `json:"canonicalUri,omitempty"`
}

type SourceTargetElement struct {
	CodeSystem        *oid    `json:"codeSystem,omitempty"`
	CodeSystemVersion *string `json:"codeSystemVersion,omitempty"`
	CanoncialUri      *string `json:"canonicalUri,omitempty"`
}

type ValueCodingType struct {
	Code         string  `json:"code,omitempty"`
	CodeSystem   *oid    `json:"codeSystem,omitempty"`
	DisplayName  *string `json:"displayName,omitempty"`
	CanoncialUri *string `json:"canonicalUri,omitempty"`
}

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
