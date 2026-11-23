# Changelog

Notable changes to this project are documented here. This changelog follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Unit tests for SSH handling, authentication, startup configuration, and
  Prometheus metrics, with a CI coverage requirement above 98%.
- Docker health checks for the SSH listener.
- Formatting and golangci-lint checks in the unit-test workflow.
- Make targets for linting, formatting, versioned Docker image builds and
  pushes, and starting or stopping the Docker Compose service.
- A manually triggered GitHub Actions workflow to publish versioned GitHub
  releases and multi-platform Docker images from `CHANGELOG.md`.
- Go test coverage uploads to Codecov.
- Weekly Dependabot updates for Go modules, Docker images, and GitHub Actions.
- GitHub Actions run summaries with workflow results and build, test, and
  release metrics.

### Changed

- Split the Go application into domain-focused packages for authentication,
  health checks, metrics, and SSH serving, with startup and logging at the
  application layer.
- Moved the Go application source into `src/`; `make build` now writes the
  executable to `dist/fakessh`.
- Moved the Dockerfile and Compose configuration into `deployment/`.
- Replaced the scratch runtime image with Distroless Static Debian 13.

## [0.1.0] - 2026-11-01

### Added

- Prometheus metrics at `/metrics` on `:2112` by default, configurable with
  `PROMETHEUS_ADDR`.
- Metrics for accepted SSH connections, active connections, password
  authentication attempts, and SSH connection errors.

### Changed

- Updated the Go baseline to 1.27 and pinned the toolchain to Go 1.27.3.
- Updated Go dependencies to their latest compatible versions.
- Exposed port 2112 in the Docker image for Prometheus scraping.
