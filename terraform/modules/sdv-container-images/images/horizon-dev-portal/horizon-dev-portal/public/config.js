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
// Local dev defaults; production is served by the portal binary from env.
window.APP_CONFIG = window.APP_CONFIG || {
  baseUrl: typeof window !== 'undefined' ? window.location.origin : '',
  /** Match vite dev server if you use `npm run dev` with a subpath; `''` when opening the app at `/`. */
  publicPath: '',
  keycloakUrl: '/auth/realms/horizon/protocol/openid-connect/token',
  keycloakClientId: 'horizon-dev-portal',
  horizonApiOAuthClientId: 'horizon-api-ci',
  horizonVersion: 'dev',
};
