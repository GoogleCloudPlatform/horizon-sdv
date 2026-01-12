# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

#!/usr/bin/env python3
import json
import time
import sys
import argparse
import subprocess
from pathlib import Path
import os
import signal
import requests
from requests import HTTPError
import tempfile
import shutil

# --- Configuration ---
HORIZON_DOMAIN = os.environ.get("HORIZON_DOMAIN", "##DOMAIN##")  # e.g., myenv.horizon-sdv.com

KEYCLOAK_URL = os.environ.get("KEYCLOAK_URL", f"https://{HORIZON_DOMAIN}/auth")
REALM = os.environ.get("REALM", "horizon")
CLIENT_ID = os.environ.get("CLIENT_ID", "mcp-gateway-registry-cli")

MCP_REGISTRY_URL = os.environ.get("MCP_REGISTRY_URL", f"https://mcp.{HORIZON_DOMAIN}")
GEMINI_CONFIG_DIR = Path.home() / ".gemini"
GEMINI_SETTINGS_FILE = GEMINI_CONFIG_DIR / "settings.json"
TOKEN_FILE = GEMINI_CONFIG_DIR / "mcp-gateway-registry-token.json"

# Daemon state; Used as a single-instance state file for both daemon and foreground sync loops
DAEMON_STATE_FILE = GEMINI_CONFIG_DIR / "mcp-gateway-sync-state.json"
DAEMON_LOG_FILE = GEMINI_CONFIG_DIR / "mcp-gateway-sync.log"

OIDC_URL = f"{KEYCLOAK_URL}/realms/{REALM}/protocol/openid-connect"
DEVICE_AUTH_URL = f"{OIDC_URL}/auth/device"
TOKEN_URL = f"{OIDC_URL}/token"

EXPIRY_SAFETY_SECONDS = 60 # Refresh tokens this many seconds before expiry


def ensure_config_dir():
    """Ensure Gemini config directory and settings file exist."""
    GEMINI_CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    if not GEMINI_SETTINGS_FILE.exists():
        with open(GEMINI_SETTINGS_FILE, 'w') as f:
            json.dump({}, f, indent=2)


def device_login():
    """Initiates Device Authorization Flow and polls for token."""
    print(f"[*] Initiating Device Auth with {CLIENT_ID}...")

    try:
      resp = requests.post(DEVICE_AUTH_URL, data={"client_id": CLIENT_ID}, timeout=30)
      resp.raise_for_status()
      device_data = resp.json()

      device_code = device_data["device_code"]
      verification_uri = device_data["verification_uri_complete"]
      interval = device_data.get("interval", 5)
      expires_in = device_data.get("expires_in", 300)

      print(f"\nPlease authenticate via browser:\n\n   {verification_uri}\n")
      print(f"Waiting for login... (Expires in {expires_in}s)")

      start_time = time.time()
      while time.time() - start_time < expires_in:
          time.sleep(interval)

          token_resp = requests.post(TOKEN_URL, data={
              "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
              "device_code": device_code,
              "client_id": CLIENT_ID
          }, timeout=30)

          if token_resp.status_code == 200:
              print("\n[+] Login Successful!")
              return token_resp.json()

          err = token_resp.json().get("error")
          if err == "authorization_pending":
              continue
          elif err == "slow_down":
              interval += 2
          else:
              raise RuntimeError(f"Device flow failed: {err}")
    except HTTPError as e:
        if e.response is not None and e.response.status_code == 401:
            raise RuntimeError(
                "Device authorization failed. This may indicate an issue with the Keycloak server. "
                "Please try again or contact your administrator."
            )
        elif e.response is not None and e.response.status_code == 403:
            raise RuntimeError(
                "Access forbidden. Your account may not have permission to use this service. "
                "Please contact your administrator."
            )
        else:
            raise RuntimeError(f"Server error ({e.response.status_code}): {e.response.text}")
    except requests.exceptions.ConnectionError as e:
        raise RuntimeError(f"Cannot connect to {DEVICE_AUTH_URL}. Check network/DNS.")
    except requests.exceptions.Timeout:
        raise RuntimeError(f"Device flow request timed out.")
    except requests.exceptions.RequestException as e:
        raise RuntimeError(f"Login failed: {e}")


