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
import { fireEvent, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PortalAccess } from '../../rbac/portalAccess';
import { apiHorizon, apiMm } from '../../utils/api';
import { renderThemed } from '../../test/render';
import { portalAccess } from '../../test/portalAccessFixture';
import { SettingsTab } from './SettingsTab';

const accessBox = vi.hoisted(() => ({ current: null as PortalAccess | null }));

vi.mock('../../utils/api', () => ({
  apiMm: vi.fn(),
  apiHorizon: vi.fn(),
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

describe('SettingsTab', () => {
  beforeEach(() => {
    localStorage.clear();
    accessBox.current = portalAccess('admin');
    vi.mocked(apiMm).mockReset();
    vi.mocked(apiHorizon).mockReset();
    vi.mocked(apiMm).mockImplementation(async (path: string) => {
      if (path === '/settings/workflows-visibility') {
        return json({});
      }
      return new Response('missing', { status: 404 });
    });
    vi.mocked(apiHorizon).mockImplementation(async (path: string) => {
      if (path === '/openapi.json') {
        return json({});
      }
      return json({ items: [{ submittedFrom: 'custom-job' }] });
    });
  });

  it('saves a restricted source list', async () => {
    const user = userEvent.setup();
    renderThemed(<SettingsTab />);

    expect(await screen.findByText('Workflows visibility')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'custom-job' })).toBeDisabled();

    await user.click(screen.getByRole('checkbox', { name: 'Show all sources (no filter)' }));
    await user.click(screen.getByRole('checkbox', { name: 'custom-job' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(vi.mocked(apiMm)).toHaveBeenCalledWith(
      '/settings/workflows-visibility',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify({
          allowedSubmittedFrom: ['api', 'developer-portal', 'horizon-cli'],
        }),
      })
    );
  });

  it('keeps the controls locked for a viewer', async () => {
    accessBox.current = portalAccess('viewer');
    renderThemed(<SettingsTab />);

    expect(await screen.findByText(/View only/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Save buffer limit' })).toBeDisabled();
  });

  it('rejects a log buffer outside the allowed range', async () => {
    const user = userEvent.setup();
    renderThemed(<SettingsTab />);

    const field = await screen.findByRole('spinbutton', { name: 'Max lines' });
    fireEvent.change(field, { target: { value: '10' } });
    await user.click(screen.getByRole('button', { name: 'Save buffer limit' }));
    expect(screen.getByText(/Must be between/)).toBeInTheDocument();
  });
});
