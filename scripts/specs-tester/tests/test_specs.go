package tests

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/quilt/quilt/stitch"

	"github.com/robertkrimen/otto"
)

// QuiltPath is the QUILT_PATH used for testing.
var QuiltPath = "/tmp/.quilt"

func configRunOnce(specPath string) error {
	getter := stitch.NewImportGetter(QuiltPath)
	getter.AutoDownload = true

	if isRemote(specPath) {
		getter.Get(specPath)
		specPath = filepath.Join(QuiltPath, specPath)
	}

	_, err := stitch.FromFile(specPath, getter)
	return err
}

func testSpec(specPath string) error {
	if err := configRunOnce(specPath); err != nil {
		errString := err.Error()
		// Print the stacktrace if it's an Otto error.
		if ottoError, ok := err.(*otto.Error); ok {
			errString = ottoError.String()
		}
		return fmt.Errorf("%s failed validation: %s \n quiltPath: %s",
			specPath, errString, QuiltPath)
	}
	return nil
}

// TestSpecs checks that the listed Quilt specs compile.
func TestSpecs() error {
	specs := []string{
		"./quilt-tester/tests/100-logs/logs.js",
		"./quilt-tester/tests/61-duplicate-cluster/duplicate-cluster.js",
		"./quilt-tester/tests/60-duplicate-cluster-setup/" +
			"duplicate-cluster-setup.js",
		"./quilt-tester/tests/40-stop/stop.js",
		"./quilt-tester/tests/30-mean/mean.js",
		"./quilt-tester/tests/20-spark/spark.js",
		"./quilt-tester/tests/15-bandwidth/bandwidth.js",
		"./quilt-tester/tests/10-network/network.js",
		"./quilt-tester/tests/outbound-public/outbound-public.js",
		"./quilt-tester/tests/inbound-public/inbound-public.js",
		"./quilt-tester/tests/elasticsearch/elasticsearch.js",
		"./quilt-tester/tests/build-dockerfile/build-dockerfile.js",
		"./quilt-tester/tests/etcd/etcd.js",

		"github.com/quilt/nginx/main.js",
		"github.com/quilt/spark/sparkPI.js",
		"github.com/quilt/wordpress/wordpress-example.js",
		"github.com/quilt/etcd/etcd-example.js",
		"github.com/quilt/zookeeper/zookeeper-example.js",
		"github.com/quilt/redis/redis-example.js",
		"github.com/quilt/mean/example.js",
		"github.com/quilt/elasticsearch/main.js",
		"github.com/quilt/kibana/main.js",
		"github.com/quilt/django/django-example.js",
		"github.com/quilt/php-apache/main.js",
		"github.com/quilt/mongo/example.js",
	}

	for _, specPath := range specs {
		err := testSpec(specPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func isRemote(path string) bool {
	return strings.HasPrefix(path, "github.com")
}
