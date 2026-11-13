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
import { ThemeProvider } from '@mui/material/styles';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactElement } from 'react';
import { describe, expect, it } from 'vitest';
import { getTheme } from '../theme';
import { HoverTooltip, RoleBadge } from './GatedControl';
import '@testing-library/jest-dom/vitest';

function renderWithTheme(ui: ReactElement) {
  return render(<ThemeProvider theme={getTheme('light')}>{ui}</ThemeProvider>);
}

describe('RoleBadge', () => {
  it('shows the role name and a lock icon', () => {
    renderWithTheme(<RoleBadge label="Admin" />);
    expect(screen.getByText('Admin')).toBeInTheDocument();
    expect(screen.getByTestId('LockOutlinedIcon')).toBeInTheDocument();
  });
});

describe('HoverTooltip', () => {
  it('renders the child alone when there is no reason to show', () => {
    renderWithTheme(
      <HoverTooltip title="">
        <button type="button">Submit</button>
      </HoverTooltip>
    );
    const button = screen.getByRole('button', { name: 'Submit' });
    expect(button.parentElement?.tagName).not.toBe('SPAN');
  });

  it('shows the reason when a disabled control is hovered', async () => {
    const user = userEvent.setup();
    renderWithTheme(
      <HoverTooltip title="Admin only">
        <button type="button" disabled>
          Submit
        </button>
      </HoverTooltip>
    );
    const button = screen.getByRole('button', { name: 'Submit' });
    expect(button.parentElement?.tagName).toBe('SPAN');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();

    await user.hover(button.parentElement!);
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Admin only');
  });

  it('stretches the hover target when fullWidth is set', () => {
    renderWithTheme(
      <HoverTooltip title="Developer only" fullWidth>
        <button type="button">Submit</button>
      </HoverTooltip>
    );
    expect(screen.getByRole('button', { name: 'Submit' }).parentElement).toHaveStyle({
      display: 'block',
      width: '100%',
    });
  });
});
