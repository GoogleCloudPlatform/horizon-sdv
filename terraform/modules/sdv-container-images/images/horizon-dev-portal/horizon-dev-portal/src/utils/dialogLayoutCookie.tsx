// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
import Box from '@mui/material/Box';
import type { SxProps, Theme } from '@mui/material/styles';
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type PointerEvent as ReactPointerEvent,
} from 'react';
import { getRouterBasename } from './publicPath';

const ONE_YEAR_SEC = 60 * 60 * 24 * 365;

export const DIALOG_LAYOUT_COOKIE_WORKFLOW_DETAIL = 'hdp_dialog_workflow_detail';
export const DIALOG_LAYOUT_COOKIE_WORKFLOW_LOGS = 'hdp_dialog_workflow_logs';

export function readDialogLayoutCookie(name: string): { width: number; height: number } | null {
  if (typeof document === 'undefined') {
    return null;
  }
  const prefix = `${name}=`;
  for (const part of document.cookie.split(';')) {
    const c = part.trim();
    if (!c.startsWith(prefix)) {
      continue;
    }
    try {
      const j = JSON.parse(decodeURIComponent(c.slice(prefix.length))) as {
        width?: unknown;
        height?: unknown;
      };
      if (typeof j.width === 'number' && typeof j.height === 'number') {
        return { width: j.width, height: j.height };
      }
    } catch {
      /* ignore */
    }
  }
  return null;
}

export function writeDialogLayoutCookie(name: string, width: number, height: number): void {
  if (typeof document === 'undefined') {
    return;
  }
  const path = getRouterBasename() || '/';
  const val = encodeURIComponent(JSON.stringify({ width, height }));
  document.cookie = `${name}=${val};path=${path};max-age=${ONE_YEAR_SEC};SameSite=Lax`;
}

type ActiveDrag = {
  move: (e: PointerEvent) => void;
  up: () => void;
};

export function useResizableDialogSize(options: {
  storageKey: string;
  defaultWidth: () => number;
  defaultHeight: () => number;
  minWidth: number;
  minHeight: number;
}) {
  const { storageKey, defaultWidth, defaultHeight, minWidth, minHeight } = options;
  const clamp = useCallback(
    (w: number, h: number) => {
      if (typeof window === 'undefined') {
        return { width: w, height: h };
      }
      const gutter = 16;
      const maxW = Math.max(1, Math.floor(window.innerWidth - gutter));
      const maxH = Math.max(1, Math.floor(window.innerHeight - gutter));
      const floorW = Math.min(minWidth, maxW);
      const floorH = Math.min(minHeight, maxH);
      return {
        width: Math.min(maxW, Math.max(floorW, w)),
        height: Math.min(maxH, Math.max(floorH, h)),
      };
    },
    [minWidth, minHeight]
  );

  const [size, setSize] = useState(() => {
    const d = clamp(defaultWidth(), defaultHeight());
    const saved = readDialogLayoutCookie(storageKey);
    if (!saved) {
      return d;
    }
    return clamp(saved.width, saved.height);
  });

  const drag = useRef<{ sx: number; sy: number; w: number; h: number } | null>(null);
  const sizeRef = useRef(size);

  const activeDrag = useRef<ActiveDrag | null>(null);

  useEffect(() => {
    sizeRef.current = size;
  }, [size]);

  useEffect(() => {
    const onResize = () => setSize((prev) => clamp(prev.width, prev.height));
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, [clamp]);

  useEffect(
    () => () => {
      const a = activeDrag.current;
      if (a) {
        window.removeEventListener('pointermove', a.move);
        window.removeEventListener('pointerup', a.up);
        window.removeEventListener('pointercancel', a.up);
        activeDrag.current = null;
      }
    },
    []
  );

  const onResizeHandleDown = useCallback(
    (e: ReactPointerEvent<HTMLDivElement>) => {
      e.preventDefault();
      e.stopPropagation();
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
      drag.current = {
        sx: e.clientX,
        sy: e.clientY,
        w: sizeRef.current.width,
        h: sizeRef.current.height,
      };

      const prev = activeDrag.current;
      if (prev) {
        window.removeEventListener('pointermove', prev.move);
        window.removeEventListener('pointerup', prev.up);
        window.removeEventListener('pointercancel', prev.up);
      }

      const move = (ev: PointerEvent) => {
        if (!drag.current) {
          return;
        }
        const dw = ev.clientX - drag.current.sx;
        const dh = ev.clientY - drag.current.sy;
        const next = clamp(drag.current.w + dw, drag.current.h + dh);
        sizeRef.current = next;
        setSize(next);
      };

      const up = () => {
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', up);
        window.removeEventListener('pointercancel', up);
        drag.current = null;
        activeDrag.current = null;
        const { width, height } = sizeRef.current;
        writeDialogLayoutCookie(storageKey, width, height);
      };

      activeDrag.current = { move, up };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', up);
      window.addEventListener('pointercancel', up);
    },
    [clamp, storageKey]
  );

  const paperSx: SxProps<Theme> = {
    position: 'relative',
    width: size.width,
    height: size.height,
    maxWidth: 'calc(100vw - 16px)',
    maxHeight: 'calc(100dvh - 16px)',
    m: 1,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  };

  const ResizeHandle = (
    <Box
      aria-hidden
      onPointerDown={onResizeHandleDown}
      sx={{
        position: 'absolute',
        right: 4,
        bottom: 4,
        width: 18,
        height: 18,
        cursor: 'nwse-resize',
        touchAction: 'none',
        borderRadius: 0.5,
        '&:hover': { bgcolor: 'action.hover' },
      }}
    />
  );

  return { paperSx, ResizeHandle };
}
