package vagrant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetACLs(t *testing.T) {
	clst := Cluster{}
	assert.Nil(t, clst.SetACLs(nil))
}
