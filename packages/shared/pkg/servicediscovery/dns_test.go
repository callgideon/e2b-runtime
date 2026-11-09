package servicediscovery

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	addrs []string
	err   error
}

func (f fakeResolver) LookupHost(context.Context, string) ([]string, error) { return f.addrs, f.err }

func TestDNSListsOneInstancePerAddress(t *testing.T) {
	t.Parallel()
	d, err := NewDNS("orchestrators.boxes.internal", 5008, fakeResolver{addrs: []string{"10.20.2.7", "10.20.1.5", "10.20.2.7"}})
	require.NoError(t, err)

	got, err := d.ListInstances(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []Instance{
		{WorkloadID: "10.20.1.5:5008", NodeID: "10.20.1.5", IPAddress: "10.20.1.5", Port: 5008, Backend: BackendDNS},
		{WorkloadID: "10.20.2.7:5008", NodeID: "10.20.2.7", IPAddress: "10.20.2.7", Port: 5008, Backend: BackendDNS},
	}, got)
}

func TestDNSNoRecordsIsAnEmptyPool(t *testing.T) {
	t.Parallel()
	d, err := NewDNS("orchestrators.boxes.internal", 5008, fakeResolver{err: &net.DNSError{Err: "no such host", Name: "x", IsNotFound: true}})
	require.NoError(t, err)

	got, err := d.ListInstances(t.Context())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestDNSResolverFailureIsAnError(t *testing.T) {
	t.Parallel()
	d, err := NewDNS("orchestrators.boxes.internal", 5008, fakeResolver{err: errors.New("timeout")})
	require.NoError(t, err)

	_, err = d.ListInstances(t.Context())
	require.Error(t, err)
}

func TestDNSRejectsEmptyNameAndZeroPort(t *testing.T) {
	t.Parallel()
	_, err := NewDNS("", 5008, nil)
	require.Error(t, err)
	_, err = NewDNS("x", 0, nil)
	require.Error(t, err)
}
