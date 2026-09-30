# Horizon SDV Agent Plugins

This directory contains the central, domain-specific AI plugins for the Horizon SDV project (e.g., AAOS, OpenBSW). These plugins equip the Antigravity AI agent with the necessary rules, skills, and tools to autonomously debug pipelines, analyze test results, and propose code fixes.

## 🚀 Installation (For Developers & IDE Users)

If you are working on a local Cloud Workstation or directly in your IDE, the easiest way to load a plugin is via the official Antigravity CLI.

Use the `agy plugin install` command and point it to the GitHub repository URL of the specific plugin.

**Syntax:**
```bash
agy plugin install https://github.com/<org>/<repo>/tree/<branch>/agent-plugins/<plugin-name>
```

**Example:**
```bash
agy plugin install https://github.com/googlecloudplatform/horizon-sdv/tree/main/agent-plugins/aaos-platform
```

This will automatically clone and register the plugin, apply its global guardrails (`rules/AGENTS.md`), load all its skills, and start any bundled FastMCP Python tools in the background.

---

## 💻 Python SDK Usage (For CI/CD & Headless Execution)

If you are running these skills programmatically in a CI/CD pipeline (e.g., Argo Workflows) using the Antigravity Python SDK, you do not need to use the CLI installation.

Instead, you can inject the specific skills directly into your `LocalAgentConfig` by pointing to the plugin's `skills` folder:

```python
from antigravity import LocalAgentConfig, Agent

# Load the skills directly from the specific plugin folder
config = LocalAgentConfig(
    skills_paths=["/path/to/horizon-sdv/agent-plugins/<plugin-name>/skills"]
)

agent = Agent(config=config)

# Trigger the agent autonomously
response = agent.run("Analyze the latest pipeline failure.")
```
