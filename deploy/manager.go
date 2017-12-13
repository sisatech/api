package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sisatech/api"
)

// ErrManagerClosed is returned whenever an operation is performed on a closed
// deployment manager.
var ErrManagerClosed = errors.New("manager closed")

// ErrInstanceNotInPool is returned whenever an instance cannot be found for the
// given instance ID within the Pool being searched.
var ErrInstanceNotInPool = errors.New("instance id not found in pool")

// Manager simplifies and automates some common deployment operations. It also
// cleans up after itself, guaranteeing that everything created by the Manager
// will be deleted from VMS if the Manager's Close function is called. The zero
// value for a Manager is not a usable Manager.
type Manager struct {
	closed bool
	lock   sync.Mutex
	client *api.Client
	pools  map[string]*Pool
}

// NewManager returns a usable manager created from an authenticated api.Client
// object.
func NewManager(client *api.Client) (*Manager, error) {
	m := new(Manager)
	m.client = client
	m.pools = make(map[string]*Pool)
	return m, nil
}

// Close prevents the Manager from performing any more operations, and cleans up
// all existing instances created by it.
func (m *Manager) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.closed = true

	list := make([]*Pool, 0)
	for _, v := range m.pools {
		list = append(list, v)
	}

	for _, pool := range list {
		err := pool.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// Pool is a custom deployment of manually managed instances.
type Pool struct {
	mgr        *Manager
	statusLock sync.RWMutex
	org        string
	name       string
	goal       *DeploymentGoal
	state      *DeploymentState
}

// NewPool creates a new custom deployment of manually managed instances for the
// named organization, with the given name.
func (m *Manager) NewPool(org, name string) (*Pool, error) {

	if m.closed {
		return nil, ErrManagerClosed
	}

	p := new(Pool)
	p.mgr = m
	p.org = org
	p.name = name
	p.goal = new(DeploymentGoal)
	p.goal.children = make(map[string]*VM)
	p.state = new(DeploymentState)
	p.state.children = make(map[string]*InstanceStatus)

	m.lock.Lock()
	defer m.lock.Unlock()
	if m.closed {
		return nil, errors.New("closed")
	}
	p.mgr.pools[p.key()] = p

	err := CreateDeployment(p.mgr.client, org, name)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Pool) key() string {
	return fmt.Sprintf("%s::%s", p.org, p.name)
}

// Instances returns an alphabetized list of instance IDs for the Pool.
// Instances that are scheduled to be created will be included in the list even
// if they have not yet been provisioned. Conversely, instances that are
// scheduled to be destroyed will not be included in the list even if they are
// still running.
func (p *Pool) Instances() []string {
	list := make([]string, 0)
	for k := range p.goal.children {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

// SpawnArgs defines all of the information needed to define an instance, and is
// used by Spawn to spawn new instances within a Pool. App should be the full
// path to an application within an organization's online repository. Version
// cannot be left empty and must be a valid ID string for the App, *a tag is not
// valid*. Use the apps.ResolveVersionToID function to handle those use-cases.
type SpawnArgs struct {
	Platform string
	App      string
	Version  string
}

// Spawn creates a new instance from the provided SpawnArgs and retrns a new
// instance ID generated for it.
func (p *Pool) Spawn(args *SpawnArgs) (string, error) {

	if p.mgr.closed {
		return "", ErrManagerClosed
	}

	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	src := make([]byte, 4)
	rand.Read(src)
	id := hex.EncodeToString(src)

	g := p.goal.Copy()
	g.Attach(id, &VM{
		Platform: args.Platform,
		App:      args.App,
		Version:  args.Version,
	})

	err := g.Push(p.mgr.client, p.org, p.name)
	if err != nil {
		return "", err
	}

	p.goal = g

	return id, nil
}

// Destroy terminates the instance named by the given ID.
func (p *Pool) Destroy(id string) error {

	if p.mgr.closed {
		return ErrManagerClosed
	}

	p.statusLock.Lock()
	defer p.statusLock.Unlock()

	p.goal.Detach(id)

	p.goal.Push(p.mgr.client, p.org, p.name)

	return nil
}

// InstanceStatus contains information returned about an instance as received
// from VMS.
type InstanceStatus struct {
	Deployer string   `json:"deployer"`
	App      string   `json:"app"`
	Version  string   `json:"version"`
	Hostname string   `json:"hostname"`
	IP       string   `json:"ip"`
	URLs     []string `json:"urls"`
}

// Status returns the last known InstanceStatus for the instance named by ID.
// This function does not poll VMS to update the InstanceStatus information. Use
// the Update function periodically to get the latest information.
func (p *Pool) Status(id string) (*InstanceStatus, error) {
	v, ok := p.state.children[id]
	if !ok {
		_, ok = p.goal.children[id]
		if !ok {
			return nil, ErrInstanceNotInPool
		}
		return &InstanceStatus{}, nil
	}
	return v, nil
}

// Close destroys the VMS deployment managed by the pool.
func (p *Pool) Close() error {
	p.statusLock.Lock()
	defer p.statusLock.Unlock()
	timeout := time.After(time.Second * 60)

	ch := make(chan error)
	go func() {
		defer func() {
			recover()
		}()
		ch <- DeleteDeployment(p.mgr.client, p.org, p.name)
	}()

	select {
	case err := <-ch:
		if err != nil {
			return err
		}
	case <-timeout:
		close(ch)
		return errors.New("cleanup timed out")
	}

	p.goal = nil
	p.state = nil
	delete(p.mgr.pools, p.key())
	return nil
}

type tuple struct {
	pl  interface{}
	err error
}

// Update polls VMS for the latest state information about the pool's VMS
// deployment. This should be called periodically, or whenever the latest
// information is required. It updates all VMs within the pool at once.
func (p *Pool) Update() error {

	if p.mgr.closed {
		return ErrManagerClosed
	}

	p.statusLock.Lock()
	defer p.statusLock.Unlock()
	timeout := time.After(time.Second * 60)

	ch := make(chan *tuple)
	go func() {
		defer func() {
			recover()
		}()
		state, err := GetDeployment(p.mgr.client, p.org, p.name)
		if err != nil {
			ch <- &tuple{err: err}
		}
		ch <- &tuple{pl: state}
	}()

	select {
	case x := <-ch:
		if x.err != nil {
			return x.err
		}
		p.state = x.pl.(*DeploymentState)
	case <-timeout:
		close(ch)
		return errors.New("cleanup timed out")
	}

	return nil
}
