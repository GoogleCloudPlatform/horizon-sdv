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
import { describe, expect, it, vi } from 'vitest';
import { authService } from '../utils/auth';
import { renderRouted } from '../test/render';
import { WelcomePage } from './WelcomePage';

vi.mock('../utils/auth', () => ({
  authService: {
    getUsername: vi.fn(),
  },
}));

describe('WelcomePage', () => {
  it('greets the signed-in user and links to the main areas', () => {
    vi.mocked(authService.getUsername).mockReturnValue('Ada');
    renderRouted(<WelcomePage />);

    expect(screen.getByRole('heading', { name: 'Welcome, Ada' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open Administration → Modules' })).toHaveAttribute(
      'href',
      '/admin/modules'
    );
    expect(screen.getByRole('link', { name: 'Administration → Settings' })).toHaveAttribute(
      'href',
      '/admin/settings'
    );
    expect(screen.getByRole('link', { name: 'Tools' })).toHaveAttribute('href', '/tools');
    expect(screen.getByRole('heading', { name: 'Modules & workflows' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Deployment guide' })).toHaveAttribute(
      'href',
      'https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/deployment_guide.md'
    );
  });

  it('omits the name when the session has no username', () => {
    vi.mocked(authService.getUsername).mockReturnValue(null);
    renderRouted(<WelcomePage />);
    expect(screen.getByRole('heading', { name: 'Welcome' })).toBeInTheDocument();
  });
});
