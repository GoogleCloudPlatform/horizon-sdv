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
import { renderRouted } from '../test/render';
import { ToolsPage } from './ToolsPage';

describe('ToolsPage', () => {
  it('selects the tab from the path and navigates when another tab is chosen', async () => {
    const user = userEvent.setup();
    renderRouted(
      <Routes>
        <Route path="/tools" element={<ToolsPage />}>
          <Route path="horizon-cli" element={<div>cli panel</div>} />
          <Route path="horizon-api" element={<div>api panel</div>} />
        </Route>
      </Routes>,
      { initialEntries: ['/tools/horizon-cli'] }
    );

    expect(screen.getByRole('heading', { name: 'Tools' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Horizon CLI' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByText('cli panel')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: 'Horizon API' }));
    expect(screen.getByText('api panel')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Horizon API' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
  });
});
