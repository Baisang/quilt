package quiltctl

import (
	"flag"
	"testing"
)

type testCommand struct {
	flagArg string
	posArg  string

	t *testing.T
}

const (
	expFlagArg = "flag"
	expPosArg  = "pos"
)

func (tCmd *testCommand) Parse(args []string) error {
	tCmd.posArg = args[0]
	return nil
}

func (tCmd *testCommand) InstallFlags(flags *flag.FlagSet) {
	flags.StringVar(&tCmd.flagArg, "arg", "", "the test arg")
}

func (tCmd *testCommand) Run() int {
	if tCmd.flagArg != expFlagArg {
		tCmd.t.Errorf("Bad argument value for testCommand: expected %s, got %s",
			expFlagArg, tCmd.flagArg)
	}
	if tCmd.posArg != expPosArg {
		tCmd.t.Errorf("Bad argument value for testCommand: expected %s, got %s",
			expPosArg, tCmd.posArg)
	}
	return 0
}

func TestArgumentParsing(t *testing.T) {
	t.Parallel()

	subcommand, err := parseSubcommand("test", &testCommand{t: t},
		[]string{"-arg", expFlagArg, expPosArg})
	if err != nil {
		t.Errorf("Unexpected error: %s\n", err.Error())
		return
	}

	subcommand.Run()
}
