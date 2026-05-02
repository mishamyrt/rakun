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
</p>

Utility that synchronizes repositories from different services to local storage. It was developed because of concerns about blocking the GitHub account.

The first time you run the utility, it creates one `tar.gz` archive per repository. On subsequent runs it compares remote HEAD commits, updates only changed repositories, and keeps an index in `.rakun-index.json`.

## Build

To build for all available platforms, run the `make` command.

```sh
make all
```

If you need only one particular platform, you can build it this way:

```sh
# make build/<platform>/<arch>/rakun
# For example, building arm32 binary for Zyxel NAS
make build/linux/arm32/rakun
```

The available build targets can be found in the [Makefile](./Makefile).

## Configuration

The utility loads configuration from a yaml file. The default is to check `rakun.config.yaml` in the current directory. To specify a custom configuration path, run the utility with the `-c` flag.

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
      skip:
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

If a group uses `namespaces`, `token` is required. If a group uses only explicit `repos`, `token` is optional and public repositories can be synchronized without it.

Each synchronized repository is stored as a full Git checkout inside an archive:

```text
<output>/<host>/<namespace>/<repo>.tar.gz
```

For example:

```text
test-result/github.com/mishamyrt/rakun.tar.gz
```

Rakun unpacks the existing archive into a temporary directory, runs `git fetch` and `git reset --hard` against the default branch, then repacks the archive. If the archive is missing or broken, Rakun recreates it from a fresh clone.

## License

GPL-3.0
