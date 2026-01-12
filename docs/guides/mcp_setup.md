# MCP Setup Guide

This guide provides instructions for setting up MCP servers in MCP Gateway Registry and how to use them with Gemini-CLI and Gemini Code Assist.

## Prerequisites
- Access to MCP Gateway Registry with appropriate permissions.
  - User must be added to either of Keycloak groups:
    - `horizon-mcp-gateway-registry-admins`: Admins can register new or edit existing MCP servers and agents. They have full access to all MCP servers, agents and this app’s API.
    - `horizon-mcp-gateway-registry-users`: Users can only view existing registered MCP servers and agents but have full use access to all MCP servers and agents; and read-only access to this app’s API.
- Workstation Images with Gemini-CLI and Gemini Code Assist installed.

### Enable MCP Servers in MCP Gateway Registry
1. Log in to MCP Gateway Registry at `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMAIN>/` using your Keycloak credentials.
2. Navigate to the "MCP Servers" section.
3. In order to access any MCP server, including the pre-registered `gerrit-mcp-server`, make sure that it is ENABLED on the app (bottom right toggle button in server card). By default, newly registered MCP servers are disabled.

### Add New MCP Server (Optional)
Steps:
1. Click on the "Register New Server" button.
2. Fill in the details.
3. Make sure you enter the path as /my-mcp-server/ with trailing slashes on both ends. See Known Issues.
4. New server should now be visible.
5. Make sure to ENABLE it before use (bottom right toggle button in server card). See Known Issues.

### Using MCP Servers with Gemini-CLI and Gemini Code Assist
Open your Cloud Workstation with Gemini-CLI and Gemini Code Assist installed.

#### Run `gemini-mcp-setup` Script
It does the following,
- Authenticates with MCP Gateway Registry via Keycloak (Device Flow), fetches JWT token, MCP servers and updates ~/.gemini/settings.json with fresh data.
- Optionally allows continuous sync with both foreground and background (daemon) modes.
- At any time, only one process/session of this script is allowed.
- The script also provides other helpful options:
  - `--login`: same as default run with no options; reuses tokens if not expired and skips login
  - `--watch`: run in foreground sync; occupies current terminal session
  - `--daemon-start`: login and directly start background sync
  - `--daemon-stop`: stop any running background sync
  - `--daemon-status`: show mode and PID of currently running sync
  - `--prune`: sync and remove any managed servers that are no longer present on MCP Gateway Registry
  - `--force`: sync and replaces all servers with only the ones present on MCP Gateway Registry

Steps:
1. Open a terminal
2. Run `gemini-mcp-setup`
3. Follow the prompts to authenticate
4. It is recommended to keep the script running in background via the script’s own daemon mode so that, in case the access_token expiry time is too short, the script will use refresh_token to keep refreshing access_token until SSO session expires.

#### Gemini-CLI
1. Open a terminal
2. Run `export GOOGLE_CLOUD_PROJECT=<your-gcp-project-id>`
3. Run `gemini`
4. Login to Gemini
5. Run `/mcp`
6. User should be able to see the pre-registered gerrit-mcp-server with status Connected.
7. Now user can run any Gerrit related query and Gemini will query the MCP server for answers.

#### Gemini Code Assist

##### Horizon Code OSS
1. Open workstation with Horizon Code OSS
2. From the left sidebar, click on the Gemini icon to open Gemini Code Assist.
3. Complete initial setup > Select “Gemini for Businesses”
4. Select “Google Cloud Project” and complete further setup
5. In chat window, enable "Agent" mode by clicking on the "Agent" button at the bottom right of the chat window.
6. Run /mcp
7. User should be able to see the pre-registered gerrit-mcp-server with status Connected.
8. Now user can run any Gerrit related query and Gemini will query the MCP server for answers.

##### Android Studio and Android Studio for Platform
1. Open workstation with Android Studio or Android Studio for Platform
2. Go to Settings > Search MCP > Enable
3. In Settings > Google Accounts > Add an account > Login
4. Open Gemini Code Assist from sidebar
5. Complete initial setup > Select “Gemini for Businesses”
6. Select “Google Cloud Project” and complete further setup
7. MCP Setup: See below

###### MCP Setup

Gemini Code Assist uses `mcp.json` file as configuration for interacting with MCP servers.
Steps:
  - We need to create a symlink for `mcp.json` file with `settings.json` file so that `gemini-mcp-setup` keeps both in sync. See Known Issues for why we need this manual setup.
  Steps:
  - Open Terminal
  - Run the following commands as per the image
  - For Android Studio 
  ```bash
  rm -rf ~/.config/Google/AndroidStudioForPlatform2025.2.2/mcp.json
  ln -s ~/.gemini/settings.json ~/.config/Google/AndroidStudioForPlatform2025.2.2/mcp.json
  ```
  - For Android Studio for Platform
  ```bash
   rm -rf ~/.config/Google/AndroidStudio2025.2.3/mcp.json
  ln -s ~/.gemini/settings.json ~/.config/Google/AndroidStudio2025.2.3/mcp.json
  ```
  - Restart IDE.
  - It is recommended to increase the token expiry time in Keycloak in order to avoid intermittent issues. See Known Issues

### Known Issues
- While registering new MCP server, make sure to enter the path with trailing slashes on both ends, e.g. `/my-mcp-server/`. Otherwise, Gemini-CLI and Gemini Code Assist will not be able to connect to the server.
- Server is disabled by default on new registration, including the pre-registered gerrit-mcp-server. Make sure to ENABLE it before use.
- The `gemini-mcp-setup` script does NOT configure the `mcp.json` config file used by Android Studio IDE right now, because in order to load refreshed or synced version of mcp.json file, the IDE must be restarted.
  - As a temporary workaround, user must create a symlink from `~/.gemini/settings.json` to the respective IDE’s mcp.json file location. See steps above.
  - Note that this is a temporary workaround that works only until JWT Bearer access_token in mcp.json does not expire.
  - Once the token expires, the file must be loaded again manually at “Settings > Tools > AI > MCP” or IDE must be restarted to load synced version of mcp.json.
  - Hence, it is recommended to increase the token expiry time in Keycloak in order to avoid intermittent MCP auth issues.