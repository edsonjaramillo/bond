# Install `bond`

`bond` is currently distributed through GitHub Releases.

## Quick install (latest release)

```bash
curl -fsSL https://github.com/edsonjaramillo/bond/releases/latest/download/install.sh | sh
```

By default this installs to `~/.local/bin`.

## Install a pinned version

```bash
curl -fsSL https://github.com/edsonjaramillo/bond/releases/latest/download/install.sh | sh -s -- --version v0.2.0
```

## Install to a custom directory

```bash
curl -fsSL https://github.com/edsonjaramillo/bond/releases/latest/download/install.sh | sh -s -- --install-dir /usr/local/bin
```

If `/usr/local/bin` needs elevated permissions:

```bash
curl -fsSL https://github.com/edsonjaramillo/bond/releases/latest/download/install.sh -o /tmp/bond-install.sh
sudo BOND_INSTALL_DIR=/usr/local/bin sh /tmp/bond-install.sh
```

## Manual install (without piping a script)

1. Download the right archive and `SHA256SUMS` from the release page.
2. Verify checksum:

```bash
sha256sum -c SHA256SUMS --ignore-missing
```

On macOS without `sha256sum`:

```bash
shasum -a 256 bond_<version>_<os>_<arch>.tar.gz
```

3. Extract and install:

```bash
tar -xzf bond_<version>_<os>_<arch>.tar.gz
install -m 0755 bond_<version>_<os>_<arch>/bond ~/.local/bin/bond
```

## Supported platforms

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Windows artifacts are not published yet.

## Troubleshooting

- `bond: command not found`
  - Ensure your install directory is on `PATH`:
    - `export PATH="$HOME/.local/bin:$PATH"`
- Unsupported OS/architecture
  - Only Linux/macOS on `amd64`/`arm64` are supported today.
- Checksum errors
  - Re-download both archive and `SHA256SUMS`; verify you selected matching files from the same release.
