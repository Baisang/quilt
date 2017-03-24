package command

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	units "github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/quilt/quilt/api"
	clientMock "github.com/quilt/quilt/api/client/mocks"
	"github.com/quilt/quilt/db"
)

func TestPsFlags(t *testing.T) {
	t.Parallel()

	expHost := "IP"

	cmd := NewPsCommand()
	err := parseHelper(cmd, []string{"-H", expHost})

	assert.NoError(t, err)
	assert.Equal(t, expHost, cmd.common.host)

	cmd = NewPsCommand()
	err = parseHelper(cmd, []string{"-no-trunc"})

	assert.NoError(t, err)
	assert.True(t, cmd.noTruncate)
}

func TestPsErrors(t *testing.T) {
	t.Parallel()

	var cmd *Ps
	var mockGetter *clientMock.Getter
	var mockClient, mockLeaderClient *clientMock.Client

	mockErr := errors.New("error")

	// Error connecting to local client
	mockGetter = new(clientMock.Getter)
	mockGetter.On("Client", mock.Anything).Return(nil, mockErr)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	assert.EqualError(t, cmd.run(), "error connecting to quilt daemon: error")
	mockGetter.AssertExpectations(t)

	// Error querying machines
	mockGetter = new(clientMock.Getter)
	mockClient = &clientMock.Client{MachineErr: mockErr}
	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)
	mockGetter.On("LeaderClient", mock.Anything).Return(nil, mockErr)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	assert.EqualError(t, cmd.run(), "unable to query machines: error")
	mockGetter.AssertExpectations(t)

	// Error connecting to leader
	mockGetter = new(clientMock.Getter)
	mockClient = new(clientMock.Client)
	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)
	mockGetter.On("LeaderClient", mock.Anything).Return(nil, mockErr)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	assert.NoError(t, cmd.run())
	mockGetter.AssertExpectations(t)

	// Error querying containers
	mockGetter = new(clientMock.Getter)
	mockClient = new(clientMock.Client)
	mockLeaderClient = &clientMock.Client{ContainerErr: mockErr}
	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)
	mockGetter.On("LeaderClient", mock.Anything).Return(mockLeaderClient, nil)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	assert.EqualError(t, cmd.run(), "unable to query containers: error")
	mockGetter.AssertExpectations(t)

	// Error querying connections from LeaderClient
	mockGetter = new(clientMock.Getter)
	mockClient = new(clientMock.Client)
	mockLeaderClient = &clientMock.Client{ConnectionErr: mockErr}
	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)
	mockGetter.On("LeaderClient", mock.Anything).Return(mockLeaderClient, nil)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	assert.EqualError(t, cmd.run(), "unable to query connections: error")
	mockGetter.AssertExpectations(t)

	// Error querying containers in queryWorkers(), but fine for Leader.
	mockGetter = new(clientMock.Getter)
	mockClient = &clientMock.Client{
		MachineReturn: []db.Machine{
			{
				PublicIP: "1.2.3.4",
				Role:     db.Worker,
			},
		},
		ContainerErr: mockErr,
	}
	mockLeaderClient = new(clientMock.Client)
	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)
	mockGetter.On("LeaderClient", mock.Anything).Return(mockLeaderClient, nil)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	assert.Equal(t, 0, cmd.Run())
	mockGetter.AssertExpectations(t)
}

func TestPsSuccess(t *testing.T) {
	t.Parallel()

	mockGetter := new(clientMock.Getter)
	mockClient := new(clientMock.Client)
	mockLeaderClient := new(clientMock.Client)

	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)
	mockGetter.On("LeaderClient", mock.Anything).Return(mockLeaderClient, nil)

	cmd := &Ps{false, &commonFlags{}, mockGetter}
	assert.Equal(t, 0, cmd.Run())
	mockGetter.AssertExpectations(t)
}

func TestQueryWorkersSuccess(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{
			StitchID: "1",
		},
	}

	machines := []db.Machine{
		{
			PublicIP: "1.2.3.4",
			Role:     db.Worker,
		},
	}

	mockGetter := new(clientMock.Getter)
	mockClient := &clientMock.Client{
		ContainerReturn: containers,
	}
	mockGetter.On("Client", mock.Anything).Return(mockClient, nil)

	cmd := &Ps{false, &commonFlags{}, mockGetter}
	result := cmd.queryWorkers(machines)
	assert.Equal(t, containers, result)
	mockGetter.AssertExpectations(t)
}

