# Bond

Bond manages reusable AI-agent Skills, Resources, and Instructions in central collections and applies them to individual projects.

## Language

**Skill**:
A directory containing a valid `SKILL.md` with YAML frontmatter that names and describes the skill. A Skill may contain additional supporting files.

**Store**:
The central collection of Skills available for installation into projects.
_Avoid_: Repository, registry

**Skill Draft**:
A not-yet-valid Skill created for editing before it can be installed into a project. A Skill Draft becomes a Skill once its `SKILL.md` passes validation.
_Avoid_: Invalid skill

**Skill Name**:
The lowercase kebab-case name declared in a Skill's frontmatter and matched by its directory basename. Skill Names are unique within a project but may appear under multiple organizational paths in the Store.

**Organization**:
A Store-only grouping that contains Stored Skills one level below the Store root. Its lowercase kebab-case name is not part of a Skill Name but is part of the Stored Skill's relative path.
_Avoid_: Namespace, category

**Stored Skill**:
A Skill held in the Store and identified there by its relative path. A Stored Skill may belong to one Organization.
_Avoid_: Global skill

**Project Skill**:
A Skill installed in a project's `.agents/skills` directory, either as a link to a Stored Skill or as an independent copy.
_Avoid_: Local skill

**Managed Skill**:
A Project Skill installed and tracked by Bond, and therefore eligible for removal by Bond.
_Avoid_: Handled skill

**Instruction**:
A reusable Markdown document held in the Instruction Store and appended to a target document.

**Instruction Store**:
The central collection of Instructions available for appending to target documents.
_Avoid_: Instruction repository, instruction registry

**Instruction Path**:
An Instruction's Instruction Store-relative identifier in the form `name.md` or `group/name.md`. The optional grouping directory and filename stem use lowercase kebab-case, and the extension is exactly `.md`.

**Resource**:
A named directory tree of reusable files whose paths are relative to a project root.
_Avoid_: Asset, template

**Resource Name**:
The lowercase kebab-case basename of a Resource's directory. Resource Names are unique within both the Resource Store and a project.

**Resource Store**:
The central collection of Resources available for installation into projects.
_Avoid_: Resource repository, resource registry

**Stored Resource**:
A Resource held in the Resource Store and identified there by name.
_Avoid_: Global resource

**Managed Resource**:
A Stored Resource installed into a project as links or independent copies and tracked by Bond, and therefore eligible for removal by Bond.
_Avoid_: Local resource, handled resource
