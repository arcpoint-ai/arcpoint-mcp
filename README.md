# Arcpoint MCP Client

Official Model Context Protocol (MCP) client for [Arcpoint](https://arcpoint.ai) - the intelligent LLM routing and observability platform.

This lightweight client connects Claude Desktop, Cursor IDE, and other MCP-compatible tools to your Arcpoint workspace, giving you:

- 📊 **Real-time analytics** - Query traces, cost breakdowns, and usage metrics
- ⚙️ **Configuration management** - Create and update routing profiles and rules
- 🔍 **Observability** - Deep insights into your LLM usage patterns
- 🚀 **Smart routing** - Configure intelligent model selection and fallbacks

## Installation

### Option 1: Go Install (Recommended)

```bash
go install github.com/arcpoint-ai/arcpoint-mcp@latest
```

### Option 2: Download Binary

Download the latest release for your platform from [GitHub Releases](https://github.com/arcpoint-ai/arcpoint-mcp/releases).

**macOS (Apple Silicon):**
```bash
curl -L https://github.com/arcpoint-ai/arcpoint-mcp/releases/latest/download/arcpoint-mcp-darwin-arm64 -o /usr/local/bin/arcpoint-mcp
chmod +x /usr/local/bin/arcpoint-mcp
```

**macOS (Intel):**
```bash
curl -L https://github.com/arcpoint-ai/arcpoint-mcp/releases/latest/download/arcpoint-mcp-darwin-amd64 -o /usr/local/bin/arcpoint-mcp
chmod +x /usr/local/bin/arcpoint-mcp
```

**Linux:**
```bash
curl -L https://github.com/arcpoint-ai/arcpoint-mcp/releases/latest/download/arcpoint-mcp-linux-amd64 -o /usr/local/bin/arcpoint-mcp
chmod +x /usr/local/bin/arcpoint-mcp
```

### Option 3: Build from Source

```bash
git clone https://github.com/arcpoint-ai/arcpoint-mcp.git
cd arcpoint-mcp
go build -o arcpoint-mcp .
```

## Configuration

### Get Your API Token

1. Sign up at [arcpoint.ai](https://arcpoint.ai)
2. Navigate to **Settings → API Tokens**
3. Create a new token with the permissions you need

### Configure Your MCP Client

#### Cursor IDE

Add to your Cursor MCP settings file:

**Location:**
- **macOS:** `~/Library/Application Support/Cursor/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
- **Linux:** `~/.config/Cursor/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json`
- **Windows:** `%APPDATA%\Cursor\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json`

**Configuration:**
```json
{
  "mcpServers": {
    "arcpoint": {
      "command": "arcpoint-mcp",
      "args": [],
      "env": {
        "ARCPOINT_API_TOKEN": "apt_your_token_here"
      }
    }
  }
}
```

#### Claude Desktop

Add to your Claude Desktop configuration:

**Location:**
- **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Linux:** `~/.config/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

**Configuration:**
```json
{
  "mcpServers": {
    "arcpoint": {
      "command": "arcpoint-mcp",
      "args": [],
      "env": {
        "ARCPOINT_API_TOKEN": "apt_your_token_here"
      }
    }
  }
}
```

### Environment Variables

- `ARCPOINT_API_TOKEN` (required) - Your Arcpoint API token
- `ARCPOINT_API_URL` (optional) - Custom API endpoint (default: `https://mcp.arcpoint.ai`)

## Available Resources

Once configured, you can access these Arcpoint resources:

### `traces://recent`
Most recent LLM request traces (last 50)

### `analytics://cost-summary`
Cost breakdown by model for the last 24 hours

### `config://profiles`
Your configured routing profiles

### `config://rules`
Your configured rules and policies

## Available Tools

### `query_traces`
Search LLM traces with filters

**Parameters:**
- `start_time` (optional) - Start time in RFC3339 format
- `end_time` (optional) - End time in RFC3339 format
- `model` (optional) - Filter by model name
- `limit` (optional) - Max results (default: 50)

### `get_cost_breakdown`
Detailed cost analysis for a time period

**Parameters:**
- `start_time` (required) - Start time in RFC3339 format
- `end_time` (required) - End time in RFC3339 format

### `create_profile`
Create a new routing profile

**Parameters:**
- `name` (required) - Profile name
- `pattern_id` (optional) - Pattern ID
- `rule_ids` (optional) - Array of rule IDs
- `routing_config` (optional) - JSON routing configuration

### `update_rule`
Update an existing rule

**Parameters:**
- `rule_id` (required) - Rule ID to update
- `enabled` (optional) - Enable/disable the rule
- `parameters` (optional) - Rule parameters

## Example Usage

Once configured, you can ask Claude or your IDE:

> "Show me my recent LLM traces"

> "What were my costs yesterday?"

> "Create a new routing profile called 'production' with fallback to GPT-4"

> "Disable the rate limiting rule"

## Troubleshooting

### "ARCPOINT_API_TOKEN environment variable is required"

Make sure you've added your API token to the configuration file. Get a token from [arcpoint.ai/settings/tokens](https://arcpoint.ai/settings/tokens).

### "Invalid API token"

Your token may be expired or revoked. Generate a new one from your Arcpoint dashboard.

### "Connection error"

Check your internet connection and verify that `https://mcp.arcpoint.ai` is accessible. If you're behind a corporate proxy, you may need to configure proxy settings.

### Client not appearing in Claude/Cursor

1. Restart Claude Desktop or Cursor after adding the configuration
2. Verify the `command` path is correct (use `which arcpoint-mcp` to find it)
3. Check the configuration file syntax is valid JSON

## Development & Self-Hosting

If you're running Arcpoint on-premises or using a development environment:

```json
{
  "mcpServers": {
    "arcpoint": {
      "command": "arcpoint-mcp",
      "args": [],
      "env": {
        "ARCPOINT_API_TOKEN": "apt_your_token_here",
        "ARCPOINT_API_URL": "http://localhost:8084"
      }
    }
  }
}
```

## Security

- API tokens are transmitted via HTTPS with TLS encryption
- Tokens are never logged or stored by the client
- The client is stateless and makes no local caching
- All communication goes directly to Arcpoint's API

## Support

- 📖 [Documentation](https://docs.arcpoint.ai)
- 💬 [Discord Community](https://discord.gg/arcpoint)
- 📧 [Email Support](mailto:support@arcpoint.ai)
- 🐛 [Report Issues](https://github.com/arcpoint-ai/arcpoint-mcp/issues)

## License

MIT License - see [LICENSE](LICENSE) for details.

## About Arcpoint

[Arcpoint](https://arcpoint.ai) is the intelligent routing layer for your LLM applications. We help you:

- Route requests to the best model for each task
- Monitor costs and usage in real-time
- Implement guardrails and compliance rules
- Optimize for performance and cost

Learn more at [arcpoint.ai](https://arcpoint.ai).

