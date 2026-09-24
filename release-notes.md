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

# Horizon SDV Release Notes

<p>Release Notes document is the public document which provides a brief information for the new features, improvements and bug fixes included in a Horizon SDV delivery.</p>
<p>The file ‘release-notes.md’ is stored in <a href="https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/release-notes.md">https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/release-notes.md</a>  directory.</p>
<p>Additional extended release notes for a particular release can be stored in /doc/extended-release-notes/ folder.</p>
<p><a href="https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/extended-release-notes/release-notes-2-0-0.md">https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/extended-release-notes/release-notes-2-0-0.md</a></p>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 4.3.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>DD.MM.2026</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p><strong>Horizon SDV 4.3.0</strong> is the minor release which extends the Horizon Developer Portal with a prebuilt, downloadable <strong>Horizon CLI</strong> for all supported platforms, and introduces the first <strong>EXPERIMENTAL partner contribution</strong> to Horizon: the <strong>RemotiveTopology (System of Systems)</strong> module contributed by RemotiveLabs.</p>

<p>Horizon 4.3.0 also delivers feature improvements and security fixes for vulnerable open-source modules used in Horizon containers.</p>

<p>Horizon SDV 4.3.0 package offers fully verified and documented upgrade patch (from Rel.4.2.0 to Rel.4.3.0). (see details in /docs/guides/upgrade_guide_4_2_0_to_4_3_0.md)</p>

<h2>New Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1841</p>
</td>

<td valign="top" width="24%"><p><strong>Horizon Portal enhancements - Download Horizon CLI</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Summary</strong></p>
<p>Signed-in Developer Portal users can download a ready-to-run Horizon CLI binary for their OS/architecture. The binary matches the CLI built into that cluster&#39;s Developer Portal image, so end users no longer need to clone the repository and install a Go development environment to build the CLI themselves. Building from source remains possible; the portal download is the documented default install path.</p>
<p>Binaries are <strong>not</strong> served as unauthenticated static files. They are served only through <code>/api/cli</code> after the same OIDC Bearer check as Module Manager.</p>
<p><strong>Supported platforms</strong></p>
<ul>
<li>
<p>Linux amd64</p>
</li>

<li>
<p>Linux arm64</p>
</li>

<li>
<p>Windows amd64</p>
</li>

<li>
<p>macOS on Apple Silicon (darwin arm64)</p>
</li>
</ul>
<p><strong>Changes</strong></p>
<p><strong>Horizon CLI (TAA-2060)</strong></p>
<ul>
<li>
<p>Add <code>horizon version [--output text|json]</code> reporting the version and build date stamped at build time (local builds report <code>dev</code>).</p>
</li>

<li>
<p>Add <code>tools/horizon/mkdist.go</code> to cross-compile the supported platforms, emit SHA-256 checksums, and write <code>manifest.json</code>.</p>
</li>
</ul>
<p><strong>Developer Portal (TAA-2060 / TAA-2111)</strong></p>
<ul>
<li>
<p>Proxy: <code>GET /api/cli/v1/manifest</code> and <code>GET /api/cli/v1/download/{os}/{arch}</code> (<code>proxy/cli.go</code>), served from <code>CLI_DIST_DIR</code> (default <code>/cli-dist</code>).</p>
</li>

<li>
<p>New <strong>Tools</strong> page at <code>/tools</code> (sidebar and Welcome page) with a tab-based layout for future tools. The <strong>Horizon CLI</strong> tab detects the user&#39;s platform, offers the preferred download, shows version and build date (dd/mm/yyyy), and allows copying the SHA-256 checksum.</p>
</li>

<li>
<p>Platform-specific installation and verification instructions on the Tools page.</p>
</li>
</ul>
<p><strong>Image / GitOps / Terraform (TAA-2060)</strong></p>
<ul>
<li>
<p>Multi-stage Developer Portal Dockerfile: the CLI is cross-compiled during the image build and shipped in <code>/cli-dist</code> of the distroless image.</p>
</li>

<li>
<p><code>tools/horizon</code> sources are staged into the portal Docker build context, so the portal image is rebuilt when CLI sources change. <code>HORIZON_CLI_VERSION</code> is taken from the portal <code>build_version</code>.</p>
</li>

<li>
<p><code>horizon-dev-portal</code> bumped <strong>1.1.0 → 4.3.0</strong> (<code>terraform/modules/base/locals.tf</code>, GitOps <code>values.yaml</code>).</p>
</li>
</ul>
<p><strong>Documentation (TAA-2080)</strong></p>
<ul>
<li>
<p><code>docs/developer_portal_user_guide.md</code>: Download from Developer Portal is the recommended install path, including macOS arm64 steps (removal of the quarantine attribute for the unsigned binary).</p>
</li>
</ul>
<p><strong>Actions</strong></p>
<ol start="1">
<li>
<p>Deploy the updated <code>horizon-dev-portal</code> image (4.3.0) per your environment rollout.</p>
</li>

