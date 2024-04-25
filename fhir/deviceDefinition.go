// Copyright 2019 - 2022 The Samply Community
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fhir

import "encoding/json"

// THIS FILE IS GENERATED BY https://github.com/samply/golang-fhir-models
// PLEASE DO NOT EDIT BY HAND

// DeviceDefinition is documented here http://hl7.org/fhir/StructureDefinition/DeviceDefinition
type DeviceDefinition struct {
	Id                      *string                               `bson:"id,omitempty" json:"id,omitempty"`
	Meta                    *Meta                                 `bson:"meta,omitempty" json:"meta,omitempty"`
	ImplicitRules           *string                               `bson:"implicitRules,omitempty" json:"implicitRules,omitempty"`
	Language                *string                               `bson:"language,omitempty" json:"language,omitempty"`
	Text                    *Narrative                            `bson:"text,omitempty" json:"text,omitempty"`
	Extension               []Extension                           `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension       []Extension                           `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	Identifier              []Identifier                          `bson:"identifier,omitempty" json:"identifier,omitempty"`
	UdiDeviceIdentifier     []DeviceDefinitionUdiDeviceIdentifier `bson:"udiDeviceIdentifier,omitempty" json:"udiDeviceIdentifier,omitempty"`
	ManufacturerString      *string                               `bson:"manufacturerString,omitempty" json:"manufacturerString,omitempty"`
	ManufacturerReference   *Reference                            `bson:"manufacturerReference,omitempty" json:"manufacturerReference,omitempty"`
	DeviceName              []DeviceDefinitionDeviceName          `bson:"deviceName,omitempty" json:"deviceName,omitempty"`
	ModelNumber             *string                               `bson:"modelNumber,omitempty" json:"modelNumber,omitempty"`
	Type                    *CodeableConcept                      `bson:"type,omitempty" json:"type,omitempty"`
	Specialization          []DeviceDefinitionSpecialization      `bson:"specialization,omitempty" json:"specialization,omitempty"`
	Version                 []string                              `bson:"version,omitempty" json:"version,omitempty"`
	Safety                  []CodeableConcept                     `bson:"safety,omitempty" json:"safety,omitempty"`
	ShelfLifeStorage        []ProductShelfLife                    `bson:"shelfLifeStorage,omitempty" json:"shelfLifeStorage,omitempty"`
	PhysicalCharacteristics *ProdCharacteristic                   `bson:"physicalCharacteristics,omitempty" json:"physicalCharacteristics,omitempty"`
	LanguageCode            []CodeableConcept                     `bson:"languageCode,omitempty" json:"languageCode,omitempty"`
	Capability              []DeviceDefinitionCapability          `bson:"capability,omitempty" json:"capability,omitempty"`
	Property                []DeviceDefinitionProperty            `bson:"property,omitempty" json:"property,omitempty"`
	Owner                   *Reference                            `bson:"owner,omitempty" json:"owner,omitempty"`
	Contact                 []ContactPoint                        `bson:"contact,omitempty" json:"contact,omitempty"`
	Url                     *string                               `bson:"url,omitempty" json:"url,omitempty"`
	OnlineInformation       *string                               `bson:"onlineInformation,omitempty" json:"onlineInformation,omitempty"`
	Note                    []Annotation                          `bson:"note,omitempty" json:"note,omitempty"`
	Quantity                *Quantity                             `bson:"quantity,omitempty" json:"quantity,omitempty"`
	ParentDevice            *Reference                            `bson:"parentDevice,omitempty" json:"parentDevice,omitempty"`
	Material                []DeviceDefinitionMaterial            `bson:"material,omitempty" json:"material,omitempty"`
}
type DeviceDefinitionUdiDeviceIdentifier struct {
	Id                *string     `bson:"id,omitempty" json:"id,omitempty"`
	Extension         []Extension `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension []Extension `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	DeviceIdentifier  string      `bson:"deviceIdentifier" json:"deviceIdentifier"`
	Issuer            string      `bson:"issuer" json:"issuer"`
	Jurisdiction      string      `bson:"jurisdiction" json:"jurisdiction"`
}
type DeviceDefinitionDeviceName struct {
	Id                *string        `bson:"id,omitempty" json:"id,omitempty"`
	Extension         []Extension    `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension []Extension    `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	Name              string         `bson:"name" json:"name"`
	Type              DeviceNameType `bson:"type" json:"type"`
}
type DeviceDefinitionSpecialization struct {
	Id                *string     `bson:"id,omitempty" json:"id,omitempty"`
	Extension         []Extension `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension []Extension `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	SystemType        string      `bson:"systemType" json:"systemType"`
	Version           *string     `bson:"version,omitempty" json:"version,omitempty"`
}
type DeviceDefinitionCapability struct {
	Id                *string           `bson:"id,omitempty" json:"id,omitempty"`
	Extension         []Extension       `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension []Extension       `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	Type              CodeableConcept   `bson:"type" json:"type"`
	Description       []CodeableConcept `bson:"description,omitempty" json:"description,omitempty"`
}
type DeviceDefinitionProperty struct {
	Id                *string           `bson:"id,omitempty" json:"id,omitempty"`
	Extension         []Extension       `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension []Extension       `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	Type              CodeableConcept   `bson:"type" json:"type"`
	ValueQuantity     []Quantity        `bson:"valueQuantity,omitempty" json:"valueQuantity,omitempty"`
	ValueCode         []CodeableConcept `bson:"valueCode,omitempty" json:"valueCode,omitempty"`
}
type DeviceDefinitionMaterial struct {
	Id                  *string         `bson:"id,omitempty" json:"id,omitempty"`
	Extension           []Extension     `bson:"extension,omitempty" json:"extension,omitempty"`
	ModifierExtension   []Extension     `bson:"modifierExtension,omitempty" json:"modifierExtension,omitempty"`
	Substance           CodeableConcept `bson:"substance" json:"substance"`
	Alternate           *bool           `bson:"alternate,omitempty" json:"alternate,omitempty"`
	AllergenicIndicator *bool           `bson:"allergenicIndicator,omitempty" json:"allergenicIndicator,omitempty"`
}
type OtherDeviceDefinition DeviceDefinition

// MarshalJSON marshals the given DeviceDefinition as JSON into a byte slice
func (r DeviceDefinition) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OtherDeviceDefinition
		ResourceType string `json:"resourceType"`
	}{
		OtherDeviceDefinition: OtherDeviceDefinition(r),
		ResourceType:          "DeviceDefinition",
	})
}

// UnmarshalDeviceDefinition unmarshals a DeviceDefinition.
func UnmarshalDeviceDefinition(b []byte) (DeviceDefinition, error) {
	var deviceDefinition DeviceDefinition
	if err := json.Unmarshal(b, &deviceDefinition); err != nil {
		return deviceDefinition, err
	}
	return deviceDefinition, nil
}
