// Copyright (c) 2026 Accenture, All Rights Reserved.
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
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest';

type FakeKeycloak = {
  token?: string;
  refreshToken?: string;
  idToken?: string;
  tokenParsed?: Record<string, unknown>;
  authenticated: boolean;
  init: Mock;
  login: Mock;
  logout: Mock;
  updateToken: Mock;
};

const kcMock = vi.hoisted(() => ({
  instances: [] as unknown[],
  ctorArgs: [] as Array<{ url: string; realm: string; clientId: string }>,
}));

vi.mock('keycloak-js', () => {
  class Keycloak {
    token?: string;
    refreshToken?: string;
    idToken?: string;
    tokenParsed?: Record<string, unknown>;
    authenticated = false;
    init = vi.fn(async () => false);
    login = vi.fn();
    logout = vi.fn();
    updateToken = vi.fn(async () => true);

    constructor(cfg: { url: string; realm: string; clientId: string }) {
      kcMock.ctorArgs.push(cfg);
      kcMock.instances.push(this);
    }
  }
  return { default: Keycloak };
});

const DEFAULT_CONFIG = {
  keycloakUrl: 'https://kc.example/auth/realms/acme/protocol/openid-connect/token',
  keycloakClientId: 'portal-x',
  publicPath: '/dev',
};

const assign = vi.fn();
const storage = new Map<string, string>();
const cookieWrites: string[] = [];
const fetchMock = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>();

function stubBrowser(appConfig: Record<string, unknown>): void {
  vi.stubGlobal('window', {
    location: {
      origin: 'https://portal.example',
      protocol: 'https:',
      search: '',
      hash: '',
      assign,
    },
    APP_CONFIG: appConfig,
  });
  vi.stubGlobal('document', {
    get cookie() {
      return '';
    },
    set cookie(value: string) {
      cookieWrites.push(value);
    },
  });
  vi.stubGlobal('sessionStorage', {
    getItem: (k: string) => storage.get(k) ?? null,
    setItem: (k: string, v: string) => {
      storage.set(k, v);
    },
    removeItem: (k: string) => {
      storage.delete(k);
    },
  });
}

function jwt(payload: Record<string, unknown>): string {
  const body = btoa(JSON.stringify(payload))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
  return `header.${body}.sig`;
}

function nowSec(): number {
  return Math.floor(Date.now() / 1000);
}

/** Re-evaluates auth.ts so each test gets a fresh singleton with a fresh Keycloak fake. */
async function loadAuth() {
  vi.resetModules();
  const mod = await import('./auth');
  return mod.authService;
}

function currentKc(): FakeKeycloak {
  return kcMock.instances[kcMock.instances.length - 1] as FakeKeycloak;
}

function lastCookie(): string {
  return cookieWrites[cookieWrites.length - 1];
}

describe('authService', () => {
  beforeEach(() => {
    kcMock.instances.length = 0;
    kcMock.ctorArgs.length = 0;
    storage.clear();
    cookieWrites.length = 0;
    assign.mockReset();
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    stubBrowser(DEFAULT_CONFIG);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  describe('construction', () => {
    it('builds the Keycloak client from the configured URL, realm, and client id', async () => {
      await loadAuth();
      expect(kcMock.ctorArgs[kcMock.ctorArgs.length - 1]).toEqual({
        url: 'https://kc.example/auth',
        realm: 'acme',
        clientId: 'portal-x',
      });
    });

    it('falls back to the default Keycloak URL and client id', async () => {
      stubBrowser({ publicPath: '/dev' });
      await loadAuth();
      expect(kcMock.ctorArgs[kcMock.ctorArgs.length - 1]).toEqual({
        url: 'https://portal.example/auth',
        realm: 'horizon',
        clientId: 'horizon-dev-portal',
      });
    });
  });

  describe('init', () => {
    it('stores tokens and mirrors the access token into the proxy cookie', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      const token = jwt({ exp: nowSec() + 600, preferred_username: 'alice' });
      kc.init.mockImplementation(async () => {
        kc.token = token;
        kc.refreshToken = 'refresh-1';
        kc.idToken = 'id-1';
        kc.authenticated = true;
        return true;
      });

      await expect(auth.init()).resolves.toBe(true);

      expect(storage.get('kc-devportal-token')).toBe(token);
      expect(storage.get('kc-devportal-refresh')).toBe('refresh-1');
      expect(storage.get('kc-devportal-id-token')).toBe('id-1');
      expect(lastCookie()).toContain(`horizon_portal_access=${encodeURIComponent(token)}`);
      expect(lastCookie()).toContain('Path=/dev/api/horizon');
      expect(lastCookie()).toContain('SameSite=Strict; Secure');
    });

    it('clears the stored session when Keycloak reports no login', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      storage.set('kc-devportal-token', 'stale');
      kc.init.mockResolvedValue(false);

      await expect(auth.init()).resolves.toBe(false);

      expect(storage.has('kc-devportal-token')).toBe(false);
      expect(lastCookie()).toContain('Max-Age=0');
    });

    it('treats a failing Keycloak init as signed out', async () => {
      const auth = await loadAuth();
      storage.set('kc-devportal-refresh', 'stale');
      currentKc().init.mockRejectedValue(new Error('network down'));

      await expect(auth.init()).resolves.toBe(false);
      expect(storage.has('kc-devportal-refresh')).toBe(false);
    });

    it('runs Keycloak init once for concurrent callers', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      kc.init.mockResolvedValue(false);

      await Promise.all([auth.init(), auth.init()]);
      expect(kc.init).toHaveBeenCalledTimes(1);

      await expect(auth.init()).resolves.toBe(false);
      expect(kc.init).toHaveBeenCalledTimes(1);
    });

    it('passes a stored session to Keycloak when there is no login callback', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      storage.set('kc-devportal-token', 'access-stored');
      storage.set('kc-devportal-refresh', 'refresh-stored');
      storage.set('kc-devportal-id-token', 'id-stored');
      kc.init.mockResolvedValue(false);

      await auth.init();

      expect(kc.init.mock.calls[0][0]).toMatchObject({
        token: 'access-stored',
        refreshToken: 'refresh-stored',
        idToken: 'id-stored',
      });
    });
  });

  describe('claims', () => {
    it('reads the username from preferred_username, then name, then sub', async () => {
      const auth = await loadAuth();
      const kc = currentKc();

      kc.tokenParsed = { preferred_username: 'alice', name: 'Alice' };
      expect(auth.getUsername()).toBe('alice');

      kc.tokenParsed = { name: 'Alice', sub: 's-1' };
      expect(auth.getUsername()).toBe('Alice');

      kc.tokenParsed = { sub: 's-1' };
      expect(auth.getUsername()).toBe('s-1');

      kc.tokenParsed = undefined;
      expect(auth.getUsername()).toBeNull();
    });

    it('resolves the OAuth client id from azp, aud, or the configured client', async () => {
      const auth = await loadAuth();
      const kc = currentKc();

      kc.tokenParsed = { azp: 'azp-client' };
      expect(auth.getOAuthClientId()).toBe('azp-client');

      kc.tokenParsed = { aud: 'aud-client' };
      expect(auth.getOAuthClientId()).toBe('aud-client');

      kc.tokenParsed = { aud: ['first', 'second'] };
      expect(auth.getOAuthClientId()).toBe('first');

      kc.tokenParsed = {};
      expect(auth.getOAuthClientId()).toBe('portal-x');
    });

    it('exposes the token and its claims', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      kc.token = 'access-1';
      kc.tokenParsed = { a: 1 };

      expect(auth.getToken()).toBe('access-1');
      expect(auth.getTokenClaims()).toEqual({ a: 1 });

      kc.tokenParsed = undefined;
      expect(auth.getTokenClaims()).toEqual({});
    });
  });

  describe('token refresh', () => {
    it('refreshes a near-expiry token through Keycloak and persists it', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      kc.token = 'old';
      kc.updateToken.mockImplementation(async () => {
        kc.token = 'new';
        return true;
      });

      await auth.ensureFreshToken();

      expect(kc.updateToken).toHaveBeenCalledWith(70);
      expect(storage.get('kc-devportal-token')).toBe('new');
    });

    it('falls back to a direct refresh_token grant when the adapter cannot refresh', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      kc.token = 'old';
      kc.refreshToken = 'refresh-1';
      kc.updateToken.mockRejectedValue(new Error('adapter failed'));
      const refreshed = jwt({ sub: 'user-1', iat: nowSec(), exp: nowSec() + 300 });
      fetchMock.mockResolvedValue(
        new Response(JSON.stringify({ access_token: refreshed, refresh_token: 'refresh-2' }), {
          status: 200,
        })
      );

      await auth.ensureFreshToken();

      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(fetchMock.mock.calls[0][0]).toBe(
        'https://kc.example/auth/realms/acme/protocol/openid-connect/token'
      );
      expect(storage.get('kc-devportal-token')).toBe(refreshed);
      expect(storage.get('kc-devportal-refresh')).toBe('refresh-2');
      expect(kc.token).toBe(refreshed);
    });

    it('returns false from a best-effort refresh when there is no refresh token', async () => {
      const auth = await loadAuth();
      currentKc().refreshToken = undefined;

      await expect(auth.refreshAccessTokenBestEffort()).resolves.toBe(false);
      expect(fetchMock).not.toHaveBeenCalled();
    });
  });

  describe('sign-in and sign-out', () => {
    it('login and logout use the portal redirect URI', async () => {
      const auth = await loadAuth();
      const kc = currentKc();
      storage.set('kc-devportal-token', 'access-1');

      auth.login();
      expect(kc.login).toHaveBeenCalledWith({ redirectUri: 'https://portal.example/dev/' });

      auth.logout();
      expect(storage.has('kc-devportal-token')).toBe(false);
      expect(kc.logout).toHaveBeenCalledWith({ redirectUri: 'https://portal.example/dev/' });
    });

    it('loginWithPassword stores the returned tokens and returns to the portal', async () => {
      const auth = await loadAuth();
      fetchMock.mockResolvedValue(
        new Response(
          JSON.stringify({
            access_token: 'pw-access',
            refresh_token: 'pw-refresh',
            id_token: 'pw-id',
          }),
          { status: 200 }
        )
      );

      await auth.loginWithPassword('alice', 'secret');

      const body = fetchMock.mock.calls[0][1]?.body as URLSearchParams;
      expect(body.get('grant_type')).toBe('password');
      expect(body.get('username')).toBe('alice');
      expect(storage.get('kc-devportal-token')).toBe('pw-access');
      expect(storage.get('kc-devportal-refresh')).toBe('pw-refresh');
      expect(storage.get('kc-devportal-id-token')).toBe('pw-id');
      expect(assign).toHaveBeenCalledWith('https://portal.example/dev/');
    });

    it('loginWithPassword surfaces the Keycloak error description or a generic message', async () => {
      const auth = await loadAuth();

      fetchMock.mockResolvedValueOnce(
        new Response(JSON.stringify({ error_description: 'Invalid user credentials' }), {
          status: 401,
        })
      );
      await expect(auth.loginWithPassword('alice', 'bad')).rejects.toThrow(
        'Invalid user credentials'
      );

      fetchMock.mockResolvedValueOnce(new Response('not json', { status: 500 }));
      await expect(auth.loginWithPassword('alice', 'bad')).rejects.toThrow('Login failed (500)');
    });

    it('sessionExpiredRedirectToLogin clears the session and sends the user to login', async () => {
      const auth = await loadAuth();
      storage.set('kc-devportal-token', 'expired');

      auth.sessionExpiredRedirectToLogin();

      expect(storage.has('kc-devportal-token')).toBe(false);
      expect(assign).toHaveBeenCalledWith('https://portal.example/dev/login');
    });
  });
});