<li>
<p>Sign in to the Developer Portal (<code>https://&lt;your-horizon-domain&gt;/developer-portal/</code>) and open <strong>Tools → Horizon CLI</strong>.</p>
</li>

<li>
<p>Download the binary for your platform, verify the SHA-256 checksum, and run <code>horizon version</code>. The version and build date match the values shown on the Tools page.</p>
</li>
</ol>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-2057</p>
</td>

<td valign="top" width="24%"><p><strong>RemotiveTopology (System of Systems) module - RemotiveLabs partner contribution [EXPERIMENTAL]</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Summary</strong></p>
<p>First partner contribution integrated into Horizon according to the partner contribution guidelines (<code>docs/contributing.md</code>), contributed by RemotiveLabs (<a href="https://github.com/GoogleCloudPlatform/horizon-sdv/pull/58">GoogleCloudPlatform/horizon-sdv#58</a>). The new <code>remotive-topology</code> module runs <a href="https://docs.remotivelabs.com/">RemotiveTopology</a> vehicle-system simulations (&quot;system of systems&quot;) on ephemeral GCE VMs, following the Horizon SDV Packer → KCC <code>ComputeInstanceTemplate</code> → ephemeral GCE driver pattern (parity with <code>cf_instance_template</code> and <code>cvd_launcher</code>).</p>
<p>The module is <strong>EXPERIMENTAL</strong>: behavior, parameters and availability can change without notice.</p>
<p><strong>Changes</strong></p>
<p><strong>New module</strong></p>
<ul>
<li>
<p><code>gitops/modules/remotive-topology</code> registered in the Module Manager catalog (hard dependency: <code>workloads-common</code>) with a Developer Portal overview page.</p>
</li>

<li>
<p>Argo workflows under <code>workloads/remotive/pipelines</code>:</p>

<ul>
<li>
<p><code>remotive-builder-image</code>: builder container image (packer, gcloud, kubectl) built through the shared <code>common-docker-image-build</code> ClusterWorkflowTemplate.</p>
</li>

<li>
<p><code>remotive-instance-template</code>: Packer bake (Docker CE, remotivebusd, remotivelabs-cli) and publication of a KCC <code>ComputeInstanceTemplate</code>.</p>
</li>

<li>
<p><code>remotive-launcher</code>: per-run ephemeral VM (KCC <code>ComputeInstance</code> and GCS message passing, no SSH from the pod) that builds and runs the topology. The topology project is provided per run as a <code>.tgz</code> / <code>.tar.gz</code> archive (<code>topologyDownloadUrl</code>, <code>topologyName</code>); no topology project is shipped in the repository.</p>
</li>
</ul>
</li>

<li>
<p>Remotive <code>ComputeInstanceTemplate</code> CRs are published into a dedicated <code>&lt;prefix&gt;remotive-kcc</code> namespace. Disabling <code>workloads-android</code> no longer removes Remotive instance templates, and disabling <code>remotive-topology</code> removes them (TAA-2110).</p>
</li>
</ul>
<p><strong>Shared components</strong></p>
<ul>
<li>
<p>MTK Connect: new <code>MTK_CONNECT_TUNNEL_LIST</code> (named TCP tunnels), <code>MTK_CONNECT_DEVICE_NAME_LIST</code>, <code>MTK_CONNECT_TUNNEL_DEVICE_NAME</code> and <code>MTK_CONNECT_TERMINAL_USER</code> parameters. Topology ports and Android adb endpoints are exposed as MTK Connect tunnels and devices.</p>
</li>

<li>
<p>Module Manager <strong>0.3.2 → 0.3.3</strong>: startup sync injects <code>MODULE_CONFIG</code> only into parent Applications, so <code>workloads-android</code> / <code>workloads-common</code> no longer stay in UPDATE IN PROGRESS after a Module Manager restart; per-module KCC namespace teardown.</p>
</li>

<li>
<p>Cloud NAT (<code>terraform/modules/sdv-network/nat.tf</code>): dynamic port allocation enabled (64-4096 ports per VM) to support many concurrent egress connections.</p>
</li>

<li>
<p>CVD Launcher: support for newer <code>cvd</code> versions.</p>
</li>
</ul>
<p><strong>Actions</strong></p>
<ol start="1">
<li>
<p>Create the RemotiveCloud auth Secret with a revocable, least-privilege service-account token: <code>kubectl -n &lt;prefix&gt;workflows create secret generic workflow-remotive-cloud-auth --from-literal=token=&lt;token&gt; --from-literal=organization=&lt;organization-id&gt;</code>.</p>
</li>

<li>
<p>Enable <code>remotive-topology</code> in the Developer Portal (Administration → Modules) and wait until its status changes to READY.</p>
</li>

<li>
<p>Run <code>remotive-builder-image</code> once, then <code>remotive-instance-template</code>, then <code>remotive-launcher</code> per topology run.</p>
</li>
</ol>
<p>Documentation: <code>workloads/remotive/README.md</code>. Example topologies: <a href="https://github.com/remotivelabs/remotivelabs-topology-examples/">remotivelabs-topology-examples</a>.</p>
</td>
</tr>
</tbody>
</table>

<h2>Improved Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1244</p>
</td>

<td valign="top" width="24%"><p><strong>[Android] Mirror sync should show some status when syncing</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Summary</strong></p>
<p>Android Mirror sync now reports visible progress during <code>repo sync</code> and enforces a configurable hard timeout, so long-running mirror syncs no longer look hung in the Jenkins console.</p>
<p><strong>Changes</strong></p>
<ul>
<li>
<p><code>repo sync</code> runs in the background with a heartbeat log line every 5 minutes (PID and elapsed time).</p>
</li>

<li>
<p>New Jenkins parameter <code>REPO_SYNC_TIMEOUT</code> (GNU <code>timeout</code> duration, default <code>20h</code>) on Sync Mirror. On timeout the whole <code>repo sync</code> process group is stopped and the job fails without retry.</p>
</li>

<li>
<p>Output is kept in a log file for post-mortem analysis.</p>
</li>
</ul>
<p><strong>Actions</strong></p>
<p>Run the Seed Workloads pipeline so the new <code>REPO_SYNC_TIMEOUT</code> parameter is added to Sync Mirror.</p>
</td>
</tr>
</tbody>
</table>

<h2>Documentation update</h2>

<ul>
<li><p>Rel.4.3.0 provides updates in Horizon documentation.</p>

<ul>
<li><p>New <strong>Upgrade Guide</strong> (/docs/guides/upgrade_guide_4_2_0_to_4_3_0.md) for Rel.4.2.0 -&gt; Rel.4.3.0 upgrade.</p>
</li>

<li><p>Developer Portal and Horizon CLI</p>

<ul>
<li><p><code>docs/developer_portal_user_guide.md</code> (Horizon CLI download and installation from the Developer Portal)</p>
</li>
</ul>
</li>

<li><p>RemotiveTopology (System of Systems) module</p>

<ul>
<li><p><code>workloads/remotive/README.md</code></p>
</li>
</ul>
</li>
</ul>
</li>
</ul>

<h2>Bug Fixes</h2>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="10%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Bug</strong></p>
</td>

<td valign="top" width="51%"><p><strong>Description</strong></p>
</td>

<td valign="top" width="15%"><p><strong>SHA</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2112</p>
</td>

<td valign="top" width="24%"><p>[Security] grpc-go module update (1.81.0 -&gt; 1.83.2)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Upgrades <code>google.golang.org/grpc</code> from <code>v1.81.0</code> to <code>v1.83.2</code> to resolve the reported security finding.</p>
<p><strong>Verification</strong></p>
<p>Deployed to a lab environment, enabled android workloads, triggered Seed jobs in Jenkins, and sanity-checked that application and admin components open correctly.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><em>pending merge</em></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2113</p>
</td>

<td valign="top" width="24%"><p>[Security] OSS Axios module update to 1.20.0</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Upgrades <code>axios</code> to <code>1.20.0</code> to resolve the reported security finding.</p>
<p><strong>Verification</strong></p>
<p>Deployed to a lab environment, enabled android workloads, triggered Seed jobs in Jenkins, and sanity-checked that application and admin components open correctly.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><em>pending merge</em></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2114</p>
</td>

<td valign="top" width="24%"><p>[Security] open-telemetry/opentelemetry-go module update to v1.46.0</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Upgrades <code>go.opentelemetry.io/otel</code> to <code>v1.46.0</code> to resolve the reported security finding.</p>
<p><strong>Verification</strong></p>
<p>Deployed to a lab environment, enabled android workloads, triggered Seed jobs in Jenkins, and sanity-checked that application and admin components open correctly.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><em>pending merge</em></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2120</p>
</td>

<td valign="top" width="24%"><p>[Security] PostCSS module update (8.5.21 -&gt; 8.5.26)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Upgrades <code>postcss</code> from <code>8.5.21</code> to <code>8.5.26</code> in <code>horizon-dev-portal</code> and <code>horizon-preflight/preflight-web</code>.</p>
<p><strong>Verification</strong></p>
<p>Deployed to a lab environment; Developer Portal CSS output and the Preflight workstation start screen render correctly.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><em>pending merge</em></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2121</p>
</td>

<td valign="top" width="24%"><p>[Security] nanoid module update (3.3.11 -&gt; 6.0.1)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Upgrades <code>nanoid</code> from <code>3.3.11</code> to <code>6.0.1</code> in <code>horizon-dev-portal</code> and <code>horizon-preflight/preflight-web</code>.</p>
<p><strong>Verification</strong></p>
<p>Deployed to a lab environment; Developer Portal CSS output and the Preflight workstation start screen render correctly.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><em>pending merge</em></p>
</li>
</ul>
</td>
</tr>
<tr>
<td valign="top" width="10%"><p>TAA-2149</p>
</td>

<td valign="top" width="24%"><p>[Download CLI] Deployment version and token path corrections after testing</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Corrections to the Horizon CLI download feature found during release testing.</p>
<p><strong>Changes</strong></p>
<ul>
<li><p>The Developer Portal image version is aligned with the release number: <code>horizon-dev-portal</code> is tagged <code>4.3.0</code> rather than continuing its own <code>1.1.x</code> line, in <code>terraform/modules/base/locals.tf</code> and the GitOps chart values.</p>
</li>
<li><p><code>horizon login</code> and <code>horizon logout</code> now print the token cache path actually in use, resolved through <code>tokenCachePath()</code>, instead of a hardcoded <code>~/.config/horizon/token.json</code> that could be wrong when the location differs.</p>
</li>
<li><p><code>docs/developer_portal_user_guide.md</code> extended with the corresponding CLI sections.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>491b4f171</code></p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Known Issues</h2>
<p><strong>Horizon CLI:</strong> Binaries downloaded from the Developer Portal are not code-signed. Windows SmartScreen may show a warning, and on macOS the quarantine attribute must be removed (<code>xattr -d com.apple.quarantine horizon</code>) before first run. See <code>docs/developer_portal_user_guide.md</code>.</p>

<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 4.2.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>01.09.2026</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p><strong>Horizon SDV 4.2.0</strong> is the minor release which includes important updates and extensions for Cloud Workstation deployment in Horizon including architecture shift to Guacamole/GNOME support and Antigravity Agent/CLI integration in new additional CW workloads with AS/ASfP/COS IDEs.</p>

<p>Horizon 4.2.0 also delivers several several feature improvements and important bug fixes including security fixes.</p>

<p>Horizon SDV 4.1.0 package offers fully verified and documented upgrade patch (from Rel.4.1.0 to Rel.4.2.0). (see details in /docs/guides/upgrade_guide_4_1_0_to_4_2_0.md)</p>

<h2>New Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1859</p>
</td>

<td valign="top" width="24%"><p><strong>Cloud Workstation Image upgrade</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Changes</strong></p>
<p>Android Studio and Android Studio for Platform (ASfP) Cloud Workstation images now use a layered Preflight → GNOME → child model instead of a self-contained noVNC / TigerVNC image.</p>
<ul>
<li>
<p>Browser access is Guacamole over RDP (<code>gnome-remote-desktop</code>). Dynamic resize and bidirectional clipboard are supported. Internal RDP/<code>guacd</code> stay inside the workstation; only port 80 is used through the Cloud Workstations gateway.</p>
</li>

<li>
<p>New images: <code>horizon-preflight</code> (loader / systemd / credential rendering) and <code>horizon-gnome</code> (shared desktop, Chrome, Gemini CLI, <code>gemini-mcp-agent</code>). ASfP and Android Studio are thin children of GNOME and no longer reinstall the desktop stack.</p>
</li>

<li>
<p>Legacy noVNC, websockify, and TigerVNC are removed from GNOME-based images.</p>
</li>

<li>
<p>Jenkins: leaf jobs remain under Workstation Images. A new Horizon AOSP Build Image Chain job (Workstation Image Chain folder) can build Preflight and/or GNOME, then ASfP and/or Android Studio, threading <code>BASE_IMAGE</code>. Each leaf job can still be run alone against a published GNOME tag.</p>
</li>

<li>
<p>Provisioning Terraform is unchanged. Select the new image with the existing <code>CONTAINER_IMAGE</code> parameter on Create/Update Configuration.</p>
</li>

<li>
<p><code>horizon-code-oss</code> is unchanged and stays on the legacy stack.</p>
</li>

<li>
<p>Follow-up on the GNOME base (TAA-2017): Chrome launcher restored after <code>dpkg-divert</code>, GNOME keyring unlocked for Secret Service, <code>antigravity://</code> handler registered, Guacamole RDP <code>resize-method=display-update</code>.</p>
</li>
</ul>
<p>Documentation: <code>docs/workloads/cloud-workstations/workstation_images.md</code>.</p>
<p><strong>Actions</strong></p>
<ol start="1">
<li>
<p>Seed Cloud Workstations so the Preflight, GNOME, ASfP, Android Studio, and Image Chain jobs exist.</p>
</li>

<li>
<p>Build and push images in order (Preflight → GNOME → children), or run Horizon AOSP Build Image Chain. Enabled base stages always push; child <code>NO_PUSH</code> still applies to IDE images.</p>
</li>

<li>
<p>On Create/Update Configuration, set <code>CONTAINER_IMAGE</code> to the new <code>horizon-android-studio</code> or <code>horizon-asfp</code> image. Use an N1 or N2 machine type with nested virtualization enabled (not <code>e2-standard-4</code> with nested virtualization off) with at least 60 GB of storage space.</p>
</li>

<li>
<p>Restart existing workstations after the config change so they pick up the new image.</p>
</li>

<li>
<p>Leave Code OSS configs on the existing Code OSS image.</p>
</li>
</ol>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1929</p>
</td>

<td valign="top" width="24%"><p><strong>Antigravity integration into CW pipelines</strong></p>
</td>

<td valign="top" width="64%"><p>Horizon Cloud Workstation images now include Google Antigravity alongside existing Gemini support.</p>
<ul>
<li>
<p>The Horizon Android Studio and Android Studio for Platform images ship the Antigravity 2.0 Agent (desktop), launchable from the GNOME app menu without autostart, so Android Studio and ASfP remain the primary IDE experience. These images also include the Antigravity CLI (<code>agy</code>).</p>
</li>

<li>
<p>The Horizon Code OSS image includes <code>agy</code> for terminal-based agent workflows.</p>
</li>
</ul>
<p>Antigravity IDE is not installed. Gemini CLI and Gemini Code Assist remain unchanged. The Agent is implemented for both remote-desktop architectures: legacy TigerVNC/X11 and for Guacamole/RDP on the GNOME-based stack.</p>
<p>A new <code>antigravity-mcp-agent</code> is included on these images to configure Horizon MCP Gateway Registry servers for Antigravity Agent and CLI (default config at <code>~/.gemini/config/mcp_config.json</code>). It shares Keycloak authentication with <code>gemini-mcp-agent</code> but writes a separate Antigravity MCP configuration. Install matrix, version pinning, and MCP setup are detailed in <code>docs/workloads/common/agentic-ai/antigravity.md</code> and <code>docs/guides/mcp_setup.md</code>.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1976</p>
</td>

<td valign="top" width="24%"><p><strong>GCS Storage module for Horizon</strong></p>
</td>

<td valign="top" width="64%"><ul>
<li>
<p>Added <code>storage-gcs-manager</code> module providing:</p>

<ul>
<li>
<p>CRDs: <code>GCSBucket</code>, <code>GCSArtifact</code></p>
</li>

<li>
<p>Operator reconciliation for GCS bucket lifecycle and artifact readiness</p>
</li>

<li>
<p>In-cluster HTTP API for GCS object operations, metadata/batch tools, signed URLs, and resumable uploads</p>
</li>
</ul>
</li>

<li>
<p>GitOps improvements for reliability:</p>

<ul>
<li>
<p>Correct Argo CD <strong>destination namespace</strong> to <code>horizon</code> for <code>mod-storage-gcs-manager</code></p>
</li>

<li>
<p>Correct <strong>sync wave ordering</strong> so CRs apply after the operator runs</p>
</li>

<li>
<p>Fixed RBAC to allow <code>*/status</code> updates</p>
</li>

<li>
<p>Fixed Helm rendering issue that caused Argo ComparisonError (<code>groupVersion shouldn&#39;t be empty</code>)</p>
</li>
</ul>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Improved Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1900</p>
</td>

<td valign="top" width="24%"><p><strong>Developer Portal update with Landing Page</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Summary</strong></p>
<ul>
<li>
<p>Move the cluster app launcher into the Horizon Developer Portal as a new Landing page (/), and relocate the previous portal home to /welcome.</p>
</li>

<li>
<p>Redirect the site root (/) to /developer-portal/ via the Gateway HTTPRoute so the Developer Portal is the single platform entry point.</p>
</li>

<li>
<p>Improve module Overview layout (viewport-height iframe) and restyle module portal/overview.html content for better fit in the portal shell.</p>
</li>

<li>
<p>Fully decommission the standalone landingpage app (container image, GitOps chart/gateway/namespace, Terraform image wiring) and update docs to match.</p>
</li>
</ul>
<p><strong>Context</strong></p>
<p>Horizon previously exposed a separate static Landing Page at the domain root and the Developer Portal under /developer-portal/. This duplicated UX and operational surface area. This change consolidates entry and app discovery in the Developer Portal and removes the old landingpage stack.</p>
<p><strong>Changes</strong></p>
<p><strong>Developer Portal (TAA-1986 / TAA-1998)</strong></p>
<ul>
<li>
<p>Add LandingPage.tsx with Developer and Admin application cards (Gerrit, Jenkins, MTK Connect, MCP Gateway Registry, Keycloak, Argo CD, Headlamp, Grafana).</p>
</li>

<li>
<p>Update routing/nav: / → Landing page, /welcome → Welcome.</p>
</li>

<li>
<p>Overview tab uses viewport-based height; module overview HTML updated (notably sample-data and storage-gcs).</p>
</li>

<li>
<p>Add Prettier/Husky/lint-staged tooling for the portal package.</p>
</li>
</ul>
<p><strong>Gateway (TAA-1999)</strong></p>
<ul>
<li>
<p>Exact / → 301 → /developer-portal/ on the horizon-dev-portal HTTPRoute.</p>
</li>
</ul>
<p><strong>Decommission &amp; docs (TAA-2000)</strong></p>
<ul>
<li>
<p>Remove landingpage image, GitOps resources, Terraform landingpage-app registration, and Argo image wiring.</p>
</li>

<li>
<p>Update deployment/gitops/upgrade/sub-env/workload docs to use Developer Portal + direct app URLs.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1885</p>
</td>

<td valign="top" width="24%"><p><strong>MTK Connect upgrade to V1.12</strong></p>
</td>

<td valign="top" width="64%"><p>Summary</p>
<p>This PR upgrades <strong>MTK Connect</strong> to <strong>V1.12.1</strong> to leverage the latest features, improvements, and bug fixes while maintaining compatibility with the Horizon SDV deployment workflow.</p>
<p><strong>Details</strong></p>
<ul>
<li>
<p>Upgraded <strong>MTK Connect</strong> to <strong>V1.12</strong> as part of the Horizon SDV deployment process.</p>
</li>

<li>
<p>Verified that the existing deployment workflow completes successfully with the upgraded version.</p>
</li>

<li>
<p>Validated the automatic creation of <strong>MTK Connect test benches</strong> after the upgrade.</p>
</li>

<li>
<p>Verified the automatic creation of <strong>CVD execution test benches</strong> to ensure existing functionality remains intact.</p>
</li>

<li>
<p>Performed deployment validation to confirm that no critical issues were introduced by the upgrade.</p>
</li>
</ul>
<p><strong>Purpose</strong></p>
<p>This upgrade enables Horizon SDV to utilize the latest MTK Connect capabilities while ensuring deployment stability, preserving existing automation workflows, and providing documented validation coverage for the new version.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1237</p>
</td>

<td valign="top" width="24%"><p><strong>[CWS] CONTAINER_IMAGE must be clear!</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Summary</strong></p>
<p>Replace the free-text Artifact Registry URL for CONTAINER_IMAGE on Create/Update Configuration with an obvious image choice near the config name (C-OSS/AS/ASfP) and an optional tag (default latest), so operators pick the IDE image without hand-editing registry paths.</p>
<p><strong>Changes</strong></p>
<ul>
<li>
<p>Convert CONTAINER_IMAGE to choiceParam (C-OSS (VCS) / AS / ASfP) next to CLOUD_WS_CONFIG_NAME on create-config and update-config.</p>
</li>

<li>
<p>Add CONTAINER_IMAGE_TAG (default latest) and append it when resolving the platform Artifact Registry URI in both Jenkinsfiles.</p>
</li>

<li>
<p>Derive image paths via CLOUD_WS_HORIZON__IMAGE_NAME bindings so ASfP/AS/Code OSS resolve to horizon-sdv/cloud-ws-images/.</p>
</li>

<li>
<p>Update config_admin_operations docs for the new parameters.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1839</p>
</td>

<td valign="top" width="24%"><p><strong>[Packer] googlecompute plugin must be capped below v1.2.5 on Argo (SIGSEGV)</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Summary</strong></p>
<p>Two things, both about making the Packer plugin version pins usable by an operator rather than only by whoever edits the repo:</p>
<ol start="1">
<li>
<p><strong>Documents the version pinning</strong> — what is pinned where, why the <code>&lt; 1.2.5</code> cap exists, and when to remove it. Also reports the installed Packer binary version in the job log, so the log shows the complete version picture.</p>
</li>

<li>
<p><strong>Exposes </strong><code>PACKER_GOOGLECOMPUTE_VERSION_MIN</code><strong> / </strong><code>MAX</code><strong> as Argo workflow parameters</strong>, matching the Jenkins jobs.</p>
</li>
</ol>
<p>Point 2 is review feedback: the parameters were Jenkins job parameters but Helm values on Argo, so the same pipeline offered different options depending on which interface started it. The request was that a parameter exposed to the end user is exposed on both interfaces or on neither.</p>
<p><strong>What changed</strong></p>
<table width="100%">
<tbody>
<tr>
<td valign="top"><p>File</p>
</td>

<td valign="top"><p>Change</p>
</td>
</tr>

<tr>
<td valign="top"><p><code>helm/templates/workflow/workflowtemplates.yaml</code></p>
</td>

<td valign="top"><p>Adds <code>packerGooglecomputeVersionMin</code> / <code>Max</code> as workflow parameters, threads them through the DAG and the step inputs, and switches the two <code>export</code> lines from chart values to <code>inputs.parameters</code>. One file renders both templates, so x86 and arm64 cannot drift.</p>
</td>
</tr>

<tr>
<td valign="top"><p><code>groovy/job.groovy</code>, <code>groovy/job_arm.groovy</code></p>
</td>

<td valign="top"><p>Removes the <code>(TAA-1839)</code> reference from the parameter descriptions. No other Jenkins job in the repo puts a ticket reference in a description.</p>
</td>
</tr>

<tr>
<td valign="top"><p><code>docs/.../cf_instance_template.md</code></p>
</td>

<td valign="top"><p>New <strong>Packer version pinning</strong> section with the pin policy; updates the three places that said Argo needed a chart redeploy.</p>
</td>
</tr>

<tr>
<td valign="top"><p><code>cf_create_instance_template.sh</code></p>
</td>

<td valign="top"><p>Reports the Packer binary version in the job log; the build-failure message no longer points at a stale location.</p>
</td>
</tr>
</tbody>
</table>
<p><code>helm/values.yaml</code> still owns the defaults (<code>1.2.3</code> / <code>1.2.5</code>). The difference is that the chart now sets the default a run starts from, instead of baking the value in at deploy time. Nothing changes for anyone who does not touch the field.</p>
<p><strong>Verification</strong></p>
<ul>
<li>
<p><code>helm template</code> and <code>helm lint</code> clean.</p>
</li>

<li>
<p><code>argo lint --offline</code> produces the same 2 pre-existing errors as <code>HEAD</code> (<code>spec.templates[].hostname</code>, a strict-decoding artifact of the CLI&#39;s bundled schema) and no new ones.</p>
</li>

<li>
<p>Rendered-manifest diff against <code>HEAD</code> is additive except for the two <code>export</code> lines.</p>
</li>

<li>
<p>Argo tst run at defaults: constraint <code>&gt;= 1.2.3, &lt; 1.2.5</code>, <code>packer init</code> selects v1.2.4, <code>StepImportOSLoginSSHKey</code> completes without the SIGSEGV.</p>
</li>
</ul>
<p>Reviewers: the check that matters is an <strong>override</strong> run, not a default run. Both the old baked value and the new default are <code>1.2.5</code>, so a default run passes whether or not the plumbing is correct. Submitting <code>-p packerGooglecomputeVersionMax=1.2.4</code> should produce constraint <code>&gt;= 1.2.3, &lt; 1.2.4</code> and install v1.2.3.</p>
<p><strong>Deliberate non-goals</strong></p>
<p><strong>Not added to </strong><code>webhookWorkflowParameters</code><strong>.</strong> The sensor helper seeds every mapped parameter with <code>value: ""</code> and then overwrites it from the request body, and an explicit <code>""</code> overrides the WorkflowTemplate default. Mapping these would mean a Portal run that omits the keys silently ignores <code>values.yaml</code> and lands on the script fallback — which is exactly the operator workflow this ticket exists to enable. <code>values.yaml</code> already records the same reasoning for <code>subnet</code> / <code>region</code> / <code>zone</code>. Worth knowing: <code>bootDiskSize</code> is already in this state — <code>values.yaml</code> says <code>250GB</code>, the script default is <code>500GB</code>, and it is webhook-mapped. Separate issue, not addressed here.</p>
<p><strong>Shell interpolation.</strong> The two new <code>export</code> lines interpolate a submit-time parameter into a shell script, which a scanner will flag. The posture is unchanged: <code>cuttlefishPostCommand</code> accepts an arbitrary shell command by design, so anyone able to submit this workflow already has code execution in that pod, and about twenty existing exports use the identical pattern.</p>
<p><code>packerUseIap</code><strong> / </strong><code>packerSshTimeout</code><strong> / </strong><code>packerIapTunnelLaunchWait</code> stay as they are: Helm values on Argo, hardcoded in the Jenkinsfile. Neither interface exposes them to end users, so user-facing parity holds, but an operator can retune them on Argo and not on Jenkins. Same class of asymmetry, opposite direction, out of scope here.</p>
<p><code>MIN &gt;= MAX</code><strong> is not validated.</strong> Each value is checked for shape, not for their relationship, so <code>min=1.3.0, max=1.2.5</code> renders <code>&gt;= 1.3.0, &lt; 1.2.5</code> and fails at <code>packer init</code> with "no matching version". Visible and self-explanatory, so left alone.</p>
</td>
</tr>
</tbody>
</table>

<h2>Documentation update</h2>

<ul>
<li><p>Rel.4.2.0 provides updates in Horizon documentation.</p>

<ul>
<li><p>New <strong>Upgrade Guide</strong> (/docs/guides/upgrade_guide_4_1_0_to_4_2_0.md) for Rel.4.1.0 -&gt; Rel.4.2.0 upgrade.</p>
</li>

<li><p>TAA-1901 Add &quot;Open in Cloud Shell&quot; to README.md. The root README includes an Open in Cloud Shell button that clones the official Horizon SDV repository and opens <code>docs/deployment_guide.md</code>.</p>
</li>

<li><p>Cloud Workstations and agentic AI</p>

<ul>
<li><p><code>docs/workloads/cloud-workstations/workstation_images.md</code></p>
</li>

<li><p><code>docs/workloads/cloud-workstations/config_admin_operations.md</code></p>
</li>

<li><p><code>docs/workloads/common/agentic-ai/antigravity.md</code></p>
</li>

<li><p><code>docs/guides/mcp_setup.md</code></p>
</li>
</ul>
</li>

<li><p>Developer Portal, GitOps, and storage modules</p>

<ul>
<li><p><code>docs/developer_portal_user_guide.md</code></p>
</li>

<li><p><code>docs/gitops.md</code></p>
</li>

<li><p><code>gitops/modules/storage-gcs-module/README.md</code></p>
</li>

<li><p><code>gitops/modules/sample-data-module/README.md</code></p>
</li>
</ul>
</li>

<li><p>Platform, upgrade paths, and seed</p>

<ul>
<li><p><code>docs/deployment_guide.md</code></p>
</li>

<li><p><code>docs/terraform.md</code></p>
</li>

<li><p><code>docs/guides/README.md</code></p>
</li>

<li><p><code>docs/guides/upgrade_guide_4_0_0_to_4_1_0.md</code> (cross-links)</p>
</li>

<li><p><code>docs/guides/sub_environments/sub_environment_deployment_guide.md</code></p>
</li>

<li><p><code>docs/workloads/seed.md</code></p>
</li>
</ul>
</li>

<li><p>Android, OpenBSW, and image template guides</p>

<ul>
<li><p><code>docs/workloads/android/environment/cf_instance_template.md</code></p>
</li>

<li><p><code>docs/workloads/android/environment/docker_image_template.md</code></p>
</li>

<li><p><code>docs/workloads/openbsw/environment/docker_image_template.md</code></p>
</li>

<li><p><code>docs/workloads/utilities/docker_image_template.md</code></p>
</li>

<li><p><code>docs/workloads/guides/workload_setup.md</code></p>
</li>

<li><p><code>docs/workloads/guides/workload_usage.md</code></p>
</li>
</ul>
</li>

<li><p>Container image security: <code>docs/guides/container_image_security_upgrade_guide.md</code></p>
</li>
</ul>
</li>
</ul>

<h2>Bug Fixes</h2>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="10%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Bug</strong></p>
</td>

<td valign="top" width="51%"><p><strong>Description</strong></p>
</td>

<td valign="top" width="15%"><p><strong>SHA</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1891</p>
</td>

<td valign="top" width="24%"><p>Gemini Code Assist in Code OSS Workstation Agent Mode Does Not Detect Configured MCP Servers</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Gemini Code Assist (GCA) chat/agent mode on Horizon Code OSS Cloud Workstations was not detecting configured MCP servers, while the Gemini CLI and <code>gemini-mcp-agent</code> bridge continued to work. The Horizon Code OSS image used the floating <code>code-oss:latest</code> predefined base, which pulls a newer bundled Code OSS / GCA stack that regressed MCP exposure in the IDE plugin. Pinning the base to <code>code-oss-1.104.3</code> restores MCP detection and use in GCA agent mode.</p>
<p><strong>Changes</strong></p>
<p><code>Dockerfile</code></p>
<p>File path: <code>workloads/cloud-workstations/pipelines/workstation-images/horizon-code-oss/Dockerfile</code></p>
<ul>
<li>
<p>Changed the <code>FROM</code> base image from <code>us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:latest</code> to <code>us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:code-oss-1.104.3</code> to stop floating on <code>:latest</code> and use a known-good predefined base version.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>2229b92169d1cfbd12782f85b6e500a749574a0c</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1797</p>
</td>

<td valign="top" width="24%"><p>"Open in Argo Workflows" moves to wrong place</p>
</td>

<td valign="top" width="51%"><p><strong>Description</strong></p>
<p>Fixes the Horizon Dev Portal &ldquo;Open in Argo Workflows&rdquo; deep link so it opens the correct workflow in the Argo UI instead of falling back to an empty workflow list. The public URL now includes both the gateway mount path (<code>/workflows</code>) and Argo&#39;s SPA base-href route (<code>/workflows/:namespace/:name</code>).</p>
<p><strong>Changes</strong></p>
<ul>
<li>
<p>Updated <code>argoWorkflowUiUrl</code> in the Dev Portal <code>ModulePage</code> to emit <code>{base}/workflows/workflows/{namespace}/{name}</code> instead of <code>{base}/workflows/{namespace}/{name}</code></p>
</li>

<li>
<p>Aligns the generated link with Argo Workflows served behind <code>--base-href /workflows/</code> on the cluster gateway</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>b1437632904b6c540496326cd788e2bab176aa67</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1920</p>
</td>

<td valign="top" width="24%"><p>[ESRlabs][Security] VPC network access is unrestricted</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Implements/modifies deny_all default firewall rules for sdv-network,<br>to ensure no unintended traffic passess the firewall.</p>
<p><strong>Changes</strong></p>
<p>Introduce defaut deny_all firewall rule for sdv-network ingress</p>
<ul>
<li>
<p>priority: 65000</p>
</li>

<li>
<p>protocols: all</p>
</li>

<li>
<p>source_range: 0.0.0.0/0<br>Change priority of deny_all firewall rule for sdv-network egress</p>
</li>

<li>
<p>previous priority: 65534</p>
</li>

<li>
<p>new priority: 65000</p>
</li>
</ul>
<p><strong>Verification</strong></p>
<p>Verification through appication on ESRLabs environment and ISD scan.</p>
<p>Change-Id: I5c656d864323b98c7bc1967cd244f63ce64a146f</p>
<ul>
<li>
<p>TAA-1920 [ESRlabs][Security] VPC ntework access is unrestricted</p>
</li>
</ul>
<p><strong>Summary</strong></p>
<p>Implements/modifies deny_all default firewall rules for sdv-network,<br>to ensure no unintended traffic passess the firewall.</p>
<p><strong>Changes</strong></p>
<p>Remove typo in comment.</p>
<p>Change-Id: Id779c0dea503173e59d73da3a78751f1431599a5</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>b10d33fd1b0687894010b28d4ae4004ee3e738cc</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1927</p>
</td>

<td valign="top" width="24%"><p>[ESRlabs][Security] GCP Subnets with Public IP to access GCP API or Service</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Implements TAA-1927<br>Ensure that Google Private Access feature is enabled for the following subnets:<br>sdv-network:</p>
<ul>
<li>
<p>sdv-subnet</p>
</li>

<li>
<p>sdv-subnet-arm64<br>sdv-gke-cluster</p>
</li>

<li>
<p>gke-sdv-cluster-&lt;hash&gt;-pe-subnet</p>
</li>
</ul>
<p><strong>Changes</strong></p>
<p>Fix for subnets of sdv-network.</p>
<p>It was found that even do the terraform/modules/sdv-network/main.tf<br>contains code to set Google Private Access to true, the<br>terraform-google-modules/network/google module in version v13+<br>reads subnet_private_access and maps it into google_ip_private_access:<br>private_ip_google_access = lookup(each.value, "subnet_private_access", "false")<br>Thus replaced private_ip_google_access with subnet_private_access.</p>
<p><strong>Fix for sdv-gke-cluster.</strong></p>
<p>As gke-sdv-cluster-&lt;hash&gt;-pe-subnet is GKE control plane subnet,<br>created automatically when the cluster is configured.<br>Found it is already reconfigured by post-create hook to enable flow logs.<br>Introduced another post-create hook to enable Google Private Access on its subnet.</p>
<p><strong>Verification</strong></p>
<p>Test deployment of new Horizon instance from scrach.<br>Verification if the Google Private Access feature is enabled on all 3 subnets mentioned above.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>722cf39ff59a01ac8f4a2c4821255c8f4f36b48c</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1792</p>
</td>

<td valign="top" width="24%"><p>Inconsistency between status of Workflow template in developer-portal and GCP Workloads</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<ul>
<li>
<p>Remap shutdown-interrupted DAG node phases from Argo <code>Failed</code>/<code>Error</code> to <code>Aborted</code> in Horizon API, matching the workflow-level phase already returned after portal abort (<code>spec.shutdown: Stop</code>).</p>
</li>

<li>
<p>Apply the same remapping in <code>BuildArchivedLogLinks()</code> so <code>nodes[]</code> and <code>archivedLogs.steps[]</code> stay consistent.</p>
</li>
</ul>
<p>Fixes <strong>TAA-1792</strong> (portal internal inconsistency). Does <strong>not</strong> change GCP Workloads behaviour — that console shows K8s pod status (container exit code), not Argo node phase.</p>
<p><strong>Problem</strong></p>
<p>After aborting a workflow from the Developer Portal (T-WF-06):</p>
<table width="100%">
<tbody>
<tr>
<td valign="top"><p>Location</p>
</td>

<td valign="top"><p>Before</p>
</td>
</tr>

<tr>
<td valign="top"><p>Workflow result / detail header</p>
</td>

<td valign="top"><p><code>Aborted</code></p>
</td>
</tr>

<tr>
<td valign="top"><p>DAG node (interrupted step)</p>
</td>

<td valign="top"><p><code>Failed</code></p>
</td>
</tr>
</tbody>
</table>
<p>Workflow-level remapping existed (<code>DisplayPhaseForAPI</code>); node-level phases were passed through raw from Argo.</p>
<p><strong>Solution</strong></p>
<ul>
<li>
<p><code>DisplayPhaseForNode()</code> + <code>nodeInterruptedByShutdown()</code> in <code>parse.go</code></p>
</li>

<li>
<p>Wired into <code>Detail()</code> and <code>archive.go</code> (<code>BuildArchivedLogLinks</code>)</p>
</li>

<li>
<p>Unit tests in <code>parse_node_phase_test.go</code></p>
</li>
</ul>
<p>Genuine pre-abort failures (e.g. <code>main: Error (exit code 1)</code>) remain <code>Failed</code>.</p>
<p><strong>Testing</strong></p>
<p>Locally, the changes were tested like this:</p>
<p>Building the horizon-api:</p>
<pre><code>❯ cd terraform/modules/sdv-container-images/images/horizon-api/horizon-api-app
❯ go build -o /tmp/horizon-api .</code></pre>
<p>Running the server by binding to services in tst cluster:</p>
<pre><code>❯ export OIDC_ISSUER_URL=&quot;https://tst.horizon-sdv.com/auth/realms/horizon&quot;
❯ export KEYCLOAK_BASE=&quot;https://tst.horizon-sdv.com/auth&quot;

❯ /tmp/horizon-api \
  --workflows-namespace=&quot;$WORKFLOWS_NAMESPACE&quot; \
  --oidc-issuer-url=&quot;$OIDC_ISSUER_URL&quot; \
  --events-webhook-url=&quot;$EVENTS_WEBHOOK_URL&quot; \
  --http-bind-address=:8082</code></pre>
<p>Then, when phase is correctly shown as aborted for an actual workflow that was aborted form the developer portal</p>
<pre><code>❯ export HORIZON_ACCESS_TOKEN=&quot;$(./tools/scripts/horizon-api/horizon-api-get-token.sh --device)&quot;
❯ curl -s -H &quot;Authorization: Bearer $HORIZON_ACCESS_TOKEN&quot; \
  http://localhost:8082/v1/workflows/webhook-smoke-4lkdf \
  | jq &#x27;.phase, (.nodes[] | select(.type==&quot;Pod&quot;) | {stage: .displayName, phase: .phase})&#x27;

&quot;Aborted&quot;
{
  &quot;stage&quot;: &quot;log-parameters&quot;,
  &quot;phase&quot;: &quot;Aborted&quot;
}</code></pre>
<p>Without the changes, the following output would be obtained from the horizon-api server. Workflow level phase shows <code>Aborted</code>, but node level shows <code>Failed</code>.</p>
<pre><code>❯ curl -s -H &quot;Authorization: Bearer $HORIZON_ACCESS_TOKEN&quot; \
  http://localhost:8082/v1/workflows/webhook-smoke-4lkdf \
  | jq &#x27;.phase, (.nodes[] | select(.type==&quot;Pod&quot;) | {stage: .displayName, phase: .phase})&#x27;

&quot;Aborted&quot;
{
  &quot;stage&quot;: &quot;log-parameters&quot;,
  &quot;phase&quot;: &quot;Failed&quot;
}</code></pre>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>cd28900f1765ff34f837e05736fcd5ebea63c729</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1957</p>
</td>

<td valign="top" width="24%"><p>[Deployment] terraform apply fails enabling VPC flow logs on GKE auto-created pe-subnet (Invalid fingerprint)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<ul>
<li>
<p>Fixes TAA-1957: <code>terraform apply</code> fails when enabling VPC flow logs on the GKE auto-created pe-subnet (<code>Invalid fingerprint</code>).</p>
</li>

<li>
<p>Hardens <code>null_resource.enable_gke_master_subnet_flow_logs</code> and <code>null_resource.enable_gke_master_subnet_private_google_access</code> in <code>terraform/modules/sdv-gke-cluster/main.tf</code>:</p>

<ul>
<li>
<p>Wait for cluster + all node pools before running</p>
</li>

<li>
<p>Run PGA after flow logs (<code>depends_on</code>) to avoid parallel updates on the same subnet</p>
</li>

<li>
<p>Retry subnet discovery and <code>gcloud</code> updates with exponential backoff</p>
</li>

<li>
<p>Use <code>interpreter = ["bash", "-c"]</code> so <code>set -o pipefail</code> works (Terraform defaults to <code>/bin/sh</code>)</p>
</li>
</ul>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>3d9f928460762da16978398b343c893928b841a1</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1799</p>
</td>

<td valign="top" width="24%"><p>"Token is not a JWT or could not be decoded" after "./horizon auth whoami" Horizon CLI command</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p><code>horizon auth whoami</code> intermittently failed with<br>"Token is not a JWT or could not be decoded: illegal base64 data at input byte N", even though the token was valid and API calls succeeded.</p>
<p>The JWT payload decoder in tools/horizon/jwt.go appended &#39;=&#39; padding to the base64url segment and then decoded it with base64.RawURLEncoding, which rejects padding. Decoding therefore failed whenever the payload length was not a multiple of 4 (remainder 2 or 3). Because the payload length changes every login (expiry, roles, etc.), the failure was intermittent. The reported "byte 1870" equals the payload length, and 1870 % 4 == 2 -- exactly the padded case.</p>
<p><strong>Changes</strong></p>
<ul>
<li>
<p>tools/horizon/jwt.go: remove the manual &#39;=&#39; padding and decode the unpadded base64url payload directly with base64.RawURLEncoding (trimming any stray padding for robustness). JWT segments are unpadded base64url by spec.</p>
</li>

<li>
<p>tools/horizon/jwt_test.go: add regression tests covering payload lengths of every valid shape (len % 4 == 0, 2, 3) and a non-JWT input.</p>
</li>
</ul>
<p><strong>Verification</strong></p>
<ul>
<li>
<p>tools/horizon: <code>go build ./...</code> succeeds.</p>
</li>

<li>
<p>tools/horizon: <code>go test -vet=off -run TestDecodeJWTPayload ./...</code> passes (both new tests green). vet is disabled only to bypass an unrelated, pre-existing format-string issue in main.go (separate ticket).</p>
</li>

<li>
<p>TODO before PR: on self-deployed instance, run <code>horizon auth login</code> then <code>horizon auth whoami</code> and attach output showing sub / preferred_username / exp for a token that previously failed.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>76ee302388c3360ad23f77e0e768599ce4d25d94</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1916</p>
</td>

<td valign="top" width="24%"><p>[Security][nginx] CVE-2026-42945 security issue</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<ul>
<li>
<p>Advance nginx to 1.31.2-alpine3.23 for <a href="https://github.com/advisories/GHSA-gcgv-v5gf-c543">GHSA-gcgv-v5gf-c543</a>.</p>
</li>

<li>
<p>Use common_nginx_version as the single source of<br>truth for landingpage, token-injector, and<br>module-overview (via config.nginx).</p>
</li>

<li>
<p>Document the pin in the container image security<br>upgrade guide.</p>
</li>
</ul>
<p><strong>Changes</strong></p>
<ul>
<li>
<p>Bump common_nginx_version and landingpage-app 1.0.2.</p>
</li>

<li>
<p>Pass the pin from base into sdv-gke-apps and publish<br>config.nginx on the root Argo CD Application.</p>
</li>

<li>
<p>Forward config.nginx via headlamp-token-injector and<br>module-manager MODULE_CONFIG.</p>
</li>

<li>
<p>Replace hardcoded nginx:1.23 with config.nginx values.</p>
</li>

<li>
<p>Update container_image_security_upgrade_guide.md.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>e6c1efc66328e35191c4bfe03c52755c2381af9e</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2062</p>
</td>

<td valign="top" width="24%"><p>Platform deployment fails on GKE 1.34.1-gke.3720000+ due to NodeLocal DNSCache</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>GKE Standard enables NodeLocal DNSCache by default from <code>1.34.1-gke.3720000</code>. Horizon <code>allow-dns</code> NetworkPolicies only allow DNS egress via <code>namespaceSelector: kube-system</code> and <code>podSelector: k8s-app=kube-dns</code>. NodeLocal DNSCache answers on the node instead of a <code>kube-dns</code> Pod, so those selectors never match and <code>default-deny-all</code> drops all DNS. Fresh deployments fail to bootstrap.</p>
<p>Impact includes Argo CD <code>horizon-sdv</code> and <code>external-dns</code> stuck in <code>ComparisonError</code> (DNS lookup timeout to <code>argocd-repo-server</code>), Keycloak stuck at <code>Init:0/1</code> with <code>dbchecker</code> looping on <code>nc: bad address &#39;postgresql&#39;</code>, and Argo CD unable to self-heal because its own control plane DNS is blocked.</p>
<p><strong>Workaround shipped for 4.2.0</strong></p>
<p>Pinned the add-on off in <code>terraform/modules/sdv-gke-cluster/main.tf</code>:</p>
<p><code>addons_config { dns_cache_config { enabled = false } }</code></p>
<p>Omitting the block is not sufficient, because the GKE API default is on even though the Terraform schema default is off. Requires cluster/node recreation.</p>
<p><strong>Proper fix (follow-up)</strong></p>
<p>Add <code>ipBlock</code> DNS egress to the <code>allow-dns</code> policies so the add-on can be re-enabled. That follow-up is not included in Rel.4.2.0.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>3ce867c37d7125b68729e0420645d4973a18e345</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-2063</p>
</td>

<td valign="top" width="24%"><p>monitoring-tools sync failure</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>
<p>Argo CD <code>monitoring-tools</code> failed to sync. <code>PodMonitoring.monitoring.googleapis.com/node-exporter</code> was rejected: <code>spec.selector: Required value</code>. The GMP CRD requires <code>spec.selector</code>. The template only set <code>endpoints</code>.</p>
<p>The app stayed <code>OutOfSync</code> / <code>SyncError</code> (other resources still apply). GMP cannot scrape <code>node-exporter</code> until a valid <code>PodMonitoring</code> exists.</p>
<p><strong>Fix</strong></p>
<p>Add <code>selector.matchLabels.app.kubernetes.io/name: node-exporter</code> in <code>gitops/apps/monitoring-tools/templates/monitoring-node-exporter.yaml</code> to match the DaemonSet, same pattern as <code>kube-state-metric</code>.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>ad9e8fc967e6e025db1e25b0bcf3adac7833cf5c</code></p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Known Issues</h2>
<p><strong>TAA-2059</strong> Android 16 Virtual Device Launch not detected using Cuttlefish v1.57 (main)</p>
<p><strong>Cuttlefish Instance Templates on cuttlefish v1.57 or higher:</strong> By default, the test pipelines use the instance templates with cuttlefish v1.41 installed (i.e. cuttlefish-vm-v1410). If building any instance templates which have cuttlefish v1.57 or higher (including main), use the following parameter in the pipeline <em>CF Instance Template</em> to revert a change made to cuttlefish v1.57+ which changes a crucial logging level and breaks the detection of the virtual devices&#39; boot status of devices on Android16 and higher.</p>
<p><em>Parameter:</em> <code>ANDROID_CUTTLEFISH_POST_COMMAND</code></p>
<p><em>Value:</em> <code>git revert --no-commit f97d360d950b89553ee45c004c5fbcb621a5577e &amp;&amp; git reset HEAD .</code></p>
<p><strong>Antigravity:</strong> Antigravity Desktop App v2.0 can fail to reopen after being closed (workaround: terminate the leftover process). See <code>docs/workloads/common/agentic-ai/antigravity.md#known-issues</code>.</p>

<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 4.1.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>26.06.2026</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p><strong>Horizon SDV 4.1.0</strong> is the minor release which extends platform capabilities with the new workloads for <strong>Android SDV</strong> build and compatibility testing.</p>
<p>Horizon 4.1.0 also delivers several critical bug fixes including security fixes for vulnerable open-source modules used in Horizon and vulnerabilities in application containers.</p>
<p>Horizon Rel.4.1.0 also delivers also several feature improvements and critical bug fixes.</p>
<p>Horizon SDV 4.1.0 package offers fully verified and documented upgrade patch (from Rel.4.0.0 to Rel.4.1.0). (see details in /docs/guides/upgrade_guide_4_0_0_to_4_1_0.md)</p>

<h2>New Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-515</p>
</td>

<td valign="top" width="24%"><p><strong>Android SDV build workload and early adaptation</strong></p>
</td>

<td valign="top" width="64%"><p><strong>For:</strong> Google partners included in EAP program (with access to Google non-public repositories on <a href="https://partner-android.googlesource.com">https://partner-android.googlesource.com</a>)</p>
<p><strong>What&#39;s new:</strong> Support for AAOS SDV Software Stack in Jenkins pipelines for workloads-android (AAOS SDV Builder, CVD Launcher, CTS Execution)</p>
<p><strong>Main changes:</strong></p>
<p><strong>Jenkins</strong></p>

<ul>
<li><p><strong>new pipeline AAOS SDV Builder</strong>, dedicated to AAOS SDV SW Stack, with additional parameters for credentials: GERRIT_AUTH_MODE, GERRIT_GITCOOKIES_BASE64, GERRIT_GITCREDENTIALS_BASE64</p>

<ul>
<li><p>Default values for pipeline parameters set for AAOS SDV</p>
</li>

<li><p>Separate subfolder in GCP Bucket used to store images produced by AAOS SDV Builder</p>
</li>

<li><p>Reference revision of the AAOS SDV SW Stack used in testing was provided in description of AAOS_REVISION parameter</p>
</li>
</ul>
</li>

<li><p><strong>android test pipelines</strong> updated to support AAOS SDV (CVD Launcher, CTS Execution)</p>

<ul>
<li><p>Important Notes section on main pipeline page includes guidance for AAOS SDV</p>
</li>

<li><p>Default values for pipeline&#39;s parameters stay unchanged (for generic AAOS)</p>
</li>

<li><p>For parameters, which require special values for AAOS SDV, recommendations or examples are provided</p>
</li>
</ul>
</li>
</ul>
<p><strong>Generic build pipelines changes:</strong></p>

<ul>
<li><p><strong>all android builders</strong> (AAOS Builder, AAOS SDV Builder, AAOS Builder ABFS)</p>

<ul>
<li><p>builders now support new parameter AAOS_MANIFEST_NAME, which allows to point the pipeline for custom manifest file</p>
</li>

<li><p>default behavior unchanged, if no need to specify custom manifest filename, default.xml will be used</p>
</li>

<li><p>builders now support nested manifest files</p>
</li>
</ul>
</li>
</ul>
<p><strong>What you need to do after update:</strong></p>

<ul>
<li><p>login to ArgoCD and ensure the instance is synced and healthy</p>
</li>

<li><p>login to Developer Poral and enable android-workloads, wait until its status changes to READY</p>
</li>

<li><p>login to Jenkins and execute Seed Workloads pipeline twice:</p>

<ul>
<li><p>first with SEED_WORKLOAD set to &#39;none&#39;</p>
</li>

<li><p>second with SEED_WORKLOAD set to &#39;android&#39; (or &#39;all&#39;) to ensure new pipeline, parameters and default values get propagated correctly</p>
</li>
</ul>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Improved Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1707</p>
</td>

<td valign="top" width="24%"><p>Configurable GKE maintenance window and exclusions</p>
</td>

<td valign="top" width="64%"><p>GKE <strong>release channel</strong>, <strong>recurring maintenance window</strong>, and <strong>maintenance exclusions</strong> are now configurable via <code>terraform/env/terraform.tfvars</code>.</p>
<p><strong>Key benefits</strong></p>

<ul>
<li><p><strong>Release channel:</strong> Pick GKE&#39;s channel cadence (<code>RAPID</code> / <code>REGULAR</code> / <code>STABLE</code> / <code>EXTENDED</code>) or keep <code>UNSPECIFIED</code> (static). This repo wires ABFS <code>auto_upgrade</code> to that choice.</p>
</li>

<li><p><strong>Recurring maintenance window:</strong> Constrain <em>when</em> automatic maintenance may <em>start</em> (time-of-day + recurrence) instead of running only on whatever default window you inherit.</p>
</li>

<li><p><strong>Maintenance exclusions:</strong> Optional date ranges that <em>defer</em> upgrades by <strong>scope</strong> (<code>NO_UPGRADES</code>, <code>NO_MINOR_*</code>, …) when you need a quiet period, within GKE&#39;s limits.</p>
</li>

<li><p><strong>In-repo walkthrough:</strong> Use the <strong>GKE</strong> block in <code>terraform/env/terraform.tfvars.sample</code> (copy + comments: channel, ABFS, window, exclusions, <strong>EXTENDED</strong> + Config Connector, two-apply for <code>NO_MINOR_*</code>). Terraform variable descriptions stay short; the sample has the detail.</p>
</li>
</ul>
<p><strong>Defaults (no behavior change)</strong></p>

<ul>
<li><p><code>sdv_cluster_version</code> maps to <code>min_master_version</code> (control plane <strong>floor</strong>). The version shown in the console or API can be <strong>newer</strong>; Terraform does <strong>not</strong> use this field to <strong>downgrade</strong> the control plane to an older string.</p>
</li>

<li><p><code>sdv_cluster_release_channel = "UNSPECIFIED"</code> (static / no channel).</p>
</li>

<li><p>ABFS build pool remains CASFS-pinned (<code>auto_upgrade = false</code> on <code>UNSPECIFIED</code>).</p>
</li>

<li><p>You may see a <code>maintenance_policy</code> diff on first apply as defaults are materialized; <code>UNSPECIFIED</code> clusters do not auto-upgrade.</p>
</li>

<li><p><strong>Release channels:</strong> <code>RAPID</code> / <code>REGULAR</code> / <code>STABLE</code> / <code>EXTENDED</code> / <code>UNSPECIFIED</code> (see <code>terraform/env/terraform.tfvars.sample</code> for <strong>EXTENDED</strong> + <strong>KCC</strong> notes).</p>
</li>

<li><p><code>EXTENDED</code> <strong>+ Config Connector (GKE add-on):</strong> GKE forbids both on one cluster. This repo keeps the add-on <strong>enabled</strong> in Terraform - see <code>terraform/env/terraform.tfvars.sample</code> before using <code>EXTENDED</code> (change the add-on or channel accordingly).</p>
</li>
</ul>
<p><strong>Key rule (reduce 400s)</strong>: <strong>do channel +</strong> <code>NO_MINOR_*</code> in <strong>two applies</strong></p>
<p>When moving from <code>UNSPECIFIED</code> <strong>(static)</strong> → <code>RAPID</code> <strong>/</strong> <code>REGULAR</code> <strong>/</strong> <code>STABLE</code> <strong>/</strong> <code>EXTENDED</code> <em>and</em> you want <code>NO_MINOR_UPGRADES</code> or <code>NO_MINOR_OR_NODE_UPGRADES</code>, apply in this order:</p>

<ol start="1">
<li><p><strong>Release channel only:</strong> set <code>sdv_cluster_release_channel</code> to your target channel and set <code>sdv_cluster_maintenance_exclusions = []</code>; <code>apply</code> until it succeeds.</p>
</li>

<li><p><strong>Exclusions:</strong> add your <code>NO_MINOR_*</code> exclusions and <code>apply</code> again.</p>
</li>
</ol>
<p>The reverse move (back to <code>UNSPECIFIED</code>) follows the same idea: <strong>clear exclusions first</strong>, then change the channel.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1818</p>
</td>

<td valign="top" width="24%"><p>[Security] Fixes for vulnerable modules in Horizon containers</p>
</td>

<td valign="top" width="64%"><p>Clears the container-security (ISD) findings across 14 platform images by forcing fresh <code>no_cache</code> rebuilds (version bumps) and fixing the four images that did <strong>not</strong> self-heal on a plain rebuild. Adds a self-contained upgrade guide and a reusable probe script so teams on older releases/forks can apply, deploy, and validate the same remediation. Validated end-to-end on live lab environments (deploy → Argo CD rollout → package probe → live spot-check).</p>

<ul>
<li><p><strong>Area:</strong> terraform/modules/base/locals.tf</p>

<ul>
<li><p><strong>Change:</strong> Bump build_version and deploy_version 1.0.0 → 1.0.1 for all 14 flagged images</p>
</li>

<li><p><strong>Why:</strong> Terraform only rebuilds when the tag changes; builds are no_cache=true, so a fresh tag pulls the latest patched packages and Argo CD rolls it out. 10 of the 14 images self-heal this way (Dockerfiles already apk/apt upgrade).</p>
</li>
</ul>
</li>

<li><p><strong>Area:</strong> landingpage-app/Dockerfile</p>

<ul>
<li><p><strong>Change:</strong> nginx base 1.28.1 → 1.28.3-alpine3.23; add USER root + RUN apk --no-cache upgrade</p>
</li>

<li><p><strong>Why:</strong> Image never patched OS packages and pinned a vulnerable nginx base (curl/expat/libpng/openssl).</p>
</li>
</ul>
</li>

<li><p><strong>Area:</strong> keycloak-post/Dockerfile</p>

<ul>
<li><p><strong>Change:</strong> Add apk update &amp;&amp; apk upgrade</p>
</li>

<li><p><strong>Why:</strong> Was missing the OS-patch step its sibling images already have.</p>
</li>
</ul>
</li>

<li><p><strong>Area:</strong> keycloak-post-gerrit/Dockerfile</p>

<ul>
<li><p><strong>Change:</strong> Pull openssh-client from the Alpine edge branch</p>
</li>

<li><p><strong>Why:</strong> Required openssh &gt;= 10.3 is not yet in Alpine 3.23 stable.</p>
</li>
</ul>
</li>

<li><p><strong>Area:</strong> mtk-connect-post-key/Dockerfile</p>

<ul>
<li><p><strong>Change:</strong> Base python:3.12 → python:3.13-slim-bookworm</p>
</li>

<li><p>Fix requires Python &gt;= 3.13.11 (CVE-2025-13836).</p>
</li>
</ul>
</li>

<li><p><strong>Area:</strong> docs/guides/container_image_security_upgrade_guide.md + docs/guides/README.md</p>

<ul>
<li><p><strong>Change:</strong> Type A / Type B upgrade paths, full 14-image testing procedure (Section #4), indexed under Maintenance &amp; security guides</p>
</li>

<li><p><strong>Why:</strong> Existing upgrade guides don&#39;t cover how forked/older environments pull and test these fixes.</p>
</li>
</ul>
</li>

<li><p><strong>Area:</strong> tools/scripts/container-images/container-image-version-bump-test.sh (new)</p>

<ul>
<li><p><strong>Change:</strong> probe, argocd, spot-check, cleanup commands</p>
</li>

<li><p><strong>Why:</strong> Repeatable validation without waiting for ISD rescan cadence; probes all 14 images at any registry tag.</p>
</li>
</ul>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1787</p>
<p>TAA-1821</p>
</td>

<td valign="top" width="24%"><p>Workloads-android (Cuttlefish templates, CVD Launcher, CTS Execution)</p>

<ul>
<li><p>[Cuttlefish] instance templates via Config Connector</p>
</li>

<li><p>CVD Launcher and CTS Execution on Argo (workloads-android)</p>
</li>
</ul>
</td>

<td valign="top" width="64%"><p><strong>For:</strong> Operators and developers using the <strong>Developer Portal</strong>, <strong>Argo Workflows UI</strong>, and <strong>Jenkins</strong> to build and manage <strong>Cuttlefish (CF) GCE instance templates</strong> on <strong>workloads-android</strong>.</p>
<p><strong>What&#39;s new</strong></p>
<p>You can create, refresh, and delete <strong>Cuttlefish instance templates</strong> through the same <strong>workloads-android</strong> module as other Android pipelines-with runs visible in <strong>Argo Workflows</strong> and submissions from the <strong>Portal</strong> (and existing <strong>Jenkins</strong> jobs where you still use them).</p>
<p><strong>GCE instance templates</strong> for this path are published via <strong>Config Connector (KCC)</strong> in the cluster (Kubernetes-managed templates that sync to GCP), instead of relying only on ad hoc <code>gcloud</code> steps from a build agent.</p>
<p><strong>Developer Portal</strong></p>

<ul>
<li><p><strong>workloads-android</strong> exposes <strong>Cuttlefish instance template</strong> workflows (<strong>x86</strong> and <strong>arm64</strong>) alongside existing Android pipelines.</p>
</li>

<li><p>Submit from the Portal with the usual workflow parameters (create, <strong>SSH key refresh only</strong>, or <strong>delete</strong>).</p>
</li>

<li><p><strong>Module disable</strong> shows clearer, updating status while Argo removes resources (less &ldquo;stuck on Ready&rdquo; during long uninstalls).</p>
</li>

<li><p>Enable/disable of <strong>workloads-android</strong> is more reliable when KCC resources are involved.</p>
</li>
</ul>
<p><strong>Argo Workflows UI</strong></p>

<ul>
<li><p>New <strong>WorkflowTemplates</strong> and <strong>Sensors</strong> for <strong>cf-instance-template</strong> (x86 / arm64), triggered by the platform <strong>workflow-dispatch</strong> webhook (same pattern as aaos-builder and related charts).</p>
</li>

<li><p>Runs use shared <strong>git credentials</strong> and cloud settings aligned with other Android workload workflows.</p>
</li>

<li><p><strong>Delete</strong> and <strong>orphan Packer disk</strong> cleanup paths are improved when builds fail or are cancelled.</p>
</li>
</ul>
<p><strong>Jenkins</strong></p>

<ul>
<li><p>Existing <strong>Cuttlefish</strong> Jenkins jobs remain supported; scripts and behaviour are aligned with the <strong>Argo</strong> pipeline (same stages: bake, SSH metadata update, delete).</p>
</li>

<li><p>Jenkins agents can still publish KCC templates where <strong>publisher RBAC</strong> is configured for the Jenkins service account.</p>
</li>
</ul>
<p><strong>Platform / operations (what you may notice)</strong></p>

<ul>
<li><p><strong>workloads-android</strong> enable and disable in the Portal should complete more predictably (template cleanup, Config Connector context, Argo sync ordering).</p>
</li>

<li><p><strong>Argo CD</strong> health for Config Connector during template work is less likely to block sync indefinitely.</p>
</li>

<li><p><strong>ARM64</strong> host images get the extra kernel modules needed for Cuttlefish on Ubuntu where the distro provides them.</p>
</li>
</ul>
<p><strong>What you need to do</strong></p>

<ol start="1">
<li><p>Build and deploy updated <strong>module-manager</strong> and <strong>horizon-dev-portal</strong> images per your environment rollout.</p>
</li>

<li><p>Ensure <strong>workloads-android</strong> is enabled and synced after upgrade.</p>
</li>

<li><p>Use the Portal or Argo UI to submit <strong>cf-instance-template-*</strong> workflows; confirm the new template appears in <strong>GCP Compute Engine → Instance templates</strong>.</p>
</li>

<li><p>For detailed operator steps and disable order, see <code>docs/workloads/android/environment/cf_instance_template.md</code>.</p>
</li>
</ol>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1828</p>
</td>

<td valign="top" width="24%"><p>[ARM64] Cuttlefish placement from Terraform (shared or dedicated subnet)</p>
</td>

<td valign="top" width="64%"><p><strong>Why:</strong> ARM64 Cuttlefish runs on <strong>bare-metal-capable</strong> GCE types in <strong>specific regions and zones</strong>. That availability differs by project, org policy, and where you deploy the platform-not every region supports the same ARM64 / &ldquo;metal&rdquo; options.</p>
<p>Earlier releases effectively <strong>hard-coded</strong> ARM64 networking and placement (region, zone, dedicated subnet). That was fine when every environment looked the same, but it blocks:</p>

<ul>
<li><p>Deploying ARM64 Cuttlefish in <strong>another allowed region/zone</strong></p>
</li>

<li><p><strong>Brownfield</strong> envs that already created a dedicated subnet under legacy names</p>
</li>

<li><p><strong>Greenfield</strong> envs that need a clean, documented default</p>
</li>
</ul>
<p>4.1.0 makes placement <strong>explicit infrastructure configuration</strong>: one source of truth in Terraform, then the same values flow to CI/CD so builds and VMs land where you intend-not where an old default assumed.</p>
<p><strong>What changed:</strong></p>

<ul>
<li><p><strong>ARM64 region, zone, and subnet</strong> are set in <code>terraform.tfvars</code> and published to <strong>Jenkins</strong>, <strong>Argo Workflows</strong>, and <code>horizon-workflow-cloud-env</code> so instance templates, workflows, and agent VMs use your deployment&#39;s placement.</p>
</li>

<li><p><code>enable_arm64</code> is renamed to <code>enable_arm64_dedicated_subnet</code> (same behaviour: dedicated ARM64 subnet vs primary <code>sdv-subnet</code>).</p>
</li>

<li><p>New <code>arm64_*</code> variables when the dedicated subnet is enabled (region, zone, subnetwork, secondary range names).</p>
</li>
</ul>
<p><strong>What you need to do</strong></p>
<p>Review <code>terraform/env/terraform.tfvars</code> against <code>terraform/env/terraform.tfvars.sample</code> and follow the upgrade guide (<code>docs/guides/upgrade_guide_4_0_0_to_4_1_0.md</code>)</p>
<p><strong>Upgrade guide 4.0.0 → 4.1.0 - See ARM64 Cuttlefish placement</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1758</p>
</td>

<td valign="top" width="24%"><p>[OpenBSW] Align workload for Rust</p>
</td>

<td valign="top" width="64%"><p>Upgrade default OpenBSW pin to 1c1450e and expose upstream Rust presets: posix-rust (POSIX) and s32k148-rust-gcc (S32K148, GCC-only).</p>

<ul>
<li><p>Jenkins: RTOS_PLATFORM adds rust; normalize NXP rust preset in bsw_build.sh; extend POSIX/NXP parameter descriptions; POSIX_PYTEST defaults follow RTOS.</p>
</li>

<li><p>Docker Image Template: install rustup/cargo target/cbindgen per upstream dev image.</p>
</li>

<li><p>POSIX pytest: target_posix_rust.fragment.toml plus merge in bsw_download_artifact.sh and bsw_build.sh when [rust.target_process] is missing upstream.</p>
</li>

<li><p>POSIX Test / BSW Builder groovy descriptions: layout parity with Builder.</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Documentation update</h2>

<ul>
<li><p>Rel.4.1.0 provides with updates in Horizon documentation</p>

<ul>
<li><p>Deployment Guide (TAA-1864)</p>

<ul>
<li><p>Ports deployment guide updates from <code>timkl7/horizon-sdv</code> (<code>timkl7/update-deplg</code>), adapted for this repository&#39;s Terraform DNS naming, Jenkins RBAC, and existing section structure.</p>
</li>

<li><p>Adds <strong>Section #3a (Optional) - Domain Setup via GCP</strong> for registering a domain via Cloud Domains, with guidance to skip when using an existing domain and registrar.</p>
</li>

<li><p>Expands <strong>Section #3b - Update Nameservers</strong> with GCP-registered domain NS delegation (<code>&lt;SUB_DOMAIN&gt;-horizon-sdv-com</code>) and step-by-step DNSSEC setup (Search Console verification and optional DS records).</p>
</li>

<li><p>Adds troubleshooting sections <strong>#6f</strong> (homepage unreachable, Argo CD <code>horizon-sdv</code> sync) and <strong>#6g</strong> (Jenkins pipeline permissions; Keycloak groups preferred, manual <code>administrators</code> / <code>developers</code> roles as fallback).</p>
</li>

<li><p>Renumbers Section <strong>-</strong> subsections (former 3a–3i → 3b–3j) and updates cross-links in upgrade guides.</p>
</li>
</ul>
</li>

<li><p>Contributing Guide (TAA-1865) (<code>docs/contributing.md</code>) by adding additional guidance for contributors based on lessons learned from recent external collaboration efforts. These updates improve the Contributor onboarding experience, establish clear expectations for testing and validation, and help ensure that submitted changes are verified in a self-deployed Horizon SDV environment before being proposed for inclusion in future releases.</p>

<ul>
<li><p>Added a new <strong>Prerequisites: Self-Deployment</strong> section outlining the requirement for contributors to deploy Horizon SDV in their own GCP project before submitting changes.</p>
</li>

<li><p>Added guidance on validating deployments and testing changes in a live environment.</p>
</li>

<li><p>Introduced a <strong>Pull Request Workflow &amp; Release Cycle</strong> section describing contribution expectations and release assignment processes.</p>
</li>

<li><p>Added a <strong>Contribution Checklist</strong> to help contributors verify readiness before submitting a PR.</p>
</li>

<li><p>Included additional <strong>Getting Help</strong> information for deployment and development workflow support.</p>
</li>
</ul>
</li>
</ul>
</li>

<li><p>The new <strong>Upgrade Guide</strong> (/docs/guides/upgrade_guide_4_0_0_to_4_1_0.md) provide guideline for Rel.4.0.0 -&gt; Rel.4.1.0 upgrade. (TAA-1868)</p>
</li>

<li><p>New <strong>Horizon modular architecture - Functional description/User Guide document</strong> (/docs/developer_portal_user_guide.md) (TAA-1826)</p>

<ul>
<li><p>Update upgrade guide 4.0.0 to 4.1.0 add platform app redeploy - (TAA-1898)</p>
</li>
</ul>
</li>

<li><p>Developer&#39;s Guide changes - TAA-1696 - complete re-write and re-structuring of the old developer_guide.md document with new information also added. Simplified the more complex documentation and consolidated as per sample documents:</p>

<ul>
<li><p><strong>workload_setup.md</strong></p>

<ul>
<li><p>access setup</p>
</li>

<li><p>gerrit setup</p>
</li>

<li><p>workload setup</p>
</li>

<li><p>appendix</p>
</li>
</ul>
</li>

<li><p><strong>workload_usage.md</strong></p>

<ul>
<li><p>android workload -&gt; linked to seperate document</p>
</li>

<li><p>openbsw workload</p>
</li>

<li><p>cloud workstationss</p>
</li>

<li><p>AI Review</p>
</li>

<li><p>Appendices</p>
</li>
</ul>
</li>

<li><p><strong>workload_usage_android.md</strong></p>

<ul>
<li><p>android builds</p>
</li>

<li><p>android devices</p>
</li>

<li><p>android testing</p>
</li>

<li><p>android change with Gerrit</p>
</li>
</ul>
</li>

<li><p><strong>android_labs.md</strong></p>

<ul>
<li><p>basic labs (linking to various workload_usage_android.md sections)</p>
</li>

<li><p>3 advanced labs</p>
</li>

<li><p>additional suggestions</p>
</li>
</ul>
</li>
</ul>
</li>
</ul>
<p>All files at docs/workloads/android/guides/images moved to docs/archive/images</p>
<p>Original developer_guide.md file moved to docs/archive</p>

<h2>Bug Fixes</h2>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="10%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Bug</strong></p>
</td>

<td valign="top" width="51%"><p><strong>Description</strong></p>
</td>

<td valign="top" width="15%"><p><strong>SHA</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1735</p>
</td>

<td valign="top" width="24%"><p>AI-Review is no longer deployed as a cluster-wide shared template with workloads-common</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p><strong>Gemini (AAOS / platform):</strong> Gemini review is delivered as an <strong>inline</strong> <code>gemini-review</code> <strong>step</strong> on the <code>aaos-builder</code> <strong>Argo</strong> <code>WorkflowTemplate</code>, running <code>workloads/common/agentic-ai/gemini/run_ai_review.sh</code> on the <strong>same workflow volumes</strong> as <strong>build</strong> (no separate Argo-synced <code>workloads/common/agentic-ai/gemini/helm</code> chart / <code>ai-review</code> template source on <code>workloads-android</code>).</p>
</li>

<li><p><strong>GitOps:</strong> <code>workloads-android</code> child Application no longer includes an extra Helm source for that Gemini chart; <code>aaos-builder</code> chart carries <code>gemini-review</code> YAML and pod affinity helpers.</p>
</li>

<li><p><strong>Jenkins / shared library:</strong> Pipelines that used the old split (AAOS, ABFS, BSW, Gemini AI Assistant utility, CVD/CTS via <code>cvdPipeline</code>) are aligned to the <strong>same</strong> <code>run_ai_review.sh</code> <strong>contract</strong> and env/hook behaviour where applicable.</p>
</li>

<li><p><strong>Scripts:</strong> Updates to <code>run_ai_review.sh</code> (and related <code>gemini_environment.sh</code> tweaks) to support the unified path; AAOS <strong>review-pre/post</strong> hooks live under <code>aaos_builder/hooks/</code>.</p>
</li>

<li><p><strong>Docs / ops:</strong> <code>gemini.md</code> and related test/docs copy updated for the inline Argo model; minor Terraform comment clarification for utility-node sizing vs Gemini jobs.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>e6ed71bbf7f390b547b95af1e71e7fbe999859ef</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1748</p>
</td>

<td valign="top" width="24%"><p>Module Manager does not keep applications aligned with the deployed Git ref</p>
</td>

<td valign="top" width="51%"><p><strong>Summary:</strong> When <code>scm_repo_branch</code> changes and Terraform rolls module-manager, enabled modules that follow the platform branch were left on the old <code>targetRevision</code>.</p>
<p>This PR separates <strong>following the platform branch</strong> (default) from <strong>explicit pinning</strong>, syncs unpinned parent Argo CD Applications on module-manager startup, and updates the Admin Modules UI to make that behavior clear.</p>
<p><strong>Changes:</strong></p>
<p><strong>Module Manager</strong></p>

<ul>
<li><p>Treat <code>moduleTargetRevisions</code> as an explicit pin only; enable without <code>targetRevision</code> follows the cluster default and does not write a pin entry.</p>
</li>

<li><p>Add <code>DELETE /modules/{idOrName}/target-revision</code> to clear a pin and reset an enabled module to the platform branch.</p>
</li>

<li><p>Add <code>TargetRevisionStartupSync</code> to patch enabled, unpinned parent Applications (<code>spec.source.targetRevision</code> and Helm <code>repo.revision</code>) to <code>--target-revision</code> on pod start.</p>
</li>

<li><p>Add <code>retry.RetryOnConflict</code> to <code>SyncApplicationTargetRevision</code> so concurrent Argo CD / helm-config updates do not silently drop branch alignment.</p>
</li>

<li><p>Expose <code>pinned</code> on module API responses and update OpenAPI.</p>
</li>
</ul>
<p><strong>Developer Portal</strong></p>

<ul>
<li><p>Admin Modules tab: show <strong>Following platform</strong> / <strong>Pinned</strong> status with the effective ref.</p>
</li>

<li><p>On-demand pin editor (Pin to ref / Pin ref before install) instead of an always-visible Git ref field.</p>
</li>

<li><p><strong>Reset to platform branch</strong> for pinned enabled modules.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>bcb66b7038d729e8eebb4799fcbc3dc512020022</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1756</p>
</td>

<td valign="top" width="24%"><p>GKE Extended release channel cannot be combined with Config Connector GKE add-on</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Update <code>terraform/env/terraform.tfvars.sample</code> cluster version from <code>1.33.10-gke.1067000</code> to <code>1.33.11-gke.1013000</code>.</p>
</li>

<li><p>Change <code>sdv_cluster_release_channel</code> from <code>UNSPECIFIED</code> to <code>STABLE</code> and document channel-based upgrade behavior.</p>
</li>

<li><p>Enable <code>sdv_cluster_maintenance_exclusions</code> example as active config and add guidance to update exclusion dates before apply.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>b247f816bc7a481ad410039266764cca4d4a63d7</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1757</p>
</td>

<td valign="top" width="24%"><p>CTS Execution - test result with non-zero failures is detected as Pass</p>
</td>

<td valign="top" width="51%"><p>CTS Execution fix for result analysis failure - now only results with explicit &quot;FAILED: 0&quot; are detected as Pass</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>4c6eeba46a9c64059e9ce07d1d8a44b407d32bf4</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1760</p>
</td>

<td valign="top" width="24%"><p>[Cuttflefish] Reinstate OS Login SSH key pruning</p>
</td>

<td valign="top" width="51%"><p>TAA-1760: fix(cf-instance-template): prune OS Login SSH keys before Packer builds - Packer googlecompute + IAP/OS Login imports a temp key per run; pre-Packer ops pruned the caller profile but that step was dropped on migration. Keys from success/fail/interrupted runs accumulate until the 32 KiB login profile limit is hit (rare in prod, common in heavy test).</p>
<p>Reinstate prune via cf_compute_rest.py prune-os-login-ssh-keys from cf_create_instance_template.sh before packer init; Packer/plugin do not manage profile size (known upstream gap).</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>c9fea32e08c4b6d6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1779</p>
</td>

<td valign="top" width="24%"><p>No access to signed urls</p>
</td>

<td valign="top" width="51%"><p>FIX to download artifacts from running workflows or from historical workflows (using signed urls)</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>ffd47bfdf6a88b93c70807ec9fa8b85f31e01d66</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1796</p>
</td>

<td valign="top" width="24%"><p>[Argo CD] UI lists zero Applications</p>
</td>

<td valign="top" width="51%"><p>TAA-1796: fix(argocd): persist RBAC in Helm values so upgrades don&#39;t reset argocd-rbac-cm</p>

<ul>
<li><p>argo-helm owned argocd-rbac-cm but Terraform omitted configs.rbac, so each upgrade wiped policy.csv and set scopes to [groups]. Horizon OIDC uses JWT roles administrators → role:admin); post-Keycloak patches were overwritten on reconcile. Add policy.csv, policy.default, and scopes: [roles] to argocd-values.yaml.tpl; keep in sync with keycloak-post-argocd/configure.sh.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>c9fea32e08c4b6d6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1800</p>
</td>

<td valign="top" width="24%"><p>[Module Manager] pod CrashLoops when workloads-android child Application uses multi-source spec.sources</p>
</td>

<td valign="top" width="51%"><p>TAA-1800 - Multi-source Argo Applications Module Manager: skip live MODULE_CONFIG spec sync for multi-source apps (invalid merge on sources[])</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1801</p>
</td>

<td valign="top" width="24%"><p>[Module Manager] Fast Module Manager toggles leave mod-workloads-* syncing while {prefix}workloads-* is still deleting</p>
</td>

<td valign="top" width="51%"><p>TAA-1801 - workloads-android / workloads-common disable, CCC, and CNRM ComputeInstanceTemplate</p>

<ul>
<li><p>Serialize prefixed child Application teardown and re-enable; terminate in-flight Argo sync; strip syncPolicy.automated during teardown.</p>
</li>

<li><p>Re-enable: wait until workflows ConfigConnectorContext is absent or not terminating before recreating the child Application.</p>
</li>

<li><p>Disable: delete all ComputeInstanceTemplate CRs in workflows namespace and wait for CCC absent (PreDelete + Module Manager); harden child finalizer handling and EnsureWorkflowsConfigConnectorContextRemoved on success paths.</p>
</li>

<li><p>After successful child teardown, clear skip-reconcile on parent mod-* so delete can complete and resources-finalizer can prune.</p>
</li>

<li><p>Argo CD (Terraform): CCC health.lua - terminating CCC healthy; healthy false without operator errors -&gt; Progressing.</p>
</li>

<li><p>Gitops: workloads-android / workloads-common syncPolicy automated.prune true, selfHeal false, and comments; module-manager RBAC for CNRM ComputeInstanceTemplate delete; prepare-pipeline-git-creds and webhook alignment with other Android workload charts.</p>
</li>

<li><p>Unblock workloads-android CCC teardown on KCC disable. Delete(workloads-android child) then remove ComputeInstanceTemplate CRs immediately so CCC can finalize during Argo prune instead of wedging MM until PreDelete clears CITs. While CCC is terminating, if the API lists zero CIT for three polls but status.errors still shows the addon &quot;cannot finalize ... ComputeInstanceTemplate&quot; wedge, clear CCC metadata.finalizers once (automated break-glass) and continue waiting until absent.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1802</p>
</td>

<td valign="top" width="24%"><p>[Portal] Shows installation in progress during Module Manager disable instead of uninstall in progress</p>
</td>

<td valign="top" width="51%"><p>TAA-1802 - Developer Portal</p>

<ul>
<li><p>Modules tab: poll during disable (READY -&gt; uninstall); steadier status while Argo prunes; related App.tsx / moduleStatus / proxy tweaks.</p>
</li>

<li><p>Keycloak post-argocd configure.sh updates for portal flows.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1804</p>
</td>

<td valign="top" width="24%"><p>[Security] golang/oauth2 module update (v0.12.0-v.0.36.0)</p>
</td>

<td valign="top" width="51%"><p>Fixes TAA-1804 by upgrading <code>golang.org/x/oauth2</code> from <code>v0.12.0</code> to <code>v0.36.0</code> (indirect) in <strong>module-manager-app</strong> and <strong>workflow-namespace-drain-app</strong>.</p>
<p><strong>Changes</strong></p>

<ul>
<li><p>Updated <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.mod</code></p>

<ul>
<li><p><code>golang.org/x/oauth2 v0.12.0 -&gt; v0.36.0 (indirect)</code></p>
</li>

<li><p><code>go mod tidy</code> removed the unused indirect <code>google.golang.org/appengine</code> line (no longer required by this module graph after the <code>oauth2</code> bump).</p>
</li>
</ul>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.sum</code> via <code>go mod tidy</code>.</p>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.mod</code></p>

<ul>
<li><p><code>golang.org/x/oauth2 v0.12.0 -&gt; v0.36.0 (indirect)</code></p>
</li>

<li><p>Same removal of indirect <code>google.golang.org/appengine</code> as above.</p>
</li>
</ul>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.sum</code> via <code>go mod tidy</code>.</p>
</li>

<li><p>No changes to Dockerfile, application source, Helm, or Terraform configuration (outside these <code>go.mod</code> / <code>go.sum</code> paths).</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>aba6cdc7b30deddec23258d198e0361c2ec6a6e9</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1806</p>
</td>

<td valign="top" width="24%"><p>[Security] golang/oauth2 module update (v0.21.0-v.0.36.0)</p>
</td>

<td valign="top" width="51%"><p>Fixes TAA-1806 by upgrading <code>golang.org/x/oauth2</code> from <code>v0.21.0</code> to <code>v0.36.0</code> (indirect) in the <strong>horizon-dev-portal</strong> Go proxy module.</p>
<p><strong>Changes</strong></p>

<ul>
<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/proxy/go.mod</code></p>

<ul>
<li><p><code>golang.org/x/oauth2 v0.21.0 -&gt; v0.36.0 (indirect)</code></p>
</li>
</ul>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/proxy/go.sum</code> (oauth2 checksum lines only; refreshed with <code>go mod tidy</code>).</p>
</li>

<li><p>No changes to Dockerfile, proxy source (<code>main.go</code>), Helm, or Terraform outside these paths.</p>
</li>
</ul>
<p><strong>Dependency note</strong></p>
<p><code>oauth2</code> is indirect; the direct dependency remains <code>github.com/coreos/go-oidc/v3 v3.11.0</code>. The bump pins the resolved <code>oauth2</code> version via <code>go.mod</code> (MVS override) without upgrading <code>go-oidc</code>.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>1390a4d5c949dc672777fa791820ad97442f51fd</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1807</p>
</td>

<td valign="top" width="24%"><p>[Security] go-jose module update (v4.0.2-v4.1.4)</p>
</td>

<td valign="top" width="51%"><p>go-jose module update to v4.1.4</p>

<ul>
<li><p>All dependencies are identified and analyzed.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>b2cce4bba421bf0198832e7ee8e5e9db4eb3e522</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1808</p>
</td>

<td valign="top" width="24%"><p>[Security] go-jose module update (v4.1.3-v4.1.4)</p>
</td>

<td valign="top" width="51%"><p>go-jose module update to v4.1.4</p>

<ul>
<li><p>All dependencies are identified and analyzed.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>af3e3040e76e9a7b4b96874209b2577e0281dc06</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1809</p>
</td>

<td valign="top" width="24%"><p>[Security] golang-jwt/jwt module update (v4.5.0-v5.3.1)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong> Fixes TAA-1809 by upgrading [Security] golang-jwt/jwt module update (v4.5.0-v5.3.1) in the module-manager and workflow-namespace-drain module.</p>
<p><strong>Changes</strong> Updated the following files:</p>

<ol start="1">
<li><p>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.mod</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.sum</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.mod</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.sum</p>
</li>
</ol>
<p>As part of the dependency refresh, the controller-runtime and related Kubernetes dependencies were updated through <strong>go mod tidy</strong>. No changes were made to Dockerfile, Helm, or Terraform outside these paths.</p>
<p><strong>Dependency note</strong> <strong>golang/jwt</strong> remains an indirect dependency. The direct dependency chain continues to come through the Kubernetes <strong>controller-runtime</strong> version.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>4b365d36741382301d724361084a805a1a9ef356</code></p>
</li>

<li><p><code>75bf091fa8db89f7f3d5d04aa6276aa94847a89f</code></p>
</li>

<li><p><code>2fcaa5bc7141f40b911d25566209410f04dc261a</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1811</p>
</td>

<td valign="top" width="24%"><p>[Security] golang.org/x/net module update (v0.19.0-v0.54.0)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong></p>

<ul>
<li><p>Resolves TAA-1811 issue: bumps indirect <code>golang.org/x/net</code> from <strong>v0.52.0</strong> to <strong>v0.54.0</strong> (Black Duck target; addresses OSS scan finding for v0.19.0 lineage).</p>
</li>

<li><p>Updates <code>go.mod</code> / <code>go.sum</code> for:</p>

<ul>
<li><p><code>horizon-api-app</code></p>
</li>

<li><p><code>module-manager-app</code></p>
</li>

<li><p><code>workflow-namespace-drain-app</code></p>
</li>
</ul>
</li>

<li><p>Regenerates checksums via <code>go mod tidy</code>; removes stale <code>golang.org/x/exp</code> from <code>module-manager</code> and <code>workflow-namespace-drain</code> <code>go.mod</code> (rebase artifact that broke <code>go build</code> with <code>missing go.sum entry</code>).</p>
</li>

<li><p>No application source, Dockerfile, or Go toolchain changes (<code>go 1.25.0</code> / <code>golang:1.25-alpine</code> unchanged).</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>fc0ac053f7c796f6601cc22088aa9578943d705c</code></p>
</li>

<li><p><code>3830c825a6829d240c56dd35c1d4c2a053c7d46a</code></p>
</li>

<li><p><code>efb978febc4fa4c845dedb735aa5b7a861bddbe3</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1814</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] Packer builds can leave orphan packer-* persistent disks in GCP</p>
</td>

<td valign="top" width="51%"><p>TAA-1814 - Packer orphan disks</p>

<ul>
<li><p>Post-packer-build / SIGINT orphan packer-* zonal disk cleanup; onExit /umbrella-creds paths in cf-instance-template workflows where applicable.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1816</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] modprobe errors on vhci-hcd</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>TAA-1816 - Host image (ARM64 family)</p>
</li>

<li><p>Packer provisioning: linux-modules-extra (or equivalent) before Cuttlefish on Ubuntu where required.</p>
</li>

<li><p>Debian not in-scope as Google not support kernel-extras</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6af770656d2bf69729b33941b</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1820</p>
</td>

<td valign="top" width="24%"><p>[Developer Portal] 502 HTTP Error response to Module Manager API call</p>
</td>

<td valign="top" width="51%"><p>Fixes the <code>502 Bad Gateway</code> returned by Developer Portal on <code>/api/mm/*</code> (Admin -&gt; Modules tab showing <code>modules: 502</code>).</p>
<p>Root cause: <code>ModuleConfigHelmStartupSync</code> aborted the Module Manager on Argo Applications using <code>spec.sources</code> (multi-source apps introduced by TAA-1787) whose <code>path</code> contained <code>gitops/</code>. The Module Manager pod crash-looped, the Service had no Ready endpoints, and the Developer Portal proxy surfaced this as 502.</p>
<p><strong>Changes</strong></p>

<ul>
<li><p>Cherry-picked from <code>env/mercedes</code> (authorship preserved):</p>

<ul>
<li><p><code>application_helm_config.go</code>: handle multi-source apps; merge <code>MODULE_CONFIG</code> only into the <code>gitops/</code> source.</p>
</li>

<li><p><code>application_helm_config_test.go</code>, <code>argo_app_test.go</code>: tests aligned with new behaviour and controller-runtime fake-client semantics.</p>
</li>
</ul>
</li>

<li><p><code>module_config_helm_startup.go</code>: per-application failure demoted from <code>return fmt.Errorf(...)</code> to <code>log.Printf(...)</code> so a single bad Application no longer aborts the manager.</p>
</li>

<li><p>Version bump (separate commit, my authorship):</p>

<ul>
<li><p><code>module-manager-app</code> image: <code>0.3.1</code> -&gt; <code>0.3.2</code> (<code>terraform/modules/base/locals.tf</code>).</p>
</li>

<li><p><code>gitops/apps/module-manager</code> chart: <code>0.3.0</code> -&gt; <code>0.3.2</code>.</p>
</li>
</ul>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>54702b69656960b72c7c6ade257697bd46e4a154</code></p>
</li>

<li><p><code>2d5e6fc39bb038a6ac80507bd4d1e943a4a4a37c</code></p>
</li>

<li><p><code>524904556ee895eea3807d1c7c0dbf579b67be04</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1822</p>
</td>

<td valign="top" width="24%"><p>Jenkins/Argo logs link GCS Console URLs with ANSI reset appended</p>
</td>

<td valign="top" width="51%"><p>TAA-1822: fix(storage): print artifact summary URLs without ANSI reset Jenkins and Argo log linkifiers were including \033[0m after Console URLs from the colored ARTIFACT SUMMARY block (%1B[0m in the href). Emit https?:// lines with plain echo; keep green formatting elsewhere.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>000add0add96e734aa6eeca6fe18c28bdddba3ac</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1823</p>
</td>

<td valign="top" width="24%"><p>[ARM64] Lack of available instances on us-central1-f</p>
</td>

<td valign="top" width="51%"><p>TAA-1823: ARM64 lack of available instances us-central1-f does not have the resources available but -b does so change Jenkins, Argo to use us-central1-b by default. Future:</p>

<ul>
<li><p>Arm metal instances are available on europe-west3</p>
</li>

<li><p>Change Horizon so we no longer tie 2nd subnet to US, let&#39;s make generic so it can be any region/zone.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>70d5aa65dbd262297a6b20cbf518b35789c8cd02</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1824</p>
</td>

<td valign="top" width="24%"><p>[ArgoWF] CF Instance Template fail when aaos-builder image is missing tag</p>
</td>

<td valign="top" width="51%"><p>TAA-1824: fix(cf_instance_template): build aaos-builder runtime image when tag missing Match aaos-builder DAG: check Artifact Registry, then templateRef aaos-builder-runtime-image build-defaults instead of kubectl child Workflow.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>5f20d94dc5f259ca7af29894144e846693410357</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1829</p>
</td>

<td valign="top" width="24%"><p>Cloud Workstation Config Creation Abort/Fails Due to Stale Terraform State Lock After Abrupt Kubernetes Pipeline Pod Termination</p>
</td>

<td valign="top" width="51%"><p>Cloud Workstation admin pipelines run Terraform on short-lived Jenkins K8s agent pods. When a pod dies mid-<code>terraform apply</code> (preemption, SIGKILL, node loss), the GCS state lock is never released, and <strong>every subsequent pipeline run blocks at</strong> <code>terraform init</code> <strong>itself</strong> with <code>googleapi: Error 412: ... conditionNotMet</code>. Today, only a manual <code>terraform force-unlock</code> recovers - platform stays down until an operator intervenes (TAA-1829).</p>
<p>This PR is a deliberately scoped, temporary mitigation (until Cloud-WS migrates to Argo Workflows + KCC). It <strong>(a) hardens the agent pod against abrupt eviction</strong> and <strong>(b) auto-recovers stranded locks before the next</strong> <code>terraform init</code> with conservative safety guards.</p>
<p><strong>Key changes</strong> <code>workloads/cloud-workstations/pipelines/utils/terraform-utils.sh</code></p>

<ul>
<li><p><strong>New</strong> <code>release_stale_terraform_lock()</code> - detects a stranded <code>.tflock</code> in GCS and removes it via <code>gcloud storage rm</code> only if <strong>all</strong> of:</p>

<ol start="1">
<li><p>Lock age ≥ <code>TF_STALE_LOCK_AFTER_SECONDS</code> (default <code>1800</code> s, wider than the ~15-20 min worst-case cluster create/delete).</p>
</li>

<li><p>The pod named in <code>Who: &lt;user&gt;@&lt;hostname&gt;</code> no longer exists in the namespace (<code>kubectl get pod</code>).</p>
</li>

<li><p>Jenkins-side <code>disableConcurrentBuilds + buildBlocker</code> already rules out a concurrent legitimate owner. Any uncertain signal (young lock, live owner pod, malformed JSON, unparseable <code>Created</code>, no <code>@host</code> in <code>Who</code>) → log a warning and <strong>leave the lock alone</strong> (terraform will then fail loudly, preserving today&#39;s safety net).</p>
</li>
</ol>
</li>

<li><p><strong>Recovery runs pre-init</strong> (not post-init). Reason: the gcs backend acquires a lock during init&#39;s state-load step (<code>If-Generation-Match: 0</code> on the <code>.tflock</code> object); a pre-existing stale lock fails that precondition with HTTP 412 and breaks init itself, so any post-init hook is unreachable in the very scenario it&#39;s meant for.</p>
</li>

<li><p><strong>Backend prefix read from</strong> <code>./backend.tf</code> (always in cwd) instead of <code>.terraform/terraform.tfstate</code> (doesn&#39;t exist pre-init).</p>
</li>

<li><p><strong>Removal uses</strong> <code>gcloud storage rm</code>, not <code>terraform force-unlock</code>. Reason: <code>terraform force-unlock</code> requires an initialised backend, which by definition isn&#39;t available pre-init. Both ultimately perform the same DELETE on the same GCS object via the same workload-identity credentials.</p>
</li>

<li><p><code>terraform apply</code> <strong>/</strong> <code>terraform destroy</code> <strong>now run with</strong> <code>-input=false -lock-timeout=120s</code> for non-interactive operation and a deterministic 2-minute lock-acquire wait that absorbs brief buildBlocker handover races.</p>
</li>
</ul>
<p><strong>Cloud-WS write Jenkinsfiles</strong> (<code>create-cluster</code>, <code>delete-cluster</code>, <code>create-config</code>, <code>update-config</code>, <code>delete-config</code>, <code>create-workstation</code>, <code>delete-workstation</code>, <code>add-workstation-user</code>, <code>remove-workstation-user</code>)</p>

<ul>
<li><p><code>cluster-autoscaler.kubernetes.io/safe-to-evict: "false"</code> <strong>annotation</strong> - prevents the cluster-autoscaler from evicting the agent during node scale-down.</p>
</li>

<li><p><code>terminationGracePeriodSeconds: 600</code> - if SIGTERM arrives, terraform has up to 10 min to finish in-flight work and release the lock cleanly.</p>
</li>

<li><p><strong>Removed hard-coded</strong> <code>hostname: jenkins-cloud-ws-build-pod</code> - K8s now uses the unique pod name as hostname, so the <code>.tflock</code>&#39;s <code>Who: &lt;user&gt;@&lt;hostname&gt;</code> captures the <strong>real pod name</strong>, which is what makes the recovery&#39;s owner-pod check reliable.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6529768a64fedf79c1e5b2a88eb3087b6c514e16</code></p>
</li>

<li><p><code>ee815b7a882f682c9728adcdfb7ae2d979467f86</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1835</p>
</td>

<td valign="top" width="24%"><p>Terraform apply fails on root Argo CD Application when scm_repo_branch differs from live targetRevision</p>
</td>

<td valign="top" width="51%"><p>TAA-1835: fix(sdv-gke-apps): force-conflicts on root Argo Application for scm_repo_branch</p>

<ul>
<li><p>Argo CD (argocd-server) and Terraform both manage spec.source.targetRevision via server-side apply. When scm_repo_branch in tfvars differs from the live ref (e.g. env/dev vs a feature branch), apply fails with an SSA field-manager conflict.</p>
</li>

<li><p>Set force_conflicts on kubectl_manifest.argocd_application so Terraform can update targetRevision and embedded Helm values (scm.branch, workloads.*.branch) from tfvars.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>89d59ac5ff0bce81bee9fa807a28253c9d917b16</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1837</p>
</td>

<td valign="top" width="24%"><p>[Module Manager] disable leaves workloads-android Argo apps healthy while Portal shows UNINSTALL IN PROGRESS</p>
</td>

<td valign="top" width="51%"><p>TAA-1837: fix(module-manager): re-delete prefixed child Application on disable</p>

<ul>
<li><p>After clearParentSkipReconcile, mod-workloads-android can reconcile from Git and recreate workloads-android before the parent Application is deleted, leaving enabled=false with healthy Argo apps and Portal UNINSTALL IN PROGRESS.</p>
</li>

<li><p>Call deletePrefixedChildApplicationIfPresent before parent delete and again after it is issued. Add unit test.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>57d5d9b7c7761f0aa75cca2957ebebb79eea964</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1843</p>
</td>

<td valign="top" width="24%"><p>GCS bucket deletion fails pipeline despite successful removal</p>
</td>

<td valign="top" width="51%"><p>This PR updates the ABFS Spanner destroy script to handle cases where <code>gcloud storage rm --recursive</code> returns a non-zero exit code even though the bucket has been successfully removed. Previously, the script relied directly on the exit code from <code>gcloud storage rm</code>. Because the script runs with <code>set -e</code>, any non-zero return code caused the job to fail immediately. In some cases, <code>gcloud storage rm</code> was returning <code>1</code> despite the bucket being deleted, causing Jenkins to report a false failure. The fix allows the delete command to complete without immediately failing the script, then verifies the actual bucket state using <code>gcloud storage buckets describe</code>.</p>
<p>With this change, the script now checks whether the bucket still exists after deletion:</p>

<ul>
<li><p>If the bucket still exists, the job fails correctly.</p>
</li>

<li><p>If the bucket no longer exists, the job succeeds correctly.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>77d6df37597e2121d31dc3f33d974e7a5e777cef</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1847</p>
<p>TAA-1848</p>
</td>

<td valign="top" width="24%"><p>[Security] spdystream module update (v0.2.0-&gt; v0.5.1)</p>
<p>[Security] spdystream module update (v.0.5.0-&gt;v.0.5.1)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong> Fixes TAA-1847, 1848 by upgrading [Security] spdystream module update (v0.2.0-&gt;v0.5.1) in the horizon-api, module-manager and workflow-namespace-drain module.</p>
<p><strong>Changes</strong> Updated the following files:</p>

<ul>
<li><p>terraform/modules/sdv-container-images/images/horizon-api/horizon-api-app/go.mod</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/horizon-api/horizon-api-app/go.sum</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.mod</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.sum</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.mod</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.sum</p>
</li>
</ul>
<p>As part of the dependency refresh, kubernetes and golang dependencies were updated through go mod tidy. No changes were made to Dockerfile, Helm, or Terraform outside these paths.</p>
<p><strong>Dependency notes</strong> spdystream remains an indirect dependency. The direct dependency chain continues to come through the Kubernetes apimachinery version.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>20e3a804dff49400c988c019ea164d77d8f1c757</code></p>
</li>

<li><p><code>ddf4012165b56856b865c69468e65025831d679f</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1849</p>
</td>

<td valign="top" width="24%"><p>[Security] PostCSS module update (8.4.31 -&gt; 8.5.15)</p>
</td>

<td valign="top" width="51%"><p>Summary [Security] PostCSS module update (8.4.31 -&gt; 8.5.15)</p>
<p>Files updated</p>

<ul>
<li><p>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/package.json</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/package-lock.json</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>12ed6f34e214cd975ca4459efe2aa907334fb07c</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1850</p>
<p>TAA-1851</p>
</td>

<td valign="top" width="24%"><p>[Security] React from Facebook module update ( 19.2.5 -&gt; 19.3.0-canary-fef12a01-20260413)</p>
<p>[Security] react-is module update ( 19.2.5 -&gt; 19.3.0-canary-fef12a01-20260413)</p>
</td>

<td valign="top" width="51%"><p><strong>Summary</strong> <code>react</code> and <code>react-is</code> has been updated from <code>19.2.5</code> to <code>19.3.0-canary-fef12a01-20260413</code> to fix security issue found in vulnerability scan.</p>
<p><strong>Files updated.</strong></p>

<ul>
<li><p>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/package.json</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/package-lock.json</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/Dockerfile</p>
</li>

<li><p>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/.npmrc</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>03737c1f394757786b645682a92db2bbba4fa96a</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1863</p>
</td>

<td valign="top" width="24%"><p>Missing Accenture Copyright Notices</p>
</td>

<td valign="top" width="51%"><p>This PR improves copyright compliance across the Horizon SDV repository by adding missing <strong>Accenture copyright notices</strong> to source files created and maintained by Accenture.</p>
<p><strong>Details</strong></p>

<ul>
<li><p>Scanned repository files to identify missing copyright notices.</p>
</li>

<li><p>Added appropriate Accenture copyright headers to eligible source files across:</p>

<ul>
<li><p><code>gitops/</code></p>
</li>

<li><p><code>workloads/</code></p>
</li>

<li><p><code>terraform/</code></p>
</li>

<li><p><code>tools/</code></p>
</li>

<li><p><code>docs/</code></p>
</li>
</ul>
</li>

<li><p>Excluded trivial files and file types where copyright notices are not applicable or technically feasible.</p>
</li>

<li><p>Improved overall copyright coverage and alignment with organizational compliance requirements.</p>
</li>
</ul>
<p><strong>Purpose</strong></p>
<p>These changes ensure that Accenture-owned source files contain the required copyright notices, improving legal compliance, ownership attribution, and repository consistency across the Horizon SDV codebase.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>bf4a41995a8277a01284e0992f6f48c988d8b1fc</code></p>
</li>

<li><p><code>c1c58825fa7883521bd3b8412952a9bacc9b6e9a</code></p>
</li>

<li><p><code>560caa3122747cad3e29785d37646793e2dd984b</code></p>
</li>

<li><p><code>bc5042ab1ad599c3f1d87e6066903dc115a40ff9</code></p>
</li>

<li><p><code>713d90ed15cb4b93cd2f419ea941dc09160503fb</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1867</p>
</td>

<td valign="top" width="24%"><p>AAOS SDV Builder not able to fetch gerrit changes</p>
</td>

<td valign="top" width="51%"><p>Found inconsistence in configuration between AAOS Builder and AAOS SDV Builder. AAOS SDV Builder sets GERRIT_API_URL to external gerrit repositories, what causes it to fail with open changes to be fetched.</p>
<p><strong>Changes</strong></p>
<p>Unify the configuration across builders to point to gerrit server in HORIZON_DOMAIN.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>bc7f0cb797938cc30c466391d96985de44a6c5c1</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1886</p>
</td>

<td valign="top" width="24%"><p>[Developer Portal] Android Workload module stuck in UPDATE IN PROGRESS state</p>
</td>

<td valign="top" width="51%"><p>After first enable on the Developer Portal Administration page, <code>workloads-android</code> stayed in <strong>UPDATE IN PROGRESS</strong> while other modules showed <strong>READY</strong>. The child Argo CD Application was healthy; the parent <code>mod-workloads-android</code> was <strong>OutOfSync + Healthy</strong> because the managed child <code>Application/workloads-android</code> drifted on <code>spec.syncPolicy.automated.selfHeal</code>.</p>
<p>Git desired <code>selfHeal: false</code>, but Argo CD did not persist that zero-value field on apply (<code>omitempty</code>). With self-heal intentionally off on parent and child, the parent never re-synced that drift, so the portal label never cleared.</p>
<p><strong>Fix</strong></p>
<p>Remove explicit <code>selfHeal: false</code> from both prefixed-module child Application templates. Self-heal defaults to <code>false</code> when omitted, so runtime behavior is unchanged; desired state now matches what Argo reliably stores (<code>automated: { prune: true }</code> only).</p>
<p><strong>Changes</strong></p>

<ul>
<li><p><code>gitops/modules/workloads-android/templates/application-workloads-android.yaml</code> - drop <code>selfHeal: false</code>; document why it must not be set explicitly</p>
</li>

<li><p><code>gitops/modules/workloads-common/templates/application-workloads-common.yaml</code> - same change for consistency (common only avoided the wedge by chance)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>3e250afef49e5ae0fe44444c4ca23c836a45cf08</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1893</p>
</td>

<td valign="top" width="24%"><p>Cloud Workstations: Disable initial concurrent builds for write/update/delete ops in pipeline suite</p>
</td>

<td valign="top" width="51%"><p>Fixes a first-build race in Cloud Workstations Jenkins write pipelines (TAA-1893) where concurrent Create and Delete config jobs could run before mutual-exclusion rules were active, leading to stale tfvars and unintended Terraform destruction via <code>for_each</code>.</p>
<p>The fix applies the existing Android Mirror pattern: <code>blockOn()</code> <strong>in seed DSL</strong> (installs <code>BuildBlockerProperty</code> at seed time, before build #1) plus <code>buildBlocker()</code> <strong>in Jenkinsfiles</strong> (runtime second layer).</p>
<p><strong>Key changes</strong></p>
<p><strong>1. Seed DSL</strong> - <code>blockOn()</code> <strong>on all 11 write jobs</strong></p>
<p><strong>Files:</strong> <code>*/groovy/job.groovy</code> under cluster-, config-, and workstation-admin/user write operations</p>
<p><strong>Reasoning:</strong> Jenkinsfile <code>options { buildBlocker() }</code> only persists <code>BuildBlockerProperty</code> into job <code>config.xml</code> <strong>after</strong> the first build completes. On build #1, the queue dispatcher sees no blocker → concurrent dispatch is allowed. This was reproduced on lab with a clean folder delete + seed. <code>blockOn()</code> is Job DSL sandbox-safe (unlike <code>configure {}</code>, which failed seed with <code>Rejecting unsandboxed method call: groovy.util.Node.div</code>) and writes the blocker into <code>config.xml</code> at seed time - matching Android Mirror and Cloud-WS image builder jobs.</p>
<p><strong>2. Jenkinsfile - include</strong> <code>update</code> in <code>buildBlocker</code> <strong>regex</strong></p>
<p><strong>Files:</strong> 19 Cloud Workstations Jenkinsfiles (write + read pipelines that declare <code>buildBlocker</code>)</p>
<p><strong>Reasoning:</strong> The original regex omitted <code>update</code>, so Update Existing Configuration was not in the mutual-exclusion set at runtime. Update can modify the same shared config Terraform state as Create/Delete - it must participate in blocking.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>45db1d3ea7e11677d0b499420cd6ef801ddde5b6</code></p>
</li>

<li><p><code>97d728abf2fedf36b88c56763abb788ad90ef8d2</code></p>
</li>

<li><p><code>38fc509a0cbe3e99f418d0efb623b55cd92e8658</code></p>
</li>

<li><p><code>b19233c5582fab788e915271bf83903f1ff7474a</code></p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Known Issues</h2>
<p><strong>TAA-1891</strong> Gemini Code Assist in Code OSS Workstation Agent Mode Does Not Detect Configured MCP Servers</p>
<p>In Horizon Code OSS, MCP servers configured for Gemini may not appear or work in the Gemini Code Assist chat window (Agent mode), including <code>/mcp</code> showing no servers. Configuration and MCP connectivity are otherwise valid.</p>
<p><strong>Workaround:</strong> Use Gemini CLI for MCP-enabled workflows until this is resolved.</p>

<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 4.0.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>14.04.2026</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p>Horizon SDV 4.0.0 is the new Horizon major release which extends platform capabilities with support for Agentic AI features and experimental implementation for new Horizon Modular Architecture designed based on ARGO application suite (Events/Workflows/CD). </p>
<p>Horizon SDV solution in Rel.4.0.0 enables coexistence of know Horizon architecture with the new modular architecture which introduces Module Manager to manage Horizon entities defined as Modules. Google KCC solution is used to spin up dynamic GCP resources required for Horizon. This release introduce limited PoC/MVP implementation with open Horizon API and CLI  with support for the first Android Build Module. New Developer Portal Application can be used to manage Horizon services in the new architecture.  </p>
<p>Horizon Rel.4.0.0 also delivers also several feature improvements and critical bug fixes.</p>
<p>Horizon SDV 3.1.0 package offers fully verified and documented upgrade patch (from Rel.3.0.0 to Rel.3.1.0). (see details in /docs/guides/upgrade_guide_3_0_0_to_3_1_0.md)</p>

<h2>New Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1272</p>
<p></p>
</td>

<td valign="top" width="24%"><p><strong>Agentic AI for Build Error Resolution</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Gemini AI–assisted review</strong> option is available to analyse failures in Android builds, CTS results and OpenBSW builds. It can be enabled in each of these build/test jobs via the Jenkins parameter <code>ENABLE_GEMINI_AI_ASSISTANT</code> (with related Gemini parameters where applicable), where it uses the provided prompts and skills. It can also be performed in a standalone job (<strong>Utilities → Gemini AI Assistant</strong>) which supports AI review of downloaded artifacts using custom prompts and skills. Gemini-assisted diagnosis is <strong>experimental</strong>; behavior, quality, and availability can change without notice.</p>
<p>Agent skills model (task vs instruction) is utilised where</p>

<ul>
<li><p><strong>Prompt</strong> = the <strong>task</strong> for this run: a short, one-line invocation (what to do now).</p>
</li>

<li><p><strong>Skill</strong> = the <strong>instruction set</strong>: role, procedure, rules, and expected output format.</p>
</li>
</ul>
<p>The skills to be used in each type of review (AAOS Build, OpenBSW Build, CTS test run) are defined in a <code>skills.yaml</code> file located in each pipeline's <code>prompt/sequenced/</code> directory. Specialized skills (e.g. triage, root-cause, proposed fixes) are defined in <code>skills.yaml</code>so behavior stays consistent and review steps stay maintainable in Git.</p>
<p>Both single pass(one prompt) and sequenced analysis(two or three prompts in order) are supported.</p>
<p><strong>Build Jobs enhancements</strong></p>
<p><strong>Android — AAOS Builder and ABFS Builder</strong></p>
<p>These pipelines build Android Automotive / Cuttlefish / AVD-style targets from manifest-driven source (standard <strong>AAOS</strong>) or from the <strong>Android Build Filesystem (ABFS)</strong> stack when using <strong>ABFS Builder</strong>. Optional <strong>Gemini AI Review</strong> runs after a <strong>failed</strong> build when <code>ENABLE_GEMINI_AI_ASSISTANT</code> is enabled and <code>AAOS_LUNCH_TARGET</code> is set, using the shared <strong>three-step</strong> flow: triage, root-cause analysis, then suggested fixes. Prompts and <code>skills.yaml</code> live under <code>workloads/android/pipelines/builds/aaos_builder/prompt/sequenced/</code>; <strong>ABFS Builder reuses that same directory</strong> so behavior stays aligned even though ABFS uses different cache paths and may prepare build logs differently (for example preserving more of <code>aaos-build.log</code> for early failures). Configuration follows the other Android build jobs: <code>GEMINI_COMMAND_LINE</code> for the headless Gemini CLI (model and flags), and <code>GEMINI_AI_EXECUTION_TIMEOUT</code> on AAOS Builder to cap how long the assistant may run. Artifact upload uses the usual storage parameters (<code>AAOS_ARTIFACT_STORAGE_SOLUTION</code>, bucket overrides, labels).</p>
<p><strong>OpenBSW — BSW Builder</strong></p>
<p><strong>BSW Builder</strong> builds <strong>Eclipse OpenBSW</strong> (POSIX and embedded targets, unit tests, optional coverage and platform builds as configured). It is <strong>not</strong> an Android job, but it follows the <strong>same overall AI Review pattern</strong>: when <code>ENABLE_GEMINI_AI_ASSISTANT</code> is <code>true</code> and the job status is <code>FAILURE</code>, <strong>AI Review</strong> stage runs <code>gemini_initialise.sh</code> then <code>gemini_analysis.sh</code> (with a bounded runtime), driven by repo prompts under <code>workloads/openbsw/pipelines/builds/bsw_builder/prompt/sequenced/</code> — <code>step1_triage.txt</code>, <code>step2_rca.txt</code>, <code>step3_fixes.txt</code> plus <code>skills.yaml</code> loaded like the Android flows.  AI Review to be treated as <strong>assistive</strong> analysis on failures, not as a substitute for manual reading of compiler and test logs.</p>

<ul>
<li><p><strong>AI analysis output:</strong> For each issue a file,<code>gemini_proposed_fixes_&lt;error_ID_or_unique_identifier&gt;_&lt;timestamp&gt;.md</code>, is created in the folder <code>gemini-assist</code>. Each such file includes a short root cause summary, location of file, a git style diff patch (where applicable), verification steps to ensure the suggested fixes work as expected and reference links if available.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1270</p>
<p></p>
</td>

<td valign="top" width="24%"><p><strong>Agentic AI for CTS Failure Triage &amp; Root Cause Analysis</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Test Jobs enhancements</strong></p>
<p>CTS Execution focuses on <strong>Tradefed / CTS results plus Cuttlefish logs</strong>; CVD Launcher focuses on <strong>Cuttlefish / CVD runtime only</strong> (no Compatibility Test Suite).</p>

<h4><strong>CTS Execution</strong></h4>
<p>The <strong>CTS Execution</strong> pipeline runs <strong>Tradefed</strong> against <strong>Cuttlefish</strong> virtual devices and publishes Compatibility Test Suite artifacts (HTML/XML under the usual result paths). Optional <strong>Gemini AI Review</strong> runs in the shared <code>cvdPipeline</code> <strong>Diagnostics</strong> stage when <code>ENABLE_GEMINI_AI_ASSISTANT</code> is enabled, the overall result is a <strong>failure</strong>, <strong>Launch Virtual Devices</strong> did not fail its stage, and the job is <strong>not</strong> in list-only mode (<code>CTS_TEST_LISTS_ONLY</code> must be false—plan-discovery runs skip AI Review). The preset is <code>'cts'</code>. Sequenced prompts under <code>workloads/android/pipelines/tests/cts_execution/prompt/sequenced/</code> drive <code>triage-cts</code>, <code>rca-cts</code>, and <code>fix-cts</code> in <code>skills.yaml</code>. When Tradefed outputs exist, analysis emphasizes <strong>which tests failed</strong> and ties them to logs; when suite summaries are missing (for example devices never came up), triage follows a <strong>CVD-oriented</strong> path that still prioritizes guest <code>kernel.log</code> and related Cuttlefish material. Set <code>GEMINI_ANALYSE_ON_SUCCESS=true</code> if you want AI Review to run on <strong>successful</strong> builds too (Gemini must still be enabled; a failing Gemini step can still fail the job). Artifact collection includes CTS result trees plus shared Cuttlefish patterns (host <code>cvd</code> logs, <code>cuttlefish_logs*.zip</code>, Wi‑Fi logs, etc.).</p>
<p><strong>CVD Launcher</strong></p>
<p>The <strong>CVD Launcher</strong> job exercises <strong>Cuttlefish</strong> (launch, MTK Connect, keep-alive) <strong>without</strong> running CTS. Its AI Review uses <code>preset: 'cvd'</code> and the sequenced bundle under <code>workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced/</code> — <code>triage-cvd</code>, <code>rca-cvd</code>, <code>fix-cvd</code> — focused on <strong>boot, launch, and runtime</strong> issues (guest <code>kernel.log</code>, <code>launcher.log</code>, host orchestration), not Tradefed assertions. The same gates apply as above except there is <strong>no</strong> CTS list-only parameter. <code>GEMINI_ANALYSE_ON_SUCCESS</code> is supported so you can analyze logs even when the pipeline <strong>passed</strong>, which helps spot questionable device health that a green build might hide; optional <strong>Utilities → Gemini AI Assistant</strong> flows documented for these workloads use the <strong>CVD Launcher</strong> prompt set when the question is purely Cuttlefish behavior.</p>

<ul>
<li><p><strong>AI analysis output:</strong> For each failure reported a file, gemini-assist/proposed_fix_{failure_type}_{timestamp}.md, is created in the folder <code>gemini-assist</code>. Each such file contains a short summary, evidence log (stack trace), technical root cause, proposed code remediation and reference links if available.</p>
</li>
</ul>
<p><strong>Utilities: Gemini AI Assistant</strong></p>

<ul>
<li><p><strong>Utilities → Gemini AI Assistant</strong> is a Jenkins job for <strong>ad-hoc</strong> runs: upload one or more prompt files (single or multi-step analysis), provide a command to fetch artifacts (for example, copying Build or CTS test-results from GCS), and run gemini-cli with parameters.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1179</p>
<p></p>
</td>

<td valign="top" width="24%"><p><strong>Modularize Horizon into separately usable services [R4: MVP]</strong></p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Architecture proposal for modularization designed and agreed, covering e.g. deployment approach for a single component (Cloud Android Orchestration, ABFS) vs. deployment of full Horizon and usage of technologies on different levels of deployment.</p>
</li>

<li><p>Modular deployment of selected components possible by a simple change of configuration.</p>
</li>

<li><p>Components that are separately deployable are defined.</p>
</li>

<li><p>Components and parts that will always be required are defined.</p>
</li>

<li><p>Components abstracted (e.g. Build as Service, CTS as Service, Virtual Device as Service,…) and isolated with clear interfaces / handover points</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1584</p>
<p></p>
</td>

<td valign="top" width="24%"><p><strong>Developer Portal [MVP]</strong></p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Developer Portal supports Keycloak authentication</p>
</li>

<li><p>Developer Portal discovers existing exposed Horizon API and show it to the user</p>
</li>

<li><p>Developer Portal controls Horizon API for running Workflow Templates</p>
</li>

<li><p>Developer Portal controls Module Manager for enabling or disabling modules (through Rest API and internal endpoint)</p>
</li>

<li><p>Developer Portal reads current module state from Module Manager</p>
</li>

<li><p>Developer Portal shows Workflow execution logs live</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1745</p>
<p></p>
</td>

<td valign="top" width="24%"><p><strong>Enable support for non-GitHub SCM repository</strong></p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Added support for non-GitHub SCM repository like Gerrit or Gitlab</p>
</li>

<li><p>new template for terraform.tfvars</p>
</li>

<li><p>Extends the Horizon SDV platform to work with any Git-based source code management system — not just GitHub. Introduces a configurable SCM layer (scm_type, scm_auth_method) that supports three authentication modes:<br/>app — GitHub App (existing behavior, GitHub-only)<br/>userpass — username/password or token via HTTP basic auth (works with Gerrit, GitLab, Gitea, Bitbucket, or any Git server)<br/>none — public repositories with no authentication</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1744</p>
<p></p>
</td>

<td valign="top" width="24%"><p><strong>Static IP Support</strong></p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Horizon enables possibility selecting zone between delegation with “NS” record and static IP assignments with “A” record</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Improved Features</h2>
<p></p>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="12%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Feature</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Description</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1313</p>
<p></p>
</td>

<td valign="top" width="24%"><p>Update OpenBSW to support latest 'main'</p>
</td>

<td valign="top" width="64%"><p>OpenBSW has been updated to incorporate the latest features and tooling enhancements:</p>

<ul>
<li><p><strong>Dockerized build environment</strong>: Docker container image updates for additional tools, packages and build support.</p>
</li>

<li><p><strong>Build and test targets</strong>: Updated to latest support.</p>
</li>
</ul>
<p><strong>Change Summary</strong></p>
<p><code>OpenBSW Workflows → Environment → Docker Image Template</code>updates:</p>

<ul>
<li><p>Tools and python updated.</p>
</li>
</ul>
<p><code>OpenBSW Workflows → Builds → BSW Builder</code> updates:</p>

<ul>
<li><p>Added default parameters for RTOS, compiler, and build configuration. These values remain overridable via the command line</p>
</li>

<li><p>Build logs and details are uploaded to bucket storage for OpenBSW → Builds → BSW Builder for archival</p>
</li>

<li><p>Documentation builds updated.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-483</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[Gerrit] Support TOPIC: build changes spanning multiple repositories</p>
</td>

<td valign="top" width="64%"><p><strong>Gerrit Builds</strong></p>

<ul>
<li><p>Gerrit now includes a <strong>Ready for Build</strong> label so users can decide when Jenkins builds should start.</p>
</li>

<li><p>To trigger a build, select <strong>REPLY → Ready for Build +1 → SEND</strong>.</p>
</li>

<li><p>The Jenkins Gerrit job triggers on a <strong>Ready for Build +1</strong> vote. It waits 2 minutes before starting to allow users to change their mind (e.g., <strong>Ready for Build 0/-1</strong>).</p>
</li>

<li><p>The Jenkins job builds all commits in <code>GERRIT_TOPIC</code>; if no topic is set, it builds the single change by Change‑Id.</p>
</li>

<li><p>The job updates Gerrit with per‑target build status and posts the overall <strong>Verified</strong> vote.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1269</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] Adjust CVD/CTS options for improved costs</p>
</td>

<td valign="top" width="64%">
<h3>Cuttlefish Updates</h3>
<p>Additional flexibility for creation of Cuttlefish instance templates to: </p>

<ul>
<li><p>let developers create Cuttlefish VM instance templates from their custom machine types, i.e. specific series, CPU and Memory configuration over the standard machine types.</p>
</li>

<li><p>provide additional options to support building Cuttlefish from other repositories as an alternative to standard Google repository</p>
</li>
</ul>
<p><strong>Jobs updated</strong></p>

<ul>
<li><p><strong>Android Workflows → Environment → CF Instance Template</strong></p>

<ul>
<li><p>Added support to define the Android Cuttlefish repository to use:<br/><code>ANDROID_CUTTLEFISH_REPO_URL</code></p>

<ul>
<li><p>If a private repo, then provide credentials by defining <code>REPO_USERNAME</code> and <code>REPO_PASSWORD</code></p>

<ul>
<li><p>Note: the password is never exposed in Jenkins nor console. Added support for custom machine types:<br/><code>CUSTOM_VM_TYPE</code><br/><code>CUSTOM_CPU</code><br/><code>CUSTOM_MEMORY</code></p>
</li>
</ul>
</li>
</ul>
</li>

<li><p>Simply unset <code>MACHINE_TYPE</code> and define custom fields that match users requirements.</p>
</li>

<li><p>Default <code>MACHINE_TYPE</code> has changed to <code>n2-standard-32</code> as a trade off for costs vs performance.</p>
</li>

<li><p><strong>Miscellaneous:</strong></p>

<ul>
<li><p>Curl Upgrade Support has been added to update the version on <code>debian-12</code> based Cuttlefish instances, see:<br/><code>CURL_UPDATE_COMMAND</code></p>

<ul>
<li><p><code>x86_64</code>:</p>

<ul>
<li><p>The default parameter will upgrade curl to 8.1x from debian backports.</p>
</li>

<li><p>Users may remove the parameter if they wish to remain on 7.88.1.</p>
</li>
</ul>
</li>

<li><p><code>ARM64</code>:</p>

<ul>
<li><p>ARM instances currently, only support ubuntu 22.04 LTS version, so the parameter has no default defined.</p>
</li>
</ul>
</li>
</ul>
</li>

<li><p>New parameter to support working around android-cuttlefish.git build issues:<br/><code>ANDROID_CUTTLEFISH_POST_COMMAND</code></p>

<ul>
<li><p>Use git commands to switch to a specific sha1 when using branches (main), or cherry-pick workarounds/fixes to tagged versions which cannot be modified by google. e.g.</p>

<ul>
<li><p><code>git cherry-pick &lt;sha1&gt;</code></p>
</li>

<li><p><code>sed -i 's|https://git.kernel.org/pub/scm/linux/kernel/git/jaegeuk/f2fs-tools|https://github.com/jaegeuk/f2fs-tools|g' base/cvd/MODULE.bazel</code></p>
</li>
</ul>
</li>
</ul>
</li>

<li><p>Options to update CTS revisions and even offer capability to pull from GCS and not just official download site (i.e. add more control to versions because the download site has updated the same revision several times in the past, changing the tests):<br/><code>CTS_ANDROID_16_URL</code><br/><code>CTS_ANDROID_15_URL</code><br/><code>CTS_ANDROID_14_URL</code></p>
</li>
</ul>
</li>

<li><p><strong>Android Workflows → Environment → CF Instance Template</strong> <strong>ARM64</strong></p>

<ul>
<li><p>Same as CF Instance Template but ARM64 is currently only supported on one machine type. So use <code>MACHINE_TYPE</code> for now.</p>
</li>
</ul>
</li>

<li><p><strong>Android Workflows → Tests → CTS Execution</strong></p>

<ul>
<li><p>The resources have changed to align with the default Cuttlefish instance <code>MACHINE_TYPE</code></p>

<ul>
<li><p><code>NUM_INSTANCES = 7, CPU=4, MEMORY = 8192</code></p>
</li>

<li><p>Adjust these to suit your test instance, these are simply set to align with the Horizon default Cuttlefish instance. For example, the ARM64 instance is a 96CPU instance and thus more resources are available.</p>
</li>
</ul>
</li>

<li><p>Failures are summarized in the test_result_failures_suite.html file with the respective CTS job and published to Jenkins.</p>
</li>
</ul>
</li>
</ul>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1473</p>
<p></p>
</td>

<td valign="top" width="24%"><p>CF - Support SSH key updates on existing instances</p>
</td>

<td valign="top" width="64%"><p>Added the UPDATE_SSH_AUTHORIZED_KEYS parameter to allow refreshing SSH keys without recreating the full instance. This reduces deployment time and minimizes costs associated with instance churn.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1474</p>
<p></p>
</td>

<td valign="top" width="24%"><p>CF reduce Jenkins noise to minimum </p>
</td>

<td valign="top" width="64%"><p>These can all be run independent of Jenkins, so reduce as much Jenkins noise in variable names.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1141</p>
<p></p>
</td>

<td valign="top" width="24%"><p>Keycloak Roles and Groups consistency improvement </p>
</td>

<td valign="top" width="64%"><p>Roles have been updated for Keycloak to use Client Roles rather than Groups. This change is applied to ArgoCD, Grafana, Headlamp and Jenkins.</p>
<p>There are two roles created in each client in Keycloak: </p>

<ul>
<li><p>administrators</p>
</li>

<li><p>viewers</p>
</li>
</ul>
<p>And there are two Groups created named administrators and viewers. Group administrators has role mapping to each client’s admin role. So if a user wants admin permissions then it can be directly assigned to group administrators. And same apply to viewers group and roles.</p>

<ul>
<li><p>Added client roles to be used for authentication and authorization</p>
</li>

<li><p>Each application must use roles in scope. No group in scope.</p>
</li>

<li><p>A new group called administrators created. If a user is part of this group then user should get admin permission to those applications.</p>
</li>

<li><p>Applications should show admin permission to users who are part of group administrators. If user us not part if group administrators then admin resources/permission should not be visible/allowed.</p>
</li>

<li><p>A new group called viewers created. If a user is part if this group then user should not have admin permission to resources.</p>
</li>

<li><p>If a user is not part of either of these two groups administrator and viewers then user should not have any permission to any resource.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1479</p>
<p></p>
</td>

<td valign="top" width="24%"><p>Support for terraform -plan in Horizon deployment flow</p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Enhanced the Horizon deployment flow by introducing additional command-line options in the <code>deploy.sh</code> script to improve deployment control and flexibility.</p>
</li>

<li><p>Added support for deployment operations using the following options:</p>

<ul>
<li><p><code>-p / --plan</code> to preview infrastructure changes.</p>
</li>

<li><p><code>-a / --apply</code> to provision or update infrastructure.</p>
</li>

<li><p><code>-d / --destroy</code> to remove Terraform-managed infrastructure resources.</p>
</li>
</ul>
</li>

<li><p>Introduced <code>-h / --help</code> option to display usage instructions and available commands for the deployment script.</p>
</li>

<li><p>Improved script behavior by ensuring Terraform initialization is executed only when deployment operations (<code>plan</code>, <code>apply</code>, or <code>destroy</code>) are triggered.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1682</p>
<p></p>
</td>

<td valign="top" width="24%"><p>Update Landing Page with MCP Server application</p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Added a new app card in the "Applications" section linking to the MCP Gateway Registry subdomain (<code>mcp.&lt;domain&gt;</code>)</p>
</li>

<li><p>The Launch button dynamically resolves the URL using <code>location.protocol + '//mcp.' + location.hostname</code>, matching the gateway route pattern defined in <code>gitops/templates/gateway-mcp-gateway-registry.yaml</code></p>
</li>

<li><p>Uses the official logo <code>mcp-gateway-registry-logo.png</code> asset for the card icon</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1644</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[Jenkins] Replace CVD_ADDITIONAL_FLAGS with full CVD_COMMAND_LINE for CVD Launcher and CTS Execution</p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Expose the full Cuttlefish launch command as a single Jenkins string parameter<br/>with a default /usr/bin/cvd create line using shell placeholders for<br/>NUM_INSTANCES, VM_CPUS, and VM_MEMORY_MB. </p>
</li>

<li><p>cvd_environment.sh applies the same default when empty; cvd_start_stop.sh runs the command after sudo HOME=..… Update cvdPipeline and Job DSL for CVD Launcher and CTS Execution; refresh cvd_launcher.md and cts_execution.md. </p>
</li>

<li><p>Expose the full Cuttlefish launch command as a single Jenkins string parameter with a default /usr/bin/cvd create line using shell placeholders for NUM_INSTANCES, VM_CPUS, and VM_MEMORY_MB. cvd_environment.sh applies the same default when empty; cvd_start_stop.sh runs the command after sudo HOME=....</p>
</li>

<li><p>Update cvdPipeline and Job DSL for CVD Launcher and CTS Execution; refresh cvd_launcher.md and cts_execution.md.</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1532</p>
<p></p>
</td>

<td valign="top" width="24%"><p>GCS metadata - support for wildcard characters in paths</p>
</td>

<td valign="top" width="64%"><p>Updated <em>get_object_list</em> function to allow wildcard characters be able to be used so that that artifacts can more easily be accessed &amp; managed. This expands the functionality of all GCS Utility pipeline jobs. Markdown files updated to reflect the support for wildcards.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1689</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[MTK Connect] testbench: configurable ADB tunnel caller.port for device interfaces</p>
</td>

<td valign="top" width="64%">
<ul>
<li><p>Set tunnel.types[].caller.port from MTK_CONNECT_TUNNEL_PORT in create-testbench.js.</p>
</li>

<li><p>Propagate via mtk_connect.sh (.env) and cvdPipeline mtk_tunnel_port on mtk_connect invocations.</p>
</li>

<li><p>Add MTK_CONNECT_TUNNEL_PORT job parameter to CVD Launcher and CTS Execution Job DSL.</p>
</li>

<li><p>Document the parameter in docs/workloads/android/tests/cvd_launcher.md and cts_execution.md.</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Documentation update</h2>

<ul>
<li><p>docs/workloads/android/guides/developer_guide.md split into a more navigable set of smaller and more focused per-topic guides which are grouped into setup and training areas. </p>
</li>

<li><p>/docs/workloads/guides/pipeline_guide.md document updated with information how Keycloak Group and Roles improvements impacts workflows management and executions.  </p>
</li>

<li><p>Rel.4.0.0 provides with several updates in Horizon documentation including e.g. <strong>Horizon Deployment Guide</strong> (/docs/deployment_guide.md). </p>
</li>

<li><p>The new Upgrade Guide (/docs/guides/upgrade_guide_3_1_0_to_4_0_0.md) provide guideline for   Rel.3.1.0 -&gt; Rel.4.0.0 upgrade. </p>
</li>
</ul>

<h2>Bug Fixes</h2>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="10%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Bug</strong></p>
</td>

<td valign="top" width="51%"><p><strong>Description</strong></p>
</td>

<td valign="top" width="15%"><p><strong>SHA</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1274</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] CTS hangs - android-cuttlefish issues</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Move to 1.31.1 with cxx crate fix. Not as good as v1.28.0 but they created v1.28.1 without the patch! v1.28.1 == v1.28.0!</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>6f385bf6b9cbc2f7e3c0b5790254cad6b59b2712</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1290</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] ARM64 builds broken on f2fs-tools (missing)</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p><code>f2fs-tools fix for dead repo</code></p>
</li>

