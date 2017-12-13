package apps

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sisatech/api"
)

// ErrVersionNotExists is returned whenever a request fails to resolve a version
// ID or tag name into an ID.
var ErrVersionNotExists = errors.New("app version does not exist")

type appsTuple struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type appsListResponse []appsTuple

// Exists checks if the named app is accessible to the client for the named
// organization.
func Exists(client *api.Client, org, app string) (bool, error) {

	dir, base := filepath.Split(app)
	if dir == "." {
		dir = ""
	}
	dir = strings.TrimSuffix(dir, "/")

	url := client.URL("images/api/v3/orgs/%s/objects/?op=list&dir=%s", org, dir)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return false, errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	pl := make(appsListResponse, 0)
	err = json.Unmarshal(data, &pl)
	if err != nil {
		return false, err
	}

	for _, tuple := range pl {
		if tuple.Name == base {
			if tuple.Type == "app" {
				return true, nil
			}
			return false, fmt.Errorf("object '%s' is type '%s'", app, tuple.Type)
		}
	}

	return false, nil
}

type versionTuple struct {
	Tag     string    `json:"tag"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

type versionListResponse []versionTuple

func (v versionListResponse) Len() int {
	return len(v)
}

func (v versionListResponse) Swap(i, j int) {
	tmp := v[i]
	v[i] = v[j]
	v[j] = tmp
}

func (v versionListResponse) Less(i, j int) bool {
	return v[i].Created.Before(v[j].Created)
}

// ResolveVersionToID attempts to resolve the provided version for the named
// app, converting it to a version ID.
func ResolveVersionToID(client *api.Client, org, app, version string) (string, error) {

	dir, base := filepath.Split(app)
	if dir == "." {
		dir = ""
	}
	dir = strings.TrimSuffix(dir, "/")

	url := client.URL("images/api/v3/orgs/%s/objects/%s?op=list&dir=%s", org, base, dir)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	v := make(versionListResponse, 0)
	err = json.Unmarshal(data, &v)
	if err != nil {
		return "", err
	}

	if len(v) == 0 {
		return "", ErrVersionNotExists
	}

	if version == "" {
		sort.Sort(v)
		return v[len(v)-1].Version, nil
	}

	for _, tuple := range v {
		if tuple.Version == version || tuple.Tag == version {
			return tuple.Version, nil
		}
	}

	return "", ErrVersionNotExists

}
