package service

import (
	"flag"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func runAllocationTestSubprocess(t *testing.T) bool {
	t.Helper()
	const childEnv = "SUB2API_ALLOCATION_TEST_CHILD"
	if os.Getenv(childEnv) == t.Name() {
		return false
	}
	// AllocsPerRun counts process-wide allocations, including background workers.
	cmd := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$",
		"-test.count=1", "-test.timeout=30s")
	if coverDir := flag.Lookup("test.gocoverdir"); coverDir != nil && coverDir.Value.String() != "" {
		cmd.Args = append(cmd.Args, "-test.gocoverdir="+coverDir.Value.String())
	}
	cmd.Env = append(os.Environ(), childEnv+"="+t.Name())
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "allocation subprocess failed:\n%s", output)
	return true
}
