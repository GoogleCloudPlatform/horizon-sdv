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
import { describe, expect, it } from 'vitest';
import { renderRouted } from '../../test/render';
import { AdminLayout } from './AdminLayout';

describe('AdminLayout', () => {
  it('shows Settings when the path is the settings page and returns to Modules', async () => {
    const user = userEvent.setup();
    renderRouted(
      <Routes>
        <Route path="/admin" element={<AdminLayout />}>
          <Route path="modules" element={<div>modules panel</div>} />
          <Route path="settings" element={<div>settings panel</div>} />
        </Route>
      </Routes>,
      { initialEntries: ['/admin/settings'] }
    );

    expect(screen.getByRole('heading', { name: 'Administration' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('settings panel')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: 'Modules' }));
    expect(screen.getByText('modules panel')).toBeInTheDocument();
  });
});
