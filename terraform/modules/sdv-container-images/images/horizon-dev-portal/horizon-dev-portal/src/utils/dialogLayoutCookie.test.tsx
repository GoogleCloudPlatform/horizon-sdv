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
import { fireEvent, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { renderThemed } from '../test/render';
import { useResizableDialogSize } from './dialogLayoutCookie';

function Harness() {
  const { paperSx, ResizeHandle } = useResizableDialogSize({
    storageKey: 'hdp_test_dialog',
    defaultWidth: () => 400,
    defaultHeight: () => 300,
    minWidth: 200,
    minHeight: 150,
  });
  const width = (paperSx as { width?: number }).width;
  return (
    <div>
      <div data-testid="width">{width}</div>
      {ResizeHandle}
    </div>
  );
}

describe('useResizableDialogSize', () => {
  afterEach(() => {
    document.cookie = 'hdp_test_dialog=;path=/;max-age=0';
  });

  it('grows the dialog from the resize handle and remembers the size', () => {
    HTMLElement.prototype.setPointerCapture = () => undefined;
    renderThemed(<Harness />);
    expect(screen.getByTestId('width')).toHaveTextContent('400');

    const handle = document.querySelector('[aria-hidden="true"]');
    if (!(handle instanceof HTMLElement)) {
      throw new Error('resize handle missing');
    }
    fireEvent.pointerDown(handle, { clientX: 10, clientY: 10, pointerId: 1 });
    fireEvent.pointerMove(window, { clientX: 40, clientY: 25 });
    fireEvent.pointerUp(window);

    expect(screen.getByTestId('width')).toHaveTextContent('430');
    expect(document.cookie).toContain('hdp_test_dialog=');
  });
});
