package buildcmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/gobuffalo/buffalo-cli/cli/plugins"
	"github.com/gobuffalo/buffalo-cli/cli/plugins/plugprint"
	"github.com/gobuffalo/buffalo-cli/internal/v1/genny/build"
	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/logger"
	"github.com/gobuffalo/meta"
	"github.com/spf13/pflag"
)

type BuildCmd struct {
	Parent                 plugins.Plugin
	Plugins                func() plugins.Plugins
	dryRun                 bool
	help                   bool
	skipAssets             bool
	skipTemplateValidation bool
	verbose                bool
	tags                   string
	stdin                  io.Reader
	stdout                 io.Writer
	stderr                 io.Writer
}

func (b *BuildCmd) SetStderr(w io.Writer) {
	b.stderr = w
}

func (b *BuildCmd) SetStdin(r io.Reader) {
	b.stdin = r
}

func (b *BuildCmd) SetStdout(w io.Writer) {
	b.stdout = w
}

func (*BuildCmd) Aliases() []string {
	return []string{"b", "bill", "install"}
}

func (b BuildCmd) Name() string {
	return "build"
}

func (b BuildCmd) String() string {
	s := b.Name()
	if b.Parent != nil {
		s = fmt.Sprintf("%s %s", b.Parent.Name(), b.Name())
	}
	return strings.TrimSpace(s)
}

func (BuildCmd) Description() string {
	return "Build the application binary, including bundling of assets (packr & webpack)"
}

func (bc *BuildCmd) builders() plugins.Plugins {
	var plugs plugins.Plugins
	if bc.Plugins == nil {
		return plugs
	}
	for _, p := range bc.Plugins() {
		switch p.(type) {
		case BeforeBuilder:
			plugs = append(plugs, p)
		case AfterBuilder:
			plugs = append(plugs, p)
		}
	}
	return plugs
}

func (bc *BuildCmd) PrintFlags(w io.Writer) error {
	flags := bc.flagSet(&build.Options{})
	flags.SetOutput(w)
	flags.PrintDefaults()
	return nil
}

func (bc *BuildCmd) flagSet(opts *build.Options) *pflag.FlagSet {
	flags := pflag.NewFlagSet(bc.String(), pflag.ContinueOnError)
	flags.SetOutput(ioutil.Discard)

	flags.BoolVar(&bc.dryRun, "dry-run", false, "runs the build 'dry'")
	flags.BoolVar(&bc.skipTemplateValidation, "skip-template-validation", false, "skip validating templates")
	flags.BoolVarP(&bc.help, "help", "h", false, "print this help")
	flags.BoolVarP(&bc.verbose, "verbose", "v", false, "print debugging information")
	flags.BoolVarP(&opts.Static, "static", "s", false, "build a static binary using  --ldflags '-linkmode external -extldflags \"-static\"'")

	flags.StringVar(&opts.LDFlags, "ldflags", "", "set any ldflags to be passed to the go build")
	flags.StringVar(&opts.Mod, "mod", "", "-mod flag for go build")
	flags.StringVarP(&opts.App.Bin, "output", "o", opts.Bin, "set the name of the binary")
	flags.StringVarP(&opts.Environment, "environment", "", "development", "set the environment for the binary")
	flags.StringVarP(&bc.tags, "tags", "t", "", "compile with specific build tags")

	if bc.Plugins == nil {
		return flags
	}

	plugs := bc.Plugins()

	for _, p := range plugs {
		bf, ok := p.(BuildFlagger)
		if !ok {
			continue
		}
		for _, f := range bf.BuildFlags() {
			flags.AddGoFlag(f)
		}
	}

	for _, p := range plugs {
		bf, ok := p.(BuildPflagger)
		if !ok {
			continue
		}
		for _, f := range bf.BuildPflags() {
			flags.AddFlag(f)
		}
	}

	return flags
}

func (bc *BuildCmd) Main(ctx context.Context, args []string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	opts := &build.Options{
		App: meta.New(pwd),
	}

	flags := bc.flagSet(opts)
	if err = flags.Parse(args); err != nil {
		return err
	}

	if bc.help {
		return plugprint.Print(bc.stdout, bc, nil)
	}

	builders := bc.builders()
	for _, p := range builders {
		if bb, ok := p.(BeforeBuilder); ok {
			if err := bb.BeforeBuild(ctx, args); err != nil {
				return err
			}
		}
	}

	run := genny.WetRunner(ctx)
	if bc.dryRun {
		run = genny.DryRunner(ctx)
	}

	if bc.verbose {
		lg := logger.New(logger.DebugLevel)
		run.Logger = lg
		opts.BuildFlags = append(opts.BuildFlags, "-v")
	}

	if len(bc.tags) > 0 {
		opts.Tags = append(opts.Tags, bc.tags)
	}

	if !bc.skipTemplateValidation {
		opts.TemplateValidators = append(
			opts.TemplateValidators,
			build.PlushValidator,
			build.GoTemplateValidator,
		)
	}
	opts.GoCommand = bc.Name()
	clean := build.Cleanup(opts)
	defer func() {
		if err := clean(run); err != nil {
			log.Fatal("build:clean", err)
		}
	}()

	bd, err := build.New(opts)
	if err != nil {
		return err
	}

	// opts.BuildVersion = cmd.buildVersion(opts)
	// fmt.Println(">>>TODO cli/build.go:106: opts ", opts)

	if err := run.With(bd); err != nil {
		return err
	}
	if err := run.Run(); err != nil {
		return err
	}

	for _, p := range builders {
		if bb, ok := p.(AfterBuilder); ok {
			if err := bb.AfterBuild(ctx, args); err != nil {
				return err
			}
		}
	}
	return nil
}

// func (bc *BuildCmd) buildVersion(opts *build.Options) string {
// 	version := opts.BuildTime.Format(time.RFC3339)
// 	vcs := opts.VCS
//
// 	if len(vcs) == 0 {
// 		return version
// 	}
//
// 	if _, err := exec.LookPath(vcs); err != nil {
// 		return version
// 	}
//
// 	var cmd *exec.Cmd
// 	switch vcs {
// 	case "git":
// 		cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
// 	case "bzr":
// 		cmd = exec.Command("bzr", "revno")
// 	default:
// 		return vcs
// 	}
//
// 	out := &bytes.Buffer{}
// 	cmd.Stdout = out
// 	if err := cmd.Run(); err != nil {
// 		return version
// 	}
//
// 	if out.String() == "" {
// 		return version
// 	}
//
// 	return strings.TrimSpace(out.String())
// }