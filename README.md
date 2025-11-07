# LSPit

If you think yet another VS Code fork with some AI bolted on represents the best DX ever, then this is **not** the tool for you!

However, if you drink at least some of the "AI assisted coding" Kool-Aid, to the extent where you regularly use a desktop LLM chat app such as Claude, and you may have even hot-rodded it with some extensions like [Desktop Commander](https://github.com/wonderwhy-er/DesktopCommanderMCP), then here's a minimal, focused Go LSP client for querying `gopls` from the command line. It empowers your AI assistant to query type information, definitions, and references without the overhead of a full editor integration.

## What's with the name?

I was about to call it `lsp-vomit` but let's be real here, the outputs of this tool are too puny to deserve being called vomit. But this tool _does_ spit out responses from the LSP server, so...

Gross? Yeah. Maybe you shouldn't have asked.

## Disclaimer

WIP, no support, made for personal use, etc. 100% vibe coded (subject to change) with absolutely no guarantees; it may format your hard drive, start a moderately successful drug cartel, or simply run away with your wife and pick-up truck (in which case you won't be needing this tool anymore, you'll have a career opportunity doing country music, so this is a win scenario really). Use at your own risk.

## Features

- **hover** - Get type information and documentation at a specific position
- **definition** - Find where a symbol is defined
- **references** - Find all references to a symbol

## Installation

```bash
go build -o lspit
```

Or install to your PATH:

```bash
go install
```

## Usage

All commands use 1-indexed line and column numbers (matching editor display):

```bash
# Get type information
./lspit hover <file> <line> <column>

# Find definition
./lspit definition <file> <line> <column>

# Find all references
./lspit references <file> <line> <column>
```

## Examples

```bash
# Get type info for the symbol at line 111, column 22
./lspit hover pkg/outboundjwt/outboundjwt.go 111 22
# Output: package redis ("github.com/redis/go-redis/v9")

# Find where it's defined
./lspit definition pkg/outboundjwt/outboundjwt.go 111 22
# Output: /path/to/file.go:17:2

# Find all usages
./lspit references pkg/outboundjwt/outboundjwt.go 111 22
# Output:
# /path/to/file.go:17:2
# /path/to/file.go:47:19
# /path/to/file.go:53:21
# ...
```

## How It Works

1. Spawns a `gopls` process and maintains a persistent connection
2. Performs LSP initialization handshake (initialize → initialized)
3. Opens the requested file via `textDocument/didOpen`
4. Executes the requested LSP method (hover, definition, references)
5. Formats and displays the results
6. Cleanly shuts down gopls (shutdown → exit)

Each invocation creates a fresh gopls instance (~2-3 seconds overhead). Larger projects may benefit from a daemon mode (future enhancement).

## Requirements

- Go 1.20+ (for building)
- gopls installed (`go install golang.org/x/tools/gopls@latest`)
- A Go workspace with proper go.mod setup

## Design Decisions

**Why a fresh gopls per query?**  
Simpler implementation, no daemon management complexity. For small-to-medium projects, the 2-3s startup cost is acceptable.

**What about caching?**  
Not implemented. Each query is independent. Add a daemon mode for persistent state if needed.

## Architecture

```
main.go       - CLI interface, argument parsing, command dispatch
client.go     - LSP client implementation, protocol handling
utils.go      - Helper functions (git root detection)
```

**Key Implementation Details:**
- Uses standard library for subprocess management and JSON-RPC
- Background goroutine reads LSP responses asynchronously
- Channel-based request/response correlation by message ID
- Proper Content-Length header formatting for LSP messages
- 0-indexed to 1-indexed position conversion for user convenience

## Future Enhancements

If you find this tool valuable, consider:

1. **Daemon mode** - Keep gopls running between queries (eliminates 2-3s startup)
2. **Batch operations** - Process multiple queries in one gopls session
3. **More LSP features** - Completion, symbols, formatting, diagnostics
4. **Caching** - Store results for unchanged files
5. **JSON output** - Machine-readable format for tool integration

## Integration Example

Use in AI assistant prompts:

```bash
# When working on Go code, query type information
./lspit hover pkg/auth/jwt.go 45 10

# Find definition before refactoring
./lspit definition pkg/auth/jwt.go 45 10

# Check impact before renaming
./lspit references pkg/auth/jwt.go 45 10
```

## Troubleshooting

**"gopls not found"**  
Install: `go install golang.org/x/tools/gopls@latest`

**"No hover information available"**  
Position might not be on a symbol. Try adjacent columns.

**Slow on large projects**  
gopls loads the entire workspace. Use daemon mode (future) or query smaller modules.
