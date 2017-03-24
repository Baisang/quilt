package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlice(t *testing.T) {
	acl := ACL{"1.2.3.4", 1, 2}
	slice := Slice([]ACL{acl})

	assert.Equal(t, slice.Len(), 1)
	assert.Equal(t, slice.Get(0), acl)
}
