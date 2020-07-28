package main

import (
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli"

	"github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/json"
	"github.com/polydawn/refmt/pretty"
	"github.com/polydawn/refmt/shared"
)

func main() {
	os.Exit(Main(os.Args, os.Stdin, os.Stdout, os.Stderr))
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	app := cli.NewApp()
	app.Name = "refmt"
	app.Authors = []cli.Author{
		cli.Author{Name: "Eric Myhre", Email: "hash@exultant.us"},
	}
	app.Commands = []cli.Command{
		//
		// Prettyprinters
		//
		cli.Command{
			Category: "prettyprint",
			Name:     "json=pretty",
			Usage:    "read json, then pretty print it",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					json.NewDecoder(stdin),
					pretty.NewEncoder(stdout),
				}.Run()
			},
		},
		cli.Command{
			Category: "prettyprint",
			Name:     "cbor=pretty",
			Usage:    "read cbor, then pretty print it",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					cbor.NewDecoder(cbor.DecodeOptions{}, stdin),
					pretty.NewEncoder(stdout),
				}.Run()
			},
		},
		cli.Command{
			Category: "prettyprint",
			Name:     "cbor.hex=pretty",
			Usage:    "read cbor in hex, then pretty print it",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					cbor.NewDecoder(cbor.DecodeOptions{}, hexReader(stdin)),
					pretty.NewEncoder(stdout),
				}.Run()
			},
		},
		cli.Command{
			Category: "prettyprint",
			Name:     "yaml=pretty",
			Usage:    "read yaml, then pretty print it",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					newYamlTokenSource(stdin),
					pretty.NewEncoder(stdout),
				}.Run()
			},
		},
		//
		// Converters
		//
		cli.Command{
			Category: "convert",
			Name:     "json=cbor",
			Usage:    "read json, emit equivalent cbor",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					json.NewDecoder(stdin),
					cbor.NewEncoder(stdout),
				}.Run()
			},
		},
		cli.Command{
			Category: "convert",
			Name:     "json=cbor.hex",
			Usage:    "read json, emit equivalent cbor in hex",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					json.NewDecoder(stdin),
					cbor.NewEncoder(hexWriter{stdout}),
				}.Run()
			},
		},
		cli.Command{
			Category: "convert",
			Name:     "cbor=json",
			Usage:    "read cbor, emit equivalent json",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					cbor.NewDecoder(cbor.DecodeOptions{}, stdin),
					json.NewEncoder(stdout, json.EncodeOptions{}),
				}.Run()
			},
		},
		cli.Command{
			Category: "convert",
			Name:     "cbor.hex=json",
			Usage:    "read cbor in hex, emit equivalent json",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					cbor.NewDecoder(cbor.DecodeOptions{}, hexReader(stdin)),
					json.NewEncoder(stdout, json.EncodeOptions{}),
				}.Run()
			},
		},
		cli.Command{
			Category: "convert",
			Name:     "yaml=json",
			Usage:    "read yaml, emit equivalent json",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					newYamlTokenSource(stdin),
					json.NewEncoder(stdout, json.EncodeOptions{}),
				}.Run()
			},
		},
		cli.Command{
			Category: "convert",
			Name:     "yaml=cbor",
			Usage:    "read yaml, emit equivalent cbor",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					newYamlTokenSource(stdin),
					cbor.NewEncoder(stdout),
				}.Run()
			},
		},
		cli.Command{
			Category: "convert",
			Name:     "yaml=cbor.hex",
			Usage:    "read yaml, emit equivalent cbor in hex",
			Action: func(c *cli.Context) error {
				return shared.TokenPump{
					newYamlTokenSource(stdin),
					cbor.NewEncoder(hexWriter{stdout}),
				}.Run()
			},
		},
	}
	app.Writer = stdout
	app.ErrWriter = stderr
	err := app.Run(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	return 0
}
