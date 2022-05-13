package config

import (
	"io/ioutil"
	"rakun/providers/github"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Path   string        `yaml:"path"`
	Git    []string      `yaml:"git,omitempty"`
	Github github.Config `yaml:"github,omitempty"`
}

func Load(path string) (Config, error) {
	var parsedData Config
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return parsedData, err
	}
	err = yaml.Unmarshal(data, &parsedData)
	return parsedData, err
}
