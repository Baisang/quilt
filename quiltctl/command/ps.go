package command

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	units "github.com/docker/go-units"
	"github.com/quilt/quilt/api"
	"github.com/quilt/quilt/api/client"
	"github.com/quilt/quilt/api/client/getter"
	"github.com/quilt/quilt/db"
	"github.com/quilt/quilt/util"

	log "github.com/Sirupsen/logrus"
)

// An arbitrary length to truncate container commands to.
const truncLength = 30

// Ps contains the options for querying machines and containers.
type Ps struct {
	noTruncate   bool
	common       *commonFlags
	clientGetter client.Getter
}

// NewPsCommand creates a new Ps command instance.
func NewPsCommand() *Ps {
	return &Ps{
		common:       &commonFlags{},
		clientGetter: getter.New(),
	}
}

// InstallFlags sets up parsing for command line flags
func (pCmd *Ps) InstallFlags(flags *flag.FlagSet) {
	pCmd.common.InstallFlags(flags)
	flags.BoolVar(&pCmd.noTruncate, "no-trunc", false, "do not truncate container"+
		" command output")
	flags.Usage = func() {
		fmt.Println("usage: quilt ps [-H=<daemon_host>]")
		fmt.Println("`ps` displays the status of quilt-managed " +
			"machines and containers.")

		flags.PrintDefaults()
	}
}

// Parse parses the command line arguments for the ps command.
func (pCmd *Ps) Parse(args []string) error {
	return nil
}

// Run retrieves and prints all machines and containers.
func (pCmd *Ps) Run() int {
	if err := pCmd.run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return 1
	}
	return 0
}

func (pCmd *Ps) run() error {
	localClient, err := pCmd.clientGetter.Client(pCmd.common.host)
	if err != nil {
		return fmt.Errorf("error connecting to quilt daemon: %s", err)
	}
	defer localClient.Close()

	var connections []db.Connection
	var containers []db.Container
	var machines []db.Machine

	connectionErr := make(chan error)
	containerErr := make(chan error)
	machineErr := make(chan error)

	go func() {
		machines, err = localClient.QueryMachines()
		machineErr <- err
	}()

	leaderClient, leadErr := pCmd.clientGetter.LeaderClient(localClient)
	if leadErr == nil {
		defer leaderClient.Close()

		go func() {
			connections, err = leaderClient.QueryConnections()
			connectionErr <- err
		}()

		go func() {
			containers, err = leaderClient.QueryContainers()
			containerErr <- err
		}()
	}

	if err := <-machineErr; err != nil {
		return fmt.Errorf("unable to query machines: %s", err)
	}

	writeMachines(os.Stdout, machines)
	fmt.Println()

	if leadErr != nil {
		log.WithError(leadErr).Debug("unable to connect to a cluster leader")
		return nil
	}
	if err := <-connectionErr; err != nil {
		return fmt.Errorf("unable to query connections: %s", err)
	}
	if err := <-containerErr; err != nil {
		return fmt.Errorf("unable to query containers: %s", err)
	}

	workerContainers := pCmd.queryWorkers(machines)
	containers = updateContainers(containers, workerContainers)

	writeContainers(os.Stdout, containers, machines, connections, !pCmd.noTruncate)

	return nil
}

func writeMachines(fd io.Writer, machines []db.Machine) {
	w := tabwriter.NewWriter(fd, 0, 0, 4, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "MACHINE\tROLE\tPROVIDER\tREGION\tSIZE\tPUBLIC IP\tSTATUS")

	for _, m := range db.SortMachines(machines) {
		status := "disconnected"
		if m.Connected {
			status = "connected"
		}

		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			util.ShortUUID(m.StitchID), m.Role, m.Provider, m.Region, m.Size,
			m.PublicIP, status)
	}
}

// queryWorkers gets a client for all connected worker machines (have a PublicIP
// and are role Worker) and returns a list of db.Container on these machines.
// If there is an error querying any machine, we skip it and attempt to return
// as many containers as possible.
func (pCmd *Ps) queryWorkers(machines []db.Machine) []db.Container {
	var workerMachines []db.Machine
	for _, m := range machines {
		if m.PublicIP != "" && m.Role == db.Worker {
			workerMachines = append(workerMachines, m)
		}
	}

	var workerContainers []db.Container
	numMachines := len(workerMachines)
	workerChannel := make(chan []db.Container, numMachines)
	for _, m := range workerMachines {
		client, err := pCmd.clientGetter.Client(api.RemoteAddress(m.PublicIP))
		if err != nil {
			numMachines = numMachines - 1
			continue
		}
		defer client.Close()

		go func() {
			qContainers, err := client.QueryContainers()
			if err != nil {
				log.WithError(err).
					Warn("QueryContainers on worker failed.")
			}
			workerChannel <- qContainers
		}()
	}

	for j := 0; j < numMachines; j++ {
		wc := <-workerChannel
		if wc == nil {
			continue
		}
		workerContainers = append(workerContainers, wc...)
	}
	return workerContainers
}

