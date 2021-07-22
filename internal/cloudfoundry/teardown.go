package cloudfoundry

import (
	"bytes"
	"fmt"
	"os"

	"github.com/paketo-buildpacks/packit/pexec"
)

type TeardownPhase interface {
	Run(home, name string) error
}

type Teardown struct {
	cli Executable
}

func NewTeardown(cli Executable) Teardown {
	return Teardown{
		cli: cli,
	}
}

func (t Teardown) Run(home, name string) error {
	logs := bytes.NewBuffer(nil)
	env := append(os.Environ(), fmt.Sprintf("CF_HOME=%s", home))

	err := t.cli.Execute(pexec.Execution{
		Args:   []string{"delete-org", name, "-f"},
		Stdout: logs,
		Stderr: logs,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to delete-org: %w\n\nOutput:\n%s", err, logs)
	}

	err = t.cli.Execute(pexec.Execution{
		Args:   []string{"delete-security-group", name, "-f"},
		Stdout: logs,
		Stderr: logs,
		Env:    env,
	})
	if err != nil {
		return fmt.Errorf("failed to delete-security-group: %w\n\nOutput:\n%s", err, logs)
	}

	err = os.RemoveAll(home)
	if err != nil {
		return err
	}

	return nil
}
