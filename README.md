![Baton Logo](./baton-logo.png)

#

`baton-workato` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-workato.svg)](https://pkg.go.dev/github.com/conductorone/baton-workato) ![main ci](https://github.com/conductorone/baton-workato/actions/workflows/main.yaml/badge.svg)

`baton-workato` is a connector for built using the [Baton SDK](https://github.com/conductorone/baton-sdk).

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## Prerequisites

You need to pass the workato-api-key:

1. Create an Workato Account.
2. Create an API KEY https://app.workato.com/members/api/clients.
3. Run it.

Obs: if you have a basic account, you can ignore the subusers using.

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-workato
baton-workato
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_DOMAIN_URL=domain_url -e BATON_API_KEY=apiKey -e BATON_USERNAME=username ghcr.io/conductorone/baton-workato:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-workato/cmd/baton-workato@main

baton-workato

baton resources
```

# Data Model

`baton-workato` will pull down information about the following resources:

- Users

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually
building spreadsheets. We welcome contributions, and ideas, no matter how
small&mdash;our goal is to make identity and permissions sprawl less painful for
everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-workato` Command Line Usage

```
baton-workato

Usage:
  baton-workato [flags]
  baton-workato [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --client-id string             The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string         The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string                  The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                         help for baton-workato
      --log-format string            The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string             The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -p, --provisioning                 This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --skip-full-sync               This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --ticketing                    This must be set to enable ticketing support ($BATON_TICKETING)
  -v, --version                      version for baton-workato
      --workato-api-key string       required: Your workato API key ($BATON_WORKATO_API_KEY)
      --workato-data-center string   Your workato data center (us, eu, jp, sg, au) default is 'us' see more on https://docs.workato.com/workato-api.html#base-url ($BATON_WORKATO_DATA_CENTER) (default "us")
```