func TestQueryWorkersFailure(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{
			StitchID: "1",
		},
	}

	machines := []db.Machine{
		{
			PublicIP: "1.2.3.4",
			Role:     db.Worker,
		},
		{
			PublicIP: "5.6.7.8",
			Role:     db.Worker,
		},
	}

	mockErr := errors.New("error")

	// Getting Worker Machine Client fails. Still query non-failing machine.
	mockClient := &clientMock.Client{
		ContainerReturn: containers,
	}
	mockGetter := new(clientMock.Getter)
	mockGetter.On("Client", api.RemoteAddress("1.2.3.4")).Return(nil, mockErr)
	mockGetter.On("Client", api.RemoteAddress("5.6.7.8")).Return(mockClient, nil)

	cmd := &Ps{false, &commonFlags{}, mockGetter}
	result := cmd.queryWorkers(machines)
	assert.Equal(t, containers, result)
	mockGetter.AssertExpectations(t)

	// Worker Machine client throws error.
	// Still get container from non-failing machine.
	mockGetter = new(clientMock.Getter)
	failingClient := &clientMock.Client{
		ContainerErr: mockErr,
	}
	mockGetter.On("Client", api.RemoteAddress("1.2.3.4")).Return(failingClient, nil)
	mockGetter.On("Client", api.RemoteAddress("5.6.7.8")).Return(mockClient, nil)

	cmd = &Ps{false, &commonFlags{}, mockGetter}
	result = cmd.queryWorkers(machines)
	assert.Equal(t, containers, result)
	mockGetter.AssertExpectations(t)
}

func TestUpdateContainers(t *testing.T) {
	t.Parallel()

	created := time.Now()

	lContainers := []db.Container{
		{
			StitchID: "1",
		},
	}

	wContainers := []db.Container{
		{
			StitchID: "1",
			Created:  created,
		},
	}

	// Test update a matching container.
	expect := wContainers
	result := updateContainers(lContainers, wContainers)
	assert.Equal(t, expect, result)

	// Test container in leader, not in worker.
	newContainer := db.Container{
		StitchID: "2",
	}
	lContainers = append(lContainers, newContainer)
	expect = append(expect, newContainer)
	result = updateContainers(lContainers, wContainers)
	assert.Equal(t, expect, result)

	// Test if lContainers empty.
	lContainers = []db.Container{}
	expect = wContainers
	result = updateContainers(lContainers, wContainers)
	assert.Equal(t, expect, result)

	// Test if wContainers empty.
	lContainers = wContainers
	wContainers = []db.Container{}
	expect = lContainers
	result = updateContainers(lContainers, wContainers)
	assert.Equal(t, expect, result)

	// Test if both empty.
	lContainers = []db.Container{}
	expect = []db.Container{}
	result = updateContainers(lContainers, wContainers)
	assert.Equal(t, expect, result)

	// Test a deployed Dockerfile.
	lContainers = []db.Container{{StitchID: "1", Image: "image"}}
	wContainers = []db.Container{
		{StitchID: "1", Image: "8.8.8.8/image", Created: created},
	}
	expect = []db.Container{{StitchID: "1", Image: "image", Created: created}}
	result = updateContainers(lContainers, wContainers)
	assert.Equal(t, expect, result)
}

