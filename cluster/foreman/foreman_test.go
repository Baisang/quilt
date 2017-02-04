package foreman

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quilt/quilt/cluster/machine"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/minion/pb"
)

type clients struct {
	clients  map[string]*fakeClient
	newCalls int
}

func TestBoot(t *testing.T) {
	conn, clients := startTest()
	RunOnce(conn)

	assert.Zero(t, clients.newCalls)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMachine()
		m.PublicIP = "1.1.1.1"
		m.PrivateIP = "1.1.1.1."
		m.CloudID = "ID"
		view.Commit(m)
		return nil
	})

	RunOnce(conn)
	assert.Equal(t, 1, clients.newCalls)
	_, ok := clients.clients["1.1.1.1"]
	assert.True(t, ok)

	RunOnce(conn)
	assert.Equal(t, 1, clients.newCalls)
	_, ok = clients.clients["1.1.1.1"]
	assert.True(t, ok)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMachine()
		m.PublicIP = "2.2.2.2"
		m.PrivateIP = "2.2.2.2"
		m.CloudID = "ID2"
		view.Commit(m)
		return nil
	})

	RunOnce(conn)
	assert.Equal(t, 2, clients.newCalls)

	_, ok = clients.clients["2.2.2.2"]
	assert.True(t, ok)

	_, ok = clients.clients["1.1.1.1"]
	assert.True(t, ok)

	RunOnce(conn)
	RunOnce(conn)
	RunOnce(conn)
	RunOnce(conn)
	assert.Equal(t, 2, clients.newCalls)

	_, ok = clients.clients["2.2.2.2"]
	assert.True(t, ok)

	_, ok = clients.clients["1.1.1.1"]
	assert.True(t, ok)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		machines := view.SelectFromMachine(func(m db.Machine) bool {
			return m.PublicIP == "1.1.1.1"
		})
		view.Remove(machines[0])
		return nil
	})

	RunOnce(conn)
	assert.Equal(t, 2, clients.newCalls)

	_, ok = clients.clients["2.2.2.2"]
	assert.True(t, ok)

	_, ok = clients.clients["1.1.1.1"]
	assert.False(t, ok)

	RunOnce(conn)
	RunOnce(conn)
	RunOnce(conn)
	RunOnce(conn)
	assert.Equal(t, 2, clients.newCalls)

	_, ok = clients.clients["2.2.2.2"]
	assert.True(t, ok)

	_, ok = clients.clients["1.1.1.1"]
	assert.False(t, ok)
}

func TestBootEtcd(t *testing.T) {
	conn, clients := startTest()
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMachine()
		m.Role = db.Master
		m.PublicIP = "m1-pub"
		m.PrivateIP = "m1-priv"
		m.CloudID = "ignored"
		view.Commit(m)

		m = view.InsertMachine()
		m.Role = db.Worker
		m.PublicIP = "w1-pub"
		m.PrivateIP = "w1-priv"
		m.CloudID = "ignored"
		view.Commit(m)
		return nil
	})
	RunOnce(conn)
	assert.Equal(t, []string{"m1-priv"}, clients.clients["w1-pub"].mc.EtcdMembers)

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMachine()
		m.Role = db.Master
		m.PublicIP = "m2-pub"
		m.PrivateIP = "m2-priv"
		m.CloudID = "ignored"
		view.Commit(m)
		return nil
	})
	RunOnce(conn)
	etcdMembers := clients.clients["w1-pub"].mc.EtcdMembers
	assert.Len(t, etcdMembers, 2)
	assert.Contains(t, etcdMembers, "m1-priv")
	assert.Contains(t, etcdMembers, "m2-priv")

	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		var toDelete = view.SelectFromMachine(func(m db.Machine) bool {
			return m.PrivateIP == "m1-priv"
		})[0]
		view.Remove(toDelete)
		return nil
	})
	RunOnce(conn)
	assert.Equal(t, []string{"m2-priv"},
		clients.clients["w1-pub"].mc.EtcdMembers)
}

func TestGetMachineRoles(t *testing.T) {
	machines := []machine.Machine{
		{
			PublicIP: "1.1.1.1",
		},
		{
			PublicIP: "2.2.2.2",
		},
	}
	workerMinion := minion{
		config: pb.MinionConfig{
			Role: pb.MinionConfig_WORKER,
		},
	}
	minions = map[string]*minion{
		"1.1.1.1": &workerMinion,
	}

	updatedMachines := GetMachineRoles(machines)
	assert.Equal(t, db.Role(db.Worker), updatedMachines[0].Role)
	assert.Equal(t, db.Role(db.None), updatedMachines[1].Role)

	minions = map[string]*minion{}
}

func TestInitForeman(t *testing.T) {
	conn := startTestWithRole(pb.MinionConfig_WORKER)
	conn.Txn(db.AllTables...).Run(func(view db.Database) error {
		m := view.InsertMachine()
		m.PublicIP = "2.2.2.2"
		m.PrivateIP = "2.2.2.2"
		m.CloudID = "ID2"
		view.Commit(m)
		return nil
	})

	Init(conn)
	for _, m := range minions {
		assert.Equal(t, db.Role(db.Worker), m.machine.Role)
	}

	conn = startTestWithRole(pb.MinionConfig_Role(-7))
	Init(conn)
	for _, m := range minions {
		assert.Equal(t, db.None, m.machine.Role)
	}
}

func startTest() (db.Conn, *clients) {
	conn := db.New()
	minions = map[string]*minion{}
	clients := &clients{make(map[string]*fakeClient), 0}
	newClient = func(ip string) (client, error) {
		if fc, ok := clients.clients[ip]; ok {
			return fc, nil
		}
		fc := &fakeClient{clients, ip, pb.MinionConfig{}}
		clients.clients[ip] = fc
		clients.newCalls++
		return fc, nil
	}
	return conn, clients
}

func startTestWithRole(role pb.MinionConfig_Role) db.Conn {
	clientInst := &clients{make(map[string]*fakeClient), 0}
	newClient = func(ip string) (client, error) {
		fc := &fakeClient{clientInst, ip, pb.MinionConfig{Role: role}}
		clientInst.clients[ip] = fc
		clientInst.newCalls++
		return fc, nil
	}
	return db.New()
}

type fakeClient struct {
	clients *clients
	ip      string
	mc      pb.MinionConfig
}

func (fc *fakeClient) setMinion(mc pb.MinionConfig) error {
	fc.mc = mc
	return nil
}

func (fc *fakeClient) getMinion() (pb.MinionConfig, error) {
	return fc.mc, nil
}

func (fc *fakeClient) Close() {
	delete(fc.clients.clients, fc.ip)
}
