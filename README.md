# FakeSSH

![License](https://img.shields.io/badge/license-GNU/GPLv3-blue?style=flat&color=green)
![Codecov](https://codecov.io/github/lbenicio/honeypot/graph/badge.svg)
![GitHub Release](https://img.shields.io/github/v/release/lbenicio/honeypot?include_prereleases&sort=semver&display_name=release&style=flat&color=blue)

A dockerized honeypot SSH server written in Go to log login attempts.
Password authentications always fail so no terminal access is given to the attacker.

[![](https://hub.docker.com/lbenicio/fakessh)](https://hub.docker.com/lbenicio/fakessh)

## Quick Start

```
make build
sudo setcap 'cap_net_bind_service=+ep' ./dist/fakessh
./dist/fakessh [optional-log-directory]
```

OR

```
docker run -it --rm -p 22:22 fffaraz/fakessh
```

OR

```
docker run -d --restart=always -p 22:22 --name fakessh fffaraz/fakessh
docker logs -f fakessh
```

### Prometheus metrics

FakeSSH exports Prometheus metrics at `http://localhost:2112/metrics` by default.
For Docker, publish both ports to scrape metrics from outside the container:

```
docker run -d --restart=always -p 22:22 -p 2112:2112 --name fakessh fffaraz/fakessh
```

Set `PROMETHEUS_ADDR` to change the metrics listener address. When running
directly on a host, use `127.0.0.1:2112` to restrict access to that host.
Exported application metrics include SSH connections, active connections,
password authentication attempts, and SSH connection errors.

### Docker and Compose

The Dockerfile and Compose configuration are in `deployment/`. The Makefile
reads the latest dated release heading in `CHANGELOG.md` and tags images with
that version and `latest`:

```
make docker-build
make docker-push
make docker-up
make docker-down
```

Log in to the image registry before running `make docker-push`. The Compose
service includes a health check for SSH listener readiness.

### Formatting and linting

Install `golangci-lint` v2.15.0 to use the project checks. `make lint` checks
Go formatting and runs static analysis; `make fmt` formats Go files and
applies available lint fixes. The unit-test workflow runs `make lint`.

### Code coverage

The CI workflow uploads Go coverage reports to Codecov. Configure the
`CODECOV_TOKEN` repository secret with the upload token for this repository.
Codecov upload status and total coverage appear in each workflow summary. To
upload from Dependabot-triggered runs, also add the token under Dependabot
secrets. Coverage is reported when Go tests produce a `coverage.out` profile.

### Releases

To publish a release, run **Publish release** from the Actions tab on the
default branch. The workflow reads the first dated version heading in
`CHANGELOG.md`, creates the matching `v<version>` GitHub release, and builds
and publishes multi-platform Docker images tagged with the version and
`latest`. It stops if that GitHub release, Git tag, or versioned Docker Hub tag
already exists.

Configure the `DOCKERHUB_USERNAME` repository variable and a Docker Hub
personal access token in the `DOCKERHUB_TOKEN` repository secret before running
the workflow. The configured account must have permission to push to
`fffaraz/fakessh`.

### See also

* [jaksi/sshesame](https://github.com/jaksi/sshesame) - A fake SSH server that lets everyone in and logs their activity.
* [shazow/ssh-chat](https://github.com/shazow/ssh-chat) - Custom SSH server written in Go. Instead of a shell, you get a chat prompt.
* [gliderlabs/ssh](https://github.com/gliderlabs/ssh) - Easy SSH servers in Golang.
* [gliderlabs/sshfront](https://github.com/gliderlabs/sshfront) - Programmable SSH frontend.
* [desaster/kippo](https://github.com/desaster/kippo) - Kippo - SSH Honeypot.
* [micheloosterhof/cowrie](https://github.com/micheloosterhof/cowrie) - Cowrie SSH/Telnet Honeypot.
* [fzerorubigd/go0r](https://github.com/fzerorubigd/go0r) - A simple ssh honeypot in golang.
* [droberson/ssh-honeypot](https://github.com/droberson/ssh-honeypot) - Fake sshd that logs ip addresses, usernames, and passwords.
* [x0rz/ssh-honeypot](https://github.com/x0rz/ssh-honeypot) - Fake sshd that logs ip addresses, usernames, and passwords.
* [tnich/honssh](https://github.com/tnich/honssh) - HonSSH is designed to log all SSH communications between a client and server.
* [Learn from your attackers - SSH HoneyPot](https://www.robertputt.co.uk/learn-from-your-attackers-ssh-honeypot.html)
* [cowrie](https://github.com/cowrie/cowrie) - Cowrie SSH/Telnet Honeypot.
