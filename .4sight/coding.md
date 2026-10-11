Unless specified, use go for the application, and for scripting needs, unless
specified otherwise, use nushell

## golang

Use go 1.27 or higher

Do not _add_ package dependencies outside of go stdlib and experimental modules
unless specifically requested or are from this list:

- opentelemetry standard go client
- openai standard go client

After code changes are ready, run package targeted:

- go fix
- golangci-lint fmt
- golangci-lint
