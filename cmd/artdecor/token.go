package main

import (
	"net/http"
	"os"

	"github.com/SanteonNL/fenix/cmd/artdecor/body"
)

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

func (c *ArtDecorApiClient) Token() (string, error) {
	body := body.UserLogin{
		Username: os.Getenv("ART_USER"),
		Password: os.Getenv("ART_PASSWORD"),
	}
	req, err := c.prepareRequest(http.MethodPost, "/token", body)
	if err != nil {
		return "", err
	}
	resp := new(TokenResponse)
	err = c.sendRequest(req, resp)
	return resp.Token, err
}
