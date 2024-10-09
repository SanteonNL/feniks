package main

import (
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func init() {
	// loading environment variables from '.env' file
	if err := godotenv.Load(".env"); err != nil {
		log.Default().Fatal("Error loading .env file")
	}
}

func main() {
	c := NewArtDecorApiClient()
	token, err := c.Token()
	if err != nil {
		log.Default().Fatal(err)
	}
	c.SetToken(token)

	var cms *[]DECORConceptMap
	if cms, err = c.ReadConceptMap(map[string]string{"prefix": os.Getenv("ART_PROJECT"), "sort": "displayName", "search": os.Getenv("ORGANIZATION")}); err != nil {
		log.Default().Fatal(err)
	}
	cms = filterConceptMaps(cms, os.Getenv("ORGANIZATION")+"_"+os.Getenv("SOURCE")+"_") // MST_HIX_

	var FILEPATH = "../../config/conceptmaps"
	var FORMAT = "json"

	for _, cm := range *cms {
		downloadURI, err := url.JoinPath(os.Getenv("ART_DOWNLOAD_URL"), *cm.Ident, "ConceptMap", cm.Id.String())
		if err != nil {
			log.Default().Fatal(err)
		}
		downloadURI += "?_format=" + FORMAT

		file, err := filepath.Abs(filepath.Join(FILEPATH, cm.DisplayName) + "." + FORMAT)
		if err != nil {
			log.Default().Fatal(err)
		}
		log.Default().Printf("%s --> %s", downloadURI, file)
		downloadFile(file, downloadURI)
	}
}
