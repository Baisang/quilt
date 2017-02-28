package network

import (
	"errors"
	"fmt"
	"strings"

	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/join"
	"github.com/quilt/quilt/stitch"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

// IPTables is an interface to *iptables.IPTables.
type IPTables interface {
	Append(string, string, ...string) error
	AppendUnique(string, string, ...string) error
	Delete(string, string, ...string) error
	List(string, string) ([]string, error)
}

func runNat(conn db.Conn) {
	tables := []db.TableType{db.ContainerTable, db.ConnectionTable, db.MinionTable}
	for range conn.TriggerTick(30, tables...).C {
		minion, err := conn.MinionSelf()
		if err != nil || !minion.SupervisorInit || minion.Role != db.Worker {
			continue
		}

		connections := conn.SelectFromConnection(nil)
		containers := conn.SelectFromContainer(func(c db.Container) bool {
			return c.IP != ""
		})

		ipt, err := iptables.New()
		if err != nil {
			log.WithError(err).Error("Failed to get iptables handle")
			continue
		}

		if err := updateNAT(ipt, containers, connections); err != nil {
			log.WithError(err).Error("Failed to update NAT rules")
		}
	}
}

// updateNAT sets up iptables rules of two categories:
// "default rules" are general rules that must be in place for the PREROUTING
// rules to work. When syncing "default rules" we don't remove any other rules
// that may be in place.
// The other type of rules are those in the PREROUTING chain of the nat table.
// They are responsible for routing traffic to specific containers. They
// overwrite any pre-existing or outdated rules.
func updateNAT(ipt IPTables, containers []db.Container,
	connections []db.Connection) error {

	publicInterface, err := getPublicInterface()
	if err != nil {
		return fmt.Errorf("get public interface: %s", err)
	}

	if err := setDefaultRules(ipt, publicInterface); err != nil {
		return err
	}

	target := routingRules(publicInterface, containers, connections)
	return syncChain(ipt, "nat", "PREROUTING", target)
}

func syncChain(ipt IPTables, table, chain string, target []string) error {
	curr, err := getRules(ipt, table, chain)
	if err != nil {
		return fmt.Errorf("iptables get: %s", err.Error())
	}

	_, rulesToDel, rulesToAdd := join.HashJoin(
		join.StringSlice(curr), join.StringSlice(target), nil, nil)

	for _, r := range rulesToDel {
		ruleSpec := strings.Split(r.(string), " ")
		if err := ipt.Delete(table, chain, ruleSpec...); err != nil {
			return fmt.Errorf("iptables delete: %s", err)
		}
	}

	for _, r := range rulesToAdd {
		ruleSpec := strings.Split(r.(string), " ")
		if err := ipt.Append(table, chain, ruleSpec...); err != nil {
			return fmt.Errorf("iptables append: %s", err)
		}
	}

	return nil
}

func getRules(ipt IPTables, table, chain string) (rules []string, err error) {
	rawRules, err := ipt.List(table, chain)
	if err != nil {
		return nil, err
	}

	for _, r := range rawRules {
		if !strings.HasPrefix(r, "-A") {
			continue
		}

		rSplit := strings.SplitN(r, " ", 3)
		if len(rSplit) != 3 {
			return nil, fmt.Errorf("malformed rule: %s", r)
		}

		rules = append(rules, rSplit[2])
	}

	return rules, nil
}

func routingRules(publicInterface string, containers []db.Container,
	connections []db.Connection) (strRules []string) {

	protocols := []string{"tcp", "udp"}
	// Map each container IP to all ports on which it can receive packets
	// from the public internet.
	portsFromWeb := make(map[string]map[int]struct{})

	for _, dbc := range containers {
		for _, conn := range connections {

			if conn.From != stitch.PublicInternetLabel {
				continue
			}

			for _, l := range dbc.Labels {

				if conn.To != l {
					continue
				}

				if _, ok := portsFromWeb[dbc.IP]; !ok {
					portsFromWeb[dbc.IP] = make(map[int]struct{})
				}

				portsFromWeb[dbc.IP][conn.MinPort] = struct{}{}
			}
		}
	}

	// Map the container's port to the same port of the host.
	for ip, ports := range portsFromWeb {
		for port := range ports {
			for _, protocol := range protocols {
				strRules = append(strRules, fmt.Sprintf(
					"-i %[1]s -p %[2]s -m %[2]s "+
						"--dport %[3]d -j DNAT "+
						"--to-destination %[4]s:%[3]d",
					publicInterface, protocol, port, ip))
			}
		}
	}

	return strRules
}

type rule struct {
	table    string
	chain    string
	ruleSpec []string
}

func setDefaultRules(ipt IPTables, publicInterface string) error {
	rules := []rule{
		{
			table:    "nat",
			chain:    "INPUT",
			ruleSpec: []string{"-j", "ACCEPT"},
		},
		{
			table:    "nat",
			chain:    "OUTPUT",
			ruleSpec: []string{"-j", "ACCEPT"},
		},
		{
			table: "nat",
			chain: "POSTROUTING",
			ruleSpec: []string{"-s", "10.0.0.0/8", "-o", publicInterface,
				"-j", "MASQUERADE"},
		},
	}
	for _, r := range rules {
		if err := ipt.AppendUnique(r.table, r.chain, r.ruleSpec...); err != nil {
			return fmt.Errorf("iptables append: %s", err)
		}
	}
	return nil
}

// getPublicInterfaceImpl gets the interface with the default route.
func getPublicInterfaceImpl() (string, error) {
	routes, err := routeList(nil, 0)
	if err != nil {
		return "", fmt.Errorf("route list: %s", err)
	}

	var defaultRoute *netlink.Route
	for _, r := range routes {
		if r.Dst == nil {
			defaultRoute = &r
			break
		}
	}

	if defaultRoute == nil {
		return "", errors.New("missing default route")
	}

	link, err := linkByIndex(defaultRoute.LinkIndex)
	if err != nil {
		return "", fmt.Errorf("default route missing interface: %s", err)
	}

	return link.Attrs().Name, err
}

var routeList = netlink.RouteList
var linkByIndex = netlink.LinkByIndex
var getPublicInterface = getPublicInterfaceImpl