<li><p><code>CF show diffs made to build</code></p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>94913c903d5d9da2fc0c497580423a1a93bfedfe</code></p>
</li>

<li><p><code>e61b8d2c8d78685fcf4250aef622dee745026f67</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1317</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] File Parameter Plugin fix</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p><code>File Parameter update</code></p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>e26ab2953ae82a850214a8d1e664fe907220e7e0</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1506</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] CF instances - missing SSH authorized_keys (transient)</p>
</td>

<td valign="top" width="51%"><p>TAA-1506: Harden SSH key setup in CF instance template: sync and verify</p>

<ul>
<li><p>Run sudo sync after writing authorized_keys so the instance disk is flushed before creating the template/disk image, avoiding missing .ssh/authorized_keys when the FS was not synced.</p>
</li>

<li><p>Verify the key was copied by reading authorized_keys from the instance and ensuring it contains the expected public key; fail fast with clear errors and ask the user to repeat the job if verification fails.</p>
</li>

<li><p>Skip deletion if simply a key update.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/882">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/882</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>2594c4cb6d23bf581bedae74e8e4b89e8332c824</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1524</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] GCS metadata not applied for wildcard artifact paths</p>
</td>

<td valign="top" width="51%"><p>TAA-1524: Fix: GCS metadata listing for artifacts with wildcard character in name</p>

