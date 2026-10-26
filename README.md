# Custom MCP Server Builder

Build + maintain MCP servers on the Hermes stack.

## Service Description

MCPBuilder automates the creation and management of Model Context Protocol (MCP) servers:

- **Template-based generation** — Create servers from YAML specs
- **Go + mcp-go framework** — Uses mark3labs/mcp-go for stdio/SSE/WS transports
- **Tool/Resource/Prompt scaffolding** — Auto-generates handler stubs
- **Dependency management** — `go mod tidy` and build automation
- **Process supervision** — Start/stop/restart with PID tracking
- **Hermes integration** — Designed for Hermes Agent stack deployment

Pricing: **$99–299/mo** (Accountability tier)

## Installation

```bash
go build -o mcpbuilder .
# or
go install github.com/jonathanngiroux-star/mcpbuilder@latest
```

## Configuration

```yaml
servers:
  - id: "my-server"
    name: "My MCP Server"
    description: "Custom tools for my workflow"
    version: "1.0.0"
    transport: "stdio"
    command: "go"
    args: ["run", "main.go"]
    working_dir: "./servers/my-server"
    tools:
      - name: "my_tool"
        description: "Does something useful"
        input_schema:
          type: object
          properties:
            input:
              type: string
          required: ["input"]
```

## CLI Interface

```bash
# Create new MCP server
mcpbuilder -action create -name "My Server" -description "Custom tools" -transport stdio -tool my_tool -tool-desc "Does something"

# List servers
mcpbuilder -action list

# Build server
mcpbuilder -action build -id <server-id>

# Install dependencies
mcpbuilder -action install -id <server-id>

# Start/stop/restart
mcpbuilder -action start -id <server-id>
mcpbuilder -action stop -id <server-id>
mcpbuilder -action restart -id <server-id>

# View logs
mcpbuilder -action logs -id <server-id>

# Remove server
mcpbuilder -action remove -id <server-id>
```

## Generated Server Structure

```
servers/my-server/
├── go.mod
├── main.go           # MCP server entry point
├── echo_handler.go   # Tool handler stubs
├── README.md
```

## Built-in Templates

- `echo` — Simple echo tool for testing
- `file-ops` — File read/write/list operations
- `http-client` — HTTP request tool
- `shell` — Command execution tool
- `sqlite` — SQLite query tool

## Pricing

| Tier | Price | Includes |
|------|-------|----------|
| Starter | $99/mo | Up to 5 servers, stdio transport, basic templates |
| Professional | $199/mo | Up to 20 servers, SSE/WS, custom templates, monitoring |
| Enterprise | $299/mo | Unlimited servers, Hermes integration, SLA, on-call |

## Hosted Version

Managed MCP hosting at **katsironni.com/mcpbuilder** — includes Hermes deployment, monitoring, SLA.

## License

MIT — see [LICENSE](LICENSE)