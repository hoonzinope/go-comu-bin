# AGENTS.md instructions for /Users/hoonzi/go-proj/commu-bin

## Skills
A skill is a set of local instructions to follow that is stored in a `SKILL.md` file. Below is the list of skills that can be used for this repository. Each entry includes a name, description, and file path so the agent can open the source for full instructions when using a specific skill.

### Available skills
- code-quality-auditor: Audit code for bugs, security, and refactor candidates. Logs findings to `.documents/review/` without modifying source code. (file: /Users/hoonzi/go-proj/commu-bin/.agents/skills/code-quality-auditor/SKILL.md)

## How to use skills
- Discovery: The list above is the canonical set of repository-local skills for this workspace.
- Trigger rules: If the user names a skill, references `$skill-name`, or the task clearly matches a listed skill's description, the agent must use that skill for the turn.
- Missing or blocked: If a named skill cannot be read, the agent should say so briefly and continue with the best fallback.

## Skill loading rules
1. Open the referenced `SKILL.md` and read only enough to follow the workflow.
2. Resolve relative paths in the skill relative to the skill directory first.
3. Load only the referenced files needed for the current task; avoid bulk-reading unrelated assets.
4. If the skill provides scripts, templates, or assets, prefer using them over recreating equivalent content manually.

## Coordination
- Use the minimal set of skills needed to complete the request.
- State which skill is being used and why in one short line before substantial work.
- Do not carry repository-local skills across turns unless the user re-mentions them or the new request clearly matches the same skill.

## Context hygiene
- Keep loaded context small and task-focused.
- Prefer summaries over pasting long skill contents into the conversation.
- Avoid deep reference chasing unless the skill requires it.
