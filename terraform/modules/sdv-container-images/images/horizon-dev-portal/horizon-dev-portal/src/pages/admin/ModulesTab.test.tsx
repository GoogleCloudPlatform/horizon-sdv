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
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PortalAccess } from '../../rbac/portalAccess';
import { apiMm } from '../../utils/api';
import { renderThemed } from '../../test/render';
import { portalAccess } from '../../test/portalAccessFixture';
import { ModulesTab } from './ModulesTab';

const accessBox = vi.hoisted(() => ({ current: null as PortalAccess | null }));

vi.mock('../../utils/api', () => ({
  apiMm: vi.fn(),
}));

vi.mock('../../rbac/AccessProvider', () => ({
  usePortalAccess: () => accessBox.current,
}));

function json(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('ModulesTab', () => {
  let enabled = false;

  beforeEach(() => {
    enabled = false;
    accessBox.current = portalAccess('admin');
    vi.mocked(apiMm).mockReset();
    vi.mocked(apiMm).mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === '/modules') {
        return json([
          {
            id: '1',
            name: 'aaos',
            enabled,
            pinned: false,
            clusterTargetRevision: 'main',
            hardDependencies: ['workloads-common'],
          },
        ]);
      }
      if (path === '/modules/aaos/status') {
        return json(
          enabled
            ? { syncStatus: 'Synced', healthStatus: 'Healthy', managedApplicationCount: 1 }
            : {}
        );
      }
      if (path === '/modules/aaos/enable' && init?.method === 'POST') {
        enabled = true;
        return new Response(null, { status: 200 });
      }
      return new Response('missing', { status: 404 });
    });
  });

  it('lists a module and enables it', async () => {
    const user = userEvent.setup();
    renderThemed(<ModulesTab />);

    expect(await screen.findByRole('heading', { name: 'aaos' })).toBeInTheDocument();
    expect(screen.getByText('NOT INSTALLED')).toBeInTheDocument();
    expect(screen.getByText(/Hard deps: workloads-common/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Pin ref before install' }));
    expect(screen.getByLabelText('Custom Git ref')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    await user.click(screen.getByRole('switch', { name: 'Disabled' }));
    expect(await screen.findByText('READY')).toBeInTheDocument();
    expect(screen.getByText('Following platform')).toBeInTheDocument();
    expect(vi.mocked(apiMm)).toHaveBeenCalledWith(
      '/modules/aaos/enable',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('shows the load error and keeps changes locked for a viewer', async () => {
    vi.mocked(apiMm).mockRejectedValue(new Error('modules down'));
    accessBox.current = portalAccess('viewer');
    renderThemed(<ModulesTab />);

    expect(await screen.findByText('modules down')).toBeInTheDocument();
  });

  it('tells a viewer the page is view only once modules have loaded', async () => {
    accessBox.current = portalAccess('viewer');
    renderThemed(<ModulesTab />);

    expect(await screen.findByText(/View only/)).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Disabled' })).toBeDisabled();
    expect(screen.getByText('Admin')).toBeInTheDocument();
  });
});
