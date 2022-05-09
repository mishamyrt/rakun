package config

import (
	"git_sync/internal/github"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Path   string        `yaml:"path"`
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
