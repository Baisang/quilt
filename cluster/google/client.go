package google

import (
	"context"
	"os"
	"path/filepath"

	compute "google.golang.org/api/compute/v1"
)

//go:generate mockery -inpkg -testonly -name=client
type client interface {
	GetInstance(project, zone, id string) (*compute.Instance, error)
	ListInstances(project, zone string, opts apiOptions) (*compute.InstanceList,
		error)
	InsertInstance(project, zone string, instance *compute.Instance) (
		*compute.Operation, error)
	DeleteInstance(project, zone, operation string) (*compute.Operation, error)
	AddAccessConfig(project, zone, instance, networkInterface string,
		accessConfig *compute.AccessConfig) (*compute.Operation, error)
	DeleteAccessConfig(project, zone, instance, accessConfig,
		networkInterface string) (*compute.Operation, error)
	GetZoneOperation(project, zone, operation string) (*compute.Operation, error)
	GetGlobalOperation(project, operation string) (*compute.Operation, error)
	ListFirewalls(project string) (*compute.FirewallList, error)
	InsertFirewall(project string, firewall *compute.Firewall) (
		*compute.Operation, error)
	PatchFirewall(project, name string, firewall *compute.Firewall) (
		*compute.Operation, error)
	DeleteFirewall(project, firewall string) (*compute.Operation, error)
	ListNetworks(project string) (*compute.NetworkList, error)
	InsertNetwork(project string, network *compute.Network) (
		*compute.Operation, error)
}

type clientImpl struct {
	gce *compute.Service
}

func newClient() (*clientImpl, error) {
	keyfile := filepath.Join(os.Getenv("HOME"), ".gce", "quilt.json")
	err := os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyfile)
	if err != nil {
		panic(err)
	}

	service, err := newComputeService(context.Background())
	if err != nil {
		return nil, err
	}

	return &clientImpl{gce: service}, nil
}

/**
 * Service: Instances
 */

func (c *clientImpl) GetInstance(project, zone, id string) (*compute.Instance, error) {
	return c.gce.Instances.Get(project, zone, id).Do()
}

type apiOptions struct {
	filter string
}

func (c *clientImpl) ListInstances(project, zone string, opts apiOptions) (
	*compute.InstanceList, error) {
	call := c.gce.Instances.List(project, zone)
	if opts.filter != "" {
		call = call.Filter(opts.filter)
	}

	return call.Do()
}

func (c *clientImpl) InsertInstance(project, zone string, instance *compute.Instance) (
	*compute.Operation, error) {
	return c.gce.Instances.Insert(project, zone, instance).Do()
}

func (c *clientImpl) DeleteInstance(project, zone, instance string) (*compute.Operation,
	error) {
	return c.gce.Instances.Delete(project, zone, instance).Do()
}

func (c *clientImpl) AddAccessConfig(project, zone, instance, networkInterface string,
	accessConfig *compute.AccessConfig) (*compute.Operation, error) {
	return c.gce.Instances.AddAccessConfig(project, zone, instance, networkInterface,
		accessConfig).Do()
}

func (c *clientImpl) DeleteAccessConfig(project, zone, instance, accessConfig,
	networkInterface string) (*compute.Operation, error) {
	return c.gce.Instances.DeleteAccessConfig(project, zone, instance,
		accessConfig, networkInterface).Do()
}

/**
 * Service: ZoneOperations
 */

func (c *clientImpl) GetZoneOperation(project, zone, operation string) (
	*compute.Operation, error) {
	return c.gce.ZoneOperations.Get(project, zone, operation).Do()
}

/**
 * Service: GlobalOperations
 */

func (c *clientImpl) GetGlobalOperation(project, operation string) (*compute.Operation,
	error) {
	return c.gce.GlobalOperations.Get(project, operation).Do()
}

/**
 * Service: Firewall
 */

func (c *clientImpl) ListFirewalls(project string) (*compute.FirewallList, error) {
	return c.gce.Firewalls.List(project).Do()
}

func (c *clientImpl) InsertFirewall(project string, firewall *compute.Firewall) (
	*compute.Operation, error) {
	return c.gce.Firewalls.Insert(project, firewall).Do()
}

func (c *clientImpl) PatchFirewall(project, name string, firewall *compute.Firewall) (
	*compute.Operation, error) {
	return c.gce.Firewalls.Patch(project, name, firewall).Do()
}

func (c *clientImpl) DeleteFirewall(project, firewall string) (
	*compute.Operation, error) {
	return c.gce.Firewalls.Delete(project, firewall).Do()
}

/**
 * Service: Networks
 */

func (c *clientImpl) ListNetworks(project string) (*compute.NetworkList, error) {
	return c.gce.Networks.List(project).Do()
}

func (c *clientImpl) InsertNetwork(project string, network *compute.Network) (
	*compute.Operation, error) {
	return c.gce.Networks.Insert(project, network).Do()
}
