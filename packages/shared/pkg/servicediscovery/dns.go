package servicediscovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
)

// HostResolver resolves a DNS name to its addresses. *net.Resolver satisfies it.
type HostResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type dnsDiscovery struct {
	NoSync

	name     string
	port     uint16
	resolver HostResolver
}

// NewDNS returns a Discoverer that resolves name on every ListInstances call and
// reports one instance per address, all on port. It suits a pool of plain hosts
// that register themselves under one DNS name (for example a Route53 private
// multivalue record or a Cloud Map service) without Nomad or Kubernetes.
//
// The node ID is left empty: the API learns each orchestrator's node ID from the
// orchestrator's own ServiceInfo RPC.
func NewDNS(name string, port uint16, resolver HostResolver) (Discoverer, error) {
	if name == "" {
		return nil, fmt.Errorf("dns discovery: empty name")
	}

	if port == 0 {
		return nil, fmt.Errorf("dns discovery: zero port for %q", name)
	}

	if resolver == nil {
		resolver = net.DefaultResolver
	}

	return &dnsDiscovery{name: name, port: port, resolver: resolver}, nil
}

func (d *dnsDiscovery) ListInstances(ctx context.Context) ([]Instance, error) {
	ctx, span := tracer.Start(ctx, "list-dns-instances")
	defer span.End()

	addrs, err := d.resolver.LookupHost(ctx, d.name)
	if err != nil {
		// No records means no nodes, not a failed refresh: a pool can scale to zero.
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return []Instance{}, nil
		}

		return nil, fmt.Errorf("dns discovery: resolve %q: %w", d.name, err)
	}

	slices.Sort(addrs)
	addrs = slices.Compact(addrs)

	instances := make([]Instance, 0, len(addrs))
	for _, addr := range addrs {
		instances = append(instances, Instance{
			WorkloadID: net.JoinHostPort(addr, strconv.Itoa(int(d.port))),
			// The address is the machine: a build records the NodeID it was placed
			// on and finds that machine again by it.
			NodeID:    addr,
			IPAddress: addr,
			Port:      d.port,
			Backend:   BackendDNS,
		})
	}

	return instances, nil
}
