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
import { ThemeProvider } from '@mui/material/styles';
import { render, type RenderOptions } from '@testing-library/react';
import type { ReactElement, ReactNode } from 'react';
import { MemoryRouter, type MemoryRouterProps } from 'react-router-dom';
import { getTheme } from '../theme';

// eslint-disable-next-line react-refresh/only-export-components -- test helper, not a hot-reload module
function ThemeWrapper({ children }: { children: ReactNode }) {
  return <ThemeProvider theme={getTheme('light')}>{children}</ThemeProvider>;
}

export function renderThemed(ui: ReactElement, options?: Omit<RenderOptions, 'wrapper'>) {
  return render(ui, { ...options, wrapper: ThemeWrapper });
}

export function renderRouted(
  ui: ReactElement,
  routerProps?: MemoryRouterProps,
  options?: Omit<RenderOptions, 'wrapper'>
) {
  return renderThemed(<MemoryRouter {...routerProps}>{ui}</MemoryRouter>, options);
}
