package consoletests

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

var Commands []string
var Stdin string

type TestConsole struct {
}

func (s TestConsole) Run(cmd string, opts ...func(*exec.Cmd)) (string, error) {
	c := &exec.Cmd{}
	for _, o := range opts {
		o(c)
	}
	Commands = append(Commands, cmd)
	Commands = append(Commands, c.Args...)
	if c.Stdin != nil {
		b, _ := io.ReadAll(c.Stdin)
		Stdin = string(b)
	}

	return "", nil
}

func Reset() {
	Commands = []string{}
	Stdin = ""
}
func (s TestConsole) Start(cmd *exec.Cmd, opts ...func(*exec.Cmd)) error {
	for _, o := range opts {
		o(cmd)
	}
	Commands = append(Commands, cmd.Args...)
	if cmd.Stdin != nil {
		b, _ := io.ReadAll(cmd.Stdin)
		Stdin = string(b)
	}
	return nil
}

func (s TestConsole) RunTemplate(st []string, template string) error {
	var errs error

	for _, svc := range st {
		out, err := s.Run(fmt.Sprintf(template, svc))
		if err != nil {
			logrus.Error(out)
			logrus.Error(err.Error())
			errs = multierror.Append(errs, err)
			continue
		}
	}
	return errs
}
