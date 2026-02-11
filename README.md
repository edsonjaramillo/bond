# bond

Bond is a CLI to manage reusable agent skills across your store and projects, syncing them into `.agents/skills` where needed.

## Install

`bond` is currently distributed through GitHub Releases (not package managers yet). Package manager support is planned soon.

Currently supported:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Install the latest release:

```bash
curl -fsSL https://github.com/edsonjaramillo/bond/releases/latest/download/install.sh | sh
```

Default install directory: `$HOME/.local/bin`

If needed, add it to `PATH` for this shell session:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Verify installation:

```bash
command -v bond
bond --version
```

For pinned versions, custom install directories, and manual checksum verification, see `INSTALL.md`.

## Usage

### Enable shell completion first (recommended)

It will allow you to tab complete skills and flags in all commands.

For `zsh` (current session):

```bash
source <(bond completion zsh)
```

For `bash` (current session):

```bash
source <(bond completion bash)
```

For `fish` (current session):

```bash
bond completion fish | source
```

### Global flags (all commands)

```bash
--color auto|always|never   # control color output
--no-level                  # hide INFO/OK/WARN/ERROR labels
```

Environment variables affecting global output:

- `BOND_NO_COLORS`: when set (to any value), color is disabled if `--color` is `auto`.
- `BOND_NO_LEVEL`: boolean (`true`/`false`, `1`/`0`) to hide or show INFO/OK/WARN/ERROR labels.
- Command-line flags override environment variables when both are set.

### Story walkthrough: from empty project to managed skills

1. Initialize the store directory. `XDG_CONFIG_HOME/bond` or `$HOME/.config/bond` by default:

```bash
bond init --store
```

2. Initialize your project skill directory:

```bash
bond init
```

2. Create a new store skill scaffold:

```bash
bond create react-best-practices --description "React best practices for agents"
```

3. Validate the new skill:

```bash
bond validate react-best-practices
```

**Optional**: Validate all discovered store skills:

```bash
bond validate --all
```

4. List skills in store and project:

```bash
bond list --store
```

5. Link the store skill into this project (main path):

```bash
bond link react-best-practices
```

6. Check current link status:

```bash
bond status
```

7. Edit the skill in your store via `$EDITOR`:

```bash
bond edit react-best-practices
```

8. Remove symlinked skills when you no longer need them in this project:

```bash
bond unlink react-best-practices
```

### Alternative sync path: copy instead of symlink

If you want project-local files instead of symlinks:

```bash
bond copy react-best-practices
```

Use `link` when you want project skills to stay connected to store files, and `copy` when you want project-local copies.

### Want to store a skill that's not in your store yet?

Make sure it is in `.agents/skills` in your project, then run:

```
bond store [name]
```