<ul>
<li><p>Code refactored to consolidate multiple metadata requests (with differing formatting requirements) into a single <em>object_get_data</em> call.<br/>Metadata request now being done using the command <em>gcloud storage object list</em> instead of <em>gcloud storage object describe</em></p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/909">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/909</a> </p>

<ul>
<li><p>Code refactored to consolidate multiple metadata requests (with differing formatting requirements) into a single object_get_data call.<br/>Metadata request now being done using the command gcloud storage object list instead of gcloud storage object describe</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/913">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/913</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>96a249959a788242e23afdd0453fcac53acc9095</p>
</li>

<li><p>5004ce19f1463c9259add9e5aec97c475f1b6b4b</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1533</p>
</td>

<td valign="top" width="24%"><p>[ABFS] Build with GERRIT_TOPIC failing on initialise (HTTP/2)</p>
</td>

<td valign="top" width="51%"><p>TAA-1533: fix(aaos): harden Gerrit git fetch for ABFS only (HTTP/1.1, TLS 1.2, retry)<br/>When ABFS_BUILDER is set (ABFS path), avoid HTTP/2 INTERNAL_ERROR, early EOF, and GnuTLS "Error decoding the received TLS packet" (curl 56) by configuring Git and retrying fetches. Non-ABFS builds keep the original single-run fetch behavior.</p>

<ul>
<li><p>configure_git_gerrit_fetch(): run only when ABFS_BUILDER != "false"; set postBuffer (500MB), keep-alive, HTTP/1.1, and TLS 1.2 for the Gerrit host.</p>
</li>

<li><p>fetch_and_cherry_pick_with_retry(): use only when ABFS; else eval REPO_CMD (topic and single-patchset paths).</p>
</li>

<li><p>Retry: up to 5 attempts, 20s delay; abort/reset between attempts.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/960">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/960</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>5003334924a16d205586f2266a1cc43972611a64</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1611</p>
</td>

<td valign="top" width="24%"><p>[Security] Dependabot - Axios is Vulnerable to Denial of Service</p>
</td>

<td valign="top" width="51%"><p>TAA-1611: [Security] Dependabot - Axios is Vulnerable to Denial of Service<br/>Dependabot reports 1 security alert for axios</p>

<ul>
<li><p>Current version is 1.13.2; a fix is available in 1.13.5 or later.</p>
</li>

<li><p>Updated the axios version to 1.13.6</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/958">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/958</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>5697512bf2315476287a3ddb0cc93508e96f0fea</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1527</p>
</td>

<td valign="top" width="24%"><p>After implementing strict firewall rule, filestore may not be available</p>
</td>

<td valign="top" width="51%"><p>TAA-1527: After implementing strict firewall rule, filestore may not be available</p>

<ul>
<li><p>reserved-ipv4-cidr: "10.152.0.0/24" added to the storage class</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/899">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/899</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>7f66bd4386a2a423b5d0682e0be2e63fe8195cbb</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1160</p>
</td>

<td valign="top" width="24%"><p>[ARM64] Lack of available instances on us-central1-b/f zone</p>
</td>

<td valign="top" width="51%"><p>TAA-1160: ARM64 instances availability</p>

<ul>
<li><p>Instance availability: us-central1-b do not have enough instances, so move to us-central1-f.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>7f66bd4386a2a423b5d0682e0be2e63fe8195cbb</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1656</p>
</td>

<td valign="top" width="24%"><p>[Gemini] Keychain init fails in AAOS and utilities Docker images: missing libsecret-1.so.0</p>
</td>

<td valign="top" width="51%"><p>TAA-1656: [Gemini] Keychain init fails in AAOS and utilities Docker images: missing libsecret-1.so.0</p>
<p>Update gemini documentation with known issues:</p>

<ul>
<li><p>missing pgrep output</p>
</li>

<li><p>Linux: CLI hangs after OAuth (GNOME Keyring / keytar)<br/>TAA-1620: [Gemini] Add libsecret-1-0 so Secret Service clients (e.g. gemini-cli, IDE credential helpers) find libsecret-1.so.0 when running inside the built images.</p>
</li>
</ul>
<p>TAA-1620: gemini: default GEMINI_FORCE_FILE_STORAGE for headless keychain/D-Bus<br/>Set GEMINI_FORCE_FILE_STORAGE=true in gemini_environment.sh so gemini-cli     </p>

<ul>
<li><p>skips libsecret/keytar (GNOME Keyring + D-Bus) in Argo/Jenkins containers without DISPLAY. Document the variable in the env header comment. Override with GEMINI_FORCE_FILE_STORAGE=false for interactive desktop use.<br/></p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>2546405452ebd760008d4a74fdff177e0cb52941 </p>
</li>

<li><p>55e57fe40d2b1c96fecbc17812f764bbd8fec72c</p>
</li>

<li><p>6cf633359eb02c5698f3dfd0958797a7f8f73181</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1519</p>
</td>

<td valign="top" width="24%"><p>Gemini CLI auto-selects preview models when preview is disabled and location is non-global</p>
</td>

<td valign="top" width="51%"><p>TAA-1519: Remove debug from Gemini<br/>Debug is too verbose and results in huge logs, in some cases 455MB. This can also impact on job performance in Jenkins, exhaust resources.<br/>TAA-1519: Reduce Jenkins build retention (job.groovy) to save built-in node pool space <br/>daysToKeep: 60 → 7, numToKeep: 200 → 50 across all pipelines<br/>TAA-1519: Increase Jenkins node pool resources 32Gi to 64Gi</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>512924f91803a23032803f6b6a68b98ad9ea2ab6</p>
</li>

<li><p>c0ef77577461417d890b9c74bcf202273855db2c</p>
</li>

<li><p>f04aaeb8662066a5996de2eb37af134e1334f364</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1663</p>
</td>

<td valign="top" width="24%"><p>[AAOS Builder] Gemini CLI step runs ~58 minutes with no clear errors in logs</p>
</td>

<td valign="top" width="51%"><p>TAA-1663: cap Step 3 AI review shell use (android-fix run_shell_command)<br/>Add Stage 3 latency guardrails in aaos_builder skills.yaml: anchor fixes on RCA context, cap read/grep/list tools, and treat run_shell_command as high-risk— forbid find/recursive grep/rg and build commands (m/ninja/bit); allow at most two bounded shell calls (e.g. head/tail/wc) on a single path already named in RCA.<br/>Tighten android-fix system_instructions (plan-then-act, prefer read_file/grep over shell). Update step3_fixes.txt with the same budgets at invoke time. Targets multi-hour Step 3 runs dominated by run_shell_command duration in headless stats.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/989">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/989</a> </p>
<p>TAA-1663: refine AI review: AAOS skills, step2 prompt, and optional step2 context cap<br/>AAOS builder (workloads/android/pipelines/ builds/aaos_builder/ prompt/sequenced/skills.yaml): </p>

<ul>
<li><p>Clarify global output rules: android-triage and android-rca stay response-only; android-fix may use write tools only for files under gemini-assist/ per FILENAME_RULE,</p>
</li>

<li><p>Tool-use guidance: narrow search scope (prefer rg with explicit paths and globs; exclude out/, out_*, and .repo); on grep failure or abort, narrow or read_file—do not widen scope.</p>
</li>

<li><p>Stage 1 latency: log-first triage from aaos-build.log and aaos-build-info.txt only (no deep tree scan), last 1000-line tail, soft cap of three read_file calls.</p>
</li>

<li><p>Stage 2 latency: triage-first (no full log re-scan; at most one 500-line tail if ambiguous); read paths from the matrix before grep; narrow grep roots; soft cap of eight ad/grep/list_directory calls with at most four grep_search; if still incomplete, finish with <a href="http://cs.android.com">cs.android.com</a> / search queries. </p>
</li>

<li><p>Triage matches expected output schema; RCA adds plan-then-act, context minimization, narrow-path search, stop when output_schema is filled; fix stage prefers search queries in headless/CI and verified URLs only when confirmable.</p>
</li>
</ul>
<p>AAOS step2 task prompt (step2_rca.txt): echo tool budget and no full-log rescan at invoke time.<br/>Shared sequenced pipeline (gemini_analysis.sh): optional cap when composing step 2 input—if GEMINI_STEP2_PRIOR_CONTEXT_BYTES is set to a positive integer, append only that many bytes of step1 output (head -c); unset or 0 = full step1 text (previous behavior for CTS/BSW and other jobs). Documented in gemini_environment.sh.<br/>AAOS-only wiring: GEMINI_STEP2_PRIOR_CONTEXT_BYTES=131072 on ai-review<br/>    ClusterWorkflowTemplate, aaos_builder and aaos_abfs_builder Jenkinsfiles (override or set 0 to disable). CTS and BSW Jenkins do not set it, so they behave as before.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/986">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/986</a> </p>
<p>TAA-1663: harden AAOS Gemini AI review (triage tail, caps, AAOS-only step2 context)</p>
<p>AAOS skills (prompt/sequenced/skills.yaml):</p>

<ul>
<li><p>Global output rules and write scope (triage/rca response-only; fix under gemini-assist/).</p>
</li>

<li><p>Narrow search guidance; Stage 1 log-first + TAA-1325-style triage limits; forbid triage grep_search and repo run_shell_command; use pipeline aaos-build.log.tail; Stage 2 caps and triage-first RCA; fix references favor queries in CI.</p>
</li>
</ul>
<p>Prompts:</p>

<ul>
<li><p>step1_triage.txt: tail-first, tool budget, no repo search in Step 1.</p>
</li>

<li><p>step2_rca.txt: tool budget and no full-log rescan.</p>
</li>
</ul>
<p>Pipeline:</p>

<ul>
<li><p>Write aaos-build.log.tail (last 2500 lines) before Gemini in ai-review CWT, aaos_builder and aaos_abfs_builder Jenkinsfiles.</p>
</li>

<li><p>Remove aaos-build.log.tail after ai-review on Argo (keep aaos-build.log for storage); Jenkins still cleans via aaos-build*.*.</p>
</li>
</ul>
<p>Shared gemini_analysis.sh:</p>

<ul>
<li><p>Optional GEMINI_STEP2_PRIOR_CONTEXT_BYTES (positive = head -c cap on step1 context for step 2); unset or 0 = full prior step (CTS/BSW unchanged).</p>
</li>

<li><p>gemini_environment.sh documents the variable.</p>
</li>
</ul>
<p>Argo ai-review CWT: GEMINI_STEP2_PRIOR_CONTEXT_BYTES=131072; Jenkins AAOS/ABFS default the<br/>same so only AAOS sequenced jobs cap step2 context by default.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/987">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/987</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>c1271dd15617a68e9901e0c029c978b42b7018ec</code></p>
</li>

<li><p><code>d305394004520202f8d320a56185f3afd76ea5cc</code></p>
</li>

<li><p>d1cab720a8bb105928e11a074353f29ff24ebd01</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1684</p>
</td>

<td valign="top" width="24%"><p>[Module Manager] Module Manager container image build fails</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Remove unused import of 'errors' from handler.go to clean up code.</p>
</li>

<li><p>Commenting out to avoid CRD not found error.</p>
</li>

<li><p>Enable workflows</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/990">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/990</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>13ba334fd636b8cc795b2b685d91b5f3e308badd</p>
</li>

<li><p>3ab8a1d5c6f7d4a17cc050e8a9b30d612ac5851c</p>
</li>

<li><p>83eaefadff4a429dd9a0557c68031c1c056e59f3</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1686</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] Warm Build Caches Jenkins job cannot select AAOS clean mode and always forces NO_CLEAN</p>
</td>

<td valign="top" width="51%"><p>TAA-1686: docs(android): align warm_build_caches README with Jenkinsfile and job.groovy<br/>Document job location, parameters (including AAOS_CLEAN first vs later stages), build stage order, tangorpro skip when AAOS_REVISION contains android-16.0.0_r, ANDROID_BUILD_ID prefixes, mirror vs no-mirror pod templates, and cache PVC settings.</p>
<p>TAA-1686: feat(android): add AAOS_CLEAN to Warm Build Caches Jenkins job<br/>Add the same AAOS_CLEAN choices as the main AAOS Builder job (NO_CLEAN, CLEAN_BUILD, CLEAN_ALL) with default NO_CLEAN. The first warm-cache stage (Build: aosp_cf_x86_64_auto) uses the parameter as-is. Later stages repeat NO_CLEAN or CLEAN_BUILD when that is what was selected; if CLEAN_ALL is selected, only the first stage runs it and later stages use CLEAN_BUILD so a full cache wipe cannot run more than once per pipeline. Replaces the previous hardcoded NO_CLEAN on every stage.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>d0b92263d63806a6782695bd2f9e6a57b51578cb</p>
</li>

<li><p>37ef6cc4d548cef8b547867234af9d2087c34f89 </p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1687</p>
</td>

<td valign="top" width="24%"><p>[Agentic AI] ABFS build does not stop at the first error, so Gemini AI Review can miss the real failure</p>
</td>

