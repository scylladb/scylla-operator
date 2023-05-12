package exectest

import (
	"k8s.io/utils/exec"
	testingexec "k8s.io/utils/exec/testing"
)

type Command struct {
	Cmd  string
	Args []string

	Stdout []byte
	Stderr []byte
	Err    error
}

func NewFakeExec(cmds ...Command) *testingexec.FakeExec {
	fe := &testingexec.FakeExec{
		ExactOrder: true,
	}
	for i := range cmds {
		c := cmds[i]
		fe.CommandScript = append(fe.CommandScript, func(_ string, _ ...string) exec.Cmd {
			fakeCmd := &testingexec.FakeCmd{}
			fakeCmd.RunScript = append(fakeCmd.RunScript, func() ([]byte, []byte, error) {
				return c.Stdout, c.Stderr, c.Err
			})
			return testingexec.InitFakeCmd(fakeCmd, c.Cmd, c.Args...)
		})
	}

	return fe
}
