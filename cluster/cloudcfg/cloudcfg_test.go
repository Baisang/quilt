package cloudcfg

import (
	"testing"

	"github.com/quilt/quilt/db"
)

func TestCloudConfig(t *testing.T) {
	cfgTemplate = "({{.QuiltImage}}) ({{.SSHKeys}}) ({{.UbuntuVersion}}) " +
		"({{.Role}})"

	res := Ubuntu([]string{"a", "b"}, "1", db.Master)
	exp := "(quilt/quilt:latest) (a\nb) (1) (Master)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}

	res = Ubuntu([]string{"a", "b"}, "1", db.Worker)
	exp = "(quilt/quilt:latest) (a\nb) (1) (Worker)"
	if res != exp {
		t.Errorf("res: %s\nexp: %s", res, exp)
	}

}
