package db

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostnameSelect(t *testing.T) {
	conn := New()
	err := conn.Txn(HostnameTable).Run(func(view Database) error {
		h := view.InsertHostname()
		h.Hostname = "hostname"
		h.IP = "ip"
		view.Commit(h)

		h = view.InsertHostname()
		h.Hostname = "hostname2"
		h.IP = "ip2"
		view.Commit(h)
		return nil
	})
	assert.NoError(t, err)

	actual := conn.SelectFromHostname(func(h Hostname) bool {
		return h.Hostname == "hostname"
	})
	assert.Equal(t, []Hostname{{ID: 1, Hostname: "hostname", IP: "ip"}}, actual)

	assert.Len(t, conn.SelectFromHostname(nil), 2)
	conn.Txn(HostnameTable).Run(func(view Database) error {
		h := view.SelectFromHostname(func(h Hostname) bool {
			return h.Hostname == "hostname2"
		})[0]
		view.Remove(h)
		return nil
	})
	assert.Len(t, conn.SelectFromHostname(nil), 1)
}

func TestHostnameSlice(t *testing.T) {
	exp := []Hostname{
		{
			ID:       1,
			Hostname: "a",
		},
		{
			ID:       2,
			Hostname: "b",
		},
		{
			ID:       3,
			Hostname: "b",
		},
	}
	toSort := []Hostname{
		{
			ID:       2,
			Hostname: "b",
		},
		{
			ID:       3,
			Hostname: "b",
		},
		{
			ID:       1,
			Hostname: "a",
		},
	}
	sort.Sort(HostnameSlice(toSort))
	assert.Equal(t, exp, toSort)

	sort.Sort(HostnameSlice(toSort))
	assert.Equal(t, toSort, toSort)

	assert.Equal(t, HostnameSlice(toSort).Get(0), exp[0])
}

func TestHostnameString(t *testing.T) {
	assert.Equal(t, "Hostname-1{Hostname=foo, IP=ip}",
		Hostname{ID: 1, Hostname: "foo", IP: "ip"}.String())
}