<td valign="top" width="51%"><p>TAA-1687: ABFS AI Review uses full build log; align triage skills with ABFS vs AAOS<br/>ABFS Jenkins copies aaos-build.log to aaos-build.log.tail so Gemini triage can see early failures (e.g. #error) that sit far above a short tail when the build does not stop on the first error. Document the intent in the Jenkinsfile. Update skills.yaml android-triage guardrails: allow grep_search scoped to the build log files, describe bounded tail/grep-prefix on AAOS/Argo versus full-copy .tail on ABFS, and keep read_file caps. Replace the step1_triage latency paragraph with a short pointer to skills.yaml so prompts stay in sync with the skill.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>c8b723c3edeb2f4185e6bb6ff93fb792a96cd235</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1691</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] Improve private repo support (private forks)</p>
</td>

<td valign="top" width="51%"><p>TAA-1691: Cuttlefish instance template private repo improvements.<br/>Fix HTTPS clone URL construction: strip trailing slashes, percent-encode credentials for embedded user:pass URLs, and require python3 when using private repos. Surface git clone/checkout failures, clone into an explicit<br/>repo directory, and clean up from the Packer working tree.<br/>Document private forks (e.g. horizon/main), derived GCP image and instance template names, and wiring the Jenkins GCE plugin via GitOps (values-jenkins.yaml) and the UI, including JENKINS_GCE_CLOUD_LABEL</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>82365d4f0def6eefaa6ff3cc38881d6d4a1c3ad5</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1650</p>
</td>

<td valign="top" width="24%"><p>Users in Viewer/Developer groups unable to view jobs on Jenkins dashboard</p>
</td>

<td valign="top" width="51%"><p>Fix for problem of Users in Viewer/Developer groups unable to view jobs on Jenkins dashboard by adding missing roles.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>e159b3306911375c4f7a3055cc37390c4f6e3f36</p>
</li>

<li><p>f2d64e1462f9b778b925beacbec995bcad828009</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1666</p>
</td>

<td valign="top" width="24%"><p>WorkflowTemplate.argoproj.io "" not found. TST deployment from env/dev not successful.</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Split <code>EventBus</code> and <code>EventSource</code> out of <code>gitops/templates/argo-events.yaml</code> Helm <code>extraObjects</code> into a dedicated root child Application (<code>argo-events-resources</code>) to separate platform CR ownership from the Helm controller release.</p>
</li>

<li><p>Moved Argo Events CR chart to <code>gitops/apps/argo-events-resources</code>.</p>
</li>

<li><p>Added explicit sync ordering (<code>argo-events</code> wave <code>4</code> -&gt; <code>argo-events-resources</code> wave <code>5</code>) so CRDs/controller are present before <code>EventBus</code>/<code>EventSource</code> apply.</p>
</li>

<li><p>Split AAOS webhook Sensors into a dedicated <code>workloads-android-aaos-webhooks</code> child Application so Android-specific webhook triggers are owned by the Android module and not rendered by the root chart.</p>
</li>

<li><p>Moved the AAOS webhook enable/disable toggle from root <code>gitops/values.yaml</code> to <code>gitops/modules/workloads-android/values.yaml</code> under <code>config.workloads.android.webhooks.enabled</code>, because root templates no longer consume that key and the module template does.</p>
</li>

<li><p>Updated comments/docs to clarify that sync waves help install ordering, but destroy-time finalizer behavior is still an operational concern and needs runtime validation.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/997">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/997</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>35104b0a3c40bc4e6034a93860bafab3e6cebe6e</p>
</li>

<li><p>07aef7da3858520b9eb78732b0549621b44a5863</p>
</li>

<li><p>893bef57d7bfa585b32cbfeb9787ce8ffa44883f</p>
</li>

<li><p>d56157e3076f3c82799fe646d89f0ef7b393d1e4</p>
</li>

<li><p>b0470eace1cc6343c6101e4af48722b092422966</p>
</li>

<li><p>a81ec5b834c260debb714b7b73664a544deb6feb</p>
</li>

<li><p>3fef1a1ba3288db10cfc41c72a8598749bf32d97</p>
</li>

<li><p>a8abaf421424b98d8bd262a201f7301fb04dc5a7</p>
</li>

<li><p>7b77a27d9acca823319241697f772da74fc1fc88</p>
</li>

<li><p>178e0c3b9071412bab4efe278e2e3a792dc4a4e9</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1693</p>
</td>

<td valign="top" width="24%"><p>[Agentic AI] Gemini AI Utility - pod evicted during run</p>
</td>

<td valign="top" width="51%"><p>TAA-1693: fix(utilities): harden Gemini AI Assistant pod against eviction</p>
<p>Align the Utility Jenkins agent with ai-review Gemini resources (16 CPU / 48Gi requests, 32 / 96Gi limits), schedule on the Android workload pool (nodeSelector + tolerations), set safe-to-evict false for autoscaler, extend idle sleep past the 2h analysis timeout, and document Android pool dependency in TODO comments.</p>

<ul>
<li><p>feat(gke): add utility node pool for Gemini AI Assistant Jenkins workload</p>
</li>
</ul>
<p>Introduce a dedicated GKE node pool (workloadLabel utility, taint workloadType=utility) sized for Vertex/Gemini CLI pod limits (aligned with ai-review resources). Default machine type is n2-standard-48 so allocatable CPU stays above 32-core limits after kube-reserved overhead</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>572d84e9b412450ae1f2b54ad6275b1fb8a00bee</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1692</p>
</td>

<td valign="top" width="24%"><p>[Agentic AI] Gemini CLI denies run_shell_command in headless --yolo mode</p>
</td>

<td valign="top" width="51%"><p>TAA-1692: gemini: workspace policy TOML for headless shell in CI</p>
<p>Fix headless Gemini runs failing with "Tool execution denied by policy" on run_shell_command: the CLI defaults shell to ask_user, which becomes deny without a TTY, even when GEMINI_COMMAND_LINE includes --yolo.</p>
<p>Add repo-maintained TOML under workloads/common/agentic-ai/gemini/policies/; gemini_initialise.sh copies every *.toml into .gemini/policies/ when Vertex auth runs and the job is Jenkins, CI=true, or Argo (ARGO_WORKFLOW_NAME), so workspace-tier rules can allow run_shell_command for interactive=false.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>02bde31a7ded989528b7567f3a4609638192f713</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1706</p>
</td>

<td valign="top" width="24%"><p>[MTK Connect] Skip testbench deletion when CVD launch fails</p>
</td>

<td valign="top" width="51%"><p>TAA-1706: fix(cvd): skip MTK testbench teardown when MTK stage never ran<br/>CVD launch failure skips "MTK Connect to Virtual Devices", so no testbench exists. Gate "MTK Connect Delete Testbench" and "Delete Offline Testbenches" on MTK_CONNECT_STAGE_ENTERED, set only when the MTK Connect stage runs.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>feb2c1cc47d87e20c562e7fb5593f2dfb1d5ae13</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1704</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] Kubernetes agent pod evicted during build</p>
</td>

<td valign="top" width="51%"><p>TAA-1704: fix(cvd): trim diagnostics pod resources and relax scheduling<br/> Align the CVD shared-library Kubernetes agent with Gemini AI Assistant requests/limits (16/32 CPU, 48Gi/96Gi) while keeping the Android build image. Drop pod anti-affinity and the aaos_pod label so the pod schedules more freely; other aaos_pod workloads may co-locate on the same node.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1009">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1009</a> </p>
<p>TAA-1704: fix(cvd): merge Jenkins pod YAML so safe-to-evict wins over cloud template<br/>Use yamlMergeStrategy merge() on the Diagnostics &amp; Teardown Kubernetes agent so the inline pod spec (including <a href="http://cluster-autoscaler.kubernetes.io/safe-to-evict">cluster-autoscaler.kubernetes.io/safe-to-evict</a>) is not overridden by the inherited Kubernetes cloud podTemplate.</p>
<p>TAA-1704: Align CVD/CTS Diagnostics Jenkins pod with AAOS Builder agent<br/>Match the kubernetes <code>builder</code> spec used in aaos_builder/Jenkinsfile: set requests and limits to 98000m CPU and 180000Mi memory, add the <code>aaos_pod</code> label, and required podAntiAffinity on <code>aaos_pod</code> so Diagnostics shares the<br/>same one-pod-per-node behaviour as other android build workloads.<br/>Set <a href="http://cluster-autoscaler.kubernetes.io/safe-to-evict">cluster-autoscaler.kubernetes.io/safe-to-evict</a> to "false" (Utility Gemini pattern) so long AI Review runs are less likely to be evicted via the Eviction API during cluster-autoscaler scale-down; node pressure can still evict.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1005">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1005</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>54e4aa21a869a985495f059c61a8785fed06b873</p>
</li>

<li><p><code>a0ba27167a62d22fbc9a44a477f872bde88e742a</code></p>
</li>

<li><p><code>62ffcf9a9a15216f6d44ca2da2a3370d4a817c1f</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1518</p>
</td>

<td valign="top" width="24%"><p>Ensure artifact summary stands out in logs</p>
</td>

<td valign="top" width="51%"><p>TAA-1518: Ensure artifact summary stands out in logs<br/>Display summary in one colour. Then display all the artifact lines in another. These will now stand out in Jenkins console and Argo Workflow logs.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/893">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/893</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>a98cb32a148c3d93fe8bafff937bf3ca3ac934b2</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1440</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] Persistent Host Build Regression (Bazel/Debian Dependency Failure)</p>
</td>

<td valign="top" width="51%"><p>Fix for [Cuttlefish] Persistent Host Build Regression (Bazel/Debian Dependency Failure)</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>29cb5fbf5e07ed322148100219a910e4ef53da60</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1665</p>
</td>

<td valign="top" width="24%"><p>Jenkins PVC uses hardcoded jenkins-rwo storage class instead of prefixed name</p>
</td>

<td valign="top" width="51%"><p>TAA-1665: fix(gitops): use prefixed storage class for Jenkins PVC in sub-environments</p>
<p>The Jenkins home PVC must reference the same StorageClass name as defined above the manifest ({{ .Values.config.namespacePrefix }}jenkins-rwo). A hardcoded jenkins-rwo breaks sub-environments where the class is prefixed and the PVC stays Pending. Aligns with int/3.1.0 / TAA-1057 naming.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>04159ba6d4f65fe4426d1f1727cbd7e65a470e07</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1664</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] authentication retry warning; instances unreachable after rebuild</p>
</td>

<td valign="top" width="51%"><p>TAA-1664: Cuttlefish - clarify Cuttlefish  user vs Docker user</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>d23ae7df4e88973f2fccc23319dc5ffe0c6040da</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1674</p>
</td>

<td valign="top" width="24%"><p>[AAOS Builder] Ambiguous parameter AAOS_BUILD_CTS - needs clarification</p>
</td>

<td valign="top" width="51%"><p>Added clarifications to ensure the user knows that by selecting the AAOS_BUILD_CTS option, only the test suite will be built and that no other target images will be built.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/982">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/982</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>3e2d8aa10a887805484cd3745f6ad84534434423</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1708</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] launch can report failure when host is missing lsof</p>
</td>

<td valign="top" width="51%"><p>TAA-1708: fix(cvd): install lsof for Cuttlefish host</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>2013780b6a29c4d350c5cfc6c11cc975579c59fb</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1690</p>
</td>

<td valign="top" width="24%"><p>[MTK Connect] start may hang on CVD Launcher testbench until pipeline timeout (exit 124)</p>
</td>

<td valign="top" width="51%"><p>TAA-1690: Skip CVD pipeline AI Review when MTK Connect --start fails</p>
<p>Set env.MTK_CONNECT_STAGE_FAILED from mtk_connect.sh exit status so Gemini does not triage CVD logs when the failure was an MTK Connect timeout or other connect-layer error. Unset when the MTK stage is skipped (CVD/CTS failure, CTS without MTK) so AI Review still runs for those cases.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>b79f39b8d5b02f5cc0d5a30a87447c262958d53d</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1688</p>
</td>

<td valign="top" width="24%"><p>[Agentic AI] Improve CTS Execution and CVD log handling for failures analysis</p>
</td>

<td valign="top" width="51%"><p>TAA-1688: fix(jenkins): pod yaml merge + safe-to-evict false to reduce CA eviction<br/> Avoid eviction issues, k8s evicting while job is running.</p>
<p>TAA-1688: fix(cvd): MTK delete stages gate on MTK_CONNECT_STAGE_ENTERED only</p>
<p>TAA-1688: feat(cvd): default CVD_COMMAND_LINE for CI without GPU or Bluetooth</p>
<p>Append --setupwizard_mode DISABLED, --enable_host_bluetooth false, and --gpu_mode guest_swiftshader to the standard cvd create line in cvd_environment.sh and the CVD Launcher / CTS Execution Jenkins parameter     defaults. This matches typical automation hosts: non-interactive boot, no host Bluetooth integration, and guest SwiftShader when GPU passthrough is not available.</p>
<p>TAA-1688: feat(gemini): per-step CLI runs, dual step3 context, strategy + docs<br/>    - gemini_analysis.sh: unique headless_output_stepN_<em>.json per step; extract from explicit JSON; step3 composed from step1 + step2 (GEMINI_STEP3_PRIOR_STEP</em>_BYTES); single-step uses step1 JSON + extraction<br/>    - gemini_environment.sh: document GEMINI_STEP3_PRIOR_STEP*_BYTES<br/>    - gemini.md: sequenced analysis (benefits, table, prompt/token budget how-to)<br/>    - token-budget follow-up</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>a35457dc4fdef442b63092f6bd8f53ffd46c40a6</code></p>
</li>

<li><p><code>7a329bf9cf7382d0f32e076721ce720c7f5b30e7</code></p>
</li>

<li><p><code>785f31e2ab66ec9f2f9547bb9f9749789db12420</code></p>
</li>

<li><p><code>9c8386f13fe104ffc7d66bde170cadc7369730d1</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1715</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] R4.0.0: upgrade Jenkins and current BOM (final)</p>
</td>

<td valign="top" width="51%"><p>Install the JDK with apt using only ${JAVA_VERSION}: add Adoptium apt when the package name starts with temurin-, then apt update/install. Remove Debian bookworm-backports and OpenJDK→Temurin fallback chains; verify with java -version and resolved JAVA_HOME.</p>

<ul>
<li><p>cf_host_initialise.sh: install_jdk_from_apt + verify_jdk; drop default-jdk from extra packages (alternatives clash)</p>
</li>

<li><p>cf_create_instance_template.sh: default JAVA_VERSION=temurin-21-jdk</p>
</li>

<li><p>job.groovy: default temurin-21-jdk (x86/Debian)</p>
</li>

<li><p>job_arm.groovy: default openjdk-21-jdk-headless (ARM/Ubuntu); Temurin optional</p>
</li>

<li><p>cf_instance_template.md: document apt package names and HTTPS egress needs</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1021">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1021</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>bb580aec1cde6c5347494198b6835fb466761f68</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1705</p>
</td>

<td valign="top" width="24%"><p>prepare-github-app-git-creds fail on fresh deploy due to CRD ordering</p>
</td>

<td valign="top" width="51%"><p>This PR fixes fresh deployment failures caused by CRD ordering for <code>prepare-github-app-git-creds</code>.<br/>The <code>ClusterWorkflowTemplate</code> is moved out of the root <code>horizon-sdv</code> app and into a dedicated child Application under <code>gitops/apps</code>, so it syncs only after <code>argo-workflows</code> is in place.</p>
<p><strong>Changes</strong></p>
<p><strong>argo-workflows-init.yaml</strong></p>
<p>File path: <code>gitops/templates/argo-workflows-init.yaml</code></p>

<ul>
<li><p>Removed inline <code>ClusterWorkflowTemplate</code> <code>prepare-github-app-git-creds</code> from the <code>git.authMethod == "app"</code> block.</p>
</li>

<li><p>Kept <code>workflow-github-app-secret</code> and <code>workflow-github-app-token-script</code> in this file.</p>
</li>

<li><p>Why: avoids the root app trying to apply a <code>ClusterWorkflowTemplate</code> before its CRD exists.</p>
</li>
</ul>
<p><strong>argo-workflows-github-app.yaml</strong></p>
<p>File path: <code>gitops/templates/argo-workflows-github-app.yaml</code></p>

<ul>
<li><p>Added a new child Argo CD <code>Application</code>, gated by <code>git.authMethod == "app"</code>.</p>
</li>

<li><p>Set child Application sync wave to <code>7</code>.</p>
</li>

<li><p>Points source path to <code>gitops/apps/argo-workflows-github-app</code>.</p>
</li>

<li><p>Why: ensures this app runs after <code>argo-workflows</code> (wave <code>6</code>) so the <code>ClusterWorkflowTemplate</code> CRD is available.</p>
</li>
</ul>
<p><strong>Chart.yaml</strong></p>
<p>File path: <code>gitops/apps/argo-workflows-github-app/Chart.yaml</code></p>

<ul>
<li><p>Added new Helm chart metadata for dedicated GitHub App workflow resources.</p>
</li>

<li><p>Uses generalized app name <code>argo-workflows-github-app</code> for future extensibility.</p>
</li>
</ul>
<p><strong>values.yaml</strong></p>
<p>File path: <code>gitops/apps/argo-workflows-github-app/values.yaml</code></p>

<ul>
<li><p>Added chart values with <code>config.namespacePrefix</code>.</p>
</li>

<li><p>Why: keeps namespace prefixing consistent with existing GitOps patterns.</p>
</li>
</ul>
<p><strong>prepare-github-app-git-creds.yaml</strong></p>
<p>File path: <code>gitops/apps/argo-workflows-github-app/templates/prepare-github-app-git-creds.yaml</code></p>

<ul>
<li><p>Added extracted <code>ClusterWorkflowTemplate</code> <code>prepare-github-app-git-creds</code>.</p>
</li>

<li><p>Resource sync wave is <code>2</code> within this child app.</p>
</li>

<li><p>Why: keeps the template logic unchanged while placing it in the correct app-level sync order.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1006">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1006</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>85d25b652e067a46e10f7e08cb1755fe4e91d95d</p>
</li>

<li><p>78bbe76a7b4b9b8a897a3d5df75f37f4421274ce</p>
</li>

<li><p>b7837325dc89c15d0957f3dd88d01626ad561c68</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1699</p>
</td>

<td valign="top" width="24%"><p>[Security] OSS Lodash module update (4.17.21-&gt;4.18.1)</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Updated <code>lodash</code> from <code>4.17.21</code> to <code>4.18.1</code> in:</p>

<ul>
<li><p><code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-horizon-api/package.json</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-horizon-api/package-lock.json</code></p>
</li>
</ul>
</li>

<li><p>This addresses TAA-1699 security remediation for <a href="https://github.com/advisories/GHSA-r5fr-rjxr-66jc"><u>CVE-2026-4800</u></a>.</p>
</li>

<li><p>Confirmed no remaining <code>lodash 4.17.x</code> references in repository lock/manifests. </p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1075">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1075</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p><code>98a877f0ba65497d7055ccfb98a05947fa8fdebb</code></p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1710</p>
</td>

<td valign="top" width="24%"><p>R4.0.0: Terraform duplicate certificate manager config, blocks deployment</p>
</td>

<td valign="top" width="51%"><p>Terraform duplicate certificate manager config</p>
<p>The fix replaces all hardcoded single-resource definitions in resource "google_certificate_manager_certificate" "horizon_sdv_cert" for_each over var.domains, so each domain gets its own uniquely-named DNS auth, certificate, and map entries (apex + wildcard) — eliminating duplicate resource name conflicts.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1032">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1032</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>ce0a3089f3d6fd44af9a219af5162d075d0edc8e</p>
</li>

<li><p>358fc69e2c165a77de879456724c7e9ace315473</p>
</li>

<li><p>e643957ebf1c5e77ebbd3514d6bcd5b72aabcd86</p>
</li>

<li><p>cc34f87f479a1f04bba3f3f70e11d0055b8b73c9</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1739</p>
</td>

<td valign="top" width="24%"><p>Add optional “analyse on success” toggle for Gemini AI Review in CVD Launcher + CTS Execution</p>
</td>

<td valign="top" width="51%"><p>TAA-1739: Allow Gemini AI Review on successful CVD/CTS runs (opt-in)</p>
<p>Add a GEMINI_ANALYSE_ON_SUCCESS job parameter for CVD Launcher and CTS Execution to optionally run Diagnostics -&gt; AI Review on SUCCESS as well as FAILURE, without turning green builds red.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1038">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1038</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>be7d36f1b48a4c4a82f3c9a16b40cae6893e579b</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1700</p>
</td>

<td valign="top" width="24%"><p>[Security] OSS Axios module update (1.13.2-&gt;1.15.0)</p>
</td>

<td valign="top" width="51%"><p>This PR updates the <strong>Axios</strong> dependency to address security vulnerabilities identified during OSS legal scanning and to ensure compliance for Horizon Release 4.0.0.</p>

<ul>
<li><p>Upgraded <strong>Axios SDK</strong> to version <strong>1.15.0</strong> to mitigate known security vulnerabilities.</p>
</li>

<li><p>Updated dependency configuration to reflect the new version.</p>
</li>

<li><p>Executed required tests for <strong>workloads/android/pipelines/tests/cvd_launcher</strong> pipeline and verified successful execution.</p>
</li>

<li><p>Verified that existing functionality continues to work as expected after the update.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1015">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1015</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>468ece95b9ead471fd383c659b25f7f256949979</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1734</p>
</td>

<td valign="top" width="24%"><p>Sample workload seed fails Job DSL with sandbox error on CLOUD_REGION</p>
</td>

<td valign="top" width="51%"><p>TAA-1734 sample workload seed fails job dsl</p>
<p>Sample workload seed fails Job DSL with sandbox error on CLOUD_REGION. Fixed templates HORIZON_SCM_* used by groovy files.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1037">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1037</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>ff335e17275db47995ca05478b400bdce63bab42</p>
</li>

<li><p>5c912d51698d2a21bb2b846715a59003c2fd9fd4</p>
</li>

<li><p>fe01d1b0fac717e88f35b39282d54006adb66288</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1733</p>
</td>

<td valign="top" width="24%"><p>kcc-webhook-cert-monitor : missing platform arch</p>
</td>

<td valign="top" width="51%"><p>TAA-1733: Fix platform arch - kcc-webhook-cert-monitor</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1044">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1044</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>da5873ab9e3a835b45986067bfa000c7f1e3fc6a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1701</p>
</td>

<td valign="top" width="24%"><p>sbx deployment is failed on WSL</p>
</td>

<td valign="top" width="51%"><p>TAA-1701 deployment failing wsl</p>
<p>Deployment fail on WSL - fix. Set open: false for the visualizer (still writes dist/bundle-analysis.html; you open it manually when you want).</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1003">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1003</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>19c0b7c16cb64111a434cec514119c9d169c437b</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1723</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] improvements from AI Review</p>
</td>

<td valign="top" width="51%"><p>TAA-1723: CVD AI Review — dual-lane preflight + Utilities Gemini AI Assistant guidance<br/>Extend the CVD Launcher sequenced skills so the same prompts/skills.yaml cover both failed and successful CVD runs:</p>

<ul>
<li><p>Add Phase 0 boot preflight to global_constraints: classify CVD_STATUS as BOOT_OK / BOOT_FAILED / BOOT_UNKNOWN from artifact-only signals ("status":"Running" in cvd-*.log, VIRTUAL_DEVICE_BOOT_COMPLETED per guest, and strong negatives in kernel.log). No pipeline/env state is consulted, so the CVD Launcher pipeline and Utilities / Gemini AI Assistant produce the same classification for the same test-results/.</p>
</li>

<li><p>Route triage-cvd / rca-cvd / fix-cvd by CVD_STATUS: BOOT_OK drives runtime-health analysis (logcat primary, bootconfig informational) and emits a single [CVD_HEALTHY] / [NO_RUNTIME_ISSUE] row when clean; fix-cvd writes a single observations note (or FIX_UNKNOWN) on healthy runs instead of fabricating AOSP diffs. BOOT_FAILED / BOOT_UNKNOWN keep the existing guest-first boot triage.</p>
</li>

<li><p>Update step1/step2/step3 prompt one-liners to reference the new lane routing.</p>
</li>
</ul>
<p>TAA-1723: cf_host_initialise: noninteractive apt; Mesa EGL/Vulkan to avoid CVD host probe errors</p>
<p>Prefix apt/apt-get with <code>sudo env DEBIAN_FRONTEND=noninteractive</code> so Packer/CI installs never block on debconf. Install libegl1-mesa, libvulkan1, and mesa-vulkan-drivers (Debian and Ubuntu) so assemble_cvd EGL/Vulkan checks do not fail on minimal images; warn and continue if that install fails.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1043">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1043</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>6b388b6121ff1757cd6a335a491227438d35178f</p>
</li>

<li><p>6ab4774f6df349f4bab5bce48476933c4a95bb50</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1737</p>
</td>

<td valign="top" width="24%"><p>[Module Manager] Module enable races cause transient Argo Application and soft-features ConfigMap errors</p>
</td>

