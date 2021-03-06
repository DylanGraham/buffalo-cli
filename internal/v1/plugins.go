package v1

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gobuffalo/buffalo-cli/internal/v1/consts"
	"github.com/gobuffalo/buffalo-cli/internal/v1/plugins"
	"github.com/gobuffalo/events"
	"github.com/markbates/oncer"
	"github.com/markbates/safe"
)

// LoadPlugins will add listeners for any plugins that support "events"
func LoadPlugins() error {
	var err error
	oncer.Do("events.LoadPlugins", func() {
		env := os.Getenv(consts.GO_ENV)
		// don't send plugins events during testing
		if env == "test" {
			return
		}
		plugs, err := plugins.Available()
		if err != nil {
			err = err
			return
		}
		for _, cmds := range plugs {
			for _, c := range cmds {
				if c.BuffaloCommand != "events" {
					continue
				}
				err := func(c plugins.Command) error {
					return safe.RunE(func() error {
						n := fmt.Sprintf("[PLUGIN] %s %s", c.Binary, c.Name)
						fn := func(e events.Event) {
							b, err := json.Marshal(e)
							if err != nil {
								fmt.Println("error trying to marshal event", e, err)
								return
							}
							cmd := exec.Command(c.Binary, c.UseCommand, string(b))
							cmd.Stderr = os.Stderr
							cmd.Stdout = os.Stdout
							cmd.Stdin = os.Stdin
							if err := cmd.Run(); err != nil {
								fmt.Println("error trying to send event", strings.Join(cmd.Args, " "), err)
							}
						}
						_, err := events.NamedListen(n, events.Filter(c.ListenFor, fn))
						if err != nil {
							return err
						}
						return nil
					})
				}(c)
				if err != nil {
					err = err
					return
				}
			}

		}
	})
	return err
}
