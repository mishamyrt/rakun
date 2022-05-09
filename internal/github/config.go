package github

type Config struct {
	Users         []string `yaml:"users"`
	Organizations []string `yaml:"organizations"`
	Token         string   `yaml:"token"`
	Ignore        []string `yaml:"ignore"`
}
