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
// @vitest-environment jsdom
import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiHorizon } from '../utils/api';
import { authService } from '../utils/auth';
import { AccessProvider, usePortalAccess } from './AccessProvider';
import { portalAccess } from '../test/portalAccessFixture';

vi.mock('../utils/api', () => ({
  apiHorizon: vi.fn(),
}));

vi.mock('../utils/auth', () => ({
  authService: {
    getTokenClaims: vi.fn(),
  },
}));

const apiHorizonMock = vi.mocked(apiHorizon);

function Probe() {
  const access = usePortalAccess();
  return <div>{access ? access.role : 'none'}</div>;
}

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('AccessProvider', () => {
  beforeEach(() => {
    apiHorizonMock.mockReset();
    vi.mocked(authService.getTokenClaims).mockReturnValue({
      email: 'ada@example.com',
      preferred_username: 'ada@example.com',
      groups: ['developers'],
    });
  });

  it('uses a valid access payload from the API', async () => {
    apiHorizonMock.mockResolvedValue(json(portalAccess('admin')));
    render(
      <AccessProvider>
        <Probe />
      </AccessProvider>
    );
    expect(await screen.findByText('admin')).toBeInTheDocument();
    expect(apiHorizonMock).toHaveBeenCalledWith('/v1/portal/access');
  });

  it('builds access from the token when the API is unavailable or returns a non-contract body', async () => {
    apiHorizonMock.mockResolvedValueOnce(new Response('down', { status: 503 }));
    const { unmount } = render(
      <AccessProvider>
        <Probe />
      </AccessProvider>
    );
    expect(await screen.findByText('developer')).toBeInTheDocument();
    unmount();

    apiHorizonMock.mockResolvedValueOnce(json({ role: 'nope' }));
    render(
      <AccessProvider>
        <Probe />
      </AccessProvider>
    );
    expect(await screen.findByText('developer')).toBeInTheDocument();
  });

  it('builds access from the token when the request throws', async () => {
    apiHorizonMock.mockRejectedValue(new Error('network down'));
    render(
      <AccessProvider>
        <Probe />
      </AccessProvider>
    );
    expect(await screen.findByText('developer')).toBeInTheDocument();
  });

  it('leaves access unknown on 401 so the API client can refresh or redirect', async () => {
    apiHorizonMock.mockResolvedValue(new Response(null, { status: 401 }));
    render(
      <AccessProvider>
        <Probe />
      </AccessProvider>
    );
    await waitFor(() => expect(apiHorizonMock).toHaveBeenCalled());
    expect(screen.getByText('none')).toBeInTheDocument();
  });
});
