// File: client/artdecor.go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SanteonNL/fenix/cmd/artdecor/internal/utils"
	"github.com/SanteonNL/fenix/cmd/artdecor/types"

	"github.com/SanteonNL/fenix/cmd/artdecor/body"
	"github.com/hashicorp/go-retryablehttp"
)

type ArtDecorApiClient struct {
	BaseURI    string
	HTTPClient *http.Client
	token      string
}

type TokenResponse struct {
	User struct {
		DBA    bool     `json:"dba"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
	}
	Token   string `json:"token"`
	Issued  string `json:"issued"`
	Expires string `json:"expires"`
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
		BaseURI:    os.Getenv("ART_API_URL"),
		HTTPClient: retryClient.StandardClient(),
	}
}

func (c *ArtDecorApiClient) Token() (string, error) {
	loginBody := body.UserLogin{
		Username: os.Getenv("ART_USER"),
		Password: os.Getenv("ART_PASSWORD"),
	}

	req, err := c.prepareRequest(http.MethodPost, "/token", loginBody)
	if err != nil {
		return "", err
	}

	resp := new(TokenResponse)
	err = c.sendRequest(req, resp)
	return resp.Token, err
}

func (c *ArtDecorApiClient) SetToken(token string) {
	if token == "" {
		return
	}
	c.token = token
}

func (c *ArtDecorApiClient) CodeSystemToValueSet(id, effectiveDate string, queryParams any) (types.DECORValueSet, error) {
	var vs types.DECORValueSet
	cs, err := c.CodeSystem(id, effectiveDate, queryParams)
	if err != nil {
		return vs, err
	}
	vs.FromCodeSystem(&cs)
	return vs, nil
}

func (c *ArtDecorApiClient) CodeSystem(id, effectiveDate string, queryParams any) (types.DECORCodeSystem, error) {
	var endpoint string = "/codesystem"
	var validParams = []string{"language", "prefix", "release"}

	if !types.IsValidOID(id) {
		return types.DECORCodeSystem{}, fmt.Errorf("invalid OID %q", id)
	}
	endpoint += "/" + id

	if effectiveDate != "" {
		if !types.IsValidDate(effectiveDate) {
			return types.DECORCodeSystem{}, fmt.Errorf("invalid date %q", effectiveDate)
		}
		endpoint += "/" + effectiveDate
	}

	query, err := utils.ParseQueryParams(queryParams, validParams)
	if err != nil {
		return types.DECORCodeSystem{}, err
	}
	if query != "" {
		endpoint += "?" + strings.TrimSuffix(query, "&")
	}

	var cs types.DECORCodeSystem
	if err := c.get(endpoint, &cs); err != nil {
		return types.DECORCodeSystem{}, err
	}
	return cs, nil
}

func (c *ArtDecorApiClient) CreateConceptMap(conceptMap types.DECORConceptMap, queryParams any) error {
	var endpoint string = "/conceptmap"
	var validParams = []string{
		"baseId", "keepIds", "refOnly", "sourceEffectiveDate", "sourceId",
		"targetDate", "prefix",
	}

	query, err := utils.ParseQueryParams(queryParams, validParams)
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

func (c *ArtDecorApiClient) ReadConceptMap(queryParams any) (*[]types.DECORConceptMap, error) {
	var endpoint string = "/conceptmap"
	var validParams = []string{
		"codeSystemEffectiveDate", "codeSystemEffectiveDate:source", "codeSystemEffectiveDate:target",
		"codeSystemId", "codeSystemId:source", "codeSystemId:target", "governanceGroupId", "includebbr",
		"max", "prefix", "resolve", "search", "sort", "sortorder", "status",
		"valueSetEffectiveDate", "valueSetEffectiveDate:source", "valueSetEffectiveDate:target",
		"valueSetId", "valueSetId:source", "valueSetId:target",
	}

	query, err := utils.ParseQueryParams(queryParams, validParams)
	if err != nil {
		return nil, err
	}
	if query != "" {
		endpoint += "?" + strings.TrimSuffix(query, "&")
	}

	resp := new(body.GetConceptMapListResponse)
	if err := c.get(endpoint, resp); err != nil {
		return nil, err
	}

	b, err := json.Marshal(resp.ConceptMap)
	if err != nil {
		return nil, err
	}

	cms := make([]types.DECORConceptMap, 0)
	if err := json.Unmarshal(b, &cms); err != nil {
		return nil, err
	}
	log.Default().Printf("Response %+v\n", cms)
	return &cms, nil
}

// HTTP helper methods
func (c *ArtDecorApiClient) get(endpoint string, response any) error {
	return c.do(http.MethodGet, endpoint, nil, response)
}

func (c *ArtDecorApiClient) post(endpoint string, body, response any) error {
	return c.do(http.MethodPost, endpoint, body, response)
}

func (c *ArtDecorApiClient) do(method, endpoint string, body, response any) error {
	req, err := c.prepareRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	c.signRequest(req)
	return c.sendRequest(req, response)
}

func (c *ArtDecorApiClient) prepareRequest(method, endpoint string, body any) (*http.Request, error) {
	uri, err := url.JoinPath(c.BaseURI, endpoint)
	if err != nil {
		return nil, err
	}

	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	uri, err = url.QueryUnescape(uri)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, uri, bodyReader)
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

func (c *ArtDecorApiClient) sendRequest(req *http.Request, response any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	log.Default().Printf("Response Status: %s", resp.Status)
	log.Default().Printf("Request URL: %s", req.URL.String())

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if response is empty
	if len(bodyBytes) == 0 {
		return fmt.Errorf("received empty response from server for URL: %s", req.URL.String())
	}

	// Log the raw response for debugging
	log.Default().Printf("Raw Response Body:\n%s\n", string(bodyBytes))

	// Check if response status is not successful (2xx)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned error status %d: %s\nBody: %s",
			resp.StatusCode, resp.Status, string(bodyBytes))
	}

	// Parse response if a target was provided
	if response != nil {
		if err := json.Unmarshal(bodyBytes, response); err != nil {
			return fmt.Errorf("failed to parse response JSON: %w\nBody: %s", err, string(bodyBytes))
		}
	}

	return nil
}
