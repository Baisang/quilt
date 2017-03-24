package command

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinionFlags(t *testing.T) {
	t.Parallel()

	expRole := "Worker"

	cmd := NewMinionCommand()
	err := parseHelper(cmd, []string{"-role", expRole})

	assert.NoError(t, err)
	assert.Equal(t, expRole, cmd.role)
}

func TestMinionFailure(t *testing.T) {
	t.Parallel()

	badRole := "Derper"
	cmd := NewMinionCommand()
	roleError := errors.New("no or improper role specified")

	cmd.role = badRole

	assert.Error(t, roleError, cmd.run())
	assert.Equal(t, 1, cmd.Run())

	cmd = NewMinionCommand()
	noRoleError := errors.New("no or improper role specified")

	assert.Error(t, noRoleError, cmd.run())
	assert.Equal(t, 1, cmd.Run())
}
