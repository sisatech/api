package market

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sisatech/api"
)

// Download ..
func Download(project, version string) (io.ReadCloser, error) {

	url := fmt.Sprintf("%s/market/api/apps/%s?refs=%s", api.OfficialDomain, project, version)

	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := http.DefaultClient
	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	return resp.Body, nil
}
