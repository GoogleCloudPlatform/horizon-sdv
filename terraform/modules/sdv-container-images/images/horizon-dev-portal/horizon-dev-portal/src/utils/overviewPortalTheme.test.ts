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
import { describe, expect, it } from 'vitest';
import { injectOverviewPortalTheme } from './overviewPortalTheme';

describe('injectOverviewPortalTheme', () => {
  it('sets the theme attribute on html and injects the bridge stylesheet before </head>', () => {
    const out = injectOverviewPortalTheme('<html><head></head><body></body></html>', 'dark');
    expect(out).toContain('<html data-hp-theme="dark">');
    expect(out).toContain('data-hp-overview-theme="1"');
    expect(out).toContain('html[data-hp-theme="dark"]');
    expect(out.indexOf('<style')).toBeLessThan(out.indexOf('</head>'));
  });

  it('replaces an existing data-hp-theme attribute and keeps other attributes', () => {
    const out = injectOverviewPortalTheme(
      '<html lang="en" data-hp-theme="light"><head></head></html>',
      'dark'
    );
    expect(out).toContain('<html lang="en" data-hp-theme="dark">');
    expect(out).not.toContain('<html lang="en" data-hp-theme="light">');
  });

  it('lowercases the html tag when it was upper case', () => {
    const out = injectOverviewPortalTheme('<HTML class="x"><head></head></HTML>', 'light');
    expect(out).toContain('<html class="x" data-hp-theme="light">');
  });

  it('prepends the bridge when the document has no head', () => {
    const out = injectOverviewPortalTheme('<p>hi</p>', 'light');
    expect(out.startsWith('<style type="text/css"')).toBe(true);
    expect(out.endsWith('<p>hi</p>')).toBe(true);
  });
});
