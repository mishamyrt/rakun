# Rakun

<img align="right" width="151" height="105"
     alt="Logo"
     src="./assets/logo@2x.png">

Utility that synchronizes data from different services to local storage. It was developed because of concerns about blocking the GitHub account.

The first time you run the utility, it will download all available data. On subsequent runs it will only download what has been updated.

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

The utility loads configuration from a yaml file. The default is to check `config.yaml` in the current directory. To specify a custom configuration path, run the utility with the -c` flag.

```sh
rakun -c my_config.yaml 
```

### Structure

```yaml
path: /local/path/where/repos/will/be/stored
git:
    - https://github.com/mishamyrt/git-sync # Single repo from any Git server
    - https://gitlab.com/mishamyrt/old_site
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

## Credits

* Raccoon Icon — [Sergey Chikin](http://sergeychikin.ru/)
