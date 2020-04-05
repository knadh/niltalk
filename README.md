# Niltalk

Niltalk is a web based disposable chat server. It allows users to create
password protected disposable, ephemeral chatrooms and invite peers to chat rooms. Rooms can
be disposed of at any time.

![niltalk](https://user-images.githubusercontent.com/547147/78459728-9f8c3180-76d8-11ea-8c0a-9cf9bfe64341.png)

## Installation
Niltalk requires a Redis instance to persist room and session states.

### Manual
- Download the [latest release](https://github.com/knadh/niltalk/releases) for your platform and extract the binary.
- Run `./niltalk --new-config` to generate a sample config.toml and add your configuration.
- Run `./niltalk` and visit http://localhost:9000.

### Docker
The official Docker image `niltalk:latest` is [available here](https://hub.docker.com/r/kailashnadh/niltalk). To try out the app, copy [docker-compose.yml](docker-compose.yml) and run `docker-compose run niltalk`.

### Customisation
The static HTML/JS/CSS assets can be customized. Copy the `static` directory from the repository, change the files, and do: `./niltalk --static-dir=/path/to/custom/static`

> This is a complete rewrite of the old version that had been dead and obsolete for several years (can be found in the `old` branch). These codebases are not compatible with each other and `master` has been overwritten.

Licensed under AGPL3