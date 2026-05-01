# Franz

> **Note:** Franz is an independent, unofficial fork based on
> [Charm's Crush](https://github.com/charmbracelet/crush) and is not affiliated
> with Charmbracelet, Inc.

<pre align="center">
┏━╸┏━┓┏━┓┏┓╻╺━┓
┣╸ ┣┳┛┣━┫┃┗┫┏━┛
╹  ╹┗╸╹ ╹╹ ╹┗━╸
</pre>
<p align="center">
    <a href="https://github.com/marang/franz-agent/releases"><img src="https://img.shields.io/github/release/marang/franz-agent" alt="Latest Release"></a>
    <a href="https://github.com/marang/franz-agent/actions"><img src="https://github.com/marang/franz-agent/actions/workflows/build.yml/badge.svg" alt="Build Status"></a>
</p>

<p align="center">Your new coding bestie, now available in your favourite terminal.<br />Your tools, your code, and your workflows, wired into your LLM of choice.</p>
<p align="center">终端里的编程新搭档，<br />无缝接入你的工具、代码与工作流，全面兼容主流 LLM 模型。</p>

## Features

- **Multi-Model:** choose from a wide range of LLMs or add your own via OpenAI- or Anthropic-compatible APIs
- **Flexible:** switch LLMs mid-session while preserving context
- **Subscription Support:** log in with ChatGPT Plus/Pro (Codex Subscription) via `openai-codex`
- **Session-Based:** maintain multiple work sessions and contexts per project
- **LSP-Enhanced:** Franz uses LSPs for additional context, just like you do
- **Extensible:** add capabilities via MCPs (`http`, `stdio`, and `sse`)
- **Usage Visibility:** check Codex subscription usage limits from CLI and TUI (`franz-agent status` and `/status`)
- **Works Everywhere:** first-class support in every terminal on macOS, Linux, Windows (PowerShell and WSL), Android, FreeBSD, OpenBSD, and NetBSD
- **Industrial Grade:** built on the Charm ecosystem, powering 25k+ applications, from leading open source projects to business-critical infrastructure

## Installation

Install with Go:

```bash
go install github.com/marang/franz-agent@latest
```

Install from AUR (Arch Linux):

```bash
# yay
yay -S franz-agent

# paru
paru -S franz-agent
```

Install Flatpak (from release bundle):

```bash
# download franz-agent.flatpak from GitHub Releases first
flatpak install --user ./franz-agent.flatpak
```

> [!WARNING]
> Productivity may increase when using Franz and you may find yourself nerd
> sniped when first using the application. If the symptoms persist, join the
> [project discussions](https://github.com/marang/franz-agent/discussions) and
> nerd snipe the rest of us.

## Getting Started

The quickest way to get started is to grab an API key for your preferred
provider such as Anthropic, OpenAI, Groq, OpenRouter, or Vercel AI Gateway and just start
Franz. You'll be prompted to enter your API key.

That said, you can also set environment variables for preferred providers.

| Environment Variable        | Provider                                           |
| --------------------------- | -------------------------------------------------- |
| `ANTHROPIC_API_KEY`         | Anthropic                                          |
| `OPENAI_API_KEY`            | OpenAI                                             |
| `VERCEL_API_KEY`            | Vercel AI Gateway                                  |
| `GEMINI_API_KEY`            | Google Gemini                                      |
| `SYNTHETIC_API_KEY`         | Synthetic                                          |
| `ZAI_API_KEY`               | Z.ai                                               |
| `MINIMAX_API_KEY`           | MiniMax                                            |
| `HF_TOKEN`                  | Hugging Face Inference                             |
| `CEREBRAS_API_KEY`          | Cerebras                                           |
| `OPENROUTER_API_KEY`        | OpenRouter                                         |
| `IONET_API_KEY`             | io.net                                             |
| `GROQ_API_KEY`              | Groq                                               |
| `VERTEXAI_PROJECT`          | Google Cloud VertexAI (Gemini)                     |
| `VERTEXAI_LOCATION`         | Google Cloud VertexAI (Gemini)                     |
| `AWS_ACCESS_KEY_ID`         | Amazon Bedrock (Claude)                            |
| `AWS_SECRET_ACCESS_KEY`     | Amazon Bedrock (Claude)                            |
| `AWS_REGION`                | Amazon Bedrock (Claude)                            |
| `AWS_PROFILE`               | Amazon Bedrock (Custom Profile)                    |
| `AWS_BEARER_TOKEN_BEDROCK`  | Amazon Bedrock                                     |
| `AZURE_OPENAI_API_ENDPOINT` | Azure OpenAI models                                |
| `AZURE_OPENAI_API_KEY`      | Azure OpenAI models (optional when using Entra ID) |
| `AZURE_OPENAI_API_VERSION`  | Azure OpenAI models                                |

### Subscriptions

If you prefer subscription-based usage, here are some plans that work well in
Franz:

- ChatGPT Plus/Pro (Codex Subscription via `openai-codex`)
- [Synthetic](https://synthetic.new/pricing)
- [GLM Coding Plan](https://z.ai/subscribe)
- [Kimi Code](https://www.kimi.com/membership/pricing)
- [MiniMax Coding Plan](https://platform.minimax.io/subscribe/coding-plan)

#### ChatGPT Plus/Pro (Codex Subscription)

You can authenticate Franz against your ChatGPT Codex subscription:

```bash
franz-agent login openai-codex
```

After login, select an `openai-codex` model (for example `gpt-5.4` or
`gpt-5.4-mini`).

You can check usage limits from:

- CLI: `franz-agent status`
- TUI: `/status`

`status` reports the current Codex subscription windows (for example 5h and
weekly usage buckets) when available.

## Configuration

> [!TIP]
> Franz ships with a builtin `franz-config` skill for configuring itself. In
> many cases you can simply ask Franz to configure itself.

Franz runs great with no configuration. That said, if you do need or want to
customize Franz, configuration can be added either local to the project itself,
or globally, with the following priority:

1. `.franz-agent.json`
2. `franz-agent.json`
3. `$HOME/.config/franz-agent/franz-agent.json`

Configuration itself is stored as a JSON object:

```json
{
  "this-setting": { "this": "that" },
  "that-setting": ["ceci", "cela"]
}
```

As an additional note, Franz also stores ephemeral data, such as application state, in one additional location:

```bash
# Unix
$HOME/.local/share/franz-agent/franz-agent.json

# Windows
%LOCALAPPDATA%\franz-agent\franz-agent.json
```

> [!TIP]
> You can override the user and data config locations by setting:
> * `FRANZ_GLOBAL_CONFIG`
> * `FRANZ_GLOBAL_DATA`

## Skills Manager

Franz includes an interactive **Skills Manager** in the TUI for managing local
skills and browsing/installing skills from `skills.sh`.

Open it from the command palette (`Ctrl+P`) using **Skills Manager**.

### What you can do

- View installed skills and their status (`enabled`, `disabled`, security/perms warnings).
- Enable, disable, delete, or fix permissions for selected installed skills.
- Search skills from `skills.sh`, multi-select, and install directly.
- Open skill detail links from search results.

### Security and permissions

- Skills installed from `skills.sh` are validated with a security audit before use.
- Installed skills are checked for restrictive file permissions; warnings are shown in the manager.
- You can run **Fix Perms** in the installed view to normalize permissions.

### LSPs

Franz can use LSPs for additional context to help inform its decisions, just
like you would. LSPs can be added manually like so:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "lsp": {
    "go": {
      "command": "gopls",
      "env": {
        "GOTOOLCHAIN": "go1.24.5"
      }
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    },
    "nix": {
      "command": "nil"
    }
  }
}
```

### MCPs

Franz also supports Model Context Protocol (MCP) servers through three
transport types: `stdio` for command-line servers, `http` for HTTP endpoints,
and `sse` for Server-Sent Events. Environment variable expansion is supported
using `$(echo $VAR)` syntax.

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "mcp": {
    "filesystem": {
      "type": "stdio",
      "command": "node",
      "args": ["/path/to/mcp-server.js"],
      "timeout": 120,
      "disabled": false,
      "disabled_tools": ["some-tool-name"],
      "env": {
        "NODE_ENV": "production"
      }
    },
    "github": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/",
      "timeout": 120,
      "disabled": false,
      "disabled_tools": ["create_issue", "create_pull_request"],
      "headers": {
        "Authorization": "Bearer $GH_PAT"
      }
    },
    "streaming-service": {
      "type": "sse",
      "url": "https://example.com/mcp/sse",
      "timeout": 120,
      "disabled": false,
      "headers": {
        "API-Key": "$(echo $API_KEY)"
      }
    }
  }
}
```

### Ignoring Files

Franz respects `.gitignore` files by default, but you can also create a
`.franzignore` file to specify additional files and directories that Franz
should ignore. This is useful for excluding files that you want in version
control but don't want Franz to consider when providing context.

The `.franzignore` file uses the same syntax as `.gitignore` and can be placed
in the root of your project or in subdirectories.

### Allowing Tools

By default, Franz will ask you for permission before running tool calls. If
you'd like, you can allow tools to be executed without prompting you for
permissions. Use this with care.

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "permissions": {
    "allowed_tools": [
      "view",
      "ls",
      "grep",
      "edit",
      "mcp_context7_get-library-doc"
    ]
  }
}
```

You can also skip all permission prompts entirely by running Franz with the
`--yolo` flag. Be very, very careful with this feature.

### Disabling Built-In Tools

If you'd like to prevent Franz from using certain built-in tools entirely, you
can disable them via the `options.disabled_tools` list. Disabled tools are
completely hidden from the agent.

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "disabled_tools": ["bash", "sourcegraph"]
  }
}
```

To disable tools from MCP servers, see the [MCP config section](#mcps).

### Disabling Skills

If you'd like to prevent Franz from using certain skills entirely, you can
disable them via the `options.disabled_skills` list. Disabled skills are hidden
from the agent, including builtin skills and skills discovered from disk.

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "disabled_skills": ["franz-config"]
  }
}
```

### Agent Skills

Franz supports the [Agent Skills](https://agentskills.io) open standard for
extending agent capabilities with reusable skill packages. Skills are folders
containing a `SKILL.md` file with instructions that Franz can discover and
activate on demand.

The global paths we looks for skills are:

* `$FRANZ_SKILLS_DIR`
* `$XDG_CONFIG_HOME/agents/skills` or `~/.config/agents/skills/`
* `$XDG_CONFIG_HOME/franz-agent/skills` or `~/.config/franz-agent/skills/`
* On Windows, we _also_ look at
  * `%LOCALAPPDATA%\agents\skills\` or `%USERPROFILE%\AppData\Local\agents\skills\`
  * `%LOCALAPPDATA%\franz-agent\skills\` or `%USERPROFILE%\AppData\Local\franz-agent\skills\`
* Additional paths configured via `options.skills_paths`

On top of that, we _also_ load skills in your project from the following
relative paths:

* `.agents/skills`
* `.franz-agent/skills`
* `.codex/skills`
* `.claude/skills`
* `.cursor/skills`

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "skills_paths": [
      "~/.config/franz-agent/skills", // Windows: "%LOCALAPPDATA%\\franz-agent\\skills",
      "./project-skills"
    ]
  }
}
```

You can get started with example skills from [anthropics/skills](https://github.com/anthropics/skills):

```bash
# Unix
mkdir -p ~/.config/franz-agent/skills
cd ~/.config/franz-agent/skills
git clone https://github.com/anthropics/skills.git _temp
mv _temp/skills/* . && rm -rf _temp
```

```powershell
# Windows (PowerShell)
mkdir -Force "$env:LOCALAPPDATA\franz-agent\skills"
cd "$env:LOCALAPPDATA\franz-agent\skills"
git clone https://github.com/anthropics/skills.git _temp
mv _temp/skills/* . ; rm -r -force _temp
```

### Desktop notifications

Franz sends desktop notifications when a tool call requires permission and when
the agent finishes its turn. They're only sent when the terminal window isn't
focused _and_ your terminal supports reporting the focus state.

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "disable_notifications": false, // default
  },
}
```

To disable desktop notifications, set `disable_notifications` to `true` in your
configuration. On macOS, notifications currently lack icons due to platform
limitations.

### Initialization

When you initialize a project, Franz analyzes your codebase and creates
a context file that helps it work more effectively in future sessions.
By default, this file is named `AGENTS.md`, but you can customize the
name and location with the `initialize_as` option:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "initialize_as": "AGENTS.md"
  }
}
```

This is useful if you prefer a different naming convention or want to
place the file in a specific directory (e.g., `CRUSH.md` or
`docs/LLMs.md`). Franz will fill the file with project-specific context
like build commands, code patterns, and conventions it discovered during
initialization.

### Attribution Settings

By default, Franz adds attribution information to Git commits and pull requests
it creates. You can customize this behavior with the `attribution` option:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "attribution": {
      "trailer_style": "co-authored-by",
      "generated_with": true
    }
  }
}
```

- `trailer_style`: Controls the attribution trailer added to commit messages
  (default: `assisted-by`)
	- `assisted-by`: Adds `Assisted-by: [Model Name] via Franz <franz-agent@charm.land>`
	  (includes the model name)
	- `co-authored-by`: Adds `Co-Authored-By: Franz <franz-agent@charm.land>`
	- `none`: No attribution trailer
- `generated_with`: When true (default), adds `💘 Generated with Franz` line to
  commit messages and PR descriptions

### Custom Providers

Franz supports custom provider configurations for both OpenAI-compatible and
Anthropic-compatible APIs.

> [!NOTE]
> Note that we support two "types" for OpenAI. Make sure to choose the right one
> to ensure the best experience!
>
> - `openai` should be used when proxying or routing requests through OpenAI.
> - `openai-compat` should be used when using non-OpenAI providers that have OpenAI-compatible APIs.

#### OpenAI-Compatible APIs

Here’s an example configuration for Deepseek, which uses an OpenAI-compatible
API. Don't forget to set `DEEPSEEK_API_KEY` in your environment.

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "providers": {
    "deepseek": {
      "type": "openai-compat",
      "base_url": "https://api.deepseek.com/v1",
      "api_key": "$DEEPSEEK_API_KEY",
      "models": [
        {
          "id": "deepseek-chat",
          "name": "Deepseek V3",
          "cost_per_1m_in": 0.27,
          "cost_per_1m_out": 1.1,
          "cost_per_1m_in_cached": 0.07,
          "cost_per_1m_out_cached": 1.1,
          "context_window": 64000,
          "default_max_tokens": 5000
        }
      ]
    }
  }
}
```

#### Anthropic-Compatible APIs

Custom Anthropic-compatible providers follow this format:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "providers": {
    "custom-anthropic": {
      "type": "anthropic",
      "base_url": "https://api.anthropic.com/v1",
      "api_key": "$ANTHROPIC_API_KEY",
      "extra_headers": {
        "anthropic-version": "2023-06-01"
      },
      "models": [
        {
          "id": "claude-sonnet-4-20250514",
          "name": "Claude Sonnet 4",
          "cost_per_1m_in": 3,
          "cost_per_1m_out": 15,
          "cost_per_1m_in_cached": 3.75,
          "cost_per_1m_out_cached": 0.3,
          "context_window": 200000,
          "default_max_tokens": 50000,
          "can_reason": true,
          "supports_attachments": true
        }
      ]
    }
  }
}
```

### Amazon Bedrock

Franz currently supports running Anthropic models through Bedrock, with caching disabled.

- A Bedrock provider will appear once you have AWS configured, i.e. `aws configure`
- Franz also expects the `AWS_REGION` or `AWS_DEFAULT_REGION` to be set
- To use a specific AWS profile set `AWS_PROFILE` in your environment, i.e. `AWS_PROFILE=myprofile franz-agent`
- Alternatively to `aws configure`, you can also just set `AWS_BEARER_TOKEN_BEDROCK`

### Vertex AI Platform

Vertex AI will appear in the list of available providers when `VERTEXAI_PROJECT` and `VERTEXAI_LOCATION` are set. You will also need to be authenticated:

```bash
gcloud auth application-default login
```

To add specific models to the configuration, configure as such:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "providers": {
    "vertexai": {
      "models": [
        {
          "id": "claude-sonnet-4@20250514",
          "name": "VertexAI Sonnet 4",
          "cost_per_1m_in": 3,
          "cost_per_1m_out": 15,
          "cost_per_1m_in_cached": 3.75,
          "cost_per_1m_out_cached": 0.3,
          "context_window": 200000,
          "default_max_tokens": 50000,
          "can_reason": true,
          "supports_attachments": true
        }
      ]
    }
  }
}
```

### Local Models

Local models can also be configured via OpenAI-compatible API. Here are two common examples:

#### Ollama

```json
{
  "providers": {
    "ollama": {
      "name": "Ollama",
      "base_url": "http://localhost:11434/v1/",
      "type": "openai-compat",
      "models": [
        {
          "name": "Qwen 3 30B",
          "id": "qwen3:30b",
          "context_window": 256000,
          "default_max_tokens": 20000
        }
      ]
    }
  }
}
```

#### LM Studio

```json
{
  "providers": {
    "lmstudio": {
      "name": "LM Studio",
      "base_url": "http://localhost:1234/v1/",
      "type": "openai-compat",
      "models": [
        {
          "name": "Qwen 3 30B",
          "id": "qwen/qwen3-30b-a3b-2507",
          "context_window": 256000,
          "default_max_tokens": 20000
        }
      ]
    }
  }
}
```

## Logging

Sometimes you need to look at logs. Luckily, Franz logs all sorts of
stuff. Logs are stored in `./.franz-agent/logs/franz-agent.log` relative to the project.

The CLI also contains some helper commands to make perusing recent logs easier:

```bash
# Print the last 1000 lines
franz-agent logs

# Print the last 500 lines
franz-agent logs --tail 500

# Follow logs in real time
franz-agent logs --follow
```