def refresh_access_token(refresh_token):
    try:
      resp = requests.post(TOKEN_URL, data={
          "grant_type": "refresh_token",
          "refresh_token": refresh_token,
          "client_id": CLIENT_ID
      }, timeout=30)
      resp.raise_for_status()
      return resp.json()
    except HTTPError as e:
        if e.response is not None and e.response.status_code == 401:
            raise RuntimeError(
                f"Refresh token expired or revoked. Please run --login to re-authenticate."
            )
        elif e.response is not None and e.response.status_code == 403:
            raise RuntimeError(
                "Access forbidden. Your account may not have permission to access this service. "
                "Please contact your administrator."
            )
        else:
            raise RuntimeError(
                f"Token refresh failed. Server error ({e.response.status_code}): {e.response.text}. "
                "Please run --login to re-authenticate."
            )
    except requests.exceptions.ConnectionError as e:
        raise RuntimeError(f"Cannot connect to {TOKEN_URL}. Check network/DNS.")
    except requests.exceptions.Timeout:
        raise RuntimeError(f"Token refresh request timed out.")
    except requests.exceptions.RequestException as e:
        raise RuntimeError(f"Token refresh failed: {e}")


def fetch_mcp_servers(access_token):
    headers = {"Authorization": f"Bearer {access_token}"}
    resp = requests.get(f"{MCP_REGISTRY_URL}/api/servers", headers=headers, timeout=30)
    resp.raise_for_status()
    data = resp.json()
    return data.get("servers", [])


def save_token_file(token_data):
    """
    Save token file atomically to prevent corruption.

    1. Write to a temporary file first
    2. If successful, replace the real file
    3. If crash happens, temp file is abandoned (real file untouched)
    """
    token_data = dict(token_data)
    token_data["obtained_at"] = int(time.time())

    # Create a temporary file in the same directory
    fd, temp_path = tempfile.mkstemp(
        dir=GEMINI_CONFIG_DIR,
        prefix=".token-temp-", # Hidden file
        suffix=".json"
    )

    # Write to temp file
    try:
        with os.fdopen(fd, 'w') as f:
            json.dump(token_data, f, indent=2)
        
        # Atomically replace old file with new file
        shutil.move(temp_path, TOKEN_FILE)
    except Exception:
        # Clean up the temp file
        try:
            os.unlink(temp_path)
        except:
            pass
        raise  # Re-raise the original exception


def load_token_file():
    with open(TOKEN_FILE, 'r') as f:
        return json.load(f)


def is_access_token_fresh(token_data, *, safety_seconds=EXPIRY_SAFETY_SECONDS):
    """Checks if access token is still valid with safety margin."""
    access_token = token_data.get("access_token")
    expires_in = token_data.get("expires_in")
    obtained_at = token_data.get("obtained_at")

    if not access_token or not isinstance(expires_in, (int, float)) or not isinstance(obtained_at, (int, float)):
        return False

    expires_at = int(obtained_at) + int(expires_in)
    now = int(time.time())
    return now < (expires_at - int(safety_seconds))


def try_get_access_token_noninteractive():
    """Tries to get access token from existing token file, refreshing if needed."""
    if not TOKEN_FILE.exists():
        return None

    try:
        token_data = load_token_file()
    except Exception:
        return None

    if is_access_token_fresh(token_data):
        return token_data.get("access_token")

    refresh_token = token_data.get("refresh_token")
    if not refresh_token:
        return None

    try:
        new_tokens = refresh_access_token(refresh_token)
    except Exception:
        return None

    save_token_file(new_tokens)
    return new_tokens.get("access_token")


def get_access_token_interactive_fallback():
    """Gets access token, trying existing session first, else interactive login."""
    access = try_get_access_token_noninteractive()
    if access:
        print("[*] Using existing session (no browser login needed).")
        return access

    token_data = device_login()
    save_token_file(token_data)
    return token_data["access_token"]


