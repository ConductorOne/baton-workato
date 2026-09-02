![Baton Logo](./baton-logo.png)

#

`baton-workato` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-workato.svg)](https://pkg.go.dev/github.com/conductorone/baton-workato) ![ci](https://github.com/conductorone/baton-workato/actions/workflows/ci.yaml/badge.svg) ![verify](https://github.com/conductorone/baton-workato/actions/workflows/verify.yaml/badge.svg)

`baton-workato` is a connector for built using the [Baton SDK](https://github.com/conductorone/baton-sdk).

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## Prerequisites

You must be an Admin or have a role with access to API Clients.

### Required Client Role permissions

| Area         | Section              | Action                      | API Endpoint                       |
|--------------|----------------------|-----------------------------|------------------------------------|
| **Projects** | Projects & Folders   | List projects               | `GET /api/projects`                |
|              | Projects & Folders   | List folders                | `GET /api/folders`                 |
| **Admin**    | Collaborators        | Get collaborators           | `GET /api/members`                 |
|              | Collaborators        | Get collaborator            | `GET /api/members/:id`             |
|              | Collaborators        | Update collaborator’s roles | `PUT /api/members/:id`             |
|              | Collaborators        | Get collaborator privileges | `GET /api/members/:id/privileges`  |
|              | Collaborator roles   | List non-system roles       | `GET /api/roles`                   |

> **Note:** The **Collaborator roles** section (including "List non-system roles") only appears in the API client permissions UI if your workspace still uses the legacy roles model. If this section is not visible, your workspace has migrated to the new RBAC v2 model (Environment roles + Project roles). In that case, legacy custom roles are not accessible via the API — and legacy custom role sync is already skipped by default (`--disable-custom-roles-sync` defaults to `true`), so no action is needed.
>
> If your workspace is still on the legacy roles model and you want C1 to keep syncing legacy custom roles, set `--disable-custom-roles-sync=false` (or `BATON_DISABLE_CUSTOM_ROLES_SYNC=false`). Otherwise, migrate your legacy custom roles to the new model using the [Role migration API](https://docs.workato.com/workato-api/role-migration.html).

### Using Workato commercial platform:

Generate an API KEY:
1. Log in to your Workato account at https://app.workato.com
2. In the top-right, click your profile icon, then go to *My Account* or *Account Settings*
3. In the left sidebar, look for *API Clients* or *API Keys*
4. Click on *+ Create API client* or *+ Generate API key*
5. Fill out the form:
   Name: A descriptive name for this client.
   Description: Optional, but helpful for tracking usage.
   Scopes or Permissions: Choose what the API key can access (e.g., recipes, jobs, folders).
6. Click Generate or Create.

After creation:
    You’ll be shown the raw API key.
    Save the credentials securely — they will not be shown again.

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
      --disable-custom-roles-sync    Disable custom roles sync ($BATON_DISABLE_CUSTOM_ROLES_SYNC) (default true)
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
      --workato-env string           Your workato environment (dev, test, prod, all) default is 'dev' ($BATON_WORKATO_ENV) (default "dev")

Use "baton-workato [command] --help" for more information about a command.
```
