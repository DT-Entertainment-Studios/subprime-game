# Subprime - The game for real estate sharks

> &#x26A0; This project contains significant AI-generated code. While the core functionality works,
> it has not been thoroughly reviewed yet. Use at your own risk.

Subprime is a online multiplayer real-estate trading board game.

## Installation

Pull from [ghcr.io], download the package from [Releases] or build using `builder/build.sh`

## Development

The development stack mocks different logins on port 8000 - 8003:

- Arteon: <http:/127.0.0.1:8000/>
- Beetle: <http://127.0.0.1:8001/>
- Camaro: <http://127.0.0.1:8002/>
- Ducato: <http://127.0.0.1:8003/>

### Local stack

You can use `run-dev-local.sh` to run the stack directly:

```bash
./run-dev-local.sh
```

### Container stack

```bash
docker compose --file docker-compose-dev.yaml up
```

### Dev container

This repository contains an Ai Agents
[`devcontainer.json`](.devcontainer/ai-agents/devcontainer.json). To develop using a dev container,
ensure you have Docker or an alternative like Podman installed and open the project in a compatible
editor like VSCode. The dev container setup will automatically configure the development
environment.

[ghcr.io]: https://ghcr.io/DT-Entertainment-Studios/subprime-game
[Releases]: https://github.com/DT-Entertainment-Studios/subprime-game/releases