def update_gemini_config(access_token, servers, *, prune=False, force=False):
    try:
        with open(GEMINI_SETTINGS_FILE, 'r') as f:
            config = json.load(f)
    except json.JSONDecodeError:
        config = {}

    if not isinstance(config, dict):
        config = {}

    # Clear existing mcpServers if force is set
    if force:
        config["mcpServers"] = {}

    # Initialize mcpServers if missing or invalid
    if "mcpServers" not in config or not isinstance(config["mcpServers"], dict):
        config["mcpServers"] = {}

    existing = config["mcpServers"]
    base = MCP_REGISTRY_URL.rstrip("/") # base URL without trailing slash

    desired_names = set()
    for server in servers:
        s_name = server.get("display_name")
        s_path = server.get("path")
        if not s_name or not s_path:
            print(f"[!] Skipping server with missing fields: {server}")
            continue

        desired_names.add(s_name) # track latest server names to prune unmanaged ones later
        s_path = "/" + s_path.strip("/")
        s_url = f"{base}{s_path}/mcp"

        existing[s_name] = {
            "httpUrl": s_url,
            "headers": {"Authorization": f"Bearer {access_token}"}
        }
        print(f"[+] Upserted MCP server (registry): {s_name}")

    # Prune previously managed servers that are no longer in the desired list if prune is set
    if prune:
        to_delete = []
        for name, entry in existing.items():
            # Check if server is managed (points to our registry)
            if is_managed_server(entry, base):
                # If managed but not in current desired list, mark for deletion
                if name not in desired_names:
                    to_delete.append(name)

        # Delete the marked servers
        for name in to_delete:
            del existing[name]
            print(f"[-] Pruned MCP server (registry, no longer exists): {name}")

    with open(GEMINI_SETTINGS_FILE, 'w') as f:
        json.dump(config, f, indent=2)

    print(f"[*] Updated {GEMINI_SETTINGS_FILE}.")


def is_managed_server(entry, registry_base_url):
    """
    Check if a server entry is managed by MCP Gateway Registry.
    Managed servers are identified by their URL pointing to the registry.
    """
    if not isinstance(entry, dict):
        return False
    
    http_url = entry.get("httpUrl", "")
    # Normalize URLs for comparison
    normalized_url = http_url.rstrip("/")
    normalized_base = registry_base_url.rstrip("/")
    
    # Server is managed if its URL starts with our registry base
    return normalized_url.startswith(normalized_base)


def fetch_servers_with_auto_retry(access_token):
    """Fetch servers, retrying once on 401 Unauthorized after refreshing token."""
    try:
        return fetch_mcp_servers(access_token)
    except HTTPError as e:
        if e.response is not None and e.response.status_code == 401:
            print("[*] Token expired, attempting refresh...")
            new_access = try_get_access_token_noninteractive()
            if new_access and new_access != access_token:
                try:
                    return fetch_mcp_servers(new_access)
                except HTTPError as e2:
                    if e2.response and e2.response.status_code == 401:
                        raise RuntimeError(
                            "Authentication failed even after token refresh. "
                            "Please run --login to re-authenticate."
                        )
                    raise # Re-raise other HTTP errors
            else:
                raise RuntimeError(
                    "Token expired and refresh failed. "
                    "Please run --login to re-authenticate."
                )
        elif e.response is not None and e.response.status_code == 403:
            raise RuntimeError(
                "Access forbidden (403). Ensure your user has appropriate MCP Registry access permissions. "
                "If you recently changed permissions, please wait a few minutes and try again."
            )
        else:
            raise RuntimeError(f"Server error ({e.response.status_code}): {e.response.text}")
    except requests.exceptions.ConnectionError as e:
        raise RuntimeError(f"Cannot connect to {MCP_REGISTRY_URL}. Check network/DNS.")
    except requests.exceptions.Timeout:
        raise RuntimeError(f"Request to {MCP_REGISTRY_URL} timed out.")
    except requests.exceptions.JSONDecodeError:
        raise RuntimeError(
            f"Registry returned invalid JSON. "
            f"Server may be experiencing issues. Please try again later."
        )
    except requests.exceptions.RequestException as e:
        raise RuntimeError(f"Request failed: {e}")


