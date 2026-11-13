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
import { afterEach, describe, expect, it } from 'vitest';
import { ThemeModeProvider, useThemeMode } from './ThemeModeProvider';
import { THEME_STORAGE_KEY } from './themeMode';

function Probe() {
  const { darkMode, toggleTheme } = useThemeMode();
  return <button onClick={toggleTheme}>{darkMode ? 'dark' : 'light'}</button>;
}

describe('ThemeModeProvider', () => {
  afterEach(() => {
    localStorage.clear();
  });

  it('starts in light mode and stores the choice when toggled', async () => {
    const user = userEvent.setup();
    render(
      <ThemeModeProvider>
        <Probe />
      </ThemeModeProvider>
    );

    expect(screen.getByRole('button', { name: 'light' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'light' }));
    expect(screen.getByRole('button', { name: 'dark' })).toBeInTheDocument();
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');
  });

  it('follows a theme change from another tab', async () => {
    render(
      <ThemeModeProvider>
        <Probe />
      </ThemeModeProvider>
    );

    window.dispatchEvent(new StorageEvent('storage', { key: THEME_STORAGE_KEY, newValue: 'dark' }));
    expect(await screen.findByRole('button', { name: 'dark' })).toBeInTheDocument();
  });

  it('refuses the hook outside the provider', () => {
    expect(() => render(<Probe />)).toThrow('useThemeMode must be used within ThemeModeProvider');
  });
});
