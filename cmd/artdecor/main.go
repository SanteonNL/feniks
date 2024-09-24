package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	// loading environment variables from '.env' file
	if err := godotenv.Load(".env"); err != nil {
		log.Default().Fatal("Error loading .env file")
	}
}

func main() {
	var ident = os.Getenv("ART_PROJECT")

	c := NewArtDecorApiClient()
	token, err := c.Token()
	if err != nil {
		log.Default().Fatal(err)
	}
	c.SetToken(token)

	//// ApacheIIDiagnose NICE CodeSystem
	// id := "2.16.840.1.113883.2.4.3.11.60.124.5.5"
	//
	//// ApacheIVDiagnose NICE CodeSystem
	// id := "2.16.840.1.113883.2.4.3.11.60.124.5.3"
	//
	// vs, err := c.CodeSystemToValueSet(id, "", nil)
	// if err != nil {
	// 	log.Default().Fatal(err)
	// }
	// log.Default().Printf("ValueSet %+v", vs)
	//
	// if os.Getenv("DEBUG") == "0" {
	// 	if err = c.CreateValueSet(vs, map[string]string{"prefix": ident}); err != nil {
	// 		log.Default().Fatal(err)
	// 	}
	// }

	// SOURCE := "HIX"
	// NAME := "/home/thscheeve/develop/fenix/cmd/artdecor/conceptmap_commonucumcodes.csv"
	//
	// cm := DECORConceptMap{}
	// // cm.FromSanteonCSV(SOURCE, "C:\\Users\\ThomScheeveSanteon\\Documents\\HipsETL\\apps\\artDecor\\conceptmap_wpai_gh_al_01.csv")
	// if err := cm.FromSanteonCSV(SOURCE, NAME); err != nil {
	// 	log.Default().Fatal(err)
	// }
	// // log.Default().Printf("ConceptMap %+v", cm)
	//
	// if os.Getenv("DEBUG") == "0" {
	// 	if err = c.CreateConceptMap(cm, map[string]string{"prefix": ident}); err != nil {
	// 		log.Default().Fatal(err)
	// 	}
	// }
	// if _, err := c.ReadConceptMap(map[string]string{"prefix": ident, "search": "[" + os.Getenv("ORGANIZATION") + "_" + SOURCE + "]"}); err != nil {
	// if _, err := c.ReadConceptMap(map[string]string{"prefix": ident, "sort": "displayName", "search": os.Getenv("ORGANIZATION")}); err != nil {
	if _, err := c.ReadConceptMap(map[string]string{"prefix": ident, "search": "MST_HIX_KOOS-CM"}); err != nil {
		// if _, err := c.ReadConceptMap(map[string]string{"prefix": ident, "search": os.Getenv("ORGANIZATION") + "_"}); err != nil {
		// if _, err := c.ReadConceptMap(map[string]string{"prefix": ident}); err != nil {
		log.Default().Fatal(err)
	}
}
