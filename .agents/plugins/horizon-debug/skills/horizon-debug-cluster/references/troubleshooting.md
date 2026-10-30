<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Troubleshooting: Horizon cluster debug

**`Reauthentication required.` / expired or missing gcloud credentials**
This is an authentication failure. Follow rule 6 in the main skill: tell the user to run `gcloud auth login <ACCOUNT>` in their own terminal. Do not run it for them. Use the account named in the error. If the error names none, use `gcloud auth login`.

**`kubectl` commands hang or return 403 Forbidden**
Confirm the account has one of the IAM roles listed in the main skill (`roles/gkehub.gatewayReader/Editor/Admin`), and confirm credentials were generated for the correct project: `gcloud config list` should show the expected `project`.

**`gcloud container fleet memberships list` returns nothing**
The account likely lacks `gkehub.viewer` (or higher) on the project, or the cluster is not fleet-registered. Tell the user this account cannot reach the cluster and stop. Do not change IAM or fleet settings, and do not work around it by installing different tools.

**`resolve-target.sh` exits 2 for a URL ("No project you can access serves ...")**
The scan only sees projects returned by `gcloud projects list` for the active account, and only projects where the account can read Certificate Manager (`certificatemanager.certmapentries.list`). Check:
- `gcloud auth list` shows the account that has access to the Horizon project.
- The URL is the Horizon environment hostname (`<env_name>.<domain_name>`), e.g. copied from the portal address bar.
- The environment still exists. If the owner knows the project ID, connect with that instead.

**`resolve-target.sh` exits 3 ("Still ambiguous")**
More than one project has the hostname in its `horizon-sdv-map`, and the DNS IP tiebreaker could not choose (DNS not yet pointing at either, or `dig` missing). Typically one is a stale or in-progress redeploy. Ask the user which project is current.

**A remembered target connects to the wrong project or account**
Remove it and resolve again: `bash <SKILL_DIR>/scripts/targets.sh remove <HOSTNAME>`. `targets.sh list` shows every remembered entry and `targets.sh path` prints the file location. Set `HORIZON_TARGETS_FILE` to use a different file.

**URL scan is slow**
Each visible project costs one Certificate Manager API call. Increase concurrency with `--parallel 32`, or connect by project ID, which needs no scan.

**`resolve-target.sh` exits 0 for a project ID but prints no `HOSTNAME=`**
The project has no `horizon-sdv-map` (Certificate Manager API disabled, no Horizon deployment, or no permission to read it). Tell the user no URL could be confirmed and ask whether to continue anyway.

**`kubectl` returns 403 after switching contexts between clusters owned by different accounts**
Connect Gateway contexts authenticate with whatever account the *active* gcloud configuration holds, not the account that created the context. Activate the matching configuration as well as the context:
```bash
gcloud config configurations list                    # find horizon-<PROJECT_ID> or default
gcloud config configurations activate <CONFIG>
kubectl config use-context connectgateway_<PROJECT_ID>_<location>_<membership>
```

**`resolve-target.sh` says the account is not signed in or credentials expired**
Follow rule 6 in the main skill: tell the user to run `gcloud auth login <ACCOUNT>` in their own terminal. Do not run it for them. Temporary lab accounts expire when the lab ends; a new lab gives a new account and project.

**Credentials work but for the wrong cluster/project**
Re-run the "Set the active GCP project" and "Generate the kubectl context" steps with the correct `<GCP_PROJECT_ID>` and `<MEMBERSHIP_NAME>`.
- `kubectl config current-context` shows which project/cluster is currently active.
- `kubectl config get-contexts` lists all cached contexts to switch back and forth without re-running `get-credentials`.

**`gcloud: command not found` after install (Windows/WSL)**
The quick-install script updates the shell rc file (e.g. `.bashrc`) for the current WSL user. Run `exec -l $SHELL` or open a new WSL terminal so `PATH` picks up the change.

**`gcloud: command not found` only when called through `wsl.exe` (Cursor on Windows)**
`wsl.exe -e bash -lc` is non-interactive, and Ubuntu's `.bashrc` returns before the lines the gcloud installer added. Start each call with `source ~/google-cloud-sdk/path.bash.inc 2>/dev/null;`. If gcloud lives elsewhere, find it in a WSL terminal with `command -v gcloud`.

**`$'\r': command not found`, `set: -: invalid option` or `syntax error: unexpected end of file` when running a script in WSL**
The script has Windows (CRLF) line endings. Copy the scripts into WSL with carriage returns removed, as in the main skill's Windows (WSL) section, and run the copies.

**Commands work in a WSL terminal but not from Cursor on Windows**
- More than one WSL distribution: `wsl.exe -l -v`, then add `-d <DISTRO>` for the one that has gcloud.
- Quoting: PowerShell 5.1 drops embedded `"` when calling `wsl.exe`. Use single quotes around the bash command, double any `'` inside it, or use `pwsh` 7.3+.
- State: each `wsl.exe` call is a new shell, so chain dependent commands in one call.

**Windows `kubectl` or Lens can't see the cluster after connecting from WSL**
The context was written to `~/.kube/config` inside WSL, not `%USERPROFILE%\.kube\config`. Use `kubectl` from WSL, or set up the Windows tools separately.

**`gcloud components install` is disabled (Linux/WSL, apt-installed gcloud)**
Use the distro package manager instead:
```bash
sudo apt-get install kubectl google-cloud-cli-gke-gcloud-auth-plugin
```

**`kubectl get ns` succeeds but returns a permissions error on specific namespaces**
This is an in-cluster RBAC issue, not a Connect Gateway/IAM issue — the fleet IAM role only grants reachability to the gateway. Ask the Horizon platform admin for a Role/ClusterRoleBinding on the required namespace.
