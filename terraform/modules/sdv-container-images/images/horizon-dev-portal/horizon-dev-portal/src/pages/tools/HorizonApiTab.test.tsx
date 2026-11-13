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
import { afterEach, describe, expect, it, vi } from 'vitest';
import { renderThemed } from '../../test/render';
import HorizonApiTab from './HorizonApiTab';

describe('HorizonApiTab', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('embeds Swagger from the same origin as the portal', () => {
    vi.stubGlobal('window', {
      location: { origin: 'https://portal.example' },
      APP_CONFIG: { publicPath: '/dev/' },
    });
    renderThemed(<HorizonApiTab />);
    expect(screen.getByTitle('Horizon API')).toHaveAttribute(
      'src',
      'https://portal.example/dev/api/horizon/swagger'
    );
  });
});
