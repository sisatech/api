package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

// OfficialDomain is the domain of the official Vorteil VMS website, go-vorteil.io
const OfficialDomain = "https://go-vorteil.io"

// Client is an HTTP client used by all APIs. It automatically handles
// authentication with VMS, and can otherwise be used in the same way a
// http.Client can by passing http.Requests to its 'Do' function. Its zero value
// is not a usable client.
type Client struct {
	jwt    string
	client *http.Client
	domain string
}

// ClientCredentials contains information needed to authenticate with VMS.
type ClientCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	AcceptedTerms bool   `json:"accepted_terms"`
	JWT           string `json:"jwt"`
}

// URL uses the fmt package to produce a formatted string from the 'format' and
// 'a' args, and then appends it to the domain the client is connect to. The
// provided 'format' string should not have a leading slash.
//
// Example:
//
//	client, _ := Authenticate("https://go-vorteil.io", &ClientCredentials{
//		Username: "example",
// 		Password: "example",
//	})
//
// 	url := client.URL("org/%s/apps/%s", "sisatech", "helloworld")
// 	fmt.Println(url)
//
// Outputs:
//
//	https://go-vorteil.io/org/sisatech/apps/helloworld
//
func (c *Client) URL(format string, a ...interface{}) string {
	return fmt.Sprintf("%s/%s", c.domain, fmt.Sprintf(format, a...))
}

// Authenticate connects to the named VMS domain and uses the provided client
// credentials to acquire a JWT for future request authentication. The provided
// domain should include the protocol information, but should not include a
// trailing slash.
//
// Example:
//
//	client, _ := Authenticate("https://go-vorteil.io", &ClientCredentials{
//		Username: "example",
// 		Password: "example",
//	})
//
func Authenticate(domain string, credentials *ClientCredentials) (*Client, error) {

	c := new(Client)
	c.client = http.DefaultClient
	c.domain = domain

	body, err := json.Marshal(credentials)
	if err != nil {
		return nil, err
	}

	url := c.URL("auth/api/login")
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var pl []byte
	pl, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	v := new(loginResponse)
	err = json.Unmarshal(pl, v)
	if err != nil {
		return nil, err
	}

	c.jwt = v.JWT
	return c, nil
}

// Do is equivalent to the Do function on a http.Client, but it will
// automatically handle authentication on the request. Do sends an HTTP request
// and returns an HTTP response.
//
// Example:
//
//	client, _ := Authenticate("https://go-vorteil.io", &ClientCredentials{
//		Username: "example",
// 		Password: "example",
//	})
//
// 	url := client.URL("org/%s/apps/%s", "sisatech", "helloworld")
//
// 	request, _ := http.NewRequest(http.MethodGet, url, nil)
//
// 	client.Do(request)
//
func (c *Client) Do(r *http.Request) (*http.Response, error) {
	r.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", c.jwt)}
	return c.client.Do(r)
}
