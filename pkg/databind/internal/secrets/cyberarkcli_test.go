package secrets

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"os"
	"os/exec"
	"testing"
)

// This wraps exec.Command in a new Go Test, this is how exec.Command itself is tested
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	// Tell Go to run a test and which test to run
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	// Flag so the spawned test knows to run
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// This test is not run directly but spawned by fakeExecCommand
func TestHelperProcess(t *testing.T) {
	// Only run if we've been spawned by fakeExecCommand
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		t.Skip("Skipping, this test is not called directly ")
		return
	}
	fmt.Fprintf(os.Stdout, "password")
}

func TestCyberArkCLI(t *testing.T) {
	cyberArkExecCommand = fakeExecCommand
	defer func() { cyberArkExecCommand = exec.Command }()

	cliStruct := CyberArkCLI{
		CLI:    "cli",
		AppID:  "appid",
		Safe:   "safe",
		Folder: "folder",
		Object: "object",
	}

	g := CyberArkCLIGatherer(&cliStruct)
	r, err := g()
	if err != nil {
		t.Errorf("cli call failed: %v ", err)
	}

	unboxed := r.(data.InterfaceMap)

	if unboxed == nil {
		t.Errorf("Result is nil")
	}

	// The passing TestHelpProcess add PASS to the output
	if unboxed["password"] != "passwordPASS" {
		t.Errorf("expected password, got %v", unboxed)
	}
}
