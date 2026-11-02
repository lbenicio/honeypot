# Changelog

Notable changes to this project are documented here. This changelog follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- A manually triggered GitHub Actions workflow to publish versioned GitHub
  releases and multi-platform Docker images from `CHANGELOG.md`.
- GitHub Actions run summaries with workflow results and build, test, and
  release metrics.

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
