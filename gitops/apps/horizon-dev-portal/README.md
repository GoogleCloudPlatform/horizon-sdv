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

# Horizon Developer Portal (GitOps)

Child chart deployed by [`gitops/templates/horizon-dev-portal.yaml`](../templates/horizon-dev-portal.yaml).

- **Image**: built from [`terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal`](../../../terraform/modules/sdv-container-images/images/horizon-dev-portal/horizon-dev-portal) (Vite SPA + Go proxy). The parent Application sets the container image from [`gitops/values.yaml`](../../values.yaml) `config.containerImages.horizondevelopmentportal` (same pattern as other workloads; Terraform supplies the full ref per environment).
- **Public path**: `config.publicPath` in this chart (default `/developer-portal`). The parent passes values from `config.horizonDevelopmentPortal.publicPath` so the same value drives the Gateway HTTPRoute, Keycloak redirect URIs, and `PUBLIC_PATH` in the portal ConfigMap. Changing a live cluster may require updating Keycloak redirect URIs for an existing client.
- **Keycloak**: set in **this** chart’s `values.yaml` under `config`, or via parent `config.horizonDevelopmentPortal`: `keycloakRealm`, `keycloakHttpPathPrefix`, `keycloakClientId`, `keycloakTokenPath`, plus the full `oidcIssuerUrl` when not using the parent Application (the parent template derives it from domain + realm + path).
- **Horizon API access**: the Go proxy forwards the signed-in user's own access token on `/api/horizon`. It first checks the token as Horizon API does (signature, issuer, expiry, access token only) and requires `horizonApiClientId` (default `horizon-api`, `HORIZON_API_CLIENT_ID` in the ConfigMap) in its `aud` claim. Keep it equal to Horizon API's `oidcClientId`. The proxy holds no client secret.
- **In-cluster dependencies**: `moduleManagerBaseUrl` and `horizonApiBaseUrl` default to `http://module-manager.{prefix}module-manager...` and `http://horizon-api.{prefix}horizon-api...`. Override via parent [`gitops/values.yaml`](../../values.yaml) `config.horizonDevelopmentPortal.moduleManagerInternalBaseUrl` / `horizonApiInternalBaseUrl` when Service names differ.

```bash
helm lint .
```
