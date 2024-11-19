// File: types/conceptmap.go
package types

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Pattern and regex definitions
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

// Validation functions
func IsValidOID(id string) bool {
	return oidRegex.MatchString(id)
}

func ValidateOID(id string) error {
	if !IsValidOID(id) {
		return fmt.Errorf("invalid OID %q", id)
	}
	return nil
}

func IsValidDate(date string) bool {
	return dateRegex.MatchString(date)
}

func ValidateDate(date string) error {
	if !IsValidDate(date) {
		return fmt.Errorf("invalid date %q", date)
	}
	return nil
}

// Custom types with validation
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

// Constructor for OID
func NewOID(id string) (oid, error) {
	if err := ValidateOID(id); err != nil {
		return "", err
	}
	return oid(id), nil
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

// Constructor for Date
func NewDate(dateStr string) (date, error) {
	if err := ValidateDate(dateStr); err != nil {
		return "", err
	}
	return date(dateStr), nil
}

// DECORConceptMap and related types
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

// Supporting types
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

type ConceptMapGroupDefinition struct {
	Source  []SourceTargetElement `json:"source"`
	Target  []SourceTargetElement `json:"target"`
	Element []ConceptMapElement   `json:"element"`
}

type ConceptMapElement struct {
	Code        *string             `json:"code,omitempty"`
	DisplayName *string             `json:"displayName,omitempty"`
	Target      []*ConceptMapTarget `json:"target,omitempty"`
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
	Text           string  `json:"#text"`
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
	Text           string  `json:"#text"`
}

type ParentOrChild struct {
	Code *string `json:"code,omitempty"`
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
	Code         string  `json:"code"`
	CodeSystem   *oid    `json:"codeSystem,omitempty"`
	DisplayName  *string `json:"displayName,omitempty"`
	CanoncialUri *string `json:"canonicalUri,omitempty"`
}
