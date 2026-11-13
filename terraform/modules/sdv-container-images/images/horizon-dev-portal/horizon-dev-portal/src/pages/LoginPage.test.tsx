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
import { authService } from '../utils/auth';
import { renderRouted } from '../test/render';
import { LoginPage } from './LoginPage';

vi.mock('../utils/auth', () => ({
  authService: {
    init: vi.fn(),
    login: vi.fn(),
    loginWithPassword: vi.fn(),
  },
}));

function renderLogin() {
  return renderRouted(
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<div>home</div>} />
    </Routes>,
    { initialEntries: ['/login'] }
  );
}

describe('LoginPage', () => {
  beforeEach(() => {
    vi.mocked(authService.init).mockReset();
    vi.mocked(authService.login).mockReset();
    vi.mocked(authService.loginWithPassword).mockReset();
  });

  it('shows a spinner while the session is being checked', () => {
    vi.mocked(authService.init).mockReturnValue(new Promise(() => {}));
    renderLogin();
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('sends an existing session to the home page', async () => {
    vi.mocked(authService.init).mockResolvedValue(true);
    renderLogin();
    expect(await screen.findByText('home')).toBeInTheDocument();
  });

  it('offers SSO and reports a failed password sign-in', async () => {
    const user = userEvent.setup();
    vi.mocked(authService.init).mockResolvedValue(false);
    vi.mocked(authService.loginWithPassword).mockRejectedValue(
      new Error('Invalid user credentials')
    );
    renderLogin();

    await user.click(await screen.findByRole('button', { name: 'Sign in with SSO (Keycloak)' }));
    expect(authService.login).toHaveBeenCalledTimes(1);

    expect(screen.getByRole('button', { name: 'Sign in' })).toBeDisabled();
    await user.type(screen.getByLabelText('Username'), 'ada');
    await user.type(screen.getByLabelText('Password'), 'secret');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(await screen.findByText('Invalid user credentials')).toBeInTheDocument();
    expect(authService.loginWithPassword).toHaveBeenCalledWith('ada', 'secret');
  });
});
