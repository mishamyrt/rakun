# Git Sync

A utility that syncs remote repositories to the local repository. It was developed because of concerns about blocking the GitHub account.

On the first run, the utility will clone the specified repositories, and on subsequent runs it will update the data via a pull

## Build

To build for all available platforms, run the `make` command.

```sh
make all
```

If you need only one particular platform, you can build it this way:

```sh
# make build/<platform>/<arch>/git-sync
# For example, building arm32 binary for Zyxel NAS
make build/linux/arm32/git-sync
```

The available build targets can be found in the [Makefile](./Makefile).

## Configuration

The utility loads configuration from a yaml file. The default is to check `config.yaml` in the current directory. To specify a custom configuration path, run the utility with the -c` flag.

```sh
git-sync -c my_config.yaml 
```

### Structure

```yaml
path: /local/path/where/repos/will/be/stored
github:
  users:  # List of users whose repositories will be synchronized
    - mishamyrt
  organizations: # List of organizations whose repositories will be synchronized
    - Paulownia-Group
  token: your_github_access_token
  ignore: # Ignore some repos
    - mishamyrt/X01BD-kernel
    - mishamyrt/X01BD-device
```
