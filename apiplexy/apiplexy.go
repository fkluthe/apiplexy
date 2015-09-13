package main

// Import apiplexy plugins in a separate block (just because it looks nicer).
import (
	_ "github.com/12foo/apiplexy/auth/hmac"
	_ "github.com/12foo/apiplexy/backend/sql"
)

import (
	"fmt"
	"github.com/12foo/apiplexy"
	"github.com/codegangsta/cli"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"text/tabwriter"
)

func listPlugins(c *cli.Context) {
	fmt.Printf("Available plugins:\n\n")
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for name, description := range apiplexy.AvailablePlugins() {
		fmt.Fprintf(w, "   %s\t%s\n", name, description)
	}
	fmt.Fprintln(w)
	w.Flush()
}

func generateConfig(c *cli.Context) {
	config, err := apiplexy.ExampleConfiguration(c.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't generate configuration: %s\n", err.Error())
		os.Exit(1)
	}
	yml, err := yaml.Marshal(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't generate configuration: %s\n", err.Error())
		os.Exit(1)
	}
	os.Stdout.Write(yml)
}

func start(c *cli.Context) {
	configPath := c.String("config")
	yml, err := ioutil.ReadFile(os.ExpandEnv(configPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't read config file: %s\n", err.Error())
		os.Exit(1)
	}
	config := apiplexy.ApiplexConfig{}
	err = yaml.Unmarshal(yml, &config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't parse configuration: %s\n", err.Error())
		os.Exit(1)
	}

	ap, err := apiplexy.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't initialize API proxy. %s\n", err.Error())
		os.Exit(2)
	}

	server := &http.Server{
		Addr:    "0.0.0.0:" + strconv.Itoa(config.Serve.Port),
		Handler: ap,
	}
	server.ListenAndServe()
}

func main() {
	app := cli.NewApp()
	app.Name = "apiplexy"
	app.Usage = "Pluggable API gateway/proxy system."
	app.Commands = []cli.Command{
		{
			Name:    "plugins",
			Usage:   "Lists available apiplexy plugins",
			Aliases: []string{"ls"},
			Action:  listPlugins,
		},
		{
			Name:    "genconf",
			Usage:   "Generates a configuration file with the specified plugins",
			Aliases: []string{"gen"},
			Action:  generateConfig,
		},
		{
			Name:   "start",
			Usage:  "Starts API proxy using specified config file",
			Action: start,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "config, c",
					Value: "apiplexy.yaml",
					Usage: "Location of configuration file",
				},
			},
		},
	}
	app.Run(os.Args)
}
