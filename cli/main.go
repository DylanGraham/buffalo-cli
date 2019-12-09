package cli

import (
	"context"

	"github.com/gobuffalo/buffalo-cli/cli/plugins"
	"github.com/gobuffalo/buffalo-cli/cli/plugins/plugprint"
	"github.com/gobuffalo/buffalo-cli/internal/v1/cmd"
	"github.com/spf13/pflag"
)

func (b *Buffalo) Main(ctx context.Context, args []string) error {
	var help bool
	flags := pflag.NewFlagSet(b.String(), pflag.ContinueOnError)
	flags.BoolVarP(&help, "help", "h", false, "print this help")
	flags.Parse(args)

	var cmds plugins.Commands
	for _, p := range b.Plugins {
		if c, ok := p.(plugins.Command); ok {
			cmds = append(cmds, c)
		}
	}

	if len(args) == 0 || (len(flags.Args()) == 0 && help) {
		plugs := make(plugins.Plugins, len(cmds))
		for i, c := range cmds {
			plugs[i] = c
		}

		return plugprint.Print(b.Stdout(), b, plugs)
	}
	if c, err := cmds.Find(args[0]); err == nil {
		b.setIO(c)
		return c.Main(ctx, args[1:])
	}

	c := cmd.RootCmd
	c.SetArgs(args)
	return c.Execute()
}

func (b *Buffalo) setIO(p plugins.Plugin) {
	if stdin, ok := p.(plugins.StdinSetter); ok {
		stdin.SetStdin(b.Stdin())
	}
	if stdout, ok := p.(plugins.StdoutSetter); ok {
		stdout.SetStdout(b.Stdout())
	}
	if stderr, ok := p.(plugins.StderrSetter); ok {
		stderr.SetStderr(b.Stderr())
	}
}