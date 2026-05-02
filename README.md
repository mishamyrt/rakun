<p align="center">
    <img width="100"
         alt="Logo"
         src="./assets/logo.svg">
</p>

<h1 align="center">Rakun</h1>

<p align="center">
  <a href="https://github.com/mishamyrt/rakun/actions/workflows/qa.yml">
    <img src="https://github.com/mishamyrt/rakun/actions/workflows/qa.yml/badge.svg" />
  </a>
  <a href="https://github.com/mishamyrt/rakun/releases/latest">
    <img src="https://img.shields.io/github/v/tag/mishamyrt/rakun?label=version" />
  </a>
</p>

Utility that downloads repositories from different services to local storage.
It was developed because of concerns about blocking my GitHub account.

The first time you run the utility, it creates one `tar.gz` archive per repository. On subsequent runs it compares remote HEAD commits, updates only changed repositories, and keeps an index in `.rakun-index.json`.

## Installation

### Brew (macOS)

```sh
brew install mishamyrt/tap/rakun
```

### From sources

```sh
go install github.com/mishamyrt/rakun@latest
```

## Configuration

The utility loads configuration from a yaml file.
The default is to check `rakun.config.yaml` in the current directory.

To specify a custom configuration path, run the utility with the `-c` flag.

```sh
rakun -c my_config.yaml 
```

Use `-o` or `--output` to choose where archives will be written. If omitted, Rakun writes into the current working directory.

```sh
rakun -c config.yaml -o ./backups
```

Use `-j` or `-jobs` to control parallel repository processing. The default is `runtime.NumCPU()`.

```sh
rakun -c config.yaml -j 8
```

### Structure

```yaml
- domain: github.com
  type: github
  token: !env $GITHUB_TOKEN
  namespaces:
    mishamyrt:
      skip: # Ignore list
        - X01BD-kernel
        - X01BD-device
    Paulownia-Group:
  repos:
    - ghostty-org/ghostty

- domain: github.enterprise.local
  type: github
  token: ghp_your_enterprise_token
  repos:
    - platform/core

- domain: gitlab.com
  type: gitlab
  token: !env $GITLAB_TOKEN
  namespaces:
    platform/core:
      skip:
        - archived-project
        - platform/core/tools/cli
  repos:
    - platform/core/api
```

Each top-level item describes one source host. For `type: github`, `namespaces` discovers every repository visible to the token for a given user or organization, and `repos` uses `owner/repo`.

For `type: gitlab`, `namespaces` uses a GitLab group path such as `platform/core`, discovers projects in that group and its subgroups, and excludes shared projects. `repos` uses the full project path such as `platform/core/api` or `platform/core/tools/cli`.

`token` may be either plain text or an environment reference:

```sh
export GITHUB_TOKEN=your_github_access_token
export GITLAB_TOKEN=your_gitlab_access_token
rakun -c config.yaml -o ./results
```

If a group uses `namespaces`, `token` is required.
If a group uses only explicit `repos`, `token` is optional and public repositories can be synchronized without it.

Each synchronized repository is stored as a full Git checkout inside an archive:

```text
<output>/<host>/<namespace>/<repo>.tar.gz
```

## License

GPL-3.0
