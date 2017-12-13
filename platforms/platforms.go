package platforms

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/sisatech/api"
)

type platformsTuple struct {
	Name string
	Type string
}

type platformsListResponse []platformsTuple

// Exists checks if the named platform is accessible to the client for the named
// organization.
func Exists(client *api.Client, org, platform string) (bool, error) {

	url := client.URL("platforms/api/v3/orgs/%s/platforms/", org)
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

	pl := make(platformsListResponse, 0)
	err = json.Unmarshal(data, &pl)
	if err != nil {
		return false, err
	}

	for _, tuple := range pl {
		if tuple.Name == platform {
			return true, nil
		}
	}

	return false, nil

}
