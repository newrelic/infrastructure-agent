package secrets

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"os"
	"os/exec"
	"testing"
)

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprintf(os.Stdout, "password")
	os.Exit(0)
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
	fmt.Fprintf(os.Stderr, "TestCyberArkCLI: got %v\n\n", r)

	unboxed := r.(data.InterfaceMap)
	fmt.Fprintf(os.Stderr, "TestCyberArkCLI: unboxed %v\n\n", unboxed)

	if unboxed == nil {
		fmt.Errorf("Result is nil")
	}

	if unboxed["password"] != "password" {
		t.Errorf("expected password, got %v", unboxed)
	}
}

// https://docs.cyberark.com/Product-Doc/OnlineHelp/AAM-CP/Latest/en/Content/CP%20and%20ASCP/CLI-Application-Password-SDK-Errors.htm?tocpath=Developer%7CCredential%20Provider%7CApplication%20Password%20SDK%7CCLI%20Application%20Password%20SDK%7C_____2
func TestCyberArkCLIErrors(t *testing.T) {

}
