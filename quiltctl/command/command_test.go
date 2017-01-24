package command

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	units "github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/NetSys/quilt/db"
)

func TestMachineFlags(t *testing.T) {
	t.Parallel()

	expHost := "IP"

	machineCmd := NewMachineCommand()
	err := parseHelper(machineCmd, []string{"-H", expHost})

	assert.NoError(t, err)
	assert.Equal(t, expHost, machineCmd.common.host)
}

func TestMachineOutput(t *testing.T) {
	t.Parallel()

	machines := []db.Machine{{
		ID:       1,
		Role:     db.Master,
		Provider: "Amazon",
		Region:   "us-west-1",
		Size:     "m4.large",
		PublicIP: "8.8.8.8",
	}}

	var b bytes.Buffer
	writeMachines(&b, machines)
	result := string(b.Bytes())

	/* By replacing space with underscore, we make the spaces explicit and whitespace
	* errors easier to debug. */
	result = strings.Replace(result, " ", "_", -1)

	exp := `ID____ROLE______PROVIDER____REGION_______SIZE` +
		`________PUBLIC_IP____CONNECTED
1_____Master____Amazon______us-west-1____m4.large____8.8.8.8______false
`

	assert.Equal(t, exp, result)
}

func TestContainerFlags(t *testing.T) {
	t.Parallel()

	expHost := "IP"

	containerCmd := NewContainerCommand()
	err := parseHelper(containerCmd, []string{"-H", expHost})

	assert.NoError(t, err)
	assert.Equal(t, expHost, containerCmd.common.host)
}

func TestContainerOutput(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{ID: 1, StitchID: 3, Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running"},
		{ID: 2, StitchID: 1, Minion: "1.1.1.1", Image: "image2",
			Labels: []string{"label1", "label2"}, Status: "scheduled"},
		{ID: 3, StitchID: 4, Minion: "1.1.1.1", Image: "image3",
			Command: []string{"cmd"},
			Labels:  []string{"label1"},
			Status:  "scheduled"},
		{ID: 4, StitchID: 7, Minion: "2.2.2.2", Image: "image1",
			Command: []string{"cmd", "3", "4"},
			Labels:  []string{"label1"}},
		{ID: 5, StitchID: 8, Image: "image1"},
	}

	machines := []db.Machine{
		{ID: 5, PublicIP: "7.7.7.7", PrivateIP: "1.1.1.1"},
		{ID: 6, PrivateIP: "2.2.2.2"},
		{ID: 7, PrivateIP: ""},
	}

	connections := []db.Connection{
		{ID: 1, From: "public", To: "label1", MinPort: 80, MaxPort: 80},
		{ID: 2, From: "notpublic", To: "label2", MinPort: 100, MaxPort: 101},
	}

	var b bytes.Buffer
	writeContainers(&b, containers, machines, connections)
	result := string(b.Bytes())

	/* By replacing space with underscore, we make the spaces explicit and whitespace
	* errors easier to debug. */
	result = strings.Replace(result, " ", "_", -1)
	expected := `ID____MACHINE______CONTAINER_________LABELS` +
		`____________STATUS_______CREATED____PUBLIC_IP
3__________________image1_cmd_1________________________running_________________
_______________________________________________________________________________
1_____Machine-5____image2____________label1,_label2____scheduled_______________7.7.7.7:80
4_____Machine-5____image3_cmd________label1____________scheduled_______________7.7.7.7:80
_______________________________________________________________________________
7_____Machine-6____image1_cmd_3_4____label1____________scheduled_______________
_______________________________________________________________________________
8_____Machine-7____image1______________________________________________________
`

	assert.Equal(t, expected, result)

	// Testing writeContainers with created time values.
	mockTime := time.Now()
	humanDuration := units.HumanDuration(time.Since(mockTime))
	mockCreatedString := fmt.Sprintf("%s ago", humanDuration)
	mockCreatedString = strings.Replace(mockCreatedString, " ", "_", -1)

	containers = []db.Container{
		{ID: 1, StitchID: 3, Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running", Created: mockTime.UTC()},
	}

	machines = []db.Machine{}
	connections = []db.Connection{}

	var c bytes.Buffer
	writeContainers(&c, containers, machines, connections)
	result = string(c.Bytes())
	expected = "ID____MACHINE____CONTAINER_______LABELS" +
		"____STATUS_____CREATED___________________PUBLIC_IP\n" +
		"3________________image1_cmd_1______________running____" +
		mockCreatedString + "____\n"

	result = strings.Replace(result, " ", "_", -1)
	assert.Equal(t, expected, result)

	// Testing writeContainers with longer durations.
	mockDuration := time.Hour
	mockTime = time.Now().Add(-mockDuration)
	humanDuration = units.HumanDuration(time.Since(mockTime))
	mockCreatedString = fmt.Sprintf("%s ago", humanDuration)
	mockCreatedString = strings.Replace(mockCreatedString, " ", "_", -1)

	containers = []db.Container{
		{ID: 1, StitchID: 3, Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running", Created: mockTime.UTC()},
	}

	machines = []db.Machine{}
	connections = []db.Connection{}

	var d bytes.Buffer
	writeContainers(&d, containers, machines, connections)
	result = string(d.Bytes())
	expected = "ID____MACHINE____CONTAINER_______LABELS" +
		"____STATUS_____CREATED______________PUBLIC_IP\n" +
		"3________________image1_cmd_1______________running____" +
		mockCreatedString + "____\n"

	result = strings.Replace(result, " ", "_", -1)
	assert.Equal(t, expected, result)
}

func TestMachineStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", machineStr(0))
	assert.Equal(t, "Machine-10", machineStr(10))
}

func TestContainerStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", containerStr("", nil))
	assert.Equal(t, "", containerStr("", []string{"arg0"}))
	assert.Equal(t, "container arg0 arg1",
		containerStr("container", []string{"arg0", "arg1"}))
}

func TestPublicIPStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", publicIPStr("", nil))
	assert.Equal(t, "", publicIPStr("", []string{"80-88"}))
	assert.Equal(t, "", publicIPStr("1.2.3.4", nil))
	assert.Equal(t, "1.2.3.4:80-88", publicIPStr("1.2.3.4", []string{"80-88"}))
	assert.Equal(t, "1.2.3.4:[70,80-88]",
		publicIPStr("1.2.3.4", []string{"70", "80-88"}))
}

func checkGetParsing(t *testing.T, args []string, expImport string, expErr error) {
	getCmd := &Get{}
	err := parseHelper(getCmd, args)

	assert.Equal(t, expErr, err)
	assert.Equal(t, expImport, getCmd.importPath)
}

func TestGetFlags(t *testing.T) {
	t.Parallel()

	expImport := "spec"
	checkGetParsing(t, []string{"-import", expImport}, expImport, nil)
	checkGetParsing(t, []string{expImport}, expImport, nil)
	checkGetParsing(t, []string{}, "", errors.New("no import specified"))
}

func checkSSHParsing(t *testing.T, args []string, expMachine int,
	expSSHArgs []string, expErr error) {

	sshCmd := NewSSHCommand()
	err := parseHelper(sshCmd, args)

	assert.Equal(t, expErr, err)
	assert.Equal(t, expMachine, sshCmd.targetMachine)
	assert.Equal(t, expSSHArgs, sshCmd.sshArgs)
}

func TestSSHFlags(t *testing.T) {
	t.Parallel()

	checkSSHParsing(t, []string{"1"}, 1, []string{}, nil)
	sshArgs := []string{"-i", "~/.ssh/key"}
	checkSSHParsing(t, append([]string{"1"}, sshArgs...), 1, sshArgs, nil)
	checkSSHParsing(t, []string{}, 0, nil,
		errors.New("must specify a target machine"))
}

func TestSSHCommandCreation(t *testing.T) {
	t.Parallel()

	exp := []string{"ssh", "quilt@host", "-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null", "-i", "~/.ssh/quilt"}
	res := runSSHCommand("host", []string{"-i", "~/.ssh/quilt"})

	assert.Equal(t, exp, res.Args)
}

func parseHelper(cmd SubCommand, args []string) error {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	cmd.InstallFlags(flags)
	flags.Parse(args)
	return cmd.Parse(flags.Args())
}
