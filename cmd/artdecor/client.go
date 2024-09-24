package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type ArtDecorApiClient struct {
	BaseURI    string
	HTTPClient *http.Client
	token      string
}

func NewArtDecorApiClient() *ArtDecorApiClient {
	jar, _ := cookiejar.New(nil)
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.HTTPClient = &http.Client{
		Jar:     jar,
		Timeout: 60 * time.Second,
	}
	return &ArtDecorApiClient{
		BaseURI:    os.Getenv("ART_URL"),
		HTTPClient: retryClient.StandardClient(),
	}
}

func (c *ArtDecorApiClient) SetToken(token string) {
	if token == "" {
		return
	}
	c.token = token
}

// CodeSystemToValueSet returns a DECOR valueSet from a DECOR codeSystem
// based on id (OID) and effective date denoting its version.
func (c *ArtDecorApiClient) CodeSystemToValueSet(id, effectiveDate string, queryParams any) (DECORValueSet, error) {
	var vs DECORValueSet
	cs, err := c.CodeSystem(id, effectiveDate, queryParams)
	if err != nil {
		return vs, err
	}
	vs.FromCodeSystem(cs)
	return vs, nil
}

// ConceptMapFromCSV returns a DECOR conceptMap from a CSV file.
func (c *ArtDecorApiClient) ConceptMapFromCSV() (DECORConceptMap, error) {
	var cm DECORConceptMap
	return cm, nil
}

// get wraps do using http.MethodGet
func (c *ArtDecorApiClient) get(endpoint string, response any) error {
	return c.do(http.MethodGet, endpoint, nil, response)
}

// post wraps do using http.MethodPost
func (c *ArtDecorApiClient) post(endpoint string, body, response any) error {
	return c.do(http.MethodPost, endpoint, body, response)
}

func (c *ArtDecorApiClient) do(method, endpoint string, body, response any) error {
	req, err := c.prepareRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	c.signRequest(req)
	if err = c.sendRequest(req, response); err != nil {
		return err
	}
	return nil
}

// prepareRequest returns a new HTTP request given a method, ART-DECOR endpoint,
// and optional body.
func (c *ArtDecorApiClient) prepareRequest(method, endpoint string, body any) (*http.Request, error) {
	uri, err := url.JoinPath(c.BaseURI, endpoint)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	uri, err = url.QueryUnescape(uri)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, uri, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json; charset=utf-8")
	return req, nil
}

func (c *ArtDecorApiClient) signRequest(req *http.Request) {
	req.Header.Set("X-Auth-Token", c.token)
}

// sendRequest sends an HTTP request and stores the HTTP response body in the value
// pointed to by response.
func (c *ArtDecorApiClient) sendRequest(req *http.Request, response any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Default().Println(resp.Status)
	if response != nil {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return err
		}
	}
	return nil
}
