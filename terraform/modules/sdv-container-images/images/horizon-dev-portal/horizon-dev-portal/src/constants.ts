// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
import vehicleLogoUrl from './assets/Vehicle.svg?url';
import accentureLogoUrl from './assets/accenture-logo.png';
import argocdLogoUrl from './assets/argo-cd-logo.png';
import argoworkflowsLogoUrl from './assets/argo-workflows-logo.png';
import gerritLogoUrl from './assets/gerrit-logo.png';
import grafanaLogoUrl from './assets/grafana-logo.png';
import headlampDarkLogoUrl from './assets/headlamp-icon-dark.svg?url';
import jenkinsLogoUrl from './assets/jenkins-logo.png';
import keycloakLogoUrl from './assets/keycloak-logo.png';
import mcpGatewayRegistryLogoUrl from './assets/mcp-gateway-registry-logo.png';

export const HORIZON_LOGO_SRC = vehicleLogoUrl;
export const ACCENTURE_LOGO_SRC = accentureLogoUrl;
export const ARGO_CD_LOGO_SRC = argocdLogoUrl;
export const ARGO_WORKFLOWS_LOGO_SRC = argoworkflowsLogoUrl;
export const GERRIT_LOGO_SRC = gerritLogoUrl;
export const GRAFANA_LOGO_SRC = grafanaLogoUrl;
export const HEADLAMP_DARK_LOGO_SRC = headlampDarkLogoUrl;
export const JENKINS_LOGO_SRC = jenkinsLogoUrl;
export const KEYCLOAK_LOGO_SRC = keycloakLogoUrl;
export const MCP_GATEWAY_REGISTRY_LOGO_SRC = mcpGatewayRegistryLogoUrl;

/** Fired after Administration → Modules changes module state so the shell sidebar re-fetches ready modules. */
export const READY_MODULES_REFRESH_EVENT = 'horizon-dev-portal:refresh-ready-modules';
