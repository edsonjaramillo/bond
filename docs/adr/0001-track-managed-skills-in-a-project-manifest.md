# Track managed skills in a project manifest

Bond records every Managed Skill in a versioned `.agents/bond-manifest.json`, including its name, Store-relative source, installation mode, and Project-relative destination. The manifest remains authoritative until Bond removes its entry, allowing `remove` and `clear` to delete only explicitly owned paths—including independent copies—rather than risking user-maintained Skills through filesystem inference.