// updateContainers returns a list of containers from the hash join
// of the leader's view of containers and the workers' views of containers.
func updateContainers(lContainers []db.Container,
	wContainers []db.Container) []db.Container {

	if len(lContainers) == 0 {
		return wContainers
	}
	if len(wContainers) == 0 {
		return lContainers
	}

	var allContainers []db.Container

	// Map StitchID to db.Container for a hash join.
	cMap := make(map[string]db.Container)
	for _, wc := range wContainers {
		cMap[wc.StitchID] = wc
	}
	// If we see a leader container matching a worker container in the map, then
	// we use the container already in the map (worker container is fresher).
	// Otherwise, we add the leader container to our list.
	for _, lc := range lContainers {
		wc, ok := cMap[lc.StitchID]
		if ok {
			// Always use the leader's view of the image name because the name
			// is modified on workers for images that are built in the
			// cluster.
			wc.Image = lc.Image
			allContainers = append(allContainers, wc)
		} else {
			allContainers = append(allContainers, lc)
		}
	}
	return allContainers
}

func writeContainers(fd io.Writer, containers []db.Container, machines []db.Machine,
	connections []db.Connection, truncate bool) {
	w := tabwriter.NewWriter(fd, 0, 0, 4, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "CONTAINER\tMACHINE\tCOMMAND\tLABELS"+
		"\tSTATUS\tCREATED\tPUBLIC IP")

	labelPublicPorts := map[string]string{}
	for _, c := range connections {
		if c.From != "public" {
			continue
		}

		labelPublicPorts[c.To] = fmt.Sprintf("%d", c.MinPort)
		if c.MinPort != c.MaxPort {
			labelPublicPorts[c.To] += fmt.Sprintf("-%d", c.MaxPort)
		}
	}

	ipIDMap := map[string]string{}
	idMachineMap := map[string]db.Machine{}
	for _, m := range machines {
		ipIDMap[m.PrivateIP] = m.StitchID
		idMachineMap[m.StitchID] = m
	}

	machineDBC := map[string][]db.Container{}
	for _, dbc := range containers {
		id := ipIDMap[dbc.Minion]
		machineDBC[id] = append(machineDBC[id], dbc)
	}

	var machineIDs []string
	for key := range machineDBC {
		machineIDs = append(machineIDs, key)
	}
	sort.Strings(machineIDs)

	for i, machineID := range machineIDs {
		if i > 0 {
			// Insert a blank line between each machine.
			// Need to print tabs in a blank line; otherwise, spacing will
			// change in subsequent lines.
			fmt.Fprintf(w, "\t\t\t\t\t\t\n")
		}

		dbcs := machineDBC[machineID]
		sort.Sort(db.ContainerSlice(dbcs))
		for _, dbc := range dbcs {
			publicPorts := []string{}
			for _, label := range dbc.Labels {
				if p, ok := labelPublicPorts[label]; ok {
					publicPorts = append(publicPorts, p)
				}
			}

			container := containerStr(dbc.Image, dbc.Command, truncate)
			labels := strings.Join(dbc.Labels, ", ")
			status := dbc.Status
			if dbc.Status == "" && dbc.Minion != "" {
				status = "scheduled"
			}

			created := ""
			if !dbc.Created.IsZero() {
				createdTime := dbc.Created.Local()
				duration := units.HumanDuration(time.Since(createdTime))
				created = fmt.Sprintf("%s ago", duration)
			}

			publicIP := publicIPStr(idMachineMap[machineID].PublicIP,
				publicPorts)

			fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				util.ShortUUID(dbc.StitchID), util.ShortUUID(machineID),
				container, labels, status, created, publicIP)
		}
	}
}

func containerStr(image string, args []string, truncate bool) string {
	if image == "" {
		return ""
	}

	container := fmt.Sprintf("%s %s", image, strings.Join(args, " "))
	if truncate && len(container) > truncLength {
		return container[:truncLength]
	}

	return container
}

func publicIPStr(hostPublicIP string, publicPorts []string) string {
	if hostPublicIP == "" || len(publicPorts) == 0 {
		return ""
	}

	if len(publicPorts) == 1 {
		return fmt.Sprintf("%s:%s", hostPublicIP, publicPorts[0])
	}

	return fmt.Sprintf("%s:[%s]", hostPublicIP, strings.Join(publicPorts, ","))
}
