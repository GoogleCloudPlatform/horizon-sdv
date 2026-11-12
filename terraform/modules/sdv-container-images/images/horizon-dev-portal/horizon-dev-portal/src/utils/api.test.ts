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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiCli, apiHorizon, apiMm, readCliJson } from './api';

const authMock = vi.hoisted(() => ({
  getToken: vi.fn<() => string | undefined>(),
  ensureFreshToken: vi.fn<() => Promise<void>>(),
  refreshAccessTokenBestEffort: vi.fn<() => Promise<boolean>>(),
  sessionExpiredRedirectToLogin: vi.fn<() => void>(),
}));

vi.mock('./auth', () => ({
  authService: authMock,
}));

vi.mock('./publicPath', () => ({
  getAppOriginBase: () => 'https://portal.example',
}));

const fetchMock = vi.fn<(url: string, init?: RequestInit) => Promise<Response>>();

function headersOf(call: number): Record<string, string> {
  return (fetchMock.mock.calls[call][1]?.headers ?? {}) as Record<string, string>;
}

describe('api helpers', () => {
  beforeEach(() => {
    authMock.getToken.mockReset().mockReturnValue('user-token');
    authMock.ensureFreshToken.mockReset().mockResolvedValue(undefined);
    authMock.refreshAccessTokenBestEffort.mockReset().mockResolvedValue(true);
    authMock.sessionExpiredRedirectToLogin.mockReset();
    fetchMock.mockReset().mockResolvedValue(new Response('{}', { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('builds the proxy URLs for each API surface', async () => {
    await apiMm('/modules');
    await apiCli('/v1/manifest');
    await apiHorizon('v1/workflows');

    expect(fetchMock.mock.calls[0][0]).toBe('https://portal.example/api/mm/modules');
    expect(fetchMock.mock.calls[1][0]).toBe('https://portal.example/api/cli/v1/manifest');
    expect(fetchMock.mock.calls[2][0]).toBe('https://portal.example/api/horizon/v1/workflows');
  });

  it('sends the user token, disables caching, and refreshes the token first', async () => {
    await apiHorizon('/v1/catalog');

    expect(authMock.ensureFreshToken).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][1]?.cache).toBe('no-store');
    expect(headersOf(0).Authorization).toBe('Bearer user-token');
  });

  it('omits the Authorization header when there is no token', async () => {
    authMock.getToken.mockReturnValue(undefined);
    await apiMm('/modules');
    expect(headersOf(0).Authorization).toBeUndefined();
  });

  it('retries a 401 after refreshing the token', async () => {
    fetchMock
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const resp = await apiHorizon('/v1/catalog');

    expect(resp.status).toBe(200);
    expect(authMock.refreshAccessTokenBestEffort).toHaveBeenCalledTimes(1);
    expect(authMock.sessionExpiredRedirectToLogin).not.toHaveBeenCalled();
  });

  it('retries a repeated 401 twice and then succeeds', async () => {
    fetchMock
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 }));

    const resp = await apiHorizon('/v1/catalog');

    expect(resp.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(authMock.refreshAccessTokenBestEffort).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[2][1]?.cache).toBe('no-store');
  });

  it('ends the session when 401 persists after both retries', async () => {
    fetchMock.mockResolvedValue(new Response(null, { status: 401 }));

    const resp = await apiMm('/modules');

    expect(resp.status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(authMock.refreshAccessTokenBestEffort).toHaveBeenCalledTimes(2);
    expect(authMock.sessionExpiredRedirectToLogin).toHaveBeenCalledTimes(1);
    expect(authMock.ensureFreshToken).toHaveBeenCalledTimes(1);
  });
});

describe('readCliJson', () => {
  it('parses a JSON response', async () => {
    const resp = new Response(JSON.stringify({ version: '4.3.0' }), {
      headers: { 'Content-Type': 'application/json' },
    });
    await expect(readCliJson<{ version: string }>(resp)).resolves.toEqual({ version: '4.3.0' });
  });

  it('explains when the dev server returned HTML instead of JSON', async () => {
    const resp = new Response('<!doctype html><html></html>', {
      status: 200,
      headers: { 'Content-Type': 'text/html' },
    });
    await expect(readCliJson(resp)).rejects.toThrow(
      'CLI API returned HTML instead of JSON (HTTP 200)'
    );
  });

  it('reports plain-text errors and empty bodies', async () => {
    await expect(
      readCliJson(new Response('boom', { status: 500, headers: { 'Content-Type': 'text/plain' } }))
    ).rejects.toThrow('CLI API returned non-JSON (HTTP 500): boom');

    await expect(
      readCliJson(new Response('', { status: 502, headers: { 'Content-Type': 'text/plain' } }))
    ).rejects.toThrow('CLI API returned non-JSON (HTTP 502): empty body');
  });
});
