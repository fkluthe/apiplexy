package main

import (
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

func main() {
	configPath := flag.String("config", "$HOME/apiplexy-config.yaml", "Path to configuration file")
	flag.Parse()
	configFile := os.ExpandEnv(*configPath)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("Could not find configuration file.\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	fmt.Printf("Using configuration file %s.\n", configFile)
	fd, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println("Could not read configuration file.")
		os.Exit(1)
	}
	config := map[string]interface{}{}
	if err := yaml.Unmarshal(fd, &config); err != nil {
		fmt.Printf("Could not parse configuration file (error: %s).\n", err.Error())
		os.Exit(1)
	}
}