<td valign="top" width="51%"><p>This branch fixes two bugs observed when enabling modules via the Horizon Developer Portal.</p>
<p><strong>Bug 1 (RBAC — hard error):</strong> When enabling the <code>sample</code> module, module-manager tried to verify that the <code>sample-module-hello</code> namespace was ready before writing the soft-features ConfigMap into it. This <code>get namespace</code> call was denied with <code>403 Forbidden</code> because the <code>module-manager-soft-features</code> ClusterRole only granted access to <code>configmaps</code>, not <code>namespaces</code>. The error surfaced as a red banner in the Developer Portal on every enable of <code>sample</code>.</p>
<p><strong>Bug 2 (race conditions — transient errors):</strong> Concurrent enable/disable requests and parallel reconciler runs (API handler + <code>ModuleCatalogReconciler</code>) could race on shared state: Argo CD Applications were created before a prior delete had completed ("object is being deleted: <a href="http://applications.argoproj.io">applications.argoproj.io</a> already exists"), and soft-features ConfigMap writes could collide ("unable to create new content in namespace ... because it is being terminated"). These were transient but consistently reproducible during rapid module toggling.</p>
<p>Both fixes are scoped entirely to <code>module-manager</code>. <code>horizon-api</code>, <code>horizon-dev-portal</code>, and KCC are not involved.</p>
<p><strong>Changes</strong></p>
<p><strong>Chart.yaml</strong></p>
<p>File path: <code>gitops/apps/module-manager/Chart.yaml</code></p>

<ul>
<li><p>Bumped <code>version</code> and <code>appVersion</code> from <code>0.2.0</code> to <code>0.2.2</code>.</p>
</li>

<li><p>Why: tracks the two fix commits as distinct releases so Argo CD records the changes as versioned diffs.</p>
</li>
</ul>
<p><strong>rbac.yaml</strong></p>
<p>File path: <code>gitops/apps/module-manager/templates/rbac.yaml</code></p>

<ul>
<li><p>Added a second rule to the <code>module-manager-soft-features</code> ClusterRole granting <code>get</code> on <code>namespaces</code> (core API group).</p>
</li>

<li><p>Why: <code>ensureNamespaceReady</code> in <code>soft_features_configmap.go</code> calls <code>apiReader.Get</code> on a <code>corev1.Namespace</code> before writing each soft-features ConfigMap. Without this verb the call returned <code>403 Forbidden</code>, which propagated out as a hard error on every soft-features sync. <code>namespaces</code> is cluster-scoped so a ClusterRole is required; only <code>get</code> is added (minimum privilege).</p>
</li>
</ul>
<p><strong>transaction.go </strong><em><strong>(new file)</strong></em></p>
<p>File path: <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/transaction.go</code></p>

<ul>
<li><p>Declares the package-level <code>var ModuleOpsMutex sync.Mutex</code>.</p>
</li>

<li><p>Why: provides a single, shared mutex that all module lifecycle entry points (REST enable, REST disable, <code>ModuleCatalogReconciler</code>) hold for the duration of their critical section, preventing concurrent mutations of <code>ModuleManagerState</code> and Argo CD Applications.</p>
</li>
</ul>
<p><strong>handler.go</strong></p>
<p>File path: <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/api/handler.go</code></p>

<ul>
<li><p>Added <code>controller.ModuleOpsMutex.Lock() / defer Unlock()</code> at the top of both <code>enableModule</code> and <code>disableModule</code>.</p>
</li>

<li><p>Replaced <code>h.client.Create(ctx, app)</code> with <code>createApplicationIdempotent(ctx, h.client, app)</code> in <code>enableOneModule</code>.</p>
</li>

<li><p>Added a call to <code>controller.RunAutoDisableSweep</code> after <code>patchDependentsSoftFeature</code> in <code>enableModule</code> (error logged, not returned).</p>
</li>

<li><p>Why: the mutex serialises concurrent API requests; idempotent create tolerates the "object is being deleted" race by retrying with exponential backoff until the old Application is gone; the post-enable sweep ensures <code>autoDisableWhenUnused</code> modules with no dependents are cleaned up immediately even when reached by a direct enable rather than only on disable or catalog change.</p>
</li>
</ul>
<p><strong>catalog_reconciler.go</strong></p>
<p>File path: <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/catalog_reconciler.go</code></p>

<ul>
<li><p>Added <code>ModuleOpsMutex.Lock() / defer Unlock()</code> at the start of <code>Reconcile</code>.</p>
</li>

<li><p>Why: the <code>ModuleCatalogReconciler</code> fires on every catalog spec change and runs the same auto-disable + soft-features sync paths as the REST handlers; holding the same mutex prevents it from racing with an in-flight enable or disable request.</p>
</li>
</ul>
<p><strong>argo_app.go</strong></p>
<p>File path: <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/api/argo_app.go</code></p>

<ul>
<li><p>Added <code>createApplicationIdempotent</code> and <code>createApplicationIdempotentWithBackoff</code>: if the Application already exists and is managed by module-manager, treat it as success; if it is terminating, retry with exponential backoff (up to ~32 s) until deleted, then re-create.</p>
</li>

<li><p>Extracted the label value <code>"horizon-sdv.io/module-manager-managed"</code> into the package-level constant <code>moduleManagerManagedLabelKey</code> used by both the builder and the new idempotency check.</p>
</li>

<li><p>Why: eliminates the "object is being deleted: <a href="http://applications.argoproj.io">applications.argoproj.io</a> already exists" error that appeared when a new enable arrived while the previous Application was still finalising.</p>
</li>
</ul>
<p><strong>soft_features_configmap.go</strong></p>
<p>File path: <code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/soft_features_configmap.go</code></p>

<ul>
<li><p>Wrapped the ConfigMap create/update inside <code>wait.ExponentialBackoffWithContext</code> (up to ~32 s, 7 steps).</p>
</li>

<li><p>Added <code>ensureNamespaceReady</code> (get namespace, check deletion timestamp before every attempt).</p>
</li>

<li><p>Added <code>isSoftFeaturesRetryableError</code>, <code>isNamespaceNotFoundError</code>, <code>isNamespaceTerminatingStatusError</code> to classify transient errors (namespace not yet created, namespace terminating, Kubernetes conflict) as retryable and hard RBAC/API errors as fatal.</p>
</li>

<li><p>Why: eliminates the "unable to create new content in namespace ... because it is being terminated" error that appeared when a soft-features write raced with a namespace teardown from a prior disable cycle. The namespace-ready guard also provides the correct hook point for the RBAC permission added in <code>rbac.yaml</code>.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1047">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1047</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>b1675790157ee88d692904608c2845a663c90f30</p>
</li>

<li><p>c3a44f693aa8df1a50cbde217460db76e3a30e14</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1732</p>
</td>

<td valign="top" width="24%"><p>Grafana does not show data on SBX instance</p>
</td>

<td valign="top" width="51%"><p>Metrics not showing in Grafana. All Dashboards were showing blank</p>

<ul>
<li><p>A new NetworkPolicy <em>allow-prometheus-egress-to-gke-metadata</em> was added in namespaces.yaml so pods labeled <a href="http://app.kubernetes.io/name:">app.kubernetes.io/name:</a> prometheus can reach 169.254.169.254/32 and 169.254.169.252/32 (GKE metadata / DNAT, per your existing comments elsewhere).<br/>A one-off kubectl apply of the same policy was run on your cluster: after that, wget to .../api/v1/query?query=up returned 200 with data.</p>
</li>
</ul>
<p>TXT and A records not created after deployment</p>

<ul>
<li><p>Adjust txtPrefix so the generated label is valid (no label ending with -). Typical patterns are a fixed prefix like extdns- / external-dns- / _externaldns style names, or a prefix where the record-type is not the last character before a dot in a way that leaves a trailing hyphen on a label.</p>
</li>

<li><p>Updated gitops/templates/external-dns.yaml to stop generating invalid ownership-TXT names like a-.demo5.horizon-sdv.com (that trailing - is what caused the IDNA errors and then external-dns pruned your A records).</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1049">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1049</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>29eab40e376ea139a2b92be48994b729a0fd251f</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1736</p>
</td>

<td valign="top" width="24%"><p>Regression: Apache 2.0 file headers were truncated when refactoring Helm/workflow</p>
</td>

<td valign="top" width="51%"><p>Fixes an Apache 2.0 license header regression introduced during Helm/workflow refactoring in commit <code>7b6f7b0</code>.</p>
<p>The refactor caused multiple files to lose parts of the standard multi-line license header (including the license URL and disclaimer block), resulting in inconsistent and incomplete license notices.</p>

<ul>
<li><p>Restores full Apache 2.0 headers where they were truncated</p>
</li>

<li><p>Adds missing header blocks to affected files</p>
</li>

<li><p>Ensures consistency with repository standards across all touched files</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1048">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1048</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4f9fb545de7b20987ffb6bf22b871c2f816a665d</p>
</li>

<li><p>f0a9847d59d0ac9f64119b965b77b445d68cd2d9</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1729</p>
</td>

<td valign="top" width="24%"><p>horizon improvements- unnecessary GCP resources</p>
</td>

<td valign="top" width="51%"><p>Fixes to makes feature " static IP support " working</p>
<p>There is option in terraform.tfvars configuration file:<br/>sdv_dns_use_static_a_records - by default is false, but in case of true DNS is able to use static IP from load balancer and create DNS "A type" record to support static IP</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1042">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1042</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4dc6de48ff244f95cf345619b91cfb3325aae8fd</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1743</p>
</td>

<td valign="top" width="24%"><p>[Security] OSS psf-requests module update (2.32.5-&gt;2.33.1)</p>
</td>

<td valign="top" width="51%"><p>OSS psf-requests module update (2.32.5-&gt;2.33.1)</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1051">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1051</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>ef8bd95c8193a12ce08cebc376d6c55890562da5</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1747</p>
</td>

<td valign="top" width="24%"><p>[Gemini CLI] Folder Trust feature introduced - breaking Horizon Agentic-AI </p>
</td>

<td valign="top" width="51%"><p>TAA-1747: gemini: default GEMINI_CLI_TRUST_WORKSPACE (headless folder trust)     GEMINI_CLI_TRUST_WORKSPACE=true avoids recent CLI security (folder trust) defaults in headless mode so workspace.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1058">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1058</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a3399b756d487443d6cfe9c5ad5ffe49c324d3e8</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1753</p>
</td>

<td valign="top" width="24%"><p>[Upgrade] Orphan keycloak groups after new groups added</p>
</td>

<td valign="top" width="51%"><p>Upgrade guide update on how to avoid orphan Keycloak Groups</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1061">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1061</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>87e567876093abe0d2b1cc4b3c7f00314a428071</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1712</p>
</td>

<td valign="top" width="24%"><p>[Upgrade] Orphan jenkins-git-creds Secret after rename to jenkins-scm-creds</p>
</td>

<td valign="top" width="51%"><p>Updated docs/guides/upgrade_guide_3_1_0_to_4_0_0.md with description how to handle orphan jenkins-git-creds secret</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1060">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1060</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>56b5040a05556db72dc09af88aa90a743ad6eac4</p>
</li>

<li><p>d6767d5a7102b0ff272f4e01836fc32ee7cf2460</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1738</p>
</td>

<td valign="top" width="24%"><p>Platform destroy blocked due to workflows namespace stuck in Terminating</p>
</td>

<td valign="top" width="51%"><p>Platform destroy could stall when the Argo Workflows namespace stayed in <code>Terminating</code>, often because <strong>Workflow</strong> CRs or their finalizers did not clear before namespaces and GitOps tore down. In parallel, <strong>Config Connector (KCC)</strong> objects could keep <code>cnrm.cloud.google.com/finalizer</code> on resources in module namespaces while workload identity or GCP auth was already going away, which also blocked namespace completion.</p>
<p>This fix addresses that by:</p>

<ol start="1">
<li><p><strong>Workflow drain controller</strong> (<code>workflow-namespace-drain-app</code>): A small operator that holds a dedicated finalizer on the <strong>root</strong> Argo CD <code>Application</code>, deletes remaining <code>Workflow</code> objects in the workflows namespace, and after a grace period <strong>clears stuck Workflow finalizers</strong>, then removes only its own finalizer. The chart is installed with <strong>Terraform </strong><code>helm_release</code>, <strong>outside</strong> the Argo app-of-apps tree, so it is <strong>not</strong> pruned during cascade destroy (see chart <code>README.md</code>).</p>
</li>

<li><p><strong>Module Manager platform drain</strong>: After disabling modules in dependency order, it <strong>strips KCC finalizers</strong> in managed destination namespaces (discovery-driven sweep of <code>*.cnrm.cloud.google.com</code> APIs) so namespaces can finish deleting when KCC can no longer reconcile. While child Applications still exist, the root reconciler <strong>re-runs</strong> the KCC stripper on a timer so objects unblock as auth disappears.</p>
</li>

<li><p><strong>Destroy order / WI</strong>: Comments in <code>terraform/modules/base/main.tf</code> document that <code>sdv_wi</code><strong> is destroyed after </strong><code>sdv_gke_apps</code><strong> and </strong><code>sdv_gke_cluster</code>, so workload identity remains usable during Argo cascade and cluster teardown.</p>
</li>

<li><p><strong>GitOps</strong>: Child Applications that ship Argo Workflows (and related samples) get <code>horizon-sdv.io/module-manager-managed: "true"</code> so Module Manager can treat them as managed during drain.</p>
</li>
</ol>
<p>Hosting the drain controller under the root GitOps <code>Application</code> would let Argo delete the drain controller first and undo teardown (documented in <code>gitops/apps/workflow-namespace-drain/README.md</code>). Relying only on KCC to finish deletes fails when Terraform removes IAM/WI while objects are still terminating (<code>kcc_drain.go</code> comments). This correction combines a <strong>Terraform-managed drain chart</strong>, <strong>root Application finalizers</strong>, <strong>Module Manager KCC finalizer handling</strong>, and <strong>documented WI destroy ordering</strong>.</p>
<p><strong>Changes</strong></p>

<h3><code>terraform/modules/sdv-gke-apps/main.tf</code></h3>
<p><strong>File path:</strong> <code>terraform/modules/sdv-gke-apps/main.tf</code></p>

<ul>
<li><p>Add <code>helm_release.workflow_namespace_drain</code> (chart <code>gitops/apps/workflow-namespace-drain</code>) with image, Argo CD namespace, workflows namespace, and root app name.</p>
</li>

<li><p>Add finalizer <code>horizon-sdv.io/workflow-namespace-drain</code> on the root Argo CD <code>Application</code> alongside existing finalizers.<br/><strong>Why:</strong> Run workflow drain outside the GitOps cascade and tie it into root app deletion order.</p>
</li>
</ul>

<h3><code>gitops/apps/workflow-namespace-drain/**</code></h3>
<p><strong>File path:</strong> <code>gitops/apps/workflow-namespace-drain/</code> (<code>Chart.yaml</code>, <code>values.yaml</code>, <code>README.md</code>, <code>templates/</code>)</p>

<ul>
<li><p><code>templates/rbac.yaml</code><strong>:</strong> Single Helm template with multiple YAML documents (<code>---</code>): <code>ServiceAccount</code>, <code>ClusterRole</code> / <code>ClusterRoleBinding</code> for Argo Workflows CRs, namespaced <code>Role</code> / <code>RoleBinding</code> in the Argo CD namespace for root <code>Application</code> finalizers. Top-of-file comments include the Apache license and the note that the <strong>release namespace is created by Terraform</strong> (<code>create_namespace=true</code>), not as a chart-rendered <code>Namespace</code> object (avoids Helm ownership conflicts).</p>
</li>

<li><p><code>templates/deployment.yaml</code><strong>:</strong> Controller <code>Deployment</code> only (unchanged split from RBAC).</p>
</li>

<li><p><code>README.md</code><strong>:</strong> Do not add this chart to <code>gitops/templates/</code> or the app-of-apps.<br/><strong>Why:</strong> Same RBAC and workload behavior as multiple small template files; one file is easier to review for RBAC; <code>Deployment</code> stays isolated for clarity.</p>
</li>
</ul>
<p><code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/**</code></p>
<p><strong>File path:</strong> <code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/</code></p>

<ul>
<li><p>New Go service: root <code>Application</code> finalizer reconciliation, workflow delete and timed finalizer clear, <code>main.go</code>, Dockerfile, modules, tests.<br/><strong>Why:</strong> Implement drain logic the Helm chart runs.</p>
</li>
</ul>
<p><code>terraform/modules/base/locals.tf</code></p>
<p><strong>File path:</strong> <code>terraform/modules/base/locals.tf</code></p>

<ul>
<li><p>Register <code>workflow-namespace-drain-app</code> in the container images map.<br/><strong>Why:</strong> Build and version the new image like other platform images.</p>
</li>
</ul>
<p><code>terraform/modules/base/main.tf</code></p>
<p><strong>File path:</strong> <code>terraform/modules/base/main.tf</code></p>

<ul>
<li><p>Comments on <code>module.sdv_wi</code> relative destroy order vs GKE apps and cluster.<br/><strong>Why:</strong> Explain why WI is torn down after cluster-side teardown still needs credentials.</p>
</li>
</ul>
<p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/**</code></p>
<p><strong>File paths:</strong></p>

<ul>
<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/kcc_drain.go</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/kcc_drain_test.go</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/platform_drain.go</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/platform_drain_test.go</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/internal/controller/platform_root_finalize_reconciler.go</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/main.go</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.mod</code></p>
</li>

<li><p>KCC finalizer removal in managed module destination namespaces; discovery client wired in <code>main.go</code>; root reconciler requeues and re-strips while child Applications remain; tests.<br/><strong>Why:</strong> Unblock namespace termination when KCC cannot complete deletes during destroy; keep sequencing in Module Manager.</p>
</li>
</ul>
<p><code>gitops/apps/module-manager/templates/rbac.yaml</code></p>
<p><strong>File path:</strong> <code>gitops/apps/module-manager/templates/rbac.yaml</code></p>

<ul>
<li><p>ClusterRole (and related bindings) for listing/patching KCC API groups during drain.<br/><strong>Why:</strong> RBAC required for the KCC finalizer sweep.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1067">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1067</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>841ef827e11600f6d3d5d909690d92125f55c5d5</p>
</li>

<li><p>76853cf77a4626601f5b2ad5075612301a610f6e</p>
</li>

<li><p>8fe113da88f7e9d9d693473eced597c729e35b05</p>
</li>

<li><p>24ebc8af5b06a48bb012c6852e7d13a546966eb5</p>
</li>

<li><p>e1840b7b3e29461ae788258fde5aaff9f47534a7</p>
</li>

<li><p>4b53f7fe710d29a3986f6443cc4bef7384e47e1a</p>
</li>

<li><p>a66a3f559892a806f55f6bc5125c1db07ec78413</p>
</li>

<li><p>62340e7fbf517a5ac5f0083404010718d5f88de2</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1772</p>
</td>

<td valign="top" width="24%"><p>Workload Identity IAM binding fails before GKE pool exists</p>
</td>

<td valign="top" width="51%"><p>Terraform apply failed with <code>Identity Pool does not exist (&lt;project&gt;.svc.id.goog)</code> when creating Workload Identity bindings on Google service accounts, because those bindings ran before the GKE cluster existed. The cluster is what enables the project Workload Identity pool. This change flips module ordering: create the GKE cluster first, then run <code>sdv_wi</code> so <code>google_service_account_iam_member</code> can bind principals under <code>PROJECT_ID.svc.id.goog</code> after the pool exists.</p>

<h3><code>terraform/modules/base/main.tf</code></h3>
<p>File path: <code>terraform/modules/base/main.tf</code></p>

<ul>
<li><p>Add <code>depends_on = [module.sdv_gke_cluster]</code> to <code>module "sdv_wi"</code>.</p>
</li>

<li><p>Remove <code>module.sdv_wi</code> from <code>module.sdv_gke_cluster</code>’s <code>depends_on</code> (keep other dependencies unchanged).</p>
</li>

<li><p>Replace the old comment block on <code>sdv_wi</code> (about GKE modules depending on WI) with the new <code>depends_on</code> wiring.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1072">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1072</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>1b3786833bb6c54ac82aeb4cec699a619bec08ad</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1774</p>
</td>

<td valign="top" width="24%"><p>[Security] OSS Axios module update (1.15.0-&gt;1.16.0)</p>
</td>

<td valign="top" width="51%"><p>TAA-1774: Remove AXIOS/WAITON from deprecated portal and OpenAPI spec<br/>Horizon-Portal will be removed at a later date and the OpenAPI spec is well out of date, so changes are largely pointless but help reduce search noise. </p>
<p>TAA-1774: Remove global wait-on/axios installs<br/>Pin MTK Connect wait-on and axios in package.json (npm overrides) and run wait-on from the local install instead of global npm plus manual overlays. Centralises dependency management, makes security updates a single file change,  and removes divergent risks having to regenerate Docker images and Cuttlefish instances.<br/><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1077">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1077</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>f59fb2e7ac7f4c74986fd5ae2060613adf3e9ba1</p>
</li>

<li><p>0a087da2a099df97e5e72a13c94f63aec8cc6b29</p>
</li>
</ul>
<p></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1727</p>
</td>

<td valign="top" width="24%"><p>[Horizon Dev Portal] HTTPRoute fails when Gateway sync wave runs before Application</p>
</td>

<td valign="top" width="51%"><p>This fix includes fix for deployment failing due to the Horizon-dev portal app's gateway http-route being provisioned before the app which caused sync failures.</p>
<p><strong>gateway-horizon-dev-portal.yaml</strong></p>
<p>File path: <code>gitops/templates/gateway-horizon-dev-portal.yaml</code></p>

<ul>
<li><p>Update Gateway http-route sync wave to <code>7</code> (After horizon-dev-portal) to resolve sync issues.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>2ed778f4cbb36d22bce28ce4ca331c365bb5d9e5</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1672</p>
</td>

<td valign="top" width="24%"><p>Mitigate axios supply-chain exposure from unpinned global wait-on installs</p>
</td>

<td valign="top" width="51%"><p>Mitigate axios transitive-dependency vulnerability in <code>wait-on</code> global installs across Android, OpenBSW, and Cuttlefish pipelines. While TAA-1700 updated the <strong>direct</strong> axios dependency in <code>package.json</code> to 1.15.0, the globally installed <code>wait-on</code> CLI still resolves its own transitive <code>axios</code> via an unpinned caret range — leaving it exposed to supply-chain attacks (e.g. the axios 1.14.1 compromise) and <a href="https://github.com/advisories/GHSA-fvcv-3m26-pcqx"><u>CVE-2026-40175</u></a> (header injection → RCE / AWS IMDSv2 bypass, CVSS 10.0).</p>
<p>This PR adds <strong>defense-in-depth</strong> by:</p>

<ol start="1">
<li><p><strong>Pinning </strong><code>wait-on</code> to an explicit version (<code>9.0.4</code>) with <code>--ignore-scripts</code> to block malicious <code>postinstall</code> hooks.</p>
</li>

<li><p><strong>Overlaying a known-safe </strong><code>axios@1.15.0</code> into <code>wait-on</code>'s <code>node_modules</code>, ensuring the transitive dependency is also patched.</p>
</li>

<li><p><strong>Parameterizing both versions</strong> (<code>WAITON_VERSION</code>, <code>AXIOS_VERSION</code>) through the full pipeline chain: Seed Job → Groovy DSL → Jenkinsfile → Dockerfile ARG / shell defaults → Helm values → Argo WorkflowTemplate → Sensor, so versions can be bumped without code changes.</p>
</li>
</ol>
<p><strong>Changes (30 files)</strong></p>
<p><strong>Seed Job &amp; Jenkins Parameterization</strong></p>

<ul>
<li><p><code>workloads/seed/Jenkinsfile</code> — Add <code>WAITON_VERSION</code> (default <code>9.0.4</code>) and <code>AXIOS_VERSION</code> (default <code>1.15.0</code>) string parameters; propagate to Android Docker Image, CF Instance Template, OpenBSW Docker Image, and Utilities Docker Image downstream jobs.</p>
</li>
</ul>
<p><strong>Android Docker Image Template</strong></p>

<ul>
<li><p><code>Dockerfile</code> — Add <code>ARG WAITON_VERSION</code> / <code>ARG AXIOS_VERSION</code>; pin <code>wait-on</code> install with <code>--ignore-scripts</code>; overlay <code>axios</code> into wait-on's <code>node_modules</code>.</p>
</li>

<li><p><code>Jenkinsfile</code> — Pass <code>WAITON_VERSION</code> and <code>AXIOS_VERSION</code> as Docker <code>build-arg</code>.</p>
</li>

<li><p><code>groovy/job.groovy</code> — Add <code>WAITON_VERSION</code> / <code>AXIOS_VERSION</code> string parameters to the Jenkins job DSL.</p>
</li>

<li><p><code>helm/values.yaml</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> values.</p>
</li>

<li><p><code>helm/templates/workflow/workflowtemplates.yaml</code> — Declare new workflow parameters.</p>
</li>

<li><p><code>helm/templates/workflow/_build.tpl</code> — Pass parameters into the build container's Docker build-args.</p>
</li>

<li><p><code>helm/templates/workflow/sensors.yaml</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> parameter declarations and webhook body → workflow parameter index mappings (indexes 7–8; existing parameters shifted accordingly).</p>
</li>
</ul>
<p><strong>Cuttlefish Instance Template (Packer / VM)</strong></p>

<ul>
<li><p><code>Jenkinsfile</code> — Pass <code>WAITON_VERSION</code> and <code>AXIOS_VERSION</code> to Packer and init scripts.</p>
</li>

<li><p><code>groovy/job.groovy</code> — Add <code>WAITON_VERSION</code> / <code>AXIOS_VERSION</code> string parameters.</p>
</li>

<li><p><code>cf_create_instance_template.sh</code> — Accept and forward both versions to Packer variables.</p>
</li>

<li><p><code>cf_environment.sh</code> — Export <code>WAITON_VERSION</code> / <code>AXIOS_VERSION</code> with defaults.</p>
</li>

<li><p><code>packer/cuttlefish.pkr.hcl</code> — Declare Packer variables; pass through to startup script.</p>
</li>

<li><p><code>cf_host_initialise.sh</code> — Pin <code>wait-on</code> install with <code>--ignore-scripts</code>; overlay safe <code>axios</code>.</p>
</li>
</ul>
<p><strong>OpenBSW Docker Image Template</strong></p>

<ul>
<li><p><code>Dockerfile</code> — Add <code>ARG WAITON_VERSION</code> / <code>ARG AXIOS_VERSION</code>; pin <code>wait-on</code> install with <code>--ignore-scripts</code>; overlay <code>axios</code> into wait-on's <code>node_modules</code>.</p>
</li>

<li><p><code>Jenkinsfile</code> — Pass <code>WAITON_VERSION</code> and <code>AXIOS_VERSION</code> as Docker <code>build-arg</code>.</p>
</li>

<li><p><code>groovy/job.groovy</code> — Add <code>WAITON_VERSION</code> / <code>AXIOS_VERSION</code> string parameters.</p>
</li>
</ul>
<p><strong>MTK Connect (Runtime)</strong></p>

<ul>
<li><p><code>mtk_connect.sh</code> — Add <code>WAITON_VERSION</code> / <code>AXIOS_VERSION</code> env defaults; pin <code>wait-on</code> install with <code>--ignore-scripts</code>; overlay safe <code>axios</code> into wait-on's global <code>node_modules</code>.</p>
</li>
</ul>

<h3>AAOS Builder Helm</h3>

<ul>
<li><p><code>workloads/android/pipelines/builds/aaos_builder/helm/values.yaml</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> values.</p>
</li>
</ul>
<p><strong>Horizon Portal UI</strong></p>

<ul>
<li><p><code>ABFSCreateContainerForm.tsx</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> fields to the ABFS container creation form.</p>
</li>

<li><p><code>SeedWorkloadsForm.tsx</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> fields to the Seed Workloads form.</p>
</li>

<li><p><code>AndroidDockerImageTemplateForm.tsx</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> fields to the Android Docker Image Template form.</p>
</li>

<li><p><code>OpenBSWDockerImageForm.tsx</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> fields to the OpenBSW Docker Image form.</p>
</li>
</ul>
<p><strong>API Documentation</strong></p>

<ul>
<li><p><code>docs/api/v1/openapi.yaml</code> — Add <code>waitonVersion</code> / <code>axiosVersion</code> properties to Android Docker Image, CF Instance Template, and OpenBSW Docker Image request schemas.</p>
</li>

<li><p><code>clients/horizon-portal/public/docs/api/v1/openapi.yaml</code> — Same (portal copy).</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>5a3e5f891461e2f8ce2eb9398aef67ce1f779658</p>
</li>

<li><p>1e4575d0f0c6da76aba704dbeec7214914e0ea98</p>
</li>

<li><p>d34c028de7306a6fb5f7093b84caf6c4494fb798</p>
</li>

<li><p>e47c96e86927e08b81d4a462023631ed526ad096</p>
</li>

<li><p>0f3878b20ce38a39dba46e0683b55701b6626a40</p>
</li>

<li><p>5a73dfe6ded6198715ea6f5506b8ffb1e31d0047</p>
</li>

<li><p>d04da45c4aee5f27c09b0540ec51a31a6d98f9c5</p>
</li>

<li><p>681c377b216231f263a7d11b276ad02bd53f87732</p>
</li>

<li><p>b6b51902c27acb75ddb0b446d1da4f2dc2aa46b2</p>
</li>

<li><p>8d9cc9c7fadf5ed428a3c26601cebf348a04baca</p>
</li>

<li><p>ce23ad3111c28174d06b1e891c9dcf17455170ce</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1685</p>
</td>

<td valign="top" width="24%"><p>[Cloud Workstations] Horizon ASfP image build fails on Cuttlefish Bazel fetch (f2fs-tools / git.kernel.org)</p>
</td>

<td valign="top" width="51%"><p>Fixes the Horizon Android Studio for Platform (ASfP) Cloud Workstation image build when the Cuttlefish stage fails because Bazel cannot fetch <code>f2fs_tools</code> from <code>git.kernel.org</code> (timeouts, egress restrictions, or deprecated hosting). After cloning <code>android-cuttlefish</code>, the image build now rewrites the Bazel <code>git_repository</code> remote to the GitHub mirror before running <code>build_packages.sh</code>.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/992">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/992</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>3655d2a460d4f4f48b71faa2652af0bbe4534dcc</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1779</p>
</td>

<td valign="top" width="24%"><p>No access to signed urls</p>
</td>

<td valign="top" width="51%"><p>Added roles/iam.serviceAccountTokenCreator to argo-workflows SA roles in terraform/env/main.tf.<br/>Added the same role to sub-env template in terraform/env/locals.tf.<br/>Also added sub-env Workload Identity mapping for horizon-api KSA (&lt;env&gt;-horizon-api/horizon-api) in terraform/env/locals.tf so prefixed environments can impersonate the same signer GSA correctly.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1076">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1076</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>ffd47bfdf6a88b93c70807ec9fa8b85f31e01d66</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1761</p>
</td>

<td valign="top" width="24%"><p>[Security] R4.0.0: upgrade Jenkins plugins to address security issues</p>
</td>

<td valign="top" width="51%"><p>Security issues and bug fixes requiring plugin updates.</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1080">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1080</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>1f455f331c810ce6ddd1b0b6926e542e1a5527c5</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top"><p>TAA-1778</p>
</td>

<td valign="top"><p>[Security]  grpc-go module update (1.58.3 -&gt; 1.81.0)</p>
</td>

<td valign="top"><p>Addresses TAA-1778 by upgrading grpc-go in the horizon-api/horizon-api-app/ from v1.80.0 to v1.81.0.0 to resolve the reported security finding.</p>
<p>Updated terraform/modules/sdv-container-images/images/horizon-api/horizon-api-app/go.mod and go.sum</p>
<p>few other components where updated to newest versions:</p>

<table width="100%">
<tbody>
<tr>
<th valign="top"><p>Component</p>
</th>

<th valign="top"><p>New version</p>
</th>

<th valign="top"><p>Type</p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://cloud.google.com/go/storage">cloud.google.com/go/storage</a></p>
</td>

<td valign="top" width="24%"><p>direct (promoted)</p>
</td>

<td valign="top" width="64%"><p>Promotion</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://google.golang.org/api">http://google.golang.org/api</a> </p>
</td>

<td valign="top" width="24%"><p>direct (promoted)</p>
</td>

<td valign="top" width="64%"><p>Promotion</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://github.com/cncf/xds/go">http://github.com/cncf/xds/go</a> </p>
</td>

<td valign="top" width="24%"><p>20260202</p>
</td>

<td valign="top" width="64%"><p>Patch</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://github.com/envoyproxy/go-control-plane/envoy">http://github.com/envoyproxy/go-control-plane/envoy</a> </p>
</td>

<td valign="top" width="24%"><p>v1.37.0</p>
</td>

<td valign="top" width="64%"><p>Minor</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://github.com/envoyproxy/protoc-gen-validate">http://github.com/envoyproxy/protoc-gen-validate</a> </p>
</td>

<td valign="top" width="24%"><p>v1.3.3</p>
</td>

<td valign="top" width="64%"><p>Patch</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://github.com/go-jose/go-jose/v4">http://github.com/go-jose/go-jose/v4</a> </p>
</td>

<td valign="top" width="24%"><p>v4.1.4</p>
</td>

<td valign="top" width="64%"><p>Patch</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p><a href="http://go.opentelemetry.io/contrib/detectors/gcp">http://go.opentelemetry.io/contrib/detectors/gcp</a> </p>
</td>

<td valign="top" width="24%"><p>v1.42.0</p>
</td>

<td valign="top" width="64%"><p>Minor</p>
</td>
</tr>
</tbody>
</table>
<p>horizon-api component<br/>Security issue caused by vulnerability in grpc-go 1.58.3 module<br/>fix for [Security] grpc-go module update (1.58.3 -&gt; 1.81.0)</p>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1081">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1081</a> </p>
</td>

<td valign="top">
<ul>
<li><p>f784fe49259880220db26a9bb772556906d2238b</p>
</li>

<li><p>0e51da4a73f72137211d0449e9c9d78e475be191</p>
</li>

<li><p>b0695c0328d5552408c70529af22105afd5e629d</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1776</p>
</td>

<td valign="top" width="24%"><p>[Security]  golang.org/x/crypto module update (v0.25.0-v.0.50.0)</p>
</td>

<td valign="top" width="51%"><p>Addresses TAA-1776 by upgrading <code>golang.org/x/crypto</code> in the Horizon Dev Portal proxy from <code>v0.25.0</code> to <code>v0.50.0</code> to resolve the reported security finding.</p>

<ul>
<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/proxy/go.mod</code></p>

<ul>
<li><p><code>golang.org/x/crypto</code> <code>v0.25.0</code> -&gt; <code>v0.50.0</code></p>
</li>

<li><p>Go version <code>1.22</code> -&gt; <code>1.25.0</code> (required by <code>x/crypto v0.50.0</code>)</p>
</li>
</ul>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/proxy/go.sum</code> for the new crypto version.</p>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal/Dockerfile</code></p>

<ul>
<li><p>Builder image <code>golang:1.22-alpine</code> -&gt; <code>golang:1.25-alpine</code> to match module/toolchain requirements.</p>
</li>
</ul>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1082">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1082</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4bb44a76fd597fa461a729dce23d46242ab844eb</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1777</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[Security] <a href="http://golang.org/x/crypto">golang.org/x/crypto</a> module update (v0.16.0-v.0.50.0)</p>
</td>

<td valign="top" width="51%"><p>Fixes TAA-1777 by remediating transitive <code>golang.org/x/crypto v0.16.0</code> findings in both <code>module-manager-app</code> and <code>workflow-namespace-drain-app</code>, pinning resolution to <code>v0.50.0</code>.</p>

<ul>
<li><p>Updated:</p>

<ul>
<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.mod</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/go.sum</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/module-manager/module-manager-app/Dockerfile</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.mod</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/go.sum</code></p>
</li>

<li><p><code>terraform/modules/sdv-container-images/images/workflow-namespace-drain/workflow-namespace-drain-app/Dockerfile</code></p>
</li>
</ul>
</li>

<li><p>Go toolchain bumped to <code>1.25.0</code> in both modules because <code>golang.org/x/crypto v0.50.0</code> requires Go <code>&gt;= 1.25</code>.</p>
</li>

<li><p>Builder images bumped from <code>golang:1.22-alpine</code> to <code>golang:1.25-alpine</code> so container builds do not fail with the updated Go requirement.</p>
</li>

<li><p>Added <code>replace golang.org/x/crypto =&gt; golang.org/x/crypto v0.50.0</code> in both <code>go.mod</code> files to enforce the transitive security pin.</p>

<ul>
<li><p>In these modules, <code>x/crypto</code> is transitive-only and not directly imported, so a plain <code>require ... // indirect</code> is removed by <code>go mod tidy</code> (lazy loading).</p>
</li>

<li><p><code>replace</code> is preserved through tidy and keeps the resolved graph at <code>v0.50.0</code> for consistent scan/build results.</p>
</li>
</ul>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1083">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1083</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>1262347c7dcc827bd0b8ffeadcf0ac0eb7b3b0d1</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1780</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[Security] <a href="http://golang.org/x/crypto">http://golang.org/x/crypto</a>  module update (v0.49.0-v.0.50.0)</p>
</td>

<td valign="top" width="51%"><p>Fixes TAA-1780 by upgrading <code>golang.org/x/crypto</code> in <code>horizon-api-app</code> from <code>v0.49.0</code> to <code>v0.50.0</code>.</p>
<p><strong>Changes</strong></p>

<ul>
<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-api/horizon-api-app/go.mod</code></p>

<ul>
<li><p><code>golang.org/x/crypto v0.49.0</code> -&gt; <code>v0.50.0</code> (indirect)</p>
</li>
</ul>
</li>

<li><p>Updated <code>terraform/modules/sdv-container-images/images/horizon-api/horizon-api-app/go.sum</code> via <code>go mod tidy</code>.</p>
</li>

<li><p>No changes to Dockerfile, source code, Helm, or Terraform configuration.</p>
</li>
</ul>
<p><a href="https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1084">https://github.com/AGBG-ASG/acn-horizon-sdv/pull/1084</a> </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>8f0d98e5cb6c5388b0fbce84f8d0c06f2cbdc1f6</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1762</p>
<p></p>
</td>

<td valign="top" width="24%"><p>[ESRlabs] [Security] OpenAPI Specification File Exposed </p>
</td>

<td valign="top" width="51%"><p>Fixes TAA-1762 by blocking public access to MCP Gateway Registry and auth-server OpenAPI/Swagger documentation endpoints on the <code>mcp.&lt;domain&gt;</code> Gateway route. This update ensures blocked documentation paths return a real <code>404</code>, which is clearer for security scanners and confirms the API spec is not exposed.</p>
<p><strong>Key Changes</strong></p>

<ul>
<li><p>Added Gateway <code>HTTPRoute</code> rules to block:</p>

<ul>
<li><p><code>/docs</code></p>
</li>

<li><p><code>/redoc</code></p>
</li>

<li><p><code>/openapi.json</code></p>
</li>

<li><p><code>/openapi.yaml</code></p>
</li>

<li><p><code>/auth-server/docs</code></p>
</li>

<li><p><code>/auth-server/redoc</code></p>
</li>

<li><p><code>/auth-server/openapi.json</code></p>
</li>

<li><p><code>/auth-server/openapi.yaml</code></p>
</li>
</ul>
</li>

<li><p>Used <code>PathPrefix</code> + <code>ReplacePrefixMatch</code> because GKE Gateway rejects <code>ReplaceFullPath</code> and requires exactly one <code>PathPrefix</code> match per <code>URLRewrite</code> rule.</p>
</li>

<li><p>Routed blocked registry documentation paths to the <code>auth-server</code> fallback after rewriting to <code>/__horizon_blocked__</code>, because the registry backend returns SPA <code>index.html</code> with <code>200</code> for unknown paths, while auth-server returns a proper <code>404</code>.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>7a9f5c883367270fe4f42dbc439836d82aaeb944</p>
</li>

<li><p>d844d7af68d860f358c9d562c75f9d0b8e77af96</p>
</li>

<li><p>73be7f518e8dda54f547187b3c16b2937877c9ca</p>
</li>

<li><p>cc566d0b5439cd84e2153405822b7c32ce5ff61b</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Known Issues</h2>
<p>TAA-1763 - Gemini Code Assist may require a license for use in workstation IDEs (especially ASfP)</p>
<p>Following recent changes to <strong>Gemini Code Assist</strong>, the pre-installed Code Assist plugin in the Cloud Workstation IDEs may now require an active <strong>Gemini Code Assist license</strong> before it will work. This has been observed consistently in <strong>Android Studio for Platform (ASfP)</strong>, and intermittently in Android Studio and Code OSS depending on which features (agent mode, IDE-side MCP) are used.</p>
<p><strong>Typical symptoms</strong></p>

<ul>
<li><p><em>"selected project doesn't have valid license"</em> in the project picker.</p>
</li>

<li><p><em>"missing a valid license for Gemini Code Assist"</em> banner.</p>
</li>

<li><p>Agent mode failing with <em>"There was a problem getting a response."</em></p>
</li>
</ul>
<p><strong>If you hit this, the deploying team needs to:</strong></p>

<ol start="1">
<li><p><strong>Billing admin - purchase a Gemini Code Assist subscription</strong> on the platform's GCP billing account. Choose <strong>Enterprise</strong> if users need agent mode or IDE-side MCP (recommended for ASfP); <strong>Standard</strong> is chat-only.</p>
</li>

<li><p><strong>Billing admin - enable automatic license assignment</strong> at <em>Cloud Console → Gemini Admin → Code Assist → Settings</em>. Seats then attach automatically the first time each user opens the IDE.</p>
</li>

<li><p><strong>Project admin - grant per-user IAM</strong> on the platform's GCP project to every Code Assist user:</p>

<ul>
<li><p><code>roles/cloudaicompanion.user</code></p>
</li>

<li><p><code>roles/serviceusage.serviceUsageConsumer</code></p>
</li>
</ul>
</li>

<li><p><strong>End user</strong> - sign into the IDE with the entitled email and select the platform's GCP project. This also enables <code>cloudaicompanion.googleapis.com</code> on the project automatically.</p>
</li>
</ol>
<blockquote><p><strong>Workaround:</strong> the embedded <strong>Gemini CLI</strong> (<code>gemini</code> in the workstation terminal) is unaffected, does not consume a Code Assist seat, and supports MCP via <code>gemini-mcp-agent</code>.</p>
</blockquote><p></p>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 3.1.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>13.03.2026</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p>Horizon SDV 3.1.0 is the minor release which extends platform capabilities with support for Sub-environments and additional MCP server configuration for Android Studio and Android Studio for Platforms IDEs. Horizon 3.1.0 also delivers several critical bug fixes including security fixes for network configurations and vulnerabilities in application containers.</p>
<p>Rel.3.1.0 defines rules for Partner Contributions Repository and recommended directory structure for third party modules provided from external Horizon Partners which are documented in <strong>contributing.md</strong> file located in the <strong>/doc</strong> directory of Horizon SDV repository. </p>
<p>Horizon SDV 3.1.0 package offers fully verified and documented upgrade patch (from Rel.3.0.0 to Rel.3.1.0). (see details in /docs/guides/upgrade_guide_3_0_0_to_3_1_0.md)</p>

<h2>New Features</h2>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1057</p>
</td>

<td valign="top" width="24%"><p>Support for Sub-Environments in Horizon SDV platform</p>
<p></p>
</td>

<td valign="top" width="64%"><p>Horizon SDV 3.1.0 introduces <strong>sub-environments</strong>: multiple isolated copies of the platform that run on the <strong>same GKE cluster</strong> as the main environment. Each sub-environment has its own namespaces (prefixed by sub-environment name, e.g. <code>sub-jenkins</code>, <code>sub-keycloak</code>), its own Argo CD instance, its own sub-domain (e.g. <code>sub.&lt;SUB_DOMAIN&gt;.&lt;HORIZON_DOMAIN&gt;</code>), and its own GCP Certificate Manager certificate, Secret Manager secrets, and Workload Identity service accounts. Sub-environments are defined entirely in <code>terraform.tfvars</code> via the <code>sdv_sub_env_configs</code> variable; no code changes are required to add or remove them. Typical use cases include giving teams isolated instances without extra clusters, testing platform changes on a branch before merge, and running a stable environment alongside a short-lived experimental one.</p>
<p><strong>Changes</strong></p>

<ul>
<li><p><strong>Terraform:</strong> New variable <code>sdv_sub_env_configs</code> in <code>terraform/env/terraform.tfvars</code> (optional; defaults to empty map). Each key is the sub-environment name; each value supplies required Keycloak passwords and optional <code>branch</code> and <code>manual_secrets</code>.</p>
</li>

<li><p><strong>Certificate Manager:</strong> DNS Authorization and certificate resources converted to <code>for_each</code> to support one certificate per sub-environment. Upgrade from 3.0.0 uses <code>moved {}</code> blocks and a name-preserving conditional to avoid destroying and recreating existing GCP resources.</p>
</li>

<li><p><strong>Argo CD:</strong> Argo CD-related Kubernetes resources managed by Terraform converted to <code>for_each</code>. One Argo CD instance per sub-environment (e.g. <code>helm_release.argocd_subenvs["sub"]</code> in <code>sub-argocd</code> namespace). Upgrade from 3.0.0 uses <code>moved {}</code> blocks to migrate state without destroying live resources.</p>
</li>

<li><p><strong>GCP:</strong> Per sub-environment: Workload Identity service accounts (e.g. <code>gke-&lt;SUB_ENV_NAME&gt;-&lt;app&gt;-sa</code>), Secret Manager secrets (prefixed <code>&lt;SUB_ENV_NAME&gt;-</code>), Certificate Manager certificate and DNS authorization for <code>&lt;SUB_ENV_NAME&gt;.&lt;SUB_DOMAIN&gt;.&lt;HORIZON_DOMAIN&gt;</code>, and Cloud DNS CNAME for certificate verification.</p>
</li>

<li><p><strong>GitOps:</strong> Helm values <code>namespacePrefix</code>, <code>isSubEnvironment</code>, and <code>environmentName</code> drive namespace and resource naming. Cluster-scoped components (External Secrets Operator, Node Exporter, Kubescape Operator, Gerrit Operator) are gated with <code>isSubEnvironment</code> and remain single-instance; sub-environments use namespace-scoped resources and the shared operators.</p>
</li>

<li><p><strong>Documentation:</strong> [Sub-Environment Deployment Guide](guides/sub_environments/sub_environment_deployment_guide.md) (configuration, deploy, access, destroy) and [Sub-Environment Developer Guide](guides/sub_environments/sub_environment_developer_guide.md) (architecture, adding apps, naming). Deployment guide referenced from main [Deployment Guide](deployment_guide.md).</p>
</li>
</ul>
<p><strong>Action Required</strong></p>

<ul>
<li><p><strong>None for existing 3.0.0 users who do not use sub-environments.</strong> Upgrade path is described in [Upgrade Guide: 3.0.0 to 3.1.0](guides/upgrade_guide_3_0_0_to_3_1_0.md); follow post-upgrade steps (e.g. delete/recreate affected resources, sync with prune) as documented.</p>
</li>

<li><p><strong>To use sub-environments:</strong> Add <code>sdv_sub_env_configs</code> to <code>terraform/env/terraform.tfvars</code> with at least <code>keycloak_admin_password</code> and <code>keycloak_horizon_admin_password</code> per sub-environment. Sub-environment names must be lowercase alphanumeric with hyphens, 1-4 characters. See [Sub-Environment Deployment Guide – Configuring Sub-Environments](guides/sub_environments/sub_environment_deployment_guide.md#configuring-sub-environments).</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Improved Features</h2>
<p></p>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="12%"><p>TAA-1328</p>
</td>

<td valign="top" width="24%"><p>MCP server configuration caching by Android Studio and ASfP IDE</p>
</td>

<td valign="top" width="64%"><p>This improvement provides the MCP configuration caching by Android Studio and ASfP IDE that makes MCP requests by Gemini Code Assist use expired tokens.</p>
<p><strong>MCP configuration caching in Android Studio and ASfP</strong></p>
<p>The Android Studio and Android Studio for Platform IDEs cache the MCP configuration (<code>mcp.json</code>) for their current session.</p>

<ul>
<li><p>This means, if we store auth tokens in <code>mcp.json</code> and later update them, the IDE will still use the old tokens from its cache.</p>
</li>

<li><p>To fix this, a standard workaround has been implemented in <code>gemini-mcp-agent</code> using the <code>--mcp-client-bridge</code> mode where each MCP server configured in <code>mcp.json</code> spawns its own MCP-client bridge.</p>
</li>

<li><p>It transparently forwards requests from the IDE to the MCP server (and vice-versa), injecting a fresh authentication token each time from <code>.gemini/settings.json</code>. This ensures seamless access without needing to restart your IDE.</p>
</li>

<li><p>Note that, structure of <code>mcp.json</code> is now slightly different from <code>settings.json</code> as <code>mcp.json</code> now configures servers in a pseduo-stdio mode using <code>command</code>, <code>args</code> and <code>env</code> blocks instead of standard <code>httpUrl</code> block so that the client-bridge can proxy requests with latest token injection.</p>
</li>
</ul>
<p><strong>Key Changes</strong></p>

<h3><code>gemini-mcp-setup.py</code></h3>

<ul>
<li><p>Renamed <code>gemini-mcp-setup.py</code> to <code>gemini-mcp-agent.py</code> to reflect its upgraded feature set.</p>
</li>

<li><p><code>gemini-mcp-agent</code> now provides an internal-use command option <code>--mcp-client-bridge</code> for IDEs like Android Studio (and ASfP) that cache configurations</p>

<ul>
<li><p>where each MCP server configured in <code>mcp.json</code> spawns its own MCP-client bridge.</p>
</li>

<li><p>The bridge uses <code>stdio</code> to communicate with the IDE, injects updated tokens from <code>.gemini/settings.json</code>, and forwards <code>JSON-RPC</code> requests to the MCP server over HTTPS (and vice-versa).</p>
</li>

<li><p>This solves the MCP config caching issue in such IDEs, ensuring seamless access without needing to restart your IDE.</p>
</li>
</ul>
</li>

<li><p>Updated <code>mcp_setup.md</code> guide for new features and improved clarity</p>
</li>
</ul>
<p><strong>Cloud-WS images (all 3):</strong></p>

<ul>
<li><p>added <code>GOOGLE_CLOUD_PROJECT</code> as dockerfile ARG and set as container ENV</p>
</li>

<li><p>passing value for <code>GOOGLE_CLOUD_PROJECT</code> from Jenkins env var <code>CLOUD_PROJECT</code></p>
</li>

<li><p>Updated descriptions in jenkinsfile for all 3 cloud-ws groovy files</p>
</li>

<li><p>Yarn GPG key fix that caused build failure</p>
</li>

<li><p>simplified and optimized image layers</p>
</li>
</ul>
<p><strong>More</strong> <strong>on</strong> <code>gemini-mcp-agent</code> <strong>changes</strong></p>

<ul>
<li><p>new func <code>discover_android_studio_mcp_file_path</code> to find mcp.json if platform is Android Studio or ASFP and set the constant <code>ANDROID_STUDIO_MCP_FILE_PATH</code></p>
</li>

<li><p>agent updates the <code>mcp.json</code> only when <code>ANDROID_STUDIO_MCP_FILE_PATH</code> holds a non-None value.</p>
</li>

<li><p>added <code>update_android_studio_mcp_file</code> which has slightly diff logic to <code>update_gemini_cli_settings_file</code> as mcp.json structure is diff from settings.json as <code>mcp.json</code> now defines MCP servers with <code>command</code> as this agent script with args <code>--mcp-client-bridge</code> and <code>--mcp-server</code> name. This option combo calls the new <code>run_mcp_client_bridge</code> function.</p>
</li>

<li><p>added new <code>run_mcp_client_bridge</code> function to read MCP JSON-RPC requests from android studio IDE (via stdio) and forward it to remote MCP server (via HTTPs)</p>
</li>

<li><p>updated <code>is_managed_server</code> function to accept <code>server_http_url</code> instead of entire block</p>
</li>

<li><p>renamed <code>ensure_config_dir</code> to <code>ensure_configs_exist</code> that always creates config files for <code>gemini-cli</code> and optionally for as/asfp only if the environment is as/asfp based</p>
</li>

<li><p>renamed <code>update_gemini_config</code> to <code>update_gemini_cli_settings_file</code></p>
</li>

<li><p>added new env var ENV_FILE_PATH to store env file path</p>
</li>

<li><p>added new func <code>load_env_config</code> to load env vars from <code>ENV_FILE_PATH</code> or <code>.env</code> file in current dir or global fallback dir of <code>~/.gemini/.env</code></p>
</li>

<li><p>updated func <code>update_android_studio_mcp_file</code> to store env vars into mcp.json file for mcp-client-bridge processes to use them</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1334</p>
</td>

<td valign="top" width="24%"><p>Generate GitHub App private key PKCS#8 format via Terraform</p>
</td>

<td valign="top" width="64%"><p>Extension to the new simplified deployment flow for Horizon SDV introduced in Rel.3.0.0.</p>

<ul>
<li><p>PKCS#8 format of the GitHub App private key is created automatically by terraform.</p>
</li>

<li><p>The variable <code>sdv_github_app_private_key_pkcs8</code> is removed.</p>
</li>

<li><p>PKCS#8 format of the GitHub App private key is stored in the GCP Secret Manager</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>GCP changes [Google]</h2>
<p>Google has changed <a href="https://support.google.com/cloud/answer/15549257#client-secret-hashing"><u>Client Secret Handling and Visibility</u></a> . This affects redeployments of the Horizon SDV platform if the <em>Client Secret</em> was not securely stored previously.</p>
<p>This secret is required by Keycloak for the Google Identity Provider (Client Secret). If the secrets do not match, OAuth 2.0 authentication will fail and users will lose access.</p>

<h3><strong>Solution:</strong></h3>

<ul>
<li><p>Create a new secret in Google Cloud:</p>

<ul>
<li><p>In Credentials, select the Horizon client secret</p>
</li>

<li><p>Disable the old secret and create a new one.</p>
</li>

<li><p>Download or copy the new secret and store it securely.</p>
</li>
</ul>
</li>

<li><p>Verify login (for apps from Landing Page) fail.</p>
</li>

<li><p>Update Keycloak:</p>

<ul>
<li><p>Go to Identity Provider → Google.</p>
</li>

<li><p>Update the Client Secret and save.</p>
</li>
</ul>
</li>

<li><p>Verify login works as expected.</p>
</li>
</ul>

<h2>Documentation update</h2>

<ul>
<li><p>Rel.3.1.0 provides with several updates in Horizon documentation including e.g. <strong>Horizon Deployment Guide</strong> (/docs/deployment_guide.md). </p>
</li>

<li><p>The new<strong> contributing.md</strong> document  (/doc/contributing.md) defines rules for Partner Contributions Repository integration and recommended directory structure for third party modules provided from external Horizon Partners. </p>
</li>

<li><p>The new Upgrade Guide (/docs/guides/upgrade_guide_3_0_0_to_3_1_0.md) provide guideline for   Rel.3.0.0 -&gt; Rel.3.1.0 upgrade. </p>
</li>
</ul>

<h2>Bug Fixes</h2>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="10%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Bug</strong></p>
</td>

<td valign="top" width="51%"><p><strong>Description</strong></p>
</td>

<td valign="top" width="15%"><p><strong>SHA</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1236</p>
</td>

<td valign="top" width="24%"><p>[Volvo] Google platform failures on jenkins-mtk-connect-apikey</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>mtk-connect-post-key: add create_or_update_jenkins_secret() so the jenkins-mtk-connect-apikey secret is created if absent (CronJob or one-off can now establish the credential; previously only updated existing secret, causing "Could not find credentials entry" when mtk-connect-post-job had not run or had failed).</p>
</li>

<li><p>mtk-connect-post configure.sh: make DELETE curls non-fatal (|| true) so 404 on first run does not exit; remove if block so any real failure exits the job visibly.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>ea84ef88c7236d582707601e368fd1803a3345c4</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1260</p>
</td>

<td valign="top" width="24%"><p>Sync Mirror pipeline hangs after modifying MIRROR_VOLUME_CAPACITY_GB during Infra creation</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Fixed issue where Filestore expansion (e.g., 4TB → 5TB) caused PVCs to remain stuck in <code>Pending</code> state with 0 capacity</p>
</li>

<li><p>Resolved Kubernetes binding conflicts caused by static PV/PVC provisioning without a StorageClass or CSI driver</p>
</li>

<li><p>Eliminated race conditions during resize where old PVCs were not released and PVs entered <code>Failed</code> state</p>
</li>

<li><p>Removed incompatible <code>ReclaimPolicy=Delete</code> usage on statically‑provisioned NFS volumes</p>
</li>

<li><p>Migrated Mirror storage from static PV/PVC management to <strong>Filestore CSI driver–based dynamic provisioning</strong></p>
</li>

<li><p>Introduced new StorageClass with:</p>

<ul>
<li><p><code>filestore.csi.storage.gke.io</code> provisioner  </p>
</li>

<li><p><code>allowVolumeExpansion=true</code> for online resize  </p>
</li>

<li><p><code>ReclaimPolicy=Retain</code> for data safety</p>
</li>
</ul>
</li>

<li><p>Simplified Terraform to manage only the PVC; CSI driver now owns PV lifecycle</p>
</li>

<li><p>Added safeguards to <strong>prevent volume downsizing</strong>, avoiding potential data loss</p>
</li>

<li><p>Standardized naming by removing legacy <code>aosp</code> references across configs and scripts</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>86bee3badf422614629752a19bcf19d8555789ef</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1326</p>
</td>

<td valign="top" width="24%"><p>Cloud WS: Create Configuration fails for region other than europe-west1</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Parameter WS_REPLICA_ZONES as default value was partially hardcoded ({CLOUD_REGION}-b, -d) )For some zones eg “us-central1-d” is not existing ( currently us-central1-a, b, c, f) .</p>
</li>

<li><p>Implemented solution: If user will not add any replica_zone values The default value will retrieve all zones in region and automatically select the first two zones in current region</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>1ea0c42ed4ccc2adcbae0126d34664af9599b79e</p>
</li>

<li><p>71a7316c70873e57da6395ee51a0a87684fe5d08</p>
</li>

<li><p>73d4f09c55f4130e1023df7546b53a37c42118cf</p>
</li>

<li><p>b8caa3676843d104b1e4fa7120dc76dbd6c9acfa</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1327</p>
</td>

<td valign="top" width="24%"><p>Cloud WS: Create Workstation pipeline fails (WS created but IAM user add fails)</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Fix: Ensure the workstation is fully created and ready before applying IAM bindings.<br/>This helps prevent concurrent IAM policy modification conflicts (409 errors)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>818bda3e6d5580c8b339b26dfe4b8dad5f28fdac</p>
</li>

<li><p>18ee772625d5abd7377906c6c9865c7be91dec0f</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1340</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] ABFS license no longer applied in deployment</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Simplified Horizon deployment dropped support of creating the ABFS license and as such, this must now be applied via Jenkins ABFS server and uploaders when action is APPLY.</p>
</li>

<li><p>Mask the license for security reasons.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>290bf5dea46d4f058d3fc96f8b67881c1efbdf9c</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1416</p>
</td>

<td valign="top" width="24%"><p>Remove obsolete ABFS secrets created via Terraform and GitOps</p>
</td>

<td valign="top" width="51%"><p>This PR removes deprecated <strong>ABFS license resources</strong> that were previously managed through Terraform and GitOps. The ABFS license is now <strong>exclusively managed by Jenkins</strong>, and all unused license-related resources and references have been cleaned up accordingly.</p>
<p><strong>Details</strong>:</p>

<ul>
<li><p>Removed the Terraform variable and references for <code>sdv_abfs_license_key_b64</code>.</p>
</li>

<li><p>Removed the Kubernetes/secret resources and references for <code>jenkins-abfs-license-b64</code>.</p>
</li>

<li><p>Cleaned up all dependent configurations and references to ensure no residual usage of the removed license resources.</p>
</li>
</ul>
<p><strong>Verification</strong></p>

<ul>
<li><p>Deployed the platform after removing the deprecated ABFS license resources.</p>
</li>

<li><p>Confirmed no deployment or runtime issues related to ABFS licensing.</p>
</li>
</ul>
<p><strong>Purpose</strong></p>
<p>These changes simplify license management by consolidating ABFS license handling within Jenkins, reduce configuration complexity in Terraform and GitOps, and prevent confusion caused by unused or legacy license resources.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a7c2bbbf6e1189b6a5119c983183bfb7001133e6</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1418</p>
</td>

<td valign="top" width="24%"><p>Fails on pkcs8_converter (jq missing)</p>
</td>

<td valign="top" width="51%"><p>TAA-1418: install jq dependency for pkcs8 conversion</p>

<ul>
<li><p>Resolves deployment failures in TAA-1418</p>
</li>

<li><p>Adds missing 'jq' binary required by the external terraform data source</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>b80c14290470ac483b8d1eb587acc20084b3a422</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1428</p>
</td>

<td valign="top" width="24%"><p>Password check incorrect (12 should mean 12)</p>
</td>

<td valign="top" width="51%"><p>TAA-1428: Correct password length check</p>
<p>If it states it should be at least 12 characters, ensure the check is correct, ie &gt;= 12 not &gt; 12!</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>f29c70246fe52a4f880a2e332660157e1459af2e</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1429</p>
</td>

<td valign="top" width="24%"><p>argocd namespace stuck in 'Terminating'</p>
</td>

<td valign="top" width="51%"><p>Update deployment script with deletion of resources which cause the namespace <code>argocd</code> to be stuck in <code>terminating</code> state indefinitely.</p>
<p><strong>Changes</strong></p>
<p>deploy.sh</p>
<p>File path: <code>tools/scripts/deployment/deploy.sh</code></p>

<ul>
<li><p>Added two new functions</p>

<ul>
<li><p><code>cleanup_gateways()</code> - Deletes the GKE Gateway which triggers the deletion of backends, load balancers and NEGs.</p>
</li>

<li><p><code>cleanup_argocd()</code> - Deletes all Apps created by <code>horizon-sdv</code> app to prevent it from being stuck in terminating state.</p>
</li>
</ul>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>d2d32295bc4580bf77fc6f59cb11301de1451636</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1430</p>
</td>

<td valign="top" width="24%"><p>Enable 'force_destroy' on buckets</p>
</td>

<td valign="top" width="51%"><p>Enable <code>force_destroy</code> for GCS buckets to destroy the buckets on Terraform destroy workflow even if it contains objects.</p>
<p><strong>Changes</strong></p>
<p>main.tf</p>
<p>File path: <code>terraform/modules/sdv-gcs/main.tf</code></p>

<ul>
<li><p>Add <code>force_destroy = true</code> to enable force destruction of GCS buckets.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>211d4564d0265b38ee789dddca7708a8982502af</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1432</p>
</td>

<td valign="top" width="24%"><p>landingpage 'exec format error'</p>
</td>

<td valign="top" width="51%"><p>landingpage 'exec format error' fix</p>
<p>Ensure docker images are built for the target platform, not the architecture of the platform they are deployed on.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4322698a334d01c2c84ab72967537063b3c557ca</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1435</p>
</td>

<td valign="top" width="24%"><p>Cross architecture support</p>
</td>

<td valign="top" width="51%"><p>Cross architecture support fix.</p>
<p>Explicitly set Docker base image platform to linux/amd64 to ensure cross-architecture deployment consistency.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>3ef9eb0b71f45bb920a9d62606118ee130895f76</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1438</p>
</td>

<td valign="top" width="24%"><p>Cuttlefish SSH key incorrectly created (blocks CF jobs)</p>
</td>

<td valign="top" width="51%"><p><strong>Cuttlefish SSH Key Update: Regenerate VM Templates</strong></p>
<p>This fix updates the SSH key generation algorithm used by Cuttlefish VM instances. To avoid any impact, regenerate the VM instance templates.</p>
<p>In Jenkins:</p>

<ul>
<li><p><code>Android Workflow → Environment → Docker Image Template → Build with Parameters</code></p>

<ul>
<li><p>Deselect <code>NO_PUSH</code> to ensure image is uploaded to registry.</p>
</li>

<li><p>Click <code>Build</code></p>
</li>
</ul>
</li>

<li><p><code>Android Workflow → Environment → CF Instance Template → Build with Parameters</code></p>

<ul>
<li><p>Set <code>ANDROID_CUTTLEFISH_REVISION=main</code></p>
</li>

<li><p>Click <code>Build</code></p>
</li>

<li><p>Repeat for the tagged version of Android Cuttlefish</p>
</li>
</ul>
</li>

<li><p><code>Android Workflow → Environment → CF Instance Template ARM64 → Build with Parameters</code></p>

<ul>
<li><p>Repeat for ARM64 if enabled.</p>
</li>

<li><p>Set <code>ANDROID_CUTTLEFISH_REVISION=main</code></p>
</li>

<li><p>Click <code>Build</code></p>
</li>

<li><p>Repeat for the tagged version of Android Cuttlefish</p>
</li>
</ul>
</li>
</ul>
<p>If SSH key issues appear in any of the following jobs, regenerate the instance templates to ensure the latest keys are installed:</p>

<ul>
<li><p><code>Android Workflow → Environment → Development Test Instance</code></p>
</li>

<li><p><code>Android Workflow → Builds → Gerrit</code></p>
</li>

<li><p><code>Android Workflow → Tests → CVD Launcher</code></p>
</li>

<li><p><code>Android Workflow → Tests → CTS Execution</code></p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>eb61aefb3e86a1e16022708a13b0657eaf5b79f0</p>
</li>

<li><p>03f52993fbf637c084e1db0f61be65f21f5c2853</p>
</li>

<li><p>172781210fba6573434ba8e9b6da2b68b0b206d3</p>
</li>

<li><p>501e12e97e89e26eb74fa7c855ca15b3e03921a0</p>
</li>

<li><p>d80ccf7323c22d2b85a2f4a8d09be4b1983c95e9</p>
</li>

<li><p>5442aecc9a0cd98ef7b98699f095b0b9332f3e9e</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1441</p>
</td>

<td valign="top" width="24%"><p>Finalize cross architecture support - R31.0</p>
</td>

<td valign="top" width="51%"><p>Updates in deployment scripts and containers to emulate <code>linux/amd64</code></p>
<p><strong>Changes</strong></p>
<p><strong>container-deploy.sh</strong></p>
<p>File path: <code>tools/scripts/deployment/container-deploy.sh</code></p>

<ul>
<li><p>Update the script to run the deployment container with <code>linux/amd64</code> emulation pinned.</p>
</li>
</ul>
<p><strong>Dockerfile</strong></p>
<p>File path: <code>tools/scripts/deployment/container/Dockerfile</code></p>

<ul>
<li><p>Update the Dockerfile to be built for <code>linux/amd64</code>.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>076c2c57434c2596e2db44ffb60e4c435f55b1a6</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1443</p>
</td>

<td valign="top" width="24%"><p>Gerrit MCP Server issues</p>
</td>

<td valign="top" width="51%"><p>Fix syntax error for <code>gerrit-mcp-server-config</code> causing <code>gerrit-mcp-server</code> deployment errors.</p>
<p><strong>Changes</strong></p>
<p>gerrit-mcp-server.yaml</p>
<p>File path: <code>gitops/apps/gerrit-mcp-server/templates/gerrit-mcp-server.yaml</code></p>

<ul>
<li><p>Remove <code>-</code> causing syntax issues.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>e6e2375372b4b16ce8d78a017818989ee911d954</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1446</p>
</td>

<td valign="top" width="24%"><p>TF OpenSSH conversion failing</p>
</td>

<td valign="top" width="51%"><p>Fixed a bug where the OpenSSH key was not being updated after the initial RSA key creation.</p>
<p>Replaced null_resource with terraform_data and added a timestamp trigger to force an idempotent conversion check on every run. This ensures that if an RSA key exists without the OpenSSH format, the conversion logic is triggered, while the grep check protects against unnecessary overwrites. </p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a1f7ce4beaa59dd9acbd09a5c2571cbb8b5af2b8</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1447</p>
</td>

<td valign="top" width="24%"><p>Shell Script Permission Denied</p>
</td>

<td valign="top" width="51%"><p>Update Dockerfiles for <code>sdv-container-images</code> module which when built with Terraform as a non-root user causes <code>permission denied</code> error for <code>configure.sh</code></p>
<p><strong>Changes</strong></p>
<p>Resolve permission related issues.</p>
<p><strong>File paths:</strong></p>

<ul>
<li><p>Grafana Post: <code>terraform/modules/sdv-container-images/images/grafana/grafana-post/Dockerfile</code></p>
</li>

<li><p>Keycloak Post Argo CD: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-argocd/Dockerfile</code></p>
</li>

<li><p>Keycloak Post Gerrit: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-gerrit/Dockerfile</code></p>
</li>

<li><p>Keycloak Post Grafana: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-grafana/Dockerfile</code></p>
</li>

<li><p>Keycloak Post Headlamp: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-headlamp/Dockerfile</code></p>
</li>

<li><p>Keycloak Post Jenkins: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-jenkins/Dockerfile</code></p>
</li>

<li><p>Keycloak Post MCP Gateway Resgistry: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-mcp-gateway-registry/Dockerfile</code></p>
</li>

<li><p>Keycloak Post MTK Connect: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post-mtk-connect/Dockerfile</code></p>
</li>

<li><p>Keycloak Post: <code>terraform/modules/sdv-container-images/images/keycloak/keycloak-post/Dockerfile</code></p>
</li>

<li><p>MTK Connect Post Key: <code>terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile</code></p>
</li>

<li><p>LandingPage App: <code>terraform/modules/sdv-container-images/images/landingpage/landingpage-app/Dockerfile</code></p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>1e1532c5ca5a2a41f8a20ceaf9012f868947aed4</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1450</p>
</td>

<td valign="top" width="24%"><p>High severity violation of security rules - "GCP DNS zones DNSSEC disabled" #4</p>
</td>

<td valign="top" width="51%"><p>DNSSEC support in GCP DNS zones enabled by default.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>363659c78c41d6a3db7cf6877ec7320eb2b443a0</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1453</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/landingpage-app container</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>CVE-2025-48174 is fixed in 1.3.0 for libavif</p>
</li>

<li><p>CVE-2026-22801 is fixed in 1.6.54-r0 for libpng </p>
</li>

<li><p>CVE-2026-22695 is fixed in 1.6.54-r0 for libpng</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1457</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/keycloak-post-headlamp container</p>
</td>

<td valign="top" width="51%"><p>32 Vulnerabilities fixed fixed in keycloak-post-headlamp container. Base OS Change - node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1458</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/keycloak-post-grafana container</p>
</td>

<td valign="top" width="51%"><p>32 Vulnerabilities fixed in keycloak-post-grafana container. Base OS Change -  Node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1459</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/keycloak-post-gerrit container</p>
</td>

<td valign="top" width="51%"><p>33 Vulnerabilities fixed in keycloak-post-gerrit container.  Base OS Change - Node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1460</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/keycloak-post-argocd container</p>
</td>

<td valign="top" width="51%"><p>33 Vulnerabilities fixed in keycloak-post-argocd container.  Base OS Change - Node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1461</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/keycloak-post container</p>
</td>

<td valign="top" width="51%"><p>33 Vulnerabilities fixed in keycloak-post container. Base OS Change -  Node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1462</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/grafana-post container</p>
</td>

<td valign="top" width="51%"><p>33 Vulnerabilities fixed in keycloak-post container.  Base OS Change-Node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1463</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/gerrit-post container</p>
</td>

<td valign="top" width="51%"><p>7 Vulnerabilities fixed in gerrit-post container. Base OS Change - Debian 12.12 → Debian 12.13</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1455</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/keycloak-post-mtk-connect container</p>
</td>

<td valign="top" width="51%"><p>32 Vulnerabilities fixed fixed in keycloak-post-mtk-connect container. Base OS Change - node:22.13.0 → node:22-bookworm</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1452</p>
</td>

<td valign="top" width="24%"><p>Vulnerabilities in /horizon-sdv/mtk-connect-post container</p>
</td>

<td valign="top" width="51%"><p>5 Vulnerabilities fixed in gerrit-post container. Base OS Change - Debian 12.12 → Debian 12.13</p>
<p>Base Image Changes:</p>

<ul>
<li><p><code>debian:12.12</code> → <code>debian:12.13</code></p>
</li>

<li><p><code>node:22.13.0</code> → <code>node:22-bookworm</code> (includes Debian 12.13)</p>
</li>

<li><p><code>python:3.9-slim</code> → <code>python:3.9-slim-bookworm</code> (explicit)</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>a2b3bbb91091cc3c9e99014c1acacac6855bce3a</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1468</p>
</td>

<td valign="top" width="24%"><p>High severity violation of security rules "GCP GKE Application-layer Secrets encryption disabled " #7</p>
</td>

<td valign="top" width="51%"><p>KMS can be deployed based on settings in <strong>terraform.tfvars</strong> - (sdv_enable_kms_encryption = false). </p>
<p>KMS implementation details:</p>

<ul>
<li><p>It is possible to use KMS to encrypt kubernetes secrets (“Application-layer secrets encryption” option in GKE)</p>
</li>

<li><p>If enabled – a KMS keyring is created, then a symmetric key (at version 1) is created inside the keyring</p>
</li>

<li><p>Encryption is fully transparent to the cluster</p>
</li>

<li><p>Once key is created – it is not easy to destroy it, it is rather that version 2 of the key will be created, and previous version 1 even if marked “destroy” – will be gone after 30 days.</p>
</li>

<li><p>Once keyring is created – IT IS NOT POSSIBLE TO DESTROY IT , so it makes trouble in terraform state when created and tried to delete it later on</p>
</li>

<li><p>KMS feature is disabled by default.</p>
</li>

<li><p>Keyring can easily be deleted only if entire GCP project is deleted.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4ea1c55f90d22d77d74a2206c7c326c3dfeef495</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1475</p>
</td>

<td valign="top" width="24%"><p>[Cuttlefish] OS Login Cleanup Script Errors - Improper Parsing &amp; Excessive Latency</p>
</td>

<td valign="top" width="51%"><p>Avoid issues with using table that can lead to erroneous values leading to us delaying 1m per loop and taking too long.</p>
<p>Make it a function so we can use elsewhere if required.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>5442aecc9a0cd98ef7b98699f095b0b9332f3e9e</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1481</p>
</td>

<td valign="top" width="24%"><p>mtk-connect-post-key Post-job container image build fails</p>
</td>

<td valign="top" width="51%"><p>The permission issue which causes the container image build to fail has been resolved.</p>
<p><strong>Changes</strong></p>
<p>Dockerfile</p>
<p>File path: <code>terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile</code></p>

<ul>
<li><p>Add <code>--chown=appuser:appuser</code> to fix permission issues.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>ef72216ba232586dea96306431a8860b64b9d5e5</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1482</p>
</td>

<td valign="top" width="24%"><p>Terraform destroy fails to delete VPC</p>
</td>

<td valign="top" width="51%"><p>This merge fixes the issue which cause terraform destroy to fail due to the failure in deletion of the VPC <code>sdv-network</code> caused due to remaining NEGs (Network Endpoint Groups).</p>
<p><strong>Changes</strong></p>
<p>deploy.sh</p>
<p>File path: <code>tools/scripts/deployment/deploy.sh</code></p>

<ul>
<li><p>Update the script's <code>cleanup_gateways()</code> function to also remove <code>http-routes</code> which triggers the deletion of NEGs.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>d24100db5874a9591404fe522be1f39617448831</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1492</p>
</td>

<td valign="top" width="24%"><p>Refactor Argo CD Application Lifecycle to Terraform-Native Cascading Delete</p>
</td>

<td valign="top" width="51%"><p>Update the Terraform module <code>sdv-gke-apps</code> module to enable cascading delete for the App of Apps <code>horizon-sdv</code> (<code>argocd_application</code>) and update dependency chain for the module <code>sdv-gke-cluster</code>.</p>
<p><strong>Changes</strong></p>
<p><strong>main.tf</strong></p>
<p>File path: <code>terraform/modules/base/main.tf</code></p>

<ul>
<li><p>Update the module <code>sdv-gke-cluster</code> with depency on <code>sdv_certificate_manager</code> and <code>sdv_ssl_policy</code> to enable deletion of GKE cluster before deletion of SSL Policy and Certificate Manager Certificates to avoid issues or errors while running Terraform destroy workflow.</p>
</li>
</ul>
<p><strong>main.tf</strong></p>
<p>File path: <code>terraform/modules/sdv-gke-apps/main.tf</code></p>

<ul>
<li><p>Update dependency, add required finalizer to enable cascading delete for the <code>horizon-sdv</code> app.</p>
</li>

<li><p>Add <code>wait= true</code> to ensure complete deletion of <code>horizon-sdv</code> app before Terraform destroy workflow proceeds to destroy other resources in the module.</p>
</li>
</ul>
<p><strong>Dockerfile</strong></p>
<p>File path: <code>tools/scripts/deployment/container/Dockerfile</code></p>

<ul>
<li><p>Remove <code>kubectl</code> from Dockerfile as it is no longer required.</p>
</li>
</ul>
<p><strong>deploy.sh</strong></p>
<p>File path: <code>tools/scripts/deployment/deploy.sh</code></p>

<ul>
<li><p>Remove <code>kubectl</code> operation from <code>deploy.sh</code> as it is no longer required to perform clean-up activities.</p>
</li>
</ul>
<p></p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>d438544bd1469a8aec19bf31fa35ecdfbb3648d1</p>
</li>

<li><p>7f1486291a1e81bb4fdd1d55c77c54d05097ec5c</p>
</li>

<li><p>f81ba04d48ab5b7b9f8f59cd85b2acc14252116c</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1493</p>
</td>

<td valign="top" width="24%"><p>Cloud-WS Image Builds: Yarn GPG Key Issue</p>
</td>

<td valign="top" width="51%"><p>Added Yarn GPG key refresh before first <code>apt-get update</code> in all Dockerfiles</p>
</td>

<td valign="top" width="15%"><p></p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1494</p>
</td>

<td valign="top" width="24%"><p>Kubernetes NetworkPolicies update breaks deployment</p>
</td>

<td valign="top" width="51%"><p>Missing closing brace breaking deployment.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>c95c4c1cbb6ff7f1e47a296868fbc094aa9b619b </p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1495</p>
</td>

<td valign="top" width="24%"><p>Security hardening breaks deployment</p>
</td>

<td valign="top" width="51%"><p>An input variable with the name "sdv_dns_dnssec_enabled" has not been declared. This variable can be declared with a variable "sdv_dns_dnssec_enabled" {} block.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>781c30d3e9c9f76c52e508cb4da2f0e7cf0fc1eb</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1498</p>
</td>

<td valign="top" width="24%"><p>Terraform local-exec fails because gcloud project is not explicitly set in script</p>
</td>

<td valign="top" width="51%"><p>Gcloud project is explicitly set in script</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4114bbaefb3305216541cce6a21f5874ff647de8</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1499</p>
</td>

<td valign="top" width="24%"><p>Terraform destroy blocks redeployment when KMS is enabled (sdv_enable_kms_encryption = true) </p>
</td>

<td valign="top" width="51%"><p>Several fixes for KMS deployment</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>fe8c58c57f440cbebb32d6ad48b567245f3a07e6</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1507</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] CF instances - Fails to connect via ssh</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Firewall: allow SSH to Cuttlefish from GKE node range (10.1.0.0/24).</p>
</li>

<li><p>Jenkins: allow controller egress SSH to Cuttlefish (22); allow agent<br/> SSH to Cuttlefish.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>b65cda8a4af97e788af259396445415c243d0919</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1508</p>
</td>

<td valign="top" width="24%"><p>[Jenkins] Fix Jenkins startup and Gerrit connectivity</p>
</td>

<td valign="top" width="51%"><p>Set noConnectionOnStartup: true for Gerrit so Jenkins starts and the UI is available without waiting for Gerrit; the plugin connects when Gerrit is reachable.</p>
<p>Add allow-jenkins-controller-egress-to-gerrit NetworkPolicy so the<br/>controller can reach Gerrit on 29418 (SSH) and 8080 (HTTP). Default-deny had limited controller egress to 80/443, so the Gerrit Trigger never connected.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>b6cb82e5122502f2225d2511d227e6715074e8f2</p>
</li>

<li><p>667a01271394ac723922cc321da365a78f62b915</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1517</p>
</td>

<td valign="top" width="24%"><p>[Cloud-WS] terminal monospace rendering &amp; gemini-mcp-agent executable broken entrypoint</p>
</td>

<td valign="top" width="51%"><p><strong>Fixes applied</strong></p>

<ul>
<li><p>move gemini-mcp-agent shebang to line 1 so the binary executes with python</p>
</li>

<li><p>install fonts-dejavu-core in android-studio, asfp, and code-oss images</p>
</li>

<li><p>set GNOME Terminal dconf defaults (DejaVu Sans Mono 12, cell width/height scale 1.0) for desktop images</p>
</li>

<li><p>run dconf update during image setup to apply terminal defaults</p>
</li>
</ul>
<p><strong>Minor changes</strong></p>

<ul>
<li><p>updated docs/guides/mcp_setup.md for clear info on gemini-mcp-agent and mcp servers settings in android studio IDE</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>101a10e02cca3979f2d3633f28ccd33fef69e39d</p>
</li>

<li><p>9f8f771b984319b687eb9d0739be2ae725094444</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1528</p>
</td>

<td valign="top" width="24%"><p>ABFS server and uploader: SSH on port 22 blocked; get_server_details / get_uploader_details and Console SSH fail.</p>
</td>

<td valign="top" width="51%"><p>Code in this PR fixes port 22 opening.<br/>And deployment issue which fixes "Error: googleapi: Error 400: The network policy addon must be enabled before updating the nodes." in file terraform/modules/sdv-gke-cluster/main.tf</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>f8758c356c6e376c2548340614f6fcdd3fe56232</p>
</li>

<li><p>4e974f84bb0c6ea5c209ec9e14e918ba25260a3c</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1529</p>
</td>

<td valign="top" width="24%"><p>Pin ABFS build node pool to a fixed GKE version so CASFS kernel module stays compatible</p>
</td>

<td valign="top" width="51%"><p>This PR pins the <strong>ABFS build node pool</strong> to a configurable GKE version to ensure <strong>CASFS kernel compatibility</strong> and prevent breakage caused by automatic node upgrades.</p>
<p><strong>Details</strong></p>

<ul>
<li><p>Introduced <code>sdv_abfs_build_node_pool_version</code> variable to configure the ABFS build node pool GKE version.</p>
</li>

<li><p>Set the node pool <code>version</code> attribute using this variable to pin the node image and kernel.</p>
</li>

<li><p>Replaced release channel usage with an explicit cluster version (<code>sdv_cluster_version</code>) to allow disabling auto-upgrade on the ABFS node pool.</p>
</li>

<li><p>Updated <code>terraform.tfvars</code> and <code>terraform.tfvars.sample</code> with pinned values (e.g. <code>1.32.7-gke.1079000</code>).</p>
</li>
</ul>
<p><strong>Purpose</strong></p>
<p>CASFS is a kernel module and must match the running node kernel. By pinning the ABFS node pool GKE version, we ensure the kernel remains stable and compatible, preventing unexpected failures caused by GKE auto-upgrades.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>f81bc7a22a434fa578190b2c28b03f5c0a9d23b6</p>
</li>

<li><p>be32e04e32df4098ffb8b27bea745008feb44916</p>
</li>

<li><p>5f2625760dabe05e85e3936cef0823161163a4ae</p>
</li>

<li><p>f4d7724799bcb36732cfcd2d56ff2468ee1f1900</p>
</li>

<li><p>44ae1d9a659a9d49f8f3dba32c791caa57b52440</p>
</li>

<li><p>30d8eb8d3309306cfb8e37e021f18c44e80b1bcc</p>
</li>

<li><p>f04bf569d1d0e08cdb3d5e6040b0e1c3ecdb35d2</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1535</p>
</td>

<td valign="top" width="24%"><p> GKE deployment fails on first run due to STABLE release channel conflict</p>
</td>

<td valign="top" width="51%"><p>Fix the error <code>Error: error creating NodePool: googleapi: Error 400: Auto_upgrade must be true when release_channel STABLE is set.</code></p>
<p>GCP requires <code>auto_upgrade = true</code> on node pools when a named release channel (STABLE/REGULAR/RAPID with REGULAR being the default option if release channel is unset) is active.<br/>Setting <code>channel = "UNSPECIFIED"</code> explicitly opts the cluster out of any release channel, removing this constraint and allowing Terraform to pin versions directly.</p>
<p>Also formatted all Terraform files in <code>terraform/</code> for alignment consistency (no logic changes).</p>
<p><strong>Changes</strong></p>
<p><strong>terraform/modules/sdv-gke-cluster/main.tf</strong></p>

<ul>
<li><p>Add <code>release_channel { channel = "UNSPECIFIED" }</code> block so the GCP API treats the cluster as unenrolled from any release channel.</p>
</li>
</ul>
<p><strong>tools/scripts/deployment/deploy.sh</strong></p>

<ul>
<li><p>Remove the <code>unenroll_cluster_release_channel</code> function as the release channel is now managed declaratively by Terraform, making the <code>gcloud</code> workaround obsolete.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>4a81e523ede0e405465dbe366148a866f571b624</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1569</p>
</td>

<td valign="top" width="24%"><p>Gerrit-Operator in ArgoCD application goes into Unknown sync state and the Gerrit application fails to sync</p>
</td>

<td valign="top" width="51%"><p>Update <code>gerrit-operator</code> <code>repoURL</code> from Googlesource to GitHub, avoiding rate limits and fixing issues with <code>gerrit-operator</code> deployment on fresh platforms.</p>
<p><strong>Changes</strong></p>
<p>gerrit-operator.yaml</p>
<p>File path: <code>gitops/templates/gerrit-operator.yaml</code></p>

<ul>
<li><p>Update <code>repoURL</code></p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>37709c24d51326d61cd2da2c833a56af2b0e29b0</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1570</p>
</td>

<td valign="top" width="24%"><p>Terraform workloads Service Account name mismatch in GCP and k8s</p>
</td>

<td valign="top" width="51%"><p>Service Account <code>sa7</code> name in <code>terraform/env/main.tf</code> should be <code>gke-tf-wl-sa</code> instead of current value of <code>gke-terraform-workloads-sa</code> to match with other instances of the SA in yaml files.</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>d055ccc982ff4ced993dd99a6a359cda5b6b571d</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1573</p>
</td>

<td valign="top" width="24%"><p>terraform apply fails with Error 400 when removing a sub-environment due to cert map referenced by TargetHTTPSProxy</p>
</td>

<td valign="top" width="51%"><p>This PR resolves two issues affecting the sandbox environment:</p>

<ul>
<li><p><strong>Fix </strong><code>terraform apply</code><strong> Error 400 on sub-environment removal</strong> - Previously, each environment (main + each sub-env) created its own <code>google_certificate_manager_certificate_map</code> via a <code>for_each</code> loop. When a sub-environment was removed, Terraform would attempt to delete its cert map while it was still referenced by the <code>TargetHTTPSProxy</code>, causing a <code>400</code> error. All certificates (main env + sub-envs) are now consolidated into a single cert map (<code>horizon-sdv-map</code>), eliminating the per-environment map lifecycle issue.</p>
</li>

<li><p><strong>Enable GKE main node pool autoscaling</strong> - The <code>sdv_main_node_pool</code> previously had a static node count with no autoscaling. Autoscaling has been enabled to allow the cluster to scale up when resource pressure occurs (e.g. Gerrit pod scheduling failures), with a configurable min/max range (default: 1-6 nodes).</p>
</li>
</ul>
<p><strong>Changes:</strong></p>
<p><strong>Certificate Manager Consolidation</strong></p>

<ul>
<li><p><code>terraform/modules/base/locals.tf</code> - Replaced per-environment <code>cert_domains_per_env</code> map with a single flat <code>cert_domains</code> map merging main and sub-env domains.</p>
</li>

<li><p><code>terraform/modules/base/main.tf</code> - Removed <code>for_each</code> from <code>module.sdv_certificate_manager</code>, calling it once with all domains. Updated <code>dns_auth_records</code> reference accordingly.</p>
</li>

<li><p><code>terraform/modules/sdv-certificate-manager/main.tf</code> - Hardcoded cert map name to <code>horizon-sdv-map</code> so it is stable across all environments.</p>
</li>

<li><p><code>gitops/templates/gateway.yaml</code> - Updated <code>networking.gke.io/certmap</code> annotation to reference the fixed name <code>horizon-sdv-map</code> instead of the namespaced name.</p>
</li>
</ul>
<p><strong>Main Node Pool Autoscaling</strong></p>

<ul>
<li><p><code>terraform/modules/sdv-gke-cluster/main.tf</code> - Enabled <code>autoscaling</code> block on <code>sdv_main_node_pool</code> using <code>min_node_count</code> / <code>max_node_count</code> variables.</p>
</li>

<li><p><code>terraform/modules/sdv-gke-cluster/variables.tf</code> - Added <code>node_pool_min_node_count</code> (default: 1) and <code>node_pool_max_node_count</code> (default: 6).</p>
</li>

<li><p><code>terraform/modules/base/variables.tf</code> - Added <code>sdv_cluster_node_pool_min_node_count</code> and <code>sdv_cluster_node_pool_max_node_count</code> to expose these as configurable inputs.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>b010250548f9df5ff7db2afd89da96acfbfa5174</p>
</li>

<li><p>831a59f8e9c4f8f8de5b5b9d525acb3b29426641</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1579</p>
</td>

<td valign="top" width="24%"><p>Cloud WS: Create Config pipeline fails due to inconsistent order of resource creation</p>
</td>

<td valign="top" width="51%">
<ul>
<li><p>Fixed Terraform apply failures caused by <code>google_workstations_workstation_config_iam_binding</code> executing before the target workstation config was fully created</p>
</li>

<li><p>Resolved consistent <code>404 Resource Not Found</code> errors from GCP IAM API due to premature policy application</p>
</li>

<li><p>Identified missing dependency in Terraform graph caused by using <code>each.key</code> (raw input string) for <code>workstation_config_id</code></p>
</li>

<li><p>Corrected implicit dependency handling by replacing hardcoded <code>each.key</code> with a direct reference to the workstation config resource attribute</p>
</li>

<li><p>Ensured Terraform now waits for successful workstation config provisioning before applying IAM bindings</p>
</li>

<li><p>Eliminated parallel execution race condition between workstation config creation and IAM policy attachment </p>
</li>
</ul>
</td>

<td valign="top" width="15%"><p>06bbd1cf74d6e47993c0d394e441ae96ea722c8c</p>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1601</p>
</td>

<td valign="top" width="24%"><p>AAOS Builder: Build that uses mirror for repo sync fails because of empty variable `MIRROR_DIR_NAME`</p>
</td>

<td valign="top" width="51%"><p>Fixes AOSP mirror path resolution in Android Jenkins pipelines by using <code>AOSP_MIRROR_DIR_NAME</code> when constructing <code>MIRROR_DIR_FULL_PATH</code>.</p>
<p>Pipeline parameters are defined as <code>AOSP_MIRROR_DIR_NAME</code>, but Jenkinsfiles were reading <code>MIRROR_DIR_NAME</code>.<br/>This mismatch could produce an invalid mirror path when <code>USE_LOCAL_AOSP_MIRROR=true</code>.</p>
<p><strong>Change</strong></p>
<p>Updated Jenkinsfiles to build mirror path with:<br/><code>.../${AOSP_MIRROR_DIR_NAME}</code> (instead of <code>.../${MIRROR_DIR_NAME}</code>).</p>
</td>

<td valign="top" width="15%">
<ul>
<li><p>952611a5c6e8ee26ff25488e03904bbe5822cc73</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1602</p>
</td>

<td valign="top" width="24%"><p>ExternalDNS does not update apex A record when load balancer IP changes</p>
</td>

<td valign="top" width="51%"><p>ExternalDNS was not updating the apex domain A record (e.g. <code>&lt;env_name&gt;.horizon-sdv.com</code>) when the Gateway load balancer was recreated, only subdomains such as <code>mcp.&lt;env_name&gt;.horizon-sdv.com</code> were updated. ExternalDNS only updates records it owns, and ownership is stored in TXT records. With the default TXT registry, no valid ownership TXT was created for the zone apex, so the apex A record was never updated. This change sets <code>txtPrefix: "%{record_type}-."</code> so the ownership TXT is created in the same zone and ExternalDNS can own and update the apex A record.</p>
<p><strong>Changes</strong></p>
<p>external-dns.yaml</p>
<p>File path: <code>gitops/templates/external-dns.yaml</code></p>

<ul>
<li><p>Add <code>txtPrefix: "%{record_type}-."</code> so ExternalDNS can create the heritage TXT for the apex and update the apex A record when the LB IP changes.</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>5e585c4f1e9548a7dbc616fc990d6313725a480f</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1605</p>
</td>

<td valign="top" width="24%"><p>cloud-ws/gemini-cli/gemini-mcp-agent: MCP tool calls fail after some time in gemini-cli due to JWT token caching</p>
</td>

<td valign="top" width="51%"><p>This fix hardens and standardizes how MCP authentication is handled across Gemini clients by using <code>mcp-client-bridge</code> for <strong>registry-managed</strong> servers, instead of relying on cached config tokens.<br/>It also updates setup documentation to reflect the actual runtime model and adds clearer operational guidance for Android Studio/ASfP cache reload behavior.</p>
<p><strong>Changes</strong></p>
<p><strong>Command-based MCP entries for registry-managed servers</strong></p>

<ul>
<li><p>Registry-managed servers are now written as <code>command + args + env</code> bridge entries instead of static <code>httpUrl + headers</code> token entries.</p>
</li>

<li><p>This is applied in both:</p>

<ul>
<li><p><code>update_gemini_cli_settings_file(...)</code></p>
</li>

<li><p><code>update_android_studio_mcp_file(...)</code></p>
</li>
</ul>
</li>
</ul>
<p> Unified bridge entry generation</p>

<ul>
<li><p>Added reusable helpers:</p>

<ul>
<li><p><code>build_bridge_env_payload()</code></p>
</li>

<li><p><code>build_bridge_server_entry(...)</code></p>
</li>

<li><p><code>get_entry_http_url(...)</code></p>
</li>
</ul>
</li>

<li><p>Added managed-entry marker: <code>MCP_GATEWAY_REGISTRY_MANAGED=1</code>.</p>
</li>
</ul>
<p><strong>Bridge now injects auth from token file, not config headers</strong></p>

<ul>
<li><p><code>run_mcp_client_bridge(...)</code> now obtains auth via token file flow (<code>~/.gemini/mcp-gateway-registry-token.json</code>) using non-interactive refresh path.</p>
</li>

<li><p>Removed dependency on cached <code>settings.json</code> bearer values for bridge auth.</p>
</li>
</ul>
<p><strong>Transport compatibility for Gemini clients</strong></p>

<ul>
<li><p>Bridge now supports both:</p>

<ul>
<li><p>MCP stdio framed protocol (<code>Content-Length</code> headers)</p>
</li>

<li><p>NDJSON mode (legacy behavior)</p>
</li>
</ul>
</li>

<li><p>Added:</p>

<ul>
<li><p><code>_bridge_read_message(...)</code></p>
</li>

<li><p><code>_bridge_write_message(...)</code></p>
</li>
</ul>
</li>
</ul>
<p><strong>Security hardening and JSON-RPC protocol correctness (id handling)\</strong></p>

<ul>
<li><p><strong> </strong>Added guard in bridge to refuse token injection for non-registry URLs</p>
</li>

<li><p>Added strict ID validation via <code>_is_valid_jsonrpc_id(...)</code>.</p>
</li>

<li><p>Bridge no longer emits error responses for notifications/no-id messages</p>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>6438c8f1b428d01fa0f296c24810e71f9c96992d</p>
</li>
</ul>
</td>
</tr>

<tr>
<td valign="top" width="10%"><p>TAA-1608</p>
</td>

<td valign="top" width="24%"><p>Cloud WS: Add Users to WS and Remove Users from WS fail due to inconsistent way of fetching WS state</p>
</td>

<td valign="top" width="51%"><p>This fixe corrects a state-validation issue in Cloud Workstation admin pipelines (<code>add user</code> / <code>remove user</code>).</p>
<p>Previously, these pipelines validated workstation state from Terraform state (<code>terraform show -json</code>), which can be stale when users start/stop workstations via <code>gcloud</code> (user pipelines).<br/>Now, validation uses live workstation state from GCP API (<code>gcloud workstations describe</code>) to make decisions based on current runtime reality.</p>
<p><strong>Key Changes</strong></p>

<ul>
<li><p>Renamed and refactored utility function:</p>

<ul>
<li><p><code>validate_workstation_state</code> -&gt; <code>assert_workstation_state</code></p>
</li>
</ul>
</li>

<li><p><code>assert_workstation_state</code> now:</p>

<ul>
<li><p>Accepts: <code>&lt;workstation&gt; &lt;config&gt; &lt;cluster&gt; &lt;region&gt; [expected_state]</code></p>
</li>

<li><p>Uses <code>get_current_workstation_state</code> (live <code>gcloud</code> lookup)</p>
</li>

<li><p>Defaults <code>expected_state</code> to <code>STATE_STOPPED</code></p>
</li>

<li><p>Fails fast for transitional states (<code>STATE_STARTING</code>, <code>STATE_STOPPING</code>, <code>STATE_REPAIRING</code>, <code>STATE_RECONCILING</code>) with retry guidance</p>
</li>
</ul>
</li>

<li><p>Updated admin scripts to pass full workstation context:</p>

<ul>
<li><p><code>workstation-admin-operations/add-workstation-user/add-workstation-user.sh</code></p>
</li>

<li><p><code>workstation-admin-operations/remove-workstation-user/remove-workstation-user.sh</code></p>
</li>
</ul>
</li>

<li><p>In add/remove scripts:</p>

<ul>
<li><p>Workstation config is read from generated workstation map (<code>output.tfvars.json</code>)</p>
</li>

<li><p>Cluster and region are read from input tfvars</p>
</li>

<li><p>State check is now: <code>assert_workstation_state ...</code></p>
</li>
</ul>
</li>
</ul>
</td>

<td valign="top" width="15%">
<ul>
<li><p>0bbeb90f60c9c3b904dae53c2c46c3bc271450ea</p>
</li>
</ul>
</td>
</tr>
</tbody>
</table>

<h2>Known Issues:</h2>
<p></p>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 3.0.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>19.12.2025</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p>Horizon SDV 3.0.0 extends platform capabilities with support for Android 15 and the latest extensions of OpenBSW. Horizon 3.0.0 also delivers multiple new feature and several improvements over Rel. 2.0.1  along with critical bug fixes.</p>
<p>The set of new features in version 3.0.0 includes, among others: </p>

<ul>
<li><p><strong>Simplified Deployment Flow:</strong> We have overhauled the deployment process to make it more intuitive and efficient. The new flow reduces complexity, minimizing the steps required to get your environment up and running.</p>
</li>

<li><p><strong>ARM64 Support (Bare Metal):</strong> We have expanded our infrastructure support to include ARM64 Bare Metal. This allows you to run your workloads natively on ARM architecture, ensuring higher performance and closer parity with automotive edge hardware.</p>
</li>

<li><p><strong>Gemini Code Assist:</strong> Supercharge your development with the integration of <strong>Gemini Code Assist </strong>and the Gerrit MCP Server. You can now leverage Google's state-of-the-art AI to generate code, explain complex logic, debug issues faster and make use of agentic code review workflows directly within your development environment.</p>
</li>

<li><p><strong>Advanced Monitoring with Grafana:</strong> Gain deeper insights into your infrastructure with our new Grafana integration. You can now visualize and monitor POD and Instance metrics in real-time, helping you optimize resource usage and diagnose performance bottlenecks quickly.</p>
</li>
</ul>

<h3>New Features</h3>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-924</p>
</td>

<td valign="top" width="24%"><p>Simplified Horizon Deployment Flow</p>
</td>

<td valign="top" width="64%"><p>Simplified and more automated deployment flow for Horizon SDV platform without GitHub Actions to let community teams could Horizon platform faster and avoid potential human error issues.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-511</p>
</td>

<td valign="top" width="24%"><p>Gemini Code Assist in R3 – Gerrit MCP Server integration</p>
</td>

<td valign="top" width="64%"><p>Use company’s codebase as a knowledge base for Gemini Code Assist within the IDE to receive code suggestions &amp; explanations tailored to known codebase, libraries and corporate standards.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-365</p>
</td>

<td valign="top" width="24%"><p>ARM64 GCP VM (Bare Metal) support for Cuttlefish</p>
</td>

<td valign="top" width="64%"><p>ARM64 GCP VM support for Android builds and testing with Cuttlefish</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-595</p>
</td>

<td valign="top" width="24%"><p>Monitoring of POD/Instance metrics with Grafana</p>
</td>

<td valign="top" width="64%"><p>Access to CPU/Memory/Storage metrics for pods and instances, to more easily investigate and debug container, pod and instance related problems and its impact on platform performance.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-944</p>
</td>

<td valign="top" width="24%"><p>Android pipeline update to Android 16</p>
</td>

<td valign="top" width="64%"><p>Support for Android16 for AAOS, CF and CTS in Horizon pipelines. </p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-946</p>
</td>

<td valign="top" width="24%"><p>Extend OpenBSW support with additional features</p>
</td>

<td valign="top" width="64%"><p>Support for Eclipse Foundation OpenBSW workload features that were not included in Horizon-SDV R2.0.0</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-889</p>
</td>

<td valign="top" width="24%"><p>Horizon R3 Security update</p>
</td>

<td valign="top" width="64%"><p>Selected open-source applications and tools which are part of Horizon SDV platform are updated to the latest stable versions</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-377</p>
</td>

<td valign="top" width="24%"><p>Google ASOP Repo Mirroring</p>
</td>

<td valign="top" width="64%"><p>NFS based mirror of AOSP repos deployed in the K8s cluster.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-947</p>
</td>

<td valign="top" width="24%"><p>ABFS update for R3</p>
</td>

<td valign="top" width="64%"><p>Corrections and minor ABFS updates delivered from Google in Release 3.0.0 timeframe.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1072</p>
</td>

<td valign="top" width="24%"><p>Cloud Artefact storage management</p>
</td>

<td valign="top" width="64%"><p>Android and OpenBSW build jobs have been modified to allow the user to specify metadata to be added to the stored artifacts during the upload process. Implementation is supported for GCP storage option only</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-1001</p>
</td>

<td valign="top" width="24%"><p>Kubernetes Dashboard SSO integration</p>
</td>

<td valign="top" width="64%"><p>Kubernetes Dashboard SSO integration</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-945</p>
</td>

<td valign="top" width="24%"><p>Replace depreciated Kaniko tool</p>
</td>

<td valign="top" width="64%"><p>Replace depreciated Google Kaniko tool for building container images with new Buildkit tool. </p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-941</p>
</td>

<td valign="top" width="24%"><p>IAA demo case.</p>
</td>

<td valign="top" width="64%"><p>Support for Partner demo in IAA Messe show. The main technical scope is to apply a binary APK file to the Android code, help building it and flash it to selected targets (Cuttlefish and potentially Pixel) according to Partner specification.</p>
</td>
</tr>
</tbody>
</table>

<h3>Improved Features</h3>
<p>See details in horizon-sdv/docs/release-notes-3-0-0.md</p>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p>TAA-1172</p>
</td>

<td valign="top" width="86%"><p>Create Workloads area in Gitops section</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-862</p>
</td>

<td valign="top" width="86%"><p>Improvements Structure of Test pipelines</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1111</p>
</td>

<td valign="top" width="86%"><p>Unified CTS Build process</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1265</p>
</td>

<td valign="top" width="86%"><p>[Gerrit] Support GERRIT_TOPIC with existing gerrit-triggers plugin</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1271</p>
</td>

<td valign="top" width="86%"><p>Support custom machine types for Cuttlefish</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1269</p>
</td>

<td valign="top" width="86%"><p>Adjust CTS/CVD options</p>
</td>
</tr>
</tbody>
</table>

<h3>Bug Fixes</h3>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p>TAA-993</p>
</td>

<td valign="top" width="86%"><p>[ABFS] Missing permission for jenkins-sa for ABFS server </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1063</p>
</td>

<td valign="top" width="86%"><p>[Security] Axios Security update 1.12.0 (dependabot)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-904</p>
</td>

<td valign="top" width="86%"><p>ABFS unmount doesn't work</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1090</p>
</td>

<td valign="top" width="86%"><p>[Android 16] Cuttlefish builds fail (x86/arm)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1080</p>
</td>

<td valign="top" width="86%"><p>[OpenBSW] Builds no longer functional (main)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1110</p>
</td>

<td valign="top" width="86%"><p>[OpenBSW] pyTest failure</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1103</p>
</td>

<td valign="top" width="86%"><p>[Android 16] CTS 16_r2 reports 15_r5</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1145</p>
</td>

<td valign="top" width="86%"><p>Update filter (gcloud compute instance-templates list)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1161</p>
</td>

<td valign="top" width="86%"><p>[ARM64] Subnet working utils too quiet </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1113</p>
</td>

<td valign="top" width="86%"><p>[ABFS] COS Images no longer available</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1118</p>
</td>

<td valign="top" width="86%"><p>[ABFS] CASFS kernel module update required (6.8.0-1029-gke)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1176</p>
</td>

<td valign="top" width="86%"><p>[CF] CTS CtsDeqpTestCases execution on main not completing in reasonable time (x86) </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1186</p>
</td>

<td valign="top" width="86%"><p>Incorrect Headlamp Token Injector Argo CD App Project</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1196</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror changes break standard builds</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1201</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror sync failures</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1200</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror URLs and branches incorrect</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1203</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror repo sync failing on HTTP 429 (rate limits)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1205</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror - no support for dev build instance </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1198</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror does not support Warm nor Gerrit Builds</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1204</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror repo sync failing - SyncFailFastError</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1214</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror ab is an</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1219</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] Host installer failures masked</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1202</p>
</td>

<td valign="top" width="86%"><p>AOSP Mirror blocking concurrent jobs incorrectly configured</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1238</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] Update to v1.31.0 - v1.30.0 has changed from stable to unstable. </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1241</p>
</td>

<td valign="top" width="86%"><p>[Android] Mirror should not be using OpenBSW nodes for jobs AM</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1247</p>
</td>

<td valign="top" width="86%"><p>[Workloads] Remove chmod and use git executable bit </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1249</p>
</td>

<td valign="top" width="86%"><p>[GCP] Client Secret now masked (security clarification)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1264</p>
</td>

<td valign="top" width="86%"><p>[CVD] Logs are no longer being archived </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1261</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] gnu.org down blocking builds</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1266</p>
</td>

<td valign="top" width="86%"><p>Pipeline does not fail when IMAGE_TAG is empty and NO_PUSH=true</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1267</p>
</td>

<td valign="top" width="86%"><p>[CWS] OSS Workstation blocking regex incorrect (non-blocking)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1258</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] VM instance template default disk too small.</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1233</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Plugin updates for  fixes</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1278</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] SSH/SCP errors on VM instance creation</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1283</p>
</td>

<td valign="top" width="86%"><p>Mismatch in githubApp secrets (TAA-1054)  </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1277</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Plugin updates for  fixes</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1279</p>
</td>

<td valign="top" width="86%"><p>[RPI] Android 16 RPI builds now failing</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1282</p>
</td>

<td valign="top" width="86%"><p>[GCP] Cluster deletion not removing load balancers</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1257</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] android-cuttlefish build failure (regression)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1273</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] android-cuttlefish CVD device issues (regression)  </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1149</p>
</td>

<td valign="top" width="86%"><p>[K8S] Reduce parallel jobs to reduce costs </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1162</p>
</td>

<td valign="top" width="86%"><p>[K8S] Revert parallel jobs change to reduce costs</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1191</p>
</td>

<td valign="top" width="86%"><p>Monitoring deployment related hotfixes</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1114</p>
</td>

<td valign="top" width="86%"><p>[ABFS] Update env/dev license (Oct'25)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1116</p>
</td>

<td valign="top" width="86%"><p>[Android] Android 15 and 16 AVD missing SPDX BOM</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1192</p>
</td>

<td valign="top" width="86%"><p>[MTKC] Support additional hosts for dev and test instances</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1207</p>
</td>

<td valign="top" width="86%"><p>Mirror/Create-Mirror: Add parameter for size of the mirror NFS PVC</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1208</p>
</td>

<td valign="top" width="86%"><p>Mirror/Sync-Mirror: Sync all mirrors when `SYNC_ALL_EXISTING_MIRRORS` is selected </p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1211</p>
</td>

<td valign="top" width="86%"><p>[Android] Simplify Dev Build instance job</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1218</p>
</td>

<td valign="top" width="86%"><p>[Grafana] ArgoCD on Dev shows 'Out Of Sync'</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1231</p>
</td>

<td valign="top" width="86%"><p>R2 - GitHub Actions workflow fails</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1038</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CF scripts - update to retain color</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-907</p>
</td>

<td valign="top" width="86%"><p>Multibranch is not supported in ABFS</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-862</p>
</td>

<td valign="top" width="86%"><p>Improvement to structure of Test pipelines</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-788</p>
</td>

<td valign="top" width="86%"><p>Jenkins AAOS Build failure - Gerrit secrets/tokens mismatch</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1088</p>
</td>

<td valign="top" width="86%"><p>[NPM] Move wait-on post node install</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1115</p>
</td>

<td valign="top" width="86%"><p>[STORAGE] Override default paths</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1160</p>
</td>

<td valign="top" width="86%"><p>[ARM64] Lack of available instances on us-central1-b/f zone</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1274</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] CTS hangs - android-cuttlefish issues</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1290</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] ARM64 builds broken on f2fs-tools (missing)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1253</p>
</td>

<td valign="top" width="86%"><p>[MTK Connect] ERROR: script returned exit code 92/1</p>
</td>
</tr>
</tbody>
</table>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 2.0.1</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>24.09.2025</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p>Hot fix release for Rel.2.0.1 with emergency fix for Helm repo endpoint issues, and minor documentation updates.</p>

<h3>New Features</h3>
<p>N/A</p>

<h3>Improved Features</h3>
<p>New simplified Release Notes format.</p>

<h3>Bug Fixes</h3>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="14%"><p><strong>Issue ID</strong></p>
</th>

<th valign="top" width="86%"><p><strong>Summary</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1002</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Install ansicolor plugin for CWS</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1005</p>
</td>

<td valign="top" width="86%"><p>Horizon provisioning failure - Due to outdated Helm install steps</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1007</p>
</td>

<td valign="top" width="86%"><p>Cloud WS - Workstation Image builds fail due to Helm Debian repo (OSS) migration</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1040</p>
</td>

<td valign="top" width="86%"><p>Remove references to private repo in Horizon files</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-1045</p>
</td>

<td valign="top" width="86%"><p>OSS Bitnami helm charts EOL</p>
</td>
</tr>
</tbody>
</table>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 2.0.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>01.09.2025</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p>Horizon SDV 2.0.0 extends Android build capabilities with the integration of Google ABFS and introduces support for Android 15. This release also adds support for OpenBSW, the first non-Android automotive software platform in Horizon. Other major enhancements include Google Cloud Workstations with access to browser based IDEs Code-OSS, Android Studio (AS), and Android Studio for Platforms (ASfP). In addition, Horizon 2.0.0 delivers multiple feature improvements over Rel. 1.1.0 along with critical bug fixes.</p>

<h3>New Features</h3>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="12%"><p><strong>ID</strong></p>
</th>

<th valign="top" width="24%"><p><strong>Feature</strong></p>
</th>

<th valign="top" width="64%"><p><strong>Description</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-8</p>
</td>

<td valign="top" width="24%"><p>ABFS for Build Workloads</p>
</td>

<td valign="top" width="64%"><p>The Horizon-SDV platform now integrates Google's Android Build Filesystem (ABFS), a filesystem and caching solution designed to accelerate AOSP source code checkouts and builds.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-9</p>
</td>

<td valign="top" width="24%"><p>Cloud Workstation integration</p>
</td>

<td valign="top" width="64%"><p>The Horizon-SDV platform now includes GCP Cloud Workstations, enabling users to launch pre-configured, and ready-to-use development environments directly in browser.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-375</p>
</td>

<td valign="top" width="24%"><p>Android 15 Support</p>
</td>

<td valign="top" width="64%"><p>Horizon previously supported Android 15 in Horizon-SDV but by default Android 14 was selected. In this release, Android 15 android-15.0.0_r36is now the default revision.TAA-381</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-381</p>
</td>

<td valign="top" width="24%"><p>Add OpenBSW build targets</p>
</td>

<td valign="top" width="64%"><p>Eclipse Foundation OpenBSW Workload As part of the R2.0.0 delivery, a new workload has been introduced to support the Eclipse Foundation OpenBSW within the Horizon SDV platform. This workload enables users to work on the OpenBSW stack for build and testing.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-915</p>
</td>

<td valign="top" width="24%"><p>Cloud Android Orchestration - Pt. 1</p>
</td>

<td valign="top" width="64%"><p>In R2.0.0 Horizon platform introduces significant improvements to Cuttlefish Virtual Devices (CVD). These enhancements include increased support for a larger number of devices, optimized device startup processes, a more robust recovery mechanism and updated the Compatibility Test Suite (CTS) Test Plans and Modules to ensure seamless integration and compatibility with CVD.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-623</p>
</td>

<td valign="top" width="24%"><p>Management of Jenkins Jobs using CasC</p>
</td>

<td valign="top" width="64%"><p>The CasC configuration has been updated to include a single job in the jenkins.yaml file, which is automatically started on each Jenkins restart. This job provides the "Build with Parameters" option, allowing users to populate the workload of their choice or all workloads.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-462</p>
</td>

<td valign="top" width="24%"><p>Kubernetes Dashboard</p>
</td>

<td valign="top" width="64%"><p>The Horizon platform now includes the Headlamp application, a web-based tool to browse Kubernetes resources and diagnose problems.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-717</p>
</td>

<td valign="top" width="24%"><p>Multiple pre-warmed disk pools</p>
</td>

<td valign="top" width="64%"><p>The Horizon is changing to the persistent volume storage for build caches to improve build times, cost and efficiency. These pools are separated by Android major version, e.g. Android 14 and 15, but also Raspberry Vanilla (RPi) targets now have their own smaller pools rather than sharing the original common pool.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-596</p>
</td>

<td valign="top" width="24%"><p>Jenkins RBAC</p>
</td>

<td valign="top" width="64%"><p>Jenkins has been configured with RBAC capability using the Role-based Authorization Strategy (ID: role-strategy) plugin.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-611</p>
</td>

<td valign="top" width="24%"><p>Argo CD SSO</p>
</td>

<td valign="top" width="64%"><p>Argo CD has been configured with SSO capabilities. It is now possible to Login to Argo CD either by using the configured admin credentials or by clicking the “Login via Keycloak” button.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-837</p>
</td>

<td valign="top" width="24%"><p>Access Control tool</p>
</td>

<td valign="top" width="64%"><p>Additional Access Control functionality provides a Python script tool and classes for managing user and access control on GCP level.</p>
</td>
</tr>
</tbody>
</table>

<h3>Improved Features</h3>
<p>N/A</p>

<h3>Bug Fixes</h3>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="14%"><p><strong>Issue ID</strong></p>
</th>

<th valign="top" width="86%"><p><strong>Summary</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-980</p>
</td>

<td valign="top" width="86%"><p>Access control issue: Workstation User Operations succeed for non-owned workstations</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-984</p>
</td>

<td valign="top" width="86%"><p>[Kaniko] Increase CPU resource limits</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-982</p>
</td>

<td valign="top" width="86%"><p>[ABFS] Uploaders not seeding new branch/tag correctly</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-981</p>
</td>

<td valign="top" width="86%"><p>[ABFS] CASFS kernel module update required (6.8.0-1027-gke)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-977</p>
</td>

<td valign="top" width="86%"><p>New Cloud Workstation configuration is created successfully, but user details are not added to the configuration</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-974</p>
</td>

<td valign="top" width="86%"><p>kube-state-metrics Service Account missing causes StatefulSet pod creation failure</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-968</p>
</td>

<td valign="top" width="86%"><p>[IAA] Elektrobit patches remain in PV and break gerrit0</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-966</p>
</td>

<td valign="top" width="86%"><p>[ABFS] Kaniko out of memory</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-953</p>
</td>

<td valign="top" width="86%"><p>Android CF/CTS: update revisions</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-964</p>
</td>

<td valign="top" width="86%"><p>[Gerrit] Propagate seed values</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-959</p>
</td>

<td valign="top" width="86%"><p>Reduce number of GCE CF VMs on startup</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-932</p>
</td>

<td valign="top" width="86%"><p>ABFS_LICENSE_B64 not propagated to k8s secrets correctly</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-958</p>
</td>

<td valign="top" width="86%"><p>[Gerrit] repo sync - ensure we reset local changes before fetch</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-781</p>
</td>

<td valign="top" width="86%"><p>GitHub environment secrets do not update when Terraform workload is executed.</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-933</p>
</td>

<td valign="top" width="86%"><p>Failure to access ABFS artifact repository</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-905</p>
</td>

<td valign="top" width="86%"><p>AAOS build does not work with ABFS</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-931</p>
</td>

<td valign="top" width="86%"><p>Create common storage script</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-930</p>
</td>

<td valign="top" width="86%"><p>Investigate build issues when using MTK Connect as HOST</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-923</p>
</td>

<td valign="top" width="86%"><p>Cuttlefish limited to 10 devices</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-921</p>
</td>

<td valign="top" width="86%"><p>[Cuttlefish] Building android-cuttlefish failing on <a href="http://gnu.org">http://gnu.org</a></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-922</p>
</td>

<td valign="top" width="86%"><p>MTK Connect device creation assumes sequential adb ports</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-920</p>
</td>

<td valign="top" width="86%"><p>Android Developer Build and Test instances leave MTK Connect testbenches in place when aborted</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-563</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Replace gsutils with gcloud storage</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-886</p>
</td>

<td valign="top" width="86%"><p>Conflict Between Role Strategy Plugin and Authorize Project Plugin</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-814</p>
</td>

<td valign="top" width="86%"><p>Android RPi builds failing: requires MESON update</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-863</p>
</td>

<td valign="top" width="86%"><p>Workloads Guide: updates for R2.0.0</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-867</p>
</td>

<td valign="top" width="86%"><p>Gerrit triggers plugin deprecated</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-890</p>
</td>

<td valign="top" width="86%"><p>Persistent Storage Audit: Internal tool removal</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-618</p>
</td>

<td valign="top" width="86%"><p>MTK Connect access control for Cuttlefish Devices</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-711</p>
</td>

<td valign="top" width="86%"><p>[Qwiklabs][Jenkins] GCE limits - VM instances blocked</p>
</td>
</tr>
</tbody>
</table>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 1.1.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>14.04.2025</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p> Minor improvements in Jenkins configuration, additional pipelines implemented for massive build cache pre-warming simplification required for Hackathon and Gerrit post  jobs cleanup.</p>

<h3>New Features</h3>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="12%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Feature</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Description</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-431</p>
</td>

<td valign="top" width="24%"><p>Jenkins R1 deployment extensions</p>
</td>

<td valign="top" width="64%"><p>Jenkins extensions to Platform Foundation deployment in Rel.1.0.0.  The new job to pre-warm build volumes.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-346</p>
</td>

<td valign="top" width="24%"><p>Support Pixel devices</p>
</td>

<td valign="top" width="64%"><p>Support for Google Pixel tablet hardware, full integration with MTK Connect.</p>
</td>
</tr>
</tbody>
</table>

<h3>Improved Features</h3>
<p>N/A</p>

<h3>Bug Fixes</h3>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Issue ID</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Summary</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-683</p>
</td>

<td valign="top" width="86%"><p>Change MTK Connect application version to 1.8.0 in helm chart</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-644</p>
</td>

<td valign="top" width="86%"><p>self-hosted runners</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-641</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Horizon Gerrit URL path breaks upstream Gerrit FETCH</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-639</p>
</td>

<td valign="top" width="86%"><p>Keycloak Sign-in Failure: Non-Admin Users Stuck on Loading Screen</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-631</p>
</td>

<td valign="top" width="86%"><p>MTK Connect license file in wrong location</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-628</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CF instance creation (connection loss)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-627</p>
</td>

<td valign="top" width="86%"><p>[Jenkins][Dev] Investigate build nodes not scaling past 13</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-622</p>
</td>

<td valign="top" width="86%"><p>Workloads documentation - wrong paths</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-615</p>
</td>

<td valign="top" width="86%"><p>Improve the Gerrit post job</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-401</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Agent losing connection to instance</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-309</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] 'Build Now' post restart</p>
</td>
</tr>
</tbody>
</table>
<hr>
<table width="100%">
<tbody>
<tr>
<td valign="top" width="14%"><p><strong>Platform</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Horizon SDV</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Version</strong></p>
</td>

<td valign="top" width="86%"><p><strong>Release 1.0.0</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p><strong>Date</strong></p>
</td>

<td valign="top" width="86%"><p><strong>18.03.2025</strong></p>
</td>
</tr>
</tbody>
</table>

<h2>Summary</h2>
<p>The main objective for Release 1.0.0 is to achieve Minimal Viable Product level for Horizon SDV platform where orchestration will be done using Terraform on GCP with the intention of deploying the tooling on the platform using a simple provisioner. Horizon SDV platform in Rel.1.0.0 supports:</p>

<ul>
<li><p>GCP platform / services.</p>
</li>

<li><p>Terraform orchestration (IaC).</p>
</li>

<li><p>IaC stored in GitHub repo and provisioned either via CLI or GitHub actions.</p>
</li>

<li><p>Platform supports Gerrit to host Android (AAOS) repos and manifests, and allows users to create their own repos.</p>

<ul>
<li><p>With some pre-submit checks: eg. voting labels: code review and manual vs automated triggered builds.</p>
</li>

<li><p>Will mirror and fork AAOSP manifests repo, and one additional code repo for demonstrating the SDV Tooling pipeline. Locally mirrored/forked manifest will be updated to point to the internally mirrored code repo, all other repos will remain using the external OSS AAOS repos hosted by Google.</p>
</li>
</ul>
</li>

<li><p>Platform supports Jenkins to allow for concurrent, multiple builds for iterative builds from changes in open review in Gerrit , full builds (manually, when user requests) and CTS testing.</p>
</li>

<li><p>Platform supports an artefact registry to hold all build artefacts and test results.</p>
</li>

<li><p>Platform supports a means to run CTS tests and use the Accenture MTK Connect solution for UI/Ux testing.</p>
</li>
</ul>

<h3>New Features</h3>

<table width="100%">
<tbody>
<tr>
<td valign="top" width="12%"><p><strong>ID</strong></p>
</td>

<td valign="top" width="24%"><p><strong>Feature</strong></p>
</td>

<td valign="top" width="64%"><p><strong>Description</strong></p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-6</p>
</td>

<td valign="top" width="24%"><p>Platform foundation</p>
</td>

<td valign="top" width="64%"><p>Platform foundation including support for: GCP, Terraform workflow, Stage 1 and Stage 2 deployment with ArgoCD, Jenkins Orchestration and Authentication support through Keycloak.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-12</p>
</td>

<td valign="top" width="24%"><p>Github Setup</p>
</td>

<td valign="top" width="64%"><p>Github support for Horizon SDV platform repositories.</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-67</p>
</td>

<td valign="top" width="24%"><p>Tooling for tooling</p>
</td>

<td valign="top" width="64%"><p>Android build pipelines support</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-5</p>
</td>

<td valign="top" width="24%"><p>Gerrit</p>
</td>

<td valign="top" width="64%"><p>Gerrit support</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-61</p>
</td>

<td valign="top" width="24%"><p>MTK Connect</p>
</td>

<td valign="top" width="64%"><p>Test connections to CVD with MTK Connect support</p>
</td>
</tr>

<tr>
<td valign="top" width="12%"><p>TAA-2</p>
</td>

<td valign="top" width="24%"><p>Android Virtual Devices</p>
</td>

<td valign="top" width="64%"><p>Pipelines for Android Virtual devices CVD and AVD.</p>
</td>
</tr>
</tbody>
</table>

<h3>Improved Features</h3>
<p>N/A</p>

<h3>Bug Fixes</h3>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="14%"><p><strong>Issue ID</strong></p>
</th>

<th valign="top" width="86%"><p><strong>Summary</strong></p>
</th>
</tr>
</tbody>
</table>

<table width="100%">
<tbody>
<tr>
<th valign="top" width="14%"><p><strong>Issue ID</strong></p>
</th>

<th valign="top" width="86%"><p><strong>Summary</strong></p>
</th>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-608</p>
</td>

<td valign="top" width="86%"><p>MTK Connect - testbench registration failing</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-593</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Jenkins config auto reload affecting builds</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-590</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CTS_DOWNLOAD_URL : strip trailing slashes</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-589</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] computeEngine: cuttlefish-vm-v110 points to incorrect instance template</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-577</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CF CVD launcher fails to boot devices</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-562</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Warnings from pipeline (Pipeline Groovy)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-532</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Stage View bug (display pipeline)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-530</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Regression: Exceptions raised on connection/instance loss</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-528</p>
</td>

<td valign="top" width="86%"><p>[MTK Connect] node warnings: MaxListenersExceededWarning</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-520</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Reinstate cuttlefish-vm termination</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-519</p>
</td>

<td valign="top" width="86%"><p>TAA-518[Jenkins] Reinstate MTKC Test bench deletion env pipeline</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-518</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CVD / CTS - hudson exceptions reported and jobs fail</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-516</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Make test jobs more defensive + improvements</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-508</p>
</td>

<td valign="top" width="86%"><p>[MTK Connect] Not terminating</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-507</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CVD/CTS test run : times out on android-14.0.0_r74</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-502</p>
</td>

<td valign="top" width="86%"><p>Re-apply pull-request trigger to GitHub workflows</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-501</p>
</td>

<td valign="top" width="86%"><p>Invent a solution for restricting GitHub workflows to a given branch</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-498</p>
</td>

<td valign="top" width="86%"><p>Gerrit-admin password is not created in Keycloak</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-496</p>
</td>

<td valign="top" width="86%"><p>[Android Studio] Arm builds throw an error due to config</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-490</p>
</td>

<td valign="top" width="86%"><p>[RPi] RPi4 again broken</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-478</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CLEAN_ALL: rsync errors</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-477</p>
</td>

<td valign="top" width="86%"><p>[Gerrit] Branch name revision incorrect for 15 - build failures</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-425</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Native Linux install of MTKC fails (unattended-upgr)</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-412</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] Russian Roulette with cache instance causing build failures</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-400</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] SSH issues</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-398</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] GCE plugin losing connection with VM instance</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-394</p>
</td>

<td valign="top" width="86%"><p>[Gerrit] Admin password stored in secrets with newline</p>
</td>
</tr>

<tr>
<td valign="top" width="14%"><p>TAA-354</p>
</td>

<td valign="top" width="86%"><p>[Jenkins] CVD adb devices not always working as expected</p>
</td>
</tr>
</tbody>
</table>
<hr>
