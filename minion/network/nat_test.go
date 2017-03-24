package network

//go:generate mockery -name=IPTables

import (
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/minion/ipdef"
	"github.com/quilt/quilt/minion/network/mocks"
	"github.com/quilt/quilt/stitch"
)

func TestUpdateNATErrors(t *testing.T) {
	ipt := &mocks.IPTables{}
	anErr := errors.New("err")

	getPublicInterface = func() (string, error) {
		return "", anErr
	}
	assert.NotNil(t, updateNAT(ipt, nil, nil))

	ipt = &mocks.IPTables{}
	ipt.On("AppendUnique", mock.Anything, mock.Anything, mock.Anything).Return(anErr)
	getPublicInterface = func() (string, error) {
		return "eth0", nil
	}
	assert.NotNil(t, updateNAT(ipt, nil, nil))

	ipt = &mocks.IPTables{}
	ipt.On("AppendUnique", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ipt.On("List", mock.Anything, mock.Anything).Return(nil, anErr)
	assert.NotNil(t, updateNAT(ipt, nil, nil))

	ipt = &mocks.IPTables{}
	ipt.On("AppendUnique", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ipt.On("List", "nat", "PREROUTING").Return(nil, nil)
	ipt.On("List", "nat", "POSTROUTING").Return(nil, anErr)
	assert.NotNil(t, updateNAT(ipt, nil, nil))
}

func TestPreroutingRules(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{
			IP:     "8.8.8.8",
			Labels: []string{"red", "blue"},
		},
		{
			IP:     "9.9.9.9",
			Labels: []string{"purple"},
		},
		{
			IP:     "10.10.10.10",
			Labels: []string{"green"},
		},
	}

	connections := []db.Connection{
		{
			From:    stitch.PublicInternetLabel,
			To:      "red",
			MinPort: 80,
		},
		{
			From:    stitch.PublicInternetLabel,
			To:      "blue",
			MinPort: 81,
		},
		{
			From:    stitch.PublicInternetLabel,
			To:      "purple",
			MinPort: 80,
		},
		{
			From:    "green",
			To:      stitch.PublicInternetLabel,
			MinPort: 80,
		},
		{
			From:    "yellow",
			To:      stitch.PublicInternetLabel,
			MinPort: 80,
		},
	}

	actual := preroutingRules("eth0", containers, connections)
	exp := []string{
		"-i eth0 -p tcp -m tcp --dport 80 -j DNAT --to-destination 8.8.8.8:80",
		"-i eth0 -p udp -m udp --dport 80 -j DNAT --to-destination 8.8.8.8:80",
		"-i eth0 -p tcp -m tcp --dport 81 -j DNAT --to-destination 8.8.8.8:81",
		"-i eth0 -p udp -m udp --dport 81 -j DNAT --to-destination 8.8.8.8:81",
		"-i eth0 -p tcp -m tcp --dport 80 -j DNAT --to-destination 9.9.9.9:80",
		"-i eth0 -p udp -m udp --dport 80 -j DNAT --to-destination 9.9.9.9:80",
	}
	assert.Equal(t, exp, actual)
}

func TestPostroutingRules(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{
			IP:     "8.8.8.8",
			Labels: []string{"red", "blue"},
		},
		{
			IP:     "9.9.9.9",
			Labels: []string{"purple"},
		},
		{
			IP:     "10.10.10.10",
			Labels: []string{"green"},
		},
	}

	connections := []db.Connection{
		{
			From:    "red",
			To:      stitch.PublicInternetLabel,
			MinPort: 80,
		},
		{
			From:    "blue",
			To:      stitch.PublicInternetLabel,
			MinPort: 81,
		},
		{
			From:    "purple",
			To:      stitch.PublicInternetLabel,
			MinPort: 80,
		},
		{
			From:    "purple",
			To:      stitch.PublicInternetLabel,
			MinPort: 81,
		},
		{
			From:    stitch.PublicInternetLabel,
			To:      "green",
			MinPort: 80,
		},
		{
			From:    "yellow",
			To:      stitch.PublicInternetLabel,
			MinPort: 80,
		},
	}

	exp := []string{
		"-s 8.8.8.8/32 -p tcp --dport 80 -o eth0 -j MASQUERADE",
		"-s 8.8.8.8/32 -p tcp --dport 81 -o eth0 -j MASQUERADE",
		"-s 8.8.8.8/32 -p udp --dport 80 -o eth0 -j MASQUERADE",
		"-s 8.8.8.8/32 -p udp --dport 81 -o eth0 -j MASQUERADE",
		"-s 9.9.9.9/32 -p tcp --dport 80 -o eth0 -j MASQUERADE",
		"-s 9.9.9.9/32 -p tcp --dport 81 -o eth0 -j MASQUERADE",
		"-s 9.9.9.9/32 -p udp --dport 80 -o eth0 -j MASQUERADE",
		"-s 9.9.9.9/32 -p udp --dport 81 -o eth0 -j MASQUERADE",
	}
	actual := postroutingRules("eth0", containers, connections)
	sort.Strings(actual)
	assert.Equal(t, exp, actual)
}

