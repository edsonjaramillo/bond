# Journal project skill mutations

Bond applies each `add`, `remove`, or `clear` invocation as one crash-safe transaction guarded by a five-second project lock. It stages filesystem changes and journals recovery before atomically updating the manifest, allowing handled failures and interrupted processes to roll back without splitting ownership state from Project Skill state; recovery refuses to overwrite paths created after an interruption and instead requires manual resolution.
