package ipdef

import (
	"fmt"
	"math/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToMac(t *testing.T) {
	for i := 0; i < 256; i++ {
		a, b, c, d := rand.Intn(256), rand.Intn(256),
			rand.Intn(256), rand.Intn(256)
		addr := net.IPv4(byte(a), byte(b), byte(c), byte(d))
		exp := fmt.Sprintf("02:00:%02x:%02x:%02x:%02x", a, b, c, d)
		assert.Equal(t, exp, IPStrToMac(addr.String()))
	}
}

func TestIFName(t *testing.T) {
	ifNameSize = 5
	assert.Equal(t, IFName("123456689"), "1234")
	assert.Equal(t, IFName("1234"), "1234")
	assert.Equal(t, IFName("123"), "123")
	assert.Equal(t, IFName("1"), "1")
	assert.Equal(t, IFName(""), "")
}