func TestGetRules(t *testing.T) {
	ipt := &mocks.IPTables{}
	ipt.On("List", "nat", "PREROUTING").Return([]string{
		"-A PREROUTING -j ACCEPT",
		"-P PREROUTING ACCEPT",
		"-A PREROUTING -i eth0 -j DNAT --to-destination 9.9.9.9:80",
	}, nil)
	actual, err := getRules(ipt, "nat", "PREROUTING")
	exp := []string{
		"-j ACCEPT",
		"-i eth0 -j DNAT --to-destination 9.9.9.9:80",
	}
	assert.NoError(t, err)
	assert.Equal(t, exp, actual)

	ipt = &mocks.IPTables{}
	ipt.On("List", "nat", "PREROUTING").Return([]string{
		"-A PREROUTING",
	}, nil)
	_, err = getRules(ipt, "nat", "PREROUTING")
	assert.NotNil(t, err)
}

func TestSyncChain(t *testing.T) {
	ipt := &mocks.IPTables{}
	ipt.On("List", "nat", "PREROUTING").Return([]string{
		"-A PREROUTING -i eth0 -j DNAT --to-destination 7.7.7.7:80",
		"-A PREROUTING -i eth0 -j DNAT --to-destination 8.8.8.8:80",
	}, nil)
	ipt.On("Delete", "nat", "PREROUTING",
		[]string{"-i", "eth0", "-j", "DNAT", "--to-destination", "7.7.7.7:80"},
	).Return(nil)
	ipt.On("Append", "nat", "PREROUTING",
		[]string{"-i", "eth0", "-j", "DNAT", "--to-destination", "9.9.9.9:80"},
	).Return(nil)
	err := syncChain(ipt, "nat", "PREROUTING", []string{
		"-i eth0 -j DNAT --to-destination 8.8.8.8:80",
		"-i eth0 -j DNAT --to-destination 9.9.9.9:80",
	})
	assert.NoError(t, err)
	ipt.AssertExpectations(t)

	anErr := errors.New("err")
	ipt = &mocks.IPTables{}
	ipt.On("List", mock.Anything, mock.Anything).Return(
		[]string{"-A PREROUTING deleteme"}, nil)
	ipt.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(anErr)
	err = syncChain(ipt, "nat", "PREROUTING", []string{})
	assert.NotNil(t, err)

	ipt = &mocks.IPTables{}
	ipt.On("List", mock.Anything, mock.Anything).Return(nil, nil)
	ipt.On("Append", mock.Anything, mock.Anything, mock.Anything).Return(anErr)
	err = syncChain(ipt, "nat", "PREROUTING", []string{"addme"})
	assert.NotNil(t, err)
}

func TestGetPublicInterface(t *testing.T) {
	routeList = func(link netlink.Link, family int) ([]netlink.Route, error) {
		return nil, errors.New("not implemented")
	}
	linkByIndex = func(index int) (netlink.Link, error) {
		if index == 5 {
			link := netlink.GenericLink{}
			link.LinkAttrs.Name = "link name"
			return &link, nil
		}
		return nil, errors.New("unknown")
	}

	res, err := getPublicInterfaceImpl()
	assert.Empty(t, res)
	assert.EqualError(t, err, "route list: not implemented")

	var routes []netlink.Route
	routeList = func(link netlink.Link, family int) ([]netlink.Route, error) {
		return routes, nil
	}
	res, err = getPublicInterfaceImpl()
	assert.Empty(t, res)
	assert.EqualError(t, err, "missing default route")

	routes = []netlink.Route{{Dst: &ipdef.QuiltSubnet}}
	res, err = getPublicInterfaceImpl()
	assert.Empty(t, res)
	assert.EqualError(t, err, "missing default route")

	routes = []netlink.Route{{LinkIndex: 2}}
	res, err = getPublicInterfaceImpl()
	assert.Empty(t, res)
	assert.EqualError(t, err, "default route missing interface: unknown")

	routes = []netlink.Route{{LinkIndex: 5}}
	res, err = getPublicInterfaceImpl()
	assert.Equal(t, "link name", res)
	assert.NoError(t, err)
}