# ---Cross platform (Win + Linux) Daemon helpers for managing both foreground and daemon (background) sync loops---
def _pid_is_running(pid: int) -> bool:
    if pid <= 0:
        return False

    if sys.platform.startswith("win"):
        try:
            result = subprocess.run(
                ["tasklist", "/FI", f"PID eq {pid}", "/FO", "CSV", "/NH"],
                capture_output=True,
                text=True,
                check=False,
                timeout=5
            )
            # CSV format is more reliable
            return str(pid) in result.stdout
        except (FileNotFoundError, subprocess.TimeoutExpired):
            return False
    else:
        # POSIX existence check: signal 0 does not kill; it only checks.
        try:
            os.kill(pid, 0)
            return True
        except OSError:
            return False


def _read_daemon_state():
    if not DAEMON_STATE_FILE.exists():
        return None
    try:
        with open(DAEMON_STATE_FILE, 'r', encoding='utf-8') as f:
            return json.load(f)
    except Exception:
        return None


def _write_daemon_state(pid: int, mode: str):
    """Write daemon state atomically using temp file pattern."""
    fd, temp_path = tempfile.mkstemp(
        dir=GEMINI_CONFIG_DIR,
        prefix=".state-temp-",
        suffix=".json"
    )
    try:
        with os.fdopen(fd, 'w') as f:
            json.dump({
                "pid": pid,
                "mode": mode,
                "script": str(Path(__file__).resolve()),
                "started_at": int(time.time()),
            }, f, indent=2)
        shutil.move(temp_path, DAEMON_STATE_FILE)
    except Exception:
        try:
            os.unlink(temp_path)
        except:
            pass
        raise


def _clear_daemon_state():
    try:
        DAEMON_STATE_FILE.unlink(missing_ok=True)
    except Exception:
        pass
    

def daemon_status():
    """Check if any sync loop (daemon or foreground) is running."""
    state = _read_daemon_state()
    if not state:
        return False, None, None

    pid = int(state.get("pid", 0)) or None
    mode = state.get("mode")
    script = state.get("script")

    # Check if the recorded script matches this script
    if script != str(Path(__file__).resolve()):
        return False, pid, mode

    # Check if the recorded PID is running
    if pid and _pid_is_running(pid):
        return True, pid, mode

    # Script not running; clean up stale state
    _clear_daemon_state()
    return False, None, None


def daemon_stop():
    running, pid, mode = daemon_status()
    if not pid:
        print("[*] Sync is not running.")
        return

    if not running:
        print(f"[*] Sync is not running (stale PID {pid}). Cleaning state.")
        _clear_daemon_state()
        return

    print(f"[*] Stopping sync (mode={mode}, PID {pid})...")
    try:
        if sys.platform.startswith("win"):
            subprocess.run(["taskkill", "/PID", str(pid), "/F"], check=False, timeout=5)  # ← ADDED: timeout
        else:
            os.kill(pid, signal.SIGTERM)
            time.sleep(0.5)
            if _pid_is_running(pid):
                os.kill(pid, signal.SIGKILL)
    except subprocess.TimeoutExpired:
        print("[!] Warning: Stop command timed out")
    finally:
        _clear_daemon_state()

    print("[*] Stop requested.")


def _start_detached_sync_process():
    """Start the current script in --watch mode but as a detached background process (daemon)."""
    python_exe = str(Path(sys.executable).resolve())
    script_path = str(Path(__file__).resolve())
    args = [python_exe, script_path, "--watch"]

    if sys.platform.startswith("win"):
        if python_exe.lower().endswith("python.exe"):
            pythonw_exe = python_exe[:-10] + "pythonw.exe"
            if Path(pythonw_exe).exists():
                python_exe = pythonw_exe
                args[0] = python_exe  # Update first arg
        
        creationflags = 0
        if hasattr(subprocess, "CREATE_NEW_PROCESS_GROUP"):
            creationflags |= subprocess.CREATE_NEW_PROCESS_GROUP
        if hasattr(subprocess, "DETACHED_PROCESS"):
            creationflags |= subprocess.DETACHED_PROCESS

        p = subprocess.Popen(
            args,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            creationflags=creationflags
        )
        return p.pid

    # POSIX detach: new session; parent can exit cleanly
    p = subprocess.Popen(
        args,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        close_fds=True,
        start_new_session=True
    )
    return p.pid


