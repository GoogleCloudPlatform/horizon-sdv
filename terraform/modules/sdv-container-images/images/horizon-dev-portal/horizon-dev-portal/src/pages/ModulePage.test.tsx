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
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PortalAccess } from '../rbac/portalAccess';
import { apiHorizon, apiMm } from '../utils/api';
import { renderRouted } from '../test/render';
import { portalAccess } from '../test/portalAccessFixture';
import { ModulePage } from './ModulePage';

const accessBox = vi.hoisted(() => ({ current: null as PortalAccess | null }));

vi.mock('../utils/api', () => ({
  apiHorizon: vi.fn(),
  apiMm: vi.fn(),
}));

vi.mock('../utils/auth', () => ({
  authService: {
    getUsername: () => 'ada',
    getOAuthClientId: () => 'horizon-dev-portal',
  },
}));

vi.mock('../rbac/AccessProvider', () => ({
  usePortalAccess: () => accessBox.current,
}));

function json(body: unknown, status = 200): Response {
  return new Response(status === 200 ? JSON.stringify(body) : String(body), {
    status,
    headers: status === 200 ? { 'Content-Type': 'application/json' } : undefined,
  });
}

const moduleBody = {
  name: 'aaos',
  enabled: true,
  applications: [{ id: 'console', title: 'Console', url: '/console' }],
};

function renderModule() {
  return renderRouted(
    <Routes>
      <Route path="/module/:name" element={<ModulePage />} />
    </Routes>,
    { initialEntries: ['/module/aaos'] }
  );
}

describe('ModulePage', () => {
  beforeEach(() => {
    accessBox.current = portalAccess('developer');
    vi.mocked(apiHorizon).mockReset();
    vi.mocked(apiMm).mockReset();
    vi.mocked(apiMm).mockImplementation(async (path: string) => {
      if (path === '/settings/workflows-visibility') {
        return json({});
      }
      if (path === '/modules/aaos') {
        return json(moduleBody);
      }
      return json('missing', 404);
    });
    vi.mocked(apiHorizon).mockImplementation(async (path: string) => {
      if (path === '/v1/catalog') {
        return json({
          entries: [
            {
              module: 'aaos',
              templateName: 'aaos-builder',
              namespace: 'argo',
              parameters: [{ name: 'branch', default: 'main' }],
            },
          ],
        });
      }
      if (path.startsWith('/v1/workflows/running')) {
        return json({
          items: [
            {
              name: 'wf-1',
              namespace: 'argo',
              phase: 'Running',
              workflowTemplate: 'aaos-builder',
              startedBy: 'ada',
              submittedFrom: 'developer-portal',
            },
          ],
        });
      }
      if (path.startsWith('/v1/workflows/history')) {
        return json({ items: [] });
      }
      if (path === '/v1/workflows/wf-1') {
        return json({
          name: 'wf-1',
          namespace: 'argo',
          module: 'aaos',
          phase: 'Running',
          workflowTemplate: 'aaos-builder',
          nodes: [],
        });
      }
      return json('missing', 404);
    });
  });

  it('shows the overview gap and the module applications', async () => {
    const user = userEvent.setup();
    renderModule();

    expect(await screen.findByRole('heading', { name: 'Module: aaos' })).toBeInTheDocument();
    expect(await screen.findByText(/No overview is configured/)).toBeInTheDocument();

    await user.click(await screen.findByRole('tab', { name: 'Applications' }));
    expect(screen.getByText('Console')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open' })).toHaveAttribute(
      'href',
      'http://localhost:3000/console'
    );
  });

  it('opens the submit dialog for a workflow template', async () => {
    const user = userEvent.setup();
    renderModule();

    await user.click(await screen.findByRole('tab', { name: 'Workflow Templates' }));
    expect(screen.getByRole('heading', { name: 'aaos-builder' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Configure & submit' }));
    expect(
      await screen.findByRole('heading', { name: 'Submit: aaos-builder' })
    ).toBeInTheDocument();
    expect(screen.getByLabelText('branch')).toBeInTheDocument();
  });

  it('opens a running workflow', async () => {
    const user = userEvent.setup();
    renderModule();

    await user.click(await screen.findByRole('tab', { name: 'Running Workflows' }));
    expect(await screen.findByText('Developer portal')).toBeInTheDocument();
    await user.click(screen.getByText('aaos-builder'));
    expect(await screen.findByRole('link', { name: 'Open in Argo Workflows' })).toBeInTheDocument();
    expect(screen.getByText('WT: aaos-builder')).toBeInTheDocument();
  });

  it('warns when the catalog cannot be loaded', async () => {
    vi.mocked(apiHorizon).mockImplementation(async (path: string) => {
      if (path === '/v1/catalog') {
        return json('nope', 500);
      }
      return json({ items: [] });
    });
    renderModule();
    expect(await screen.findByText('catalog 500')).toBeInTheDocument();
  });
});