func TestMachineOutput(t *testing.T) {
	t.Parallel()

	machines := []db.Machine{{
		StitchID: "1",
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

	exp := `MACHINE____ROLE______PROVIDER____REGION_______SIZE` +
		`________PUBLIC_IP____STATUS
1__________Master____Amazon______us-west-1____m4.large____8.8.8.8______disconnected
`

	assert.Equal(t, exp, result)
}

func TestContainerOutput(t *testing.T) {
	t.Parallel()

	containers := []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running"},
		{ID: 2, StitchID: "1", Minion: "1.1.1.1", Image: "image2",
			Labels: []string{"label1", "label2"}, Status: "scheduled"},
		{ID: 3, StitchID: "4", Minion: "1.1.1.1", Image: "image3",
			Command: []string{"cmd"},
			Labels:  []string{"label1"},
			Status:  "scheduled"},
		{ID: 4, StitchID: "7", Minion: "2.2.2.2", Image: "image1",
			Command: []string{"cmd", "3", "4"},
			Labels:  []string{"label1"}},
		{ID: 5, StitchID: "8", Image: "image1"},
	}

	machines := []db.Machine{
		{StitchID: "5", PublicIP: "7.7.7.7", PrivateIP: "1.1.1.1"},
		{StitchID: "6", PrivateIP: "2.2.2.2"},
		{StitchID: "7", PrivateIP: ""},
	}

	connections := []db.Connection{
		{ID: 1, From: "public", To: "label1", MinPort: 80, MaxPort: 80},
		{ID: 2, From: "notpublic", To: "label2", MinPort: 100, MaxPort: 101},
	}

	var b bytes.Buffer
	writeContainers(&b, containers, machines, connections, true)
	result := string(b.Bytes())

	/* By replacing space with underscore, we make the spaces explicit and whitespace
	* errors easier to debug. */
	result = strings.Replace(result, " ", "_", -1)

	expected := `CONTAINER____MACHINE____COMMAND___________LABELS________` +
		`____STATUS_______CREATED____PUBLIC_IP
3_______________________image1_cmd_1________________________running_________________
____________________________________________________________________________________
1____________5__________image2____________label1,_label2____scheduled__________` +
		`_____7.7.7.7:80
4____________5__________image3_cmd________label1____________scheduled__________` +
		`_____7.7.7.7:80
____________________________________________________________________________________
7____________6__________image1_cmd_3_4____label1____________scheduled_______________
____________________________________________________________________________________
8____________7__________image1______________________________________________________
`

	assert.Equal(t, expected, result)

	// Testing writeContainers with created time values.
	mockTime := time.Now()
	humanDuration := units.HumanDuration(time.Since(mockTime))
	mockCreatedString := fmt.Sprintf("%s ago", humanDuration)
	mockCreatedString = strings.Replace(mockCreatedString, " ", "_", -1)

	containers = []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running", Created: mockTime.UTC()},
	}

	machines = []db.Machine{}
	connections = []db.Connection{}

	var c bytes.Buffer
	writeContainers(&c, containers, machines, connections, true)
	result = string(c.Bytes())
	expected = `CONTAINER____MACHINE____COMMAND_________LABELS____STATUS___` +
		`__CREATED___________________PUBLIC_IP
3_______________________image1_cmd_1______________running____` + mockCreatedString +
		`____
`

	result = strings.Replace(result, " ", "_", -1)
	assert.Equal(t, expected, result)

	// Testing writeContainers with longer durations.
	mockDuration := time.Hour
	mockTime = time.Now().Add(-mockDuration)
	humanDuration = units.HumanDuration(time.Since(mockTime))
	mockCreatedString = fmt.Sprintf("%s ago", humanDuration)
	mockCreatedString = strings.Replace(mockCreatedString, " ", "_", -1)

	containers = []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1"},
			Status: "running", Created: mockTime.UTC()},
	}

	machines = []db.Machine{}
	connections = []db.Connection{}

	var d bytes.Buffer
	writeContainers(&d, containers, machines, connections, true)
	result = string(d.Bytes())
	expected = `CONTAINER____MACHINE____COMMAND_________LABELS____STATUS___` +
		`__CREATED______________PUBLIC_IP
3_______________________image1_cmd_1______________running____` + mockCreatedString +
		`____
`

	result = strings.Replace(result, " ", "_", -1)
	assert.Equal(t, expected, result)
	containers = []db.Container{
		{ID: 1, StitchID: "3", Minion: "3.3.3.3", IP: "1.2.3.4",
			Image: "image1", Command: []string{"cmd", "1", "&&", "cmd",
				"91283403472903847293014320984723908473248-23843984"},
			Status: "running", Created: mockTime.UTC()},
	}

	machines = []db.Machine{}
	connections = []db.Connection{}

	// Test that long outputs are truncated when `truncate` is true
	var e bytes.Buffer
	writeContainers(&e, containers, machines, connections, true)
	result = string(e.Bytes())
	expected = `CONTAINER____MACHINE____COMMAND___________________________LABELS_` +
		`___STATUS_____CREATED______________PUBLIC_IP
3_______________________image1_cmd_1_&&_cmd_9128340347______________running____` +
		mockCreatedString + `____
`
	result = strings.Replace(result, " ", "_", -1)
	assert.Equal(t, expected, result)

	// Test that long outputs are not truncated when `truncate` is true
	var f bytes.Buffer
	writeContainers(&f, containers, machines, connections, false)
	result = string(f.Bytes())
	expected = `CONTAINER____MACHINE____COMMAND___________________________________` +
		`________________________________LABELS____STATUS_____CREATED_________` +
		`_____PUBLIC_IP
3_______________________image1_cmd_1_&&_cmd_91283403472903847293014320984723908473248` +
		`-23843984______________running____` + mockCreatedString + `____
`
	result = strings.Replace(result, " ", "_", -1)
	assert.Equal(t, expected, result)
}

func TestContainerStr(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", containerStr("", nil, false))
	assert.Equal(t, "", containerStr("", []string{"arg0"}, false))
	assert.Equal(t, "container arg0 arg1",
		containerStr("container", []string{"arg0", "arg1"}, false))
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
