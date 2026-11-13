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
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fetchNavModuleNames } from './navModules';
import { apiHorizon } from './utils/api';
import { authService } from './utils/auth';
import { portalAccess } from './test/portalAccessFixture';
import App from './App';

vi.mock('./utils/auth', () => ({
  authService: {
    init: vi.fn(),
    getUsername: vi.fn(() => 'ada'),
    logout: vi.fn(),
    login: vi.fn(),
    getTokenClaims: vi.fn(() => ({})),
  },
}));

vi.mock('./utils/api', () => ({
  apiHorizon: vi.fn(),
}));

vi.mock('./navModules', () => ({
  fetchNavModuleNames: vi.fn(),
}));

describe('App', () => {
  beforeEach(() => {
    localStorage.clear();
    window.APP_CONFIG = { horizonVersion: '4.3.0', publicPath: '' };
    vi.mocked(authService.init).mockReset();
    vi.mocked(authService.logout).mockReset();
    vi.mocked(fetchNavModuleNames).mockReset();
    vi.mocked(fetchNavModuleNames).mockResolvedValue(['aaos']);
    vi.mocked(apiHorizon).mockResolvedValue(
      new Response(JSON.stringify(portalAccess('developer')), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    );
  });

  it('sends a signed-out visitor to the login page', async () => {
    vi.mocked(authService.init).mockResolvedValue(false);
    render(<App />);
    expect(
      await screen.findByRole('button', { name: 'Sign in with SSO (Keycloak)' })
    ).toBeInTheDocument();
  });

  it('shows the shell, ready modules, and signs out', async () => {
    const user = userEvent.setup();
    vi.mocked(authService.init).mockResolvedValue(true);
    render(<App />);

    // The permanent drawer is hidden below the sm breakpoint, and jsdom does not apply that media query.
    expect(await screen.findByRole('link', { name: 'aaos', hidden: true })).toHaveAttribute(
      'href',
      '/module/aaos'
    );
    expect(screen.getByText('ada')).toBeInTheDocument();
    expect(screen.getByText('Horizon 4.3.0')).toBeInTheDocument();
    expect(
      await screen.findByRole('button', { name: 'Your role is Developer. Open your access' })
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'logout' }));
    expect(authService.logout).toHaveBeenCalledTimes(1);
  });
});
