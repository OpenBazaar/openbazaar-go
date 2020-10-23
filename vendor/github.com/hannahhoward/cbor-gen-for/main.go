package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "cbor-gen-for",
		Usage: "Generate CBOR encoders for types",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name: "map-encoding",
			},
		},
		Action: func(c *cli.Context) error {
			filename := os.Getenv("GOFILE")
			path, _ := os.Getwd()
			err := Generator{
				Filename:    filename,
				Path:        path,
				Package:     os.Getenv("GOPACKAGE"),
				GenStructs:  c.Args().Slice(),
				MapEncoding: c.Bool("map-encoding"),
			}.GenerateCborTypes()

			return err
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
