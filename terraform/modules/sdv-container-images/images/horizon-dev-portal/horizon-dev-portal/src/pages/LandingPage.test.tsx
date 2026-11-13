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

  it('keeps every application locked while access is unknown', () => {
    accessBox.current = null;
    renderThemed(<LandingPage />);
    expect(within(card('Gerrit')).getByRole('button', { name: 'Open' })).toBeDisabled();
    expect(within(card('Keycloak')).getByRole('button', { name: 'Open' })).toBeDisabled();
  });
});
