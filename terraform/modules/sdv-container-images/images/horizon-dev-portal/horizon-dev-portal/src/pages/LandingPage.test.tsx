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
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PortalAccess } from '../rbac/portalAccess';
import { renderThemed } from '../test/render';
import { portalAccess } from '../test/portalAccessFixture';
import { LandingPage } from './LandingPage';

const accessBox = vi.hoisted(() => ({ current: null as PortalAccess | null }));

vi.mock('../rbac/AccessProvider', () => ({
  usePortalAccess: () => accessBox.current,
}));

function card(title: string): HTMLElement {
  const root = screen.getByText(title).closest('.MuiCard-root');
  if (!root) {
    throw new Error(`no card for ${title}`);
  }
  return root as HTMLElement;
}

describe('LandingPage', () => {
  beforeEach(() => {
    accessBox.current = portalAccess('developer');
  });

  it('lets a developer open shared apps and locks admin apps', () => {
    renderThemed(<LandingPage />);

    expect(screen.getByRole('heading', { name: 'Landing page' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Applications' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(within(card('Gerrit')).getByRole('link', { name: 'Open' })).toHaveAttribute(
      'href',
      '/gerrit'
    );
    expect(within(card('Keycloak')).getByRole('button', { name: 'Open' })).toBeDisabled();
    expect(within(card('Grafana')).getByText('Admin')).toBeInTheDocument();
  });

  it('lets an admin open the admin applications', () => {
    accessBox.current = portalAccess('admin');
    renderThemed(<LandingPage />);
    expect(within(card('Grafana')).getByRole('link', { name: 'Open' })).toHaveAttribute(
      'href',
      '/grafana'
    );
  });

  it('explains a locked admin app with the admin-applications reason', async () => {
    const user = userEvent.setup();
    renderThemed(<LandingPage />);
    const button = within(card('Keycloak')).getByRole('button', { name: 'Open' });
    await user.hover(button.parentElement!);
    expect(await screen.findByRole('tooltip')).toHaveTextContent(
      'Admin applications manage the whole platform'
    );
  });

  it('explains a locked Argo Workflows card with the Argo reason', async () => {
    const access = portalAccess('viewer');
    const platform = access.sections.find((section) => section.id === 'platform');
    const openArgo = platform?.functions.find((fn) => fn.id === 'platform.openArgoWorkflows');
    if (!openArgo) {
      throw new Error('missing Argo grant');
    }
    openArgo.grants = { viewer: false, developer: true, admin: true };
    accessBox.current = access;
    const user = userEvent.setup();
    renderThemed(<LandingPage />);
    const button = within(card('Argo Workflows')).getByRole('button', { name: 'Open' });
    await user.hover(button.parentElement!);
    const tooltip = await screen.findByRole('tooltip');
    expect(tooltip).toHaveTextContent(
      'Opening Argo Workflows shows every workflow on the platform.'
    );
    expect(tooltip).not.toHaveTextContent('Admin applications manage the whole platform');
  });

  it('keeps every application locked while access is unknown', () => {
    accessBox.current = null;
    renderThemed(<LandingPage />);
    expect(within(card('Gerrit')).getByRole('button', { name: 'Open' })).toBeDisabled();
    expect(within(card('Keycloak')).getByRole('button', { name: 'Open' })).toBeDisabled();
  });
});