def ensure_no_parallel_sync():
    running, pid, mode = daemon_status()
    if running:
        raise SystemExit(
            f"Sync is already running (mode={mode}, PID={pid}). "
            f"Stop it first. If its a daemon, use '--daemon-stop' option."
        )


def daemon_start():
    """Start daemon background sync with verification."""
    ensure_no_parallel_sync()

    spawned_pid = _start_detached_sync_process()
    print(f"[*] Started background sync (initial PID {spawned_pid}).")
    
    max_wait = 5
    start = time.time()
    while time.time() - start < max_wait:
        time.sleep(0.5)
        running, actual_pid, mode = daemon_status()
        if running and actual_pid:
            print(f"[*] Daemon confirmed running (PID {actual_pid}).")
            return
    
    print(f"[!] Warning: Could not confirm daemon started within {max_wait}s.")
    print(f"[!] Check {DAEMON_LOG_FILE} for errors.")


def sync_loop(*, prune=False, force=False):
    """
    Foreground sync loop (refresh token + update Gemini config).
    When started via --daemon-start, it runs detached in the background.
    """

    # Prevent parallel runs (also auto-cleans stale state)
    ensure_no_parallel_sync()

    # Write current PID and mode to state file
    mode = "foreground" if sys.stdin.isatty() else "daemon"  # detect if run in terminal (a tty)
    _write_daemon_state(os.getpid(), mode)

    # Set up logging for daemon mode
    log_handle = None
    if mode == "daemon":
        log_handle = open(DAEMON_LOG_FILE, "a", encoding="utf-8", buffering=1)  # line-buffered
        sys.stdout = log_handle
        sys.stderr = log_handle
        print(f"\n[*] Daemon started at {time.strftime('%Y-%m-%d %H:%M:%S')}")

    # Load existing token file
    try:
        token_data = load_token_file()
    except Exception:
        raise SystemExit(f"Token file not found/invalid: {TOKEN_FILE}. Run --login first.")

    # Loop starts here
    try:
        while True:
            refresh_token = token_data.get("refresh_token")
            if not refresh_token:
                raise SystemExit("No refresh_token available; login again.")

            expires_in = token_data.get("expires_in", 300)
            sleep_time = max(10, int(expires_in) - EXPIRY_SAFETY_SECONDS)
            try:
                time.sleep(sleep_time)
            except KeyboardInterrupt:
                # User pressed Ctrl+C during sleep
                raise  # Re-raise to outer handler

            try:
                new_tokens = refresh_access_token(refresh_token)
            except RuntimeError as e:
                # refresh_access_token already converted to RuntimeError
                raise SystemExit(f"Token refresh failed: {e}. SSO session likely expired. Please run --login to re-authenticate.")
            except Exception as e:
                # Unexpected error
                raise SystemExit(f"Unexpected error during token refresh: {e}")

            token_data = new_tokens
            save_token_file(token_data)

            servers = fetch_mcp_servers(token_data["access_token"])
            if not servers:
                # In watch mode, log warning but don't crash
                print(f"[!] No servers in registry at {time.strftime('%H:%M:%S')}")
                # Skip config update this cycle
            else:
                update_gemini_config(token_data["access_token"], servers, prune=prune, force=force)
    except KeyboardInterrupt:
        # Graceful shutdown
        if mode == "foreground":
            print("\n[*] Sync stopped by user (Ctrl+C)")
        # Daemon mode: just exit silently
    except Exception as e:
        print(f"[!] Unexpected error in sync loop: {e}")
        import traceback
        traceback.print_exc()
    finally:
        _clear_daemon_state()

        if log_handle:
            try:
                log_handle.flush()
                log_handle.close()
                sys.stdout = sys.__stdout__
                sys.stderr = sys.__stderr__
            except:
                pass


