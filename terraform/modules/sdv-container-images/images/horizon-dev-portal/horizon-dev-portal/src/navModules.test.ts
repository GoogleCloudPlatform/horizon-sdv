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
import { apiMm } from './utils/api';
import { fetchNavModuleNameIfRelevant, fetchNavModuleNames } from './navModules';
import type { ModuleResponse, StatusResponse } from './types';

vi.mock('./utils/api', () => ({
  apiMm: vi.fn(),
}));

const apiMmMock = vi.mocked(apiMm);

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

const READY: StatusResponse = {
  syncStatus: 'Synced',
  healthStatus: 'Healthy',
  managedApplicationCount: 1,
} as StatusResponse;

const PENDING: StatusResponse = { managedApplicationCount: 0 } as StatusResponse;

const DELETING: StatusResponse = {
  applicationDeletionTimestamp: '2026-01-01T00:00:00Z',
} as StatusResponse;

const MODULES = [
  { name: 'zeta', enabled: true },
  { name: 'alpha', enabled: true },
  { name: 'gamma', enabled: true },
  { name: 'beta', enabled: false },
] as ModuleResponse[];

const STATUS_BY_MODULE: Record<string, StatusResponse> = {
  zeta: READY,
  alpha: PENDING,
  gamma: DELETING,
  beta: { remainingManagedApplications: 0 } as StatusResponse,
};

describe('navModules', () => {
  beforeEach(() => {
    apiMmMock.mockReset();
    apiMmMock.mockImplementation(async (path: string) => {
      if (path === '/modules') {
        return json(MODULES);
      }
      const match = /^\/modules\/([^/]+)\/status$/.exec(path);
      const name = match ? decodeURIComponent(match[1]) : '';
      return json(STATUS_BY_MODULE[name] ?? {});
    });
    vi.stubGlobal('window', {
      setTimeout: (fn: () => void, ms: number) => globalThis.setTimeout(fn, ms),
      clearTimeout: (id: ReturnType<typeof setTimeout>) => globalThis.clearTimeout(id),
    });
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('lists only READY or tearing-down modules, sorted by name', async () => {
    await expect(fetchNavModuleNames()).resolves.toEqual(['gamma', 'zeta']);
  });

  it('returns an empty list when the module list request fails', async () => {
    apiMmMock.mockResolvedValueOnce(new Response('down', { status: 500 }));
    await expect(fetchNavModuleNames()).resolves.toEqual([]);
  });

  it('keeps a module only when it is READY or UNINSTALL IN PROGRESS', async () => {
    await expect(fetchNavModuleNameIfRelevant(MODULES[0])).resolves.toBe('zeta');
    await expect(fetchNavModuleNameIfRelevant(MODULES[1])).resolves.toBeNull();
    await expect(fetchNavModuleNameIfRelevant(MODULES[2])).resolves.toBe('gamma');
  });

  it('drops a module whose status request fails or throws', async () => {
    apiMmMock.mockResolvedValueOnce(new Response('nope', { status: 503 }));
    await expect(fetchNavModuleNameIfRelevant(MODULES[0])).resolves.toBeNull();

    apiMmMock.mockRejectedValueOnce(new Error('network down'));
    await expect(fetchNavModuleNameIfRelevant(MODULES[0])).resolves.toBeNull();
  });

  it('gives up on a slow status request after 25 seconds', async () => {
    vi.useFakeTimers();
    apiMmMock.mockImplementationOnce(
      (_path: string, init?: RequestInit) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
        })
    );

    const pending = fetchNavModuleNameIfRelevant(MODULES[0]);
    await vi.advanceTimersByTimeAsync(25_000);
    await expect(pending).resolves.toBeNull();
  });
});
