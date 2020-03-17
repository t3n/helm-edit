package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
	"helm.sh/helm/v3/cmd/helm/require"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
)

var (
	settings = cli.New()
)

func NewEditCmd(out io.Writer, args []string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "edit [RELEASE]",
		Short:        "Edit user specified values of a release",
		Long:         "Edit user specified values of a release",
		SilenceUsage: true,
		Args:         require.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := new(action.Configuration)
			helmDriver := os.Getenv("HELM_DRIVER")
			if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, debug); err != nil {
				log.Fatal(err)
			}

			tmpfile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("helm-edit-%s", args[0]))
			if err != nil {
				return err
			}

			defer os.Remove(tmpfile.Name())

			currentValues, err := getValues(cfg, args[0])

			tmpfile.Write(currentValues)

			if err := tmpfile.Close(); err != nil {
				return err
			}

			editor := strings.Split(os.Getenv("EDITOR"), " ")
			command := exec.Command(editor[0], append(editor[1:], tmpfile.Name())...)
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			if err := command.Run(); err != nil {
				return err
			}

			newValues, err := ioutil.ReadFile(tmpfile.Name())
			if err != nil {
				return err
			}

			if string(currentValues) != string(newValues) {
				upgrade(cfg, args[0], tmpfile.Name())
			} else {
			}

			return err
		},
	}
	return cmd
}

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func getValues(cfg *action.Configuration, name string) (raw []byte, err error) {
	client := action.NewGetValues(cfg)

	vals, err := client.Run(name)
	if err != nil {
		return nil, err
	}

	raw, err = yaml.Marshal(vals)
	if err != nil {
		return nil, err
	}

	return raw, nil
}

func upgrade(cfg *action.Configuration, name, file string) (err error) {
	client := action.NewUpgrade(cfg)
	client.Namespace = settings.Namespace()
	valueOpts := &values.Options{}
	valueOpts.ValueFiles = []string{file}

	vals, err := valueOpts.MergeValues(getter.All(settings))
	if err != nil {
		return err
	}

	rel, err := cfg.Releases.Last(name)

	_, err = client.Run(rel.Name, rel.Chart, vals)
	if err != nil {
		return errors.Wrap(err, "UPGRADE FAILED")
	}

	return err
}