def main():
    parser = argparse.ArgumentParser(
        description="""
        Automatic MCP servers config setup for Gemini CLI.
        
        Authenticates with MCP Gateway Registry via Keycloak (Device Flow), fetches JWT token, MCP servers and updates ~/.gemini/settings.json with fresh data.
        Optionally allows continuous sync with both foreground and background (daemon) modes.""",
        formatter_class=argparse.RawDescriptionHelpFormatter
    )

    primary_mode = parser.add_mutually_exclusive_group()
    primary_mode.add_argument(
        "--login",
        action="store_true",
        help="Default mode. Interactive login if needed. Reuses existing token/refresh token first to avoid repeated browser login."
    )
    primary_mode.add_argument(
        "--watch",
        action="store_true",
        help="Continuously sync server list and refresh token in foreground mode. Use Ctrl+C to stop."
    )
    primary_mode.add_argument(
        "--daemon-status",
        action="store_true",
        help="Show background sync status and exit."
    )
    primary_mode.add_argument(
        "--daemon-stop",
        action="store_true",
        help="Stop background sync and exit."
    )

    parser.add_argument(
        "--daemon-start",
        action="store_true",
        help="Continuously sync server list and refresh token in (background) daemon mode."
    )

    config_mode = parser.add_mutually_exclusive_group()
    config_mode.add_argument(
        "--prune",
        action="store_true",
        help="Sync removes managed servers no longer in registry. Does not affect non-managed servers."
    )
    config_mode.add_argument(
        "--force",
        action="store_true",
        help="Sync replaces entire mcpServers block with only registry servers (more destructive, removes all non-managed servers)."
    )

    args = parser.parse_args()
    ensure_config_dir()

    if (args.daemon_status or args.daemon_stop or args.watch) and args.daemon_start:
        parser.error("--daemon-start is not valid with options --watch, --daemon-status, or --daemon-stop.")
    if (args.daemon_status or args.daemon_stop) and (args.prune or args.force):
        parser.error("--prune and --force are not valid with options --daemon-status or --daemon-stop.")

    if not HORIZON_DOMAIN or HORIZON_DOMAIN == "##DOMAIN##":
        raise SystemExit(
            "ERROR: HORIZON_DOMAIN environment variable not set.\n"
            "Example: export HORIZON_DOMAIN=myenv.horizon-sdv.com"
        )

    if args.daemon_status:
        running, pid, mode = daemon_status()
        if running:
            print(f"[*] Sync is running (mode={mode}, PID {pid}).")
        else:
            print("[*] Sync is not running.")
        return

    if args.daemon_stop:
        daemon_stop()
        return

    if args.watch:
        print("[*] Starting foreground continuous sync (Ctrl+C to stop)...")
        print(f"[*] Options: prune={args.prune}, force={args.force}")
        sync_loop(prune=args.prune, force=args.force)
        return

    # Default flow
    if not args.login:
        args.login = True

    ensure_no_parallel_sync()
    access_token = get_access_token_interactive_fallback()
    servers = fetch_servers_with_auto_retry(access_token)
    if not servers:
        raise RuntimeError(f"No servers returned from {MCP_REGISTRY_URL}/api/servers.")

    if args.force:
        print("[!] --force is set: this will replace the entire mcpServers block in settings.json.")

    update_gemini_config(access_token, servers, prune=args.prune, force=args.force)

    if args.daemon_start:
        daemon_start()
        print("[*] Done.")
        return

    is_running, pid, mode = daemon_status()
    if is_running:
        print(f"[*] Background sync is already active (PID {pid}, mode={mode}).")
    else:
        try:
            answer = input("Start background sync for this session now? (y/N): ").strip().lower()
            if answer in ("y", "yes"):
                daemon_start()
            elif answer in ("n", "no", ""):
                print("[*] Background sync not started.")
        except KeyboardInterrupt:
            print("\n[*] Operation cancelled.")

    print("[*] Done.")


if __name__ == "__main__":
    main()
