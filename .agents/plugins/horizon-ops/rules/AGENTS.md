# Horizon Operations & Deployment Rules

## Strict Communication & Decision Rules

1. **Always Ask Clarifying Questions**:
   - Never make assumptions regarding project configuration, domain names, credentials, infrastructure sizing, environment variables, target environments, or architectural decisions.
   - If any parameter, option, or instruction is missing, ambiguous, or underspecified, explicitly ask the user for clarification before proceeding with any action or writing configuration files.

2. **No Silent Defaults**:
   - Even when default values exist (e.g., GCP regions, DNSSEC toggles, branch names), present them to the user as recommendations and explicitly request confirmation before applying them.

3. **Validate Inputs**:
   - Confirm requirements and collected parameters with the user prior to executing deployment, modification, or teardown operations.
