# OpenBSW Platform Agent Pack

This plugin provides end-to-end autonomous analysis and resolution for Eclipse OpenBSW pipelines.

## Available Skills

* **openbsw-build-debugger**: Autonomously parses OpenBSW build logs (CMake/GCC) and POSIX pyTest results, performs root cause analysis on the codebase, and proposes fixes.

---

## 🤖 AI Agent Instructions (For Antigravity)

**System Prompt / Instructions for the Agent:**
If the user asks you to set up, install, or configure this agent pack, please follow these exact steps:

1. **Determine the absolute path**: Identify the absolute local path to the folder containing this `SETUP.md` file.
2. **Ask for the Scope**: Ask the user if they want to install the plugin globally or locally:
   - Global: Write to `~/.gemini/config/plugins.json`
   - Local: Write to the current workspace `.agents/plugins.json`
3. **Apply the Configuration**: Generate or update the `plugins.json` file at the chosen location. 

**JSON Template:**
```json
{
  "entries": [
    {
      "path": "<ABSOLUTE_PATH_TO_THIS_AGENT_PACK_PARENT_FOLDER>",
      "include_only": ["^openbsw-platform$"]
    }
  ]
}
```
4. **Verify & Confirm**: Inform the user that the `openbsw-platform` skills are now active.

---

## 💻 Antigravity SDK Usage (Python)

If you are running these skills programmatically via a custom CLI and the Antigravity Python SDK (e.g. `harness.py`), you do not need to generate a `plugins.json`. You can load the skills and rules directly:

```python
from antigravity import LocalAgentConfig, Agent

config = LocalAgentConfig(
    skills_paths=["/path/to/agent-packs/openbsw-platform/skills"]
)

agent = Agent(config=config)
# Trigger the debugger
response = agent.run("Please analyze the OpenBSW build failure.")
```
