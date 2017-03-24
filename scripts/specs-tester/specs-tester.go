package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/quilt/quilt/scripts/specs-tester/tests"
)

func main() {
	defer os.RemoveAll(tests.QuiltPath)

	testsByName := map[string]func() error{
		"TestReadme": tests.TestReadme,
		"TestSpecs":  tests.TestSpecs,
	}

	var names []string
	for name := range testsByName {
		names = append(names, name)
	}
	sort.Strings(names)

	failed := false
	for _, name := range names {
		test := testsByName[name]
		if result := test(); result != nil {
			fmt.Printf("FAILED\t%s: %s\n", name, result.Error())
			failed = true
		} else {
			fmt.Printf("PASSED\t%s\n", name)
		}
	}

	if failed {
		os.Exit(1)
	}
	os.Exit(0)
}