Want more logging? Run `franz-agent` with the `--debug` flag, or enable it in the
config:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "debug": true,
    "debug_lsp": true
  }
}
```

## Provider Auto-Updates

By default, Franz automatically checks for the latest and greatest list of
providers and models. This means that when new providers and models are
available, or when model metadata changes, Franz automatically updates your local
configuration.

### Disabling automatic provider updates

For those with restricted internet access, or those who prefer to work in
air-gapped environments, this might not be want you want, and this feature can
be disabled.

To disable automatic provider updates, set `disable_provider_auto_update` into
your `franz-agent.json` config:

```json
{
  "$schema": "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json",
  "options": {
    "disable_provider_auto_update": true
  }
}
```

Or set the `FRANZ_DISABLE_PROVIDER_AUTO_UPDATE` environment variable:

```bash
export FRANZ_DISABLE_PROVIDER_AUTO_UPDATE=1
```

### Manually updating providers

Manually updating providers is possible with the `franz-agent update-providers`
command:

```bash
# Update providers remotely.
franz-agent update-providers

# Update providers from a custom provider registry base URL.
franz-agent update-providers https://example.com/

# Update providers from a local file.
franz-agent update-providers /path/to/local-providers.json

# Reset providers to the embedded version, embedded at franz-agent at build time.
franz-agent update-providers embedded

