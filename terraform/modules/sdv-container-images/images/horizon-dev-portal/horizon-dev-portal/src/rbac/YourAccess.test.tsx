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
import type { PortalAccess } from './portalAccess';
import { renderThemed } from '../test/render';
import { portalAccess } from '../test/portalAccessFixture';
import { YourAccessButton } from './YourAccess';

const accessBox = vi.hoisted(() => ({ current: null as PortalAccess | null }));

vi.mock('./AccessProvider', () => ({
  usePortalAccess: () => accessBox.current,
}));

describe('YourAccessButton', () => {
  beforeEach(() => {
    accessBox.current = null;
  });

  it('renders nothing while access is still loading', () => {
    const { container } = renderThemed(<YourAccessButton />);
    expect(container).toBeEmptyDOMElement();
  });

  it('opens the permission matrix for the signed-in role and closes it again', async () => {
    const user = userEvent.setup();
    accessBox.current = portalAccess('developer');
    renderThemed(<YourAccessButton />);

    await user.click(
      screen.getByRole('button', { name: 'Your role is Developer. Open your access' })
    );
    expect(screen.getByRole('heading', { name: 'Your access' })).toBeInTheDocument();
    expect(screen.getByText('Signed in as ada@example.com')).toBeInTheDocument();
    expect(screen.getByText('Developer (you)')).toBeInTheDocument();
    expect(screen.getByText('Portal settings')).toBeInTheDocument();
    expect(screen.getAllByLabelText('allowed').length).toBeGreaterThan(0);
    expect(screen.getAllByLabelText('denied').length).toBeGreaterThan(0);

    await user.keyboard('{Escape}');
    expect(screen.queryByRole('heading', { name: 'Your access' })).not.toBeInTheDocument();
  });
});
