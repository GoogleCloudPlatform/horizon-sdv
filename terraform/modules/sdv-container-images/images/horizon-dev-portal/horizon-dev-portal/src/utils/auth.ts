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
import Keycloak from 'keycloak-js';
import type { KeycloakTokenParsed } from 'keycloak-js';
import { config } from './config';
import { getRouterBasename } from './publicPath';

const PORTAL_ACCESS_COOKIE = 'horizon_portal_access';

const SK_TOKEN = 'kc-devportal-token';
const SK_REFRESH = 'kc-devportal-refresh';
const SK_ID = 'kc-devportal-id-token';

function parseKeycloakConfig(): { url: string; realm: string } {
  const keycloakUrl = config.getString(
    'keycloakUrl',
    '/auth/realms/horizon/protocol/openid-connect/token'
  );
  const absolute = keycloakUrl.startsWith('/')
    ? new URL(keycloakUrl, window.location.origin).href
    : keycloakUrl;
  const realmMatch = absolute.match(/\/realms\/([^/]+)/);
  const realm = realmMatch ? realmMatch[1] : 'horizon';
  const url = absolute.split('/realms/')[0];
  return { url, realm };
}

function redirectUri(): string {
  const base = getRouterBasename();
  return `${window.location.origin}${base}/`;
}

function tokenEndpoint(): string {
  const { url, realm } = parseKeycloakConfig();
  return `${url.replace(/\/$/, '')}/realms/${realm}/protocol/openid-connect/token`;
}

function loginPath(): string {
  const base = getRouterBasename();
  return `${base}/login`;
}

/**
 * Mirrors the user access token into a cookie scoped to the Horizon proxy so browser
 * navigations (embedded Swagger) present the same Keycloak token as fetch() calls.
 */
function syncPortalAccessCookie(token: string | undefined): void {
  const secure = window.location.protocol === 'https:' ? '; Secure' : '';
  const path = `${getRouterBasename()}/api/horizon`;
  const clear = `${PORTAL_ACCESS_COOKIE}=; Path=${path}; Max-Age=0; SameSite=Strict${secure}`;
  if (!token || token.length > 3800) {
    document.cookie = clear;
    return;
  }
  const parsed = decodeJwtPayload(token);
  const exp = typeof parsed?.exp === 'number' ? parsed.exp : 0;
  const maxAge = exp > 0 ? Math.max(30, exp - Math.floor(Date.now() / 1000)) : 300;
  document.cookie = `${PORTAL_ACCESS_COOKIE}=${encodeURIComponent(token)}; Path=${path}; Max-Age=${maxAge}; SameSite=Strict${secure}`;
}

/** Decode JWT payload (no signature verification — display/session use only). */
function decodeJwtPayload(token: string): KeycloakTokenParsed | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) {
      return null;
    }
    let b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    while (b64.length % 4) {
      b64 += '=';
    }
    const json = atob(b64);
    return JSON.parse(json) as KeycloakTokenParsed;
  } catch {
    return null;
  }
}

class AuthService {
  private static instance: AuthService;
  private keycloak: Keycloak;
  private initInFlight: Promise<boolean> | null = null;
  private keycloakInitFinished = false;

  private constructor() {
    const { url, realm } = parseKeycloakConfig();
    const clientId = config.getString('keycloakClientId', 'horizon-dev-portal');
    this.keycloak = new Keycloak({ url, realm, clientId });
  }

  static getInstance(): AuthService {
    if (!AuthService.instance) {
      AuthService.instance = new AuthService();
    }
    return AuthService.instance;
  }

  private clearStoredSession(): void {
    sessionStorage.removeItem(SK_TOKEN);
    sessionStorage.removeItem(SK_REFRESH);
    sessionStorage.removeItem(SK_ID);
    syncPortalAccessCookie(undefined);
  }

  async init(): Promise<boolean> {
    if (this.initInFlight !== null) {
      return this.initInFlight;
    }
    if (this.keycloakInitFinished) {
      return this.keycloak.authenticated === true && !!this.keycloak.token;
    }
    this.initInFlight = (async () => {
      try {
        return await this.runInit();
      } finally {
        this.keycloakInitFinished = true;
        this.initInFlight = null;
      }
    })();
    return this.initInFlight;
  }

  private async runInit(): Promise<boolean> {
    const hasCallback = /[?&#](code|error)=/.test(window.location.search + window.location.hash);
    const storedToken = sessionStorage.getItem(SK_TOKEN) ?? undefined;
    const storedRefresh = sessionStorage.getItem(SK_REFRESH) ?? undefined;
    const storedId = sessionStorage.getItem(SK_ID) ?? undefined;

    const initOptions: Parameters<typeof this.keycloak.init>[0] = {
      pkceMethod: 'S256',
      checkLoginIframe: false,
      redirectUri: redirectUri(),
      responseMode: 'query',
    };

    if (!hasCallback && storedToken && storedRefresh) {
      initOptions.token = storedToken;
      initOptions.refreshToken = storedRefresh;
      initOptions.idToken = storedId;
    }

    let authenticated: boolean;
    try {
      authenticated = await this.keycloak.init(initOptions);
    } catch {
      this.clearStoredSession();
      return false;
    }

    if (authenticated && this.keycloak.token) {
      sessionStorage.setItem(SK_TOKEN, this.keycloak.token);
      if (this.keycloak.refreshToken) {
        sessionStorage.setItem(SK_REFRESH, this.keycloak.refreshToken);
      }
      if (this.keycloak.idToken) {
        sessionStorage.setItem(SK_ID, this.keycloak.idToken);
      }
      syncPortalAccessCookie(this.keycloak.token);
    } else if (!hasCallback) {
      this.clearStoredSession();
    }

    return authenticated;
  }

  login(): void {
    this.keycloak.login({ redirectUri: redirectUri() });
  }

  logout(): void {
    this.clearStoredSession();
    this.keycloak.logout({ redirectUri: redirectUri() });
  }

  getToken(): string | undefined {
    return this.keycloak.token;
  }

  private persistTokensFromKeycloak(): void {
    if (!this.keycloak.token) {
      return;
    }
    sessionStorage.setItem(SK_TOKEN, this.keycloak.token);
    if (this.keycloak.refreshToken) {
      sessionStorage.setItem(SK_REFRESH, this.keycloak.refreshToken);
    }
    if (this.keycloak.idToken) {
      sessionStorage.setItem(SK_ID, this.keycloak.idToken);
    }
    syncPortalAccessCookie(this.keycloak.token);
  }

  /**
   * Pushes tokens into the Keycloak adapter after a manual refresh_token grant.
   */
  private applyTokensFromRefreshResponse(data: {
    access_token: string;
    refresh_token?: string;
    id_token?: string;
  }): boolean {
    const access = data.access_token;
    const parsed = decodeJwtPayload(access);
    if (!parsed) {
      return false;
    }
    const kc = this.keycloak as Keycloak & {
      token?: string;
      refreshToken?: string;
      idToken?: string;
      tokenParsed?: KeycloakTokenParsed;
      refreshTokenParsed?: KeycloakTokenParsed;
      idTokenParsed?: KeycloakTokenParsed;
      authenticated?: boolean;
      subject?: string;
      sessionId?: string;
      realmAccess?: KeycloakTokenParsed['realm_access'];
      resourceAccess?: KeycloakTokenParsed['resource_access'];
      timeSkew?: number;
    };
    kc.token = access;
    kc.tokenParsed = parsed;
    kc.authenticated = true;
    kc.subject = typeof parsed.sub === 'string' ? parsed.sub : undefined;
    kc.sessionId = typeof parsed.sid === 'string' ? parsed.sid : undefined;
    kc.realmAccess = parsed.realm_access;
    kc.resourceAccess = parsed.resource_access;
    if (typeof parsed.iat === 'number') {
      kc.timeSkew = Math.floor(Date.now() / 1000) - parsed.iat;
    }
    if (data.refresh_token) {
      kc.refreshToken = data.refresh_token;
      kc.refreshTokenParsed = decodeJwtPayload(data.refresh_token) ?? undefined;
    }
    if (data.id_token) {
      kc.idToken = data.id_token;
      kc.idTokenParsed = decodeJwtPayload(data.id_token) ?? undefined;
    }
    this.persistTokensFromKeycloak();
    return true;
  }

  /**
   * Keycloak-js refresh, then direct refresh_token POST if the adapter fails (network/CORS quirks).
   */
  async refreshAccessTokenBestEffort(): Promise<boolean> {
    const rtStored = sessionStorage.getItem(SK_REFRESH);
    if (!this.keycloak.refreshToken && rtStored) {
      const kc = this.keycloak as Keycloak & { refreshToken?: string };
      kc.refreshToken = rtStored;
    }
    const rt = this.keycloak.refreshToken ?? rtStored;
    if (!rt) {
      return false;
    }
    try {
      await this.keycloak.updateToken(-1);
      this.persistTokensFromKeycloak();
      return !!this.keycloak.token;
    } catch {
      // fall through to manual grant
    }
    const clientId = config.getString('keycloakClientId', 'horizon-dev-portal');
    try {
      const resp = await fetch(tokenEndpoint(), {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({
          grant_type: 'refresh_token',
          refresh_token: rt,
          client_id: clientId,
        }),
      });
      if (!resp.ok) {
        return false;
      }
      const data = (await resp.json()) as {
        access_token: string;
        refresh_token?: string;
        id_token?: string;
      };
      if (!data.access_token) {
        return false;
      }
      this.applyTokensFromRefreshResponse(data);
      return !!this.keycloak.token;
    } catch {
      return false;
    }
  }

  /**
   * Refreshes the access token via Keycloak if it expires within minValiditySeconds.
   * Call before Horizon / Module Manager API requests so long sessions keep working.
   */
  async ensureFreshToken(minValiditySeconds = 70): Promise<void> {
    if (!this.keycloak.token && sessionStorage.getItem(SK_TOKEN)) {
      this.applyTokensFromRefreshResponse({
        access_token: sessionStorage.getItem(SK_TOKEN)!,
        refresh_token: sessionStorage.getItem(SK_REFRESH) ?? undefined,
        id_token: sessionStorage.getItem(SK_ID) ?? undefined,
      }); // rehydrate adapter if init left tokens only in sessionStorage
    }
    if (!this.keycloak.token) {
      return;
    }
    try {
      await this.keycloak.updateToken(minValiditySeconds);
      this.persistTokensFromKeycloak();
    } catch {
      await this.refreshAccessTokenBestEffort();
    }
  }

  /**
   * Session is no longer valid for API calls — clear storage and send user to login.
   */
  sessionExpiredRedirectToLogin(): void {
    this.clearStoredSession();
    window.location.assign(`${window.location.origin}${loginPath()}`);
  }

  getUsername(): string | null {
    const tp = this.keycloak.tokenParsed as Record<string, string> | undefined;
    return tp?.preferred_username ?? tp?.name ?? tp?.sub ?? null;
  }

  /** Parsed access-token claims for the portal access mock. Display and role resolution only. */
  getTokenClaims(): Record<string, unknown> {
    return (this.keycloak.tokenParsed ?? {}) as Record<string, unknown>;
  }

  /**
   * OAuth/OIDC client id for the current access token (`azp`), or configured public client id as fallback.
   * Used for display in the Developer Portal only (not sent to Horizon API).
   */
  getOAuthClientId(): string | null {
    const tp = this.keycloak.tokenParsed as { azp?: string; aud?: string | string[] } | undefined;
    if (tp?.azp && typeof tp.azp === 'string') {
      return tp.azp;
    }
    if (typeof tp?.aud === 'string') {
      return tp.aud;
    }
    if (Array.isArray(tp?.aud) && typeof tp.aud[0] === 'string') {
      return tp.aud[0];
    }
    return config.getString('keycloakClientId', 'horizon-dev-portal');
  }

  /**
   * Resource-owner password grant (requires Direct Access Grants on the client).
   */
  async loginWithPassword(username: string, password: string): Promise<void> {
    const clientId = config.getString('keycloakClientId', 'horizon-dev-portal');
    const body = new URLSearchParams({
      grant_type: 'password',
      client_id: clientId,
      username,
      password,
      scope: 'openid',
    });
    const resp = await fetch(tokenEndpoint(), {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
    });
    if (!resp.ok) {
      const err = await resp.json().catch(() => ({}));
      throw new Error(
        (err as { error_description?: string }).error_description || `Login failed (${resp.status})`
      );
    }
    const data = (await resp.json()) as {
      access_token: string;
      refresh_token?: string;
      id_token?: string;
    };
    sessionStorage.setItem(SK_TOKEN, data.access_token);
    if (data.refresh_token) {
      sessionStorage.setItem(SK_REFRESH, data.refresh_token);
    }
    if (data.id_token) {
      sessionStorage.setItem(SK_ID, data.id_token);
    }
    window.location.assign(redirectUri());
  }
}

export const authService = AuthService.getInstance();