# For more info:
franz-agent update-providers --help
```

## Metrics

Franz records pseudonymous usage metrics (tied to a device-specific hash),
which maintainers rely on to inform development and support priorities. The
metrics include solely usage metadata; prompts and responses are NEVER
collected.

Details on exactly what’s collected are in the source code ([here](https://github.com/marang/franz-agent/tree/main/internal/event)
and [here](https://github.com/marang/franz-agent/blob/main/internal/llm/agent/event.go)).

You can opt out of metrics collection at any time by setting the environment
variable by setting the following in your environment:

```bash
export FRANZ_DISABLE_METRICS=1
```

Or by setting the following in your config:

```json
{
  "options": {
    "disable_metrics": true
  }
}
```

Franz also respects the `DO_NOT_TRACK` convention which can be enabled via
`export DO_NOT_TRACK=1`.

## Q&A

### Why is clipboard copy and paste not working?

Installing an extra tool might be needed on Unix-like environments.

| Environment         | Tool                     |
| ------------------- | ------------------------ |
| Windows             | Native support           |
| macOS               | Native support           |
| Linux/BSD + Wayland | `wl-copy` and `wl-paste` |
| Linux/BSD + X11     | `xclip` or `xsel`        |

## Contributing

See the [contributing guide](https://github.com/marang/franz-agent?tab=contributing-ov-file#contributing).

## Whatcha think?

We'd love to hear your thoughts on this project. You can find us on:

- [GitHub Issues](https://github.com/marang/franz-agent/issues)
- [GitHub Discussions](https://github.com/marang/franz-agent/discussions)

## License

[FSL-1.1-MIT](https://github.com/marang/franz-agent/raw/main/LICENSE.md)

---

Based on [Charm's Crush](https://github.com/charmbracelet/crush), maintained
independently by the Franz project.
