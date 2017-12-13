package deploy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sisatech/api"
)

// VM TODO
type VM struct {
	Platform string
	App      string
	Version  string
}

// MarshalJSON TODO
func (x *VM) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"platform":      x.Platform,
		"app":           x.App,
		"type":          "vm",
		"version":       x.Version,
		"customization": nil,
	}
	return json.Marshal(&m)
}

// DeploymentGoal TODO
type DeploymentGoal struct {
	children map[string]*VM
}

// Attach TODO
func (g *DeploymentGoal) Attach(id string, vm *VM) {
	g.children[id] = vm
}

// Detach TODO
func (g *DeploymentGoal) Detach(id string) {
	delete(g.children, id)
}

// MarshalJSON TODO
func (g *DeploymentGoal) MarshalJSON() ([]byte, error) {

	children, err := json.Marshal(g.children)
	if err != nil {
		return nil, err
	}

	s := fmt.Sprintf("{\"type\":\"subtree\",\"children\":%s}", string(children))

	return []byte(s), nil
}

// Copy TODO
func (g *DeploymentGoal) Copy() *DeploymentGoal {
	n := new(DeploymentGoal)
	n.children = make(map[string]*VM)
	for k, v := range g.children {
		n.children[k] = v
	}
	return n
}

// Push TODO
func (g *DeploymentGoal) Push(client *api.Client, org, name string) error {
	pl, err := json.Marshal(g)
	if err != nil {
		return err
	}

	url := client.URL("deployments/api/v3/orgs/%s/deployments/%s", org, name)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(pl))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	return nil
}

// DeploymentState TODO
type DeploymentState struct {
	children map[string]*InstanceStatus
}

type statePL struct {
	Children map[string]interface{} `json:"children"`
	URLs     []string               `json:"urls"`
}

// UnmarshalJSON ..
func (s *DeploymentState) UnmarshalJSON(data []byte) error {

	m := make(map[string]interface{})
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}

	v, ok := m["state"]
	if !ok {
		return errors.New("missing 'state' key")
	}

	data, err = json.Marshal(v)
	if err != nil {
		return err
	}

	pl := new(statePL)
	err = json.Unmarshal(data, pl)
	if err != nil {
		return err
	}

	s.children = make(map[string]*InstanceStatus)
	for k, v := range pl.Children {

		x, ok := v.(map[string]interface{})
		if !ok {
			return errors.New("bad json")
		}

		v, ok = x["vm"]
		if !ok {
			return errors.New("bad json")
		}

		data, err = json.Marshal(v)
		if err != nil {
			return err
		}

		i := new(InstanceStatus)
		err = json.Unmarshal(data, i)
		if err != nil {
			return err
		}

		s.children[k] = i
	}

	return nil
}

// GetDeployment returns a DeploymentState object representing the state of the
// named deployment for the given organization. TODO
func GetDeployment(client *api.Client, org, name string) (*DeploymentState, error) {

	req, err := http.NewRequest(http.MethodGet, client.URL("deployments/api/v3/orgs/%s/deployments/%s", org, name), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	pl, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	state := new(DeploymentState)
	err = json.Unmarshal(pl, state)
	if err != nil {
		return nil, err
	}

	return state, nil
}

// CreateDeployment creates a new empty deployment with the given name for the
// named organization, using the provided api.Client.
func CreateDeployment(client *api.Client, org, name string) error {

	data := []byte("{\"type\":\"subtree\",\"children\":{}}")

	req, err := http.NewRequest(http.MethodPut, client.URL("deployments/api/v3/orgs/%s/deployments/%s", org, name), bytes.NewReader(data))
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	return nil
}

// DeleteDeployment deletes the named deployment from the named organization
// using the provided api.Client.
func DeleteDeployment(client *api.Client, org, name string) error {

	req, err := http.NewRequest(http.MethodDelete, client.URL("deployments/api/v3/orgs/%s/deployments/%s", org, name), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return errors.New(resp.Status)
	}

	return nil
}
