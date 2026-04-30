package config

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Groups []Group
}

type Group struct {
	Domain     string                `yaml:"domain"`
	Type       string                `yaml:"type"`
	Token      Token                 `yaml:"token,omitempty"`
	Namespaces map[string]*Namespace `yaml:"namespaces,omitempty"`
	Repos      []string              `yaml:"repos,omitempty"`
}

type Namespace struct {
	Skip []string `yaml:"skip,omitempty"`
}

type Token struct {
	Value string
	Set   bool
}

func (t *Token) UnmarshalYAML(node *yaml.Node) error {
	t.Set = true
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("token must be a scalar")
	}

	switch node.Tag {
	case "!env":
		envName := strings.TrimSpace(strings.TrimPrefix(node.Value, "$"))
		if envName == "" {
			return fmt.Errorf("!env token must reference an environment variable")
		}

		value, ok := os.LookupEnv(envName)
		if !ok {
			return fmt.Errorf("environment variable %s is not set", envName)
		}
		t.Value = value
	default:
		t.Value = node.Value
	}

	return nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return Config{}, err
	}
	if err := validateRootNode(&root); err != nil {
		return Config{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var groups []Group
	if err := decoder.Decode(&groups); err != nil {
		return Config{}, err
	}

	parsed := Config{Groups: groups}
	return parsed, parsed.Validate()
}

func validateRootNode(root *yaml.Node) error {
	if len(root.Content) == 0 {
		return fmt.Errorf("config must contain at least one provider group")
	}

	node := root.Content[0]
	if node.Kind == yaml.MappingNode {
		return legacyRootError(node)
	}
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("config root must be a list of provider groups")
	}

	return nil
}

func legacyRootError(node *yaml.Node) error {
	keys := map[string]bool{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		keys[keyNode.Value] = true
	}

	if keys["output"] {
		return fmt.Errorf("output is no longer supported in config; use -o or --output")
	}
	if keys["path"] || keys["git"] || keys["github"] || keys["sources"] {
		return fmt.Errorf("legacy config format is no longer supported; use a top-level list of grouped providers")
	}
	return fmt.Errorf("config root must be a list of provider groups")
}

func (g Group) Validate(index int) error {
	if strings.TrimSpace(g.Domain) == "" {
		return fmt.Errorf("groups[%d].domain is required", index)
	}
	if err := validateDomain(g.Domain); err != nil {
		return fmt.Errorf("groups[%d].domain %v", index, err)
	}
	if len(g.Namespaces) == 0 && len(g.Repos) == 0 {
		return fmt.Errorf("groups[%d] must define at least one namespace or repo", index)
	}
	if len(g.Namespaces) > 0 && strings.TrimSpace(g.Token.Value) == "" {
		return fmt.Errorf("groups[%d].token is required when namespaces are configured", index)
	}

	switch g.Type {
	case "github":
		for namespace := range g.Namespaces {
			if strings.TrimSpace(namespace) == "" {
				return fmt.Errorf("groups[%d].namespaces contains an empty namespace", index)
			}
		}
		for repoIndex, repo := range g.Repos {
			if _, _, err := SplitRepoRef(repo); err != nil {
				return fmt.Errorf("groups[%d].repos[%d] %v", index, repoIndex, err)
			}
		}
	case "gitlab":
		for namespace := range g.Namespaces {
			if err := ValidateGitLabNamespaceRef(namespace); err != nil {
				return fmt.Errorf("groups[%d].namespaces[%q] %v", index, namespace, err)
			}
		}
		for repoIndex, repo := range g.Repos {
			if err := ValidateGitLabProjectRef(repo); err != nil {
				return fmt.Errorf("groups[%d].repos[%d] %v", index, repoIndex, err)
			}
		}
	default:
		return fmt.Errorf("groups[%d].type must be github or gitlab", index)
	}

	return nil
}

func (c Config) Validate() error {
	if len(c.Groups) == 0 {
		return fmt.Errorf("config must contain at least one provider group")
	}
	for index, group := range c.Groups {
		if err := group.Validate(index); err != nil {
			return err
		}
	}
	return nil
}

func SplitRepoRef(repo string) (string, string, error) {
	clean := strings.Trim(strings.TrimSpace(repo), "/")
	parts := strings.Split(clean, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("must be in owner/repo format")
	}
	if strings.Contains(parts[0], ":") || strings.Contains(parts[1], ":") {
		return "", "", fmt.Errorf("must be in owner/repo format")
	}
	if strings.HasSuffix(parts[1], ".git") {
		return "", "", fmt.Errorf("must omit the .git suffix")
	}
	return parts[0], parts[1], nil
}

func ValidateGitLabNamespaceRef(namespace string) error {
	return validateGitLabPath(namespace, 1, "must be a GitLab group path")
}

func ValidateGitLabProjectRef(repo string) error {
	return validateGitLabPath(repo, 2, "must be a GitLab project path like group/project")
}

func validateGitLabPath(value string, minSegments int, formatError string) error {
	clean := strings.Trim(strings.TrimSpace(value), "/")
	parts := strings.Split(clean, "/")
	if len(parts) < minSegments {
		return errors.New(formatError)
	}
	for _, part := range parts {
		if part == "" || strings.Contains(part, ":") {
			return errors.New(formatError)
		}
	}
	if strings.HasSuffix(parts[len(parts)-1], ".git") {
		return fmt.Errorf("must omit the .git suffix")
	}
	return nil
}

func validateDomain(domain string) error {
	if strings.Contains(domain, "://") {
		return fmt.Errorf("must not include a URL scheme")
	}

	parsed, err := url.Parse("//" + domain)
	if err != nil {
		return err
	}
	if parsed.Host == "" || parsed.Path != "" || strings.ContainsAny(domain, "/?#") {
		return fmt.Errorf("must be a host name")
	}

	return nil
}
