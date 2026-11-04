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
import { authService } from './auth';
import { getAppOriginBase } from './publicPath';

function authHeaders(): HeadersInit {
  const t = authService.getToken();
  if (!t) {
    return {};
  }
  return { Authorization: `Bearer ${t}` };
}

async function fetchWithAuth(url: string, init: RequestInit): Promise<Response> {
  await authService.ensureFreshToken();

  const doFetch = () =>
    fetch(url, {
      ...init,
      cache: 'no-store',
      headers: {
        ...authHeaders(),
        ...init.headers,
      },
    });

  let resp = await doFetch();

  // 401 is a stale or rejected user JWT. Refresh and retry; the proxy forwards this token to Horizon API.
  for (let attempt = 0; resp.status === 401 && attempt < 2; attempt++) {
    await authService.refreshAccessTokenBestEffort();
    resp = await doFetch();
  }

  if (resp.status === 401) {
    authService.sessionExpiredRedirectToLogin();
  }

  return resp;
}

/** Module Manager via proxy (user JWT required). */
export async function apiMm(path: string, init: RequestInit = {}): Promise<Response> {
  const originBase = getAppOriginBase();
  const url = `${originBase}/api/mm${path.startsWith('/') ? path : `/${path}`}`;
  return fetchWithAuth(url, init);
}

/** Horizon CLI download API via proxy (user JWT required, same as apiMm). */
export async function apiCli(path: string, init: RequestInit = {}): Promise<Response> {
  const originBase = getAppOriginBase();
  const url = `${originBase}/api/cli${path.startsWith('/') ? path : `/${path}`}`;
  return fetchWithAuth(url, init);
}

/**
 * Parse a CLI API JSON body. Vite's SPA fallback can return HTML with HTTP 200 when
 * `/api/cli` is not handled; that used to surface as `Unexpected token '<'`.
 */
export async function readCliJson<T>(resp: Response): Promise<T> {
  const ct = (resp.headers.get('content-type') ?? '').toLowerCase();
  if (!ct.includes('json')) {
    const preview = (await resp.text()).trim().slice(0, 160);
    if (preview.startsWith('<')) {
      throw new Error(
        `CLI API returned HTML instead of JSON (HTTP ${resp.status}). For npm run dev, from tools/horizon run:
         go run mkdist.go %TEMP%\\horizon-cli-dist <horizon_version>, e.g.: go run mkdist.go %TEMP%\\horizon-cli-dist 4.3.0`
      );
    }
    throw new Error(`CLI API returned non-JSON (HTTP ${resp.status}): ${preview || 'empty body'}`);
  }
  return resp.json() as Promise<T>;
}

/**
 * Horizon API via the same-origin proxy. The caller’s Keycloak access token is forwarded unchanged
 * (no horizon-api-ci client-credentials swap).
 */
export async function apiHorizon(path: string, init: RequestInit = {}): Promise<Response> {
  const originBase = getAppOriginBase();
  const url = `${originBase}/api/horizon${path.startsWith('/') ? path : `/${path}`}`;
  return fetchWithAuth(url, init);
}
