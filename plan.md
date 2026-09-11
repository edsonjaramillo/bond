# Resource Commands Plan

## CLI

```text
bond resources
bond resources add [--copy] <resource> [resource...]
bond resources remove <resource> [resource...]
```

- `resources` is root-level, parallel to `skills`.
- Bare `resources` prints help.
- No `list`, `clear`, or `update` initially.
- Successful mutations are silent; failures use stderr and exit status 1.
- Cobra’s native shell completion remains available.

## Resource model

- Resource names are flat lowercase kebab-case.
- Stored at:
  - `$XDG_CONFIG_HOME/bond/resources`
  - macOS fallback: `~/Library/Application Support/bond/resources`
  - other fallback: `~/.config/bond/resources`
- Each Stored Resource must be a real directory.
- Parent Store components may be symlinks.
- Valid contents are directories, regular files, and safe relative symlinks.
- Absolute, escaping, or otherwise unsafe symlinks and special files are rejected.
- Empty directories are ignored.
- A Resource with no installable leaf entries is invalid.

## Adding

- Contents are installed relative to the exact working directory.
- Multiple Resources are processed left-to-right in one transaction.
- Repeated arguments and already-managed Resources are errors.
- Existing real directories may be traversed and shared.
- Existing leaf destinations, symlinked ancestors, and non-directory ancestors are collisions.
- Identical existing files are still collisions.
- Cross-Resource leaf collisions reject the whole invocation.
- Paths destructively overlapping Managed Skills or other managed artifacts are rejected.
- Bond metadata paths and `.agents/.bond-*` are reserved.
- Targets on filesystems different from the transaction area are rejected.

Default link mode:

- Regular files become absolute symlinks into the Resource Store.
- Safe source symlinks are recreated as relative symlinks.
- Installed topology is a snapshot; new Store paths do not appear automatically.
- Relative symlink targets must be materialized by an installed leaf.

`--copy` mode:

- Applies to every Resource in the invocation.
- Recursively copies regular files.
- Preserves file and newly created directory permission bits.
- Does not preserve ownership or timestamps.
- Recreates safe relative symlinks.
- Existing merged directory permissions remain unchanged.

## Directory policy

- The manifest tracks only installed leaves, not directories.
- Parent directories are created as needed.
- Empty source directories are not created.
- Successful Resource removal never removes directories.
- Failed transactions still roll back directories created by that transaction.

## Ownership and manifest

- Upgrade the existing unified project manifest to version 2.
- Resource records contain the Resource name, installation mode, and owned relative leaf paths.
- Version-1 manifests remain version 1 through Skill-only operations.
- The first Resource addition upgrades the manifest to version 2.
- Manifests never downgrade, including after removing the last Resource.
- Older Bond versions consequently reject upgraded manifests.
- `docs/adr/0003-unify-skill-and-resource-ownership.md` records this trade-off.

## Removing

- Accepts one or more Managed Resource names.
- All names must be managed; otherwise nothing changes.
- Removal uses the manifest and works even if the Stored Resource is missing.
- Missing destination leaves are treated as stale ownership and cleaned from the manifest.
- Modified copies and replaced regular files/symlinks are deleted because manifest ownership is authoritative.
- If an owned leaf has become a directory, removal refuses rather than deleting recursively.
- Directories are always retained.

## Transactions

- Each invocation is locked, preflighted, journaled, crash-safe, and all-or-nothing.
- Preflight aggregates diagnostics in argument order.
- Resource and Skill mutations share the project manifest and transaction infrastructure.
- Recovery preserves conflicting filesystem entries for manual intervention.

## Dynamic completion

- `resources add` completes valid, not-yet-managed Stored Resource names.
- `resources remove` completes currently Managed Resource names.
- Already-entered arguments are excluded.
- Completion prefix-filters, suppresses filesystem/manifest errors, and returns `NoFileComp`.

## Documentation

- `CONTEXT.md`: Resource terminology and Resource Name.
- `docs/adr/0003-unify-skill-and-resource-ownership.md`: unified manifest-v2 decision.
