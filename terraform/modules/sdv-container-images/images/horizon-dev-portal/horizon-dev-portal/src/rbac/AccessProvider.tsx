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
import { createContext, useContext, useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { apiHorizon } from '../utils/api';
import type { PortalAccess } from './portalAccess';

const AccessContext = createContext<PortalAccess | null>(null);

function isPortalAccess(value: unknown): value is PortalAccess {
  if (!value || typeof value !== 'object') {
    return false;
  }
  const body = value as Partial<PortalAccess>;
  return (
    (body.role === 'viewer' || body.role === 'developer' || body.role === 'admin') &&
    Array.isArray(body.sections) &&
    Array.isArray(body.roles)
  );
}

/**
 * Loads GET /v1/portal/access once the user is signed in.
 * Only a 200 body that matches the contract is shown. HTTP 401 is left to
 * the API client, which refreshes or sends the user to login. Any other
 * response, including 403, 5xx, and a body that is not the contract, leaves
 * access unknown so the UI stays closed instead of inventing grants from the token.
 */
export function AccessProvider({ children }: { children: ReactNode }) {
  const [access, setAccess] = useState<PortalAccess | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const resp = await apiHorizon('/v1/portal/access');
        if (cancelled || resp.status === 401 || !resp.ok) {
          return;
        }
        const body: unknown = await resp.json();
        if (!cancelled && isPortalAccess(body)) {
          setAccess(body);
        }
      } catch {
        // Leave access unknown. A client-built matrix can grant more than the server.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return <AccessContext.Provider value={access}>{children}</AccessContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components -- hook is tied to this provider module
export function usePortalAccess(): PortalAccess | null {
  return useContext(AccessContext);
}
