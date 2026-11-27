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

// Run: node --test storage-mirror-role.test.mjs (no dependencies; not part of the image).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  ensureStorageMirrorAdminRole,
  STORAGE_MIRROR_ADMIN_ROLE,
  STORAGE_MIRROR_ADMIN_GROUP,
  REALM_ADMIN_ROLE,
} from './storage-mirror-role.mjs';

function fakeKeycloak({ groups = [{ id: 'g1', name: STORAGE_MIRROR_ADMIN_GROUP }], realmAdmin = true } = {}) {
  const roles = new Map();
  if (realmAdmin) roles.set(REALM_ADMIN_ROLE, { id: 'ra', name: REALM_ADMIN_ROLE });
  const groupRoles = new Map();
  const composites = new Map();
  const writes = [];
  return {
    writes,
    groupRoles,
    composites,
    roles: {
      findOneByName: async ({ name }) => roles.get(name),
      create: async ({ name }) => { writes.push(`create ${name}`); roles.set(name, { id: `id-${name}`, name }); },
      getCompositeRoles: async ({ id }) => composites.get(id) || [],
      createComposite: async ({ roleId }, list) => {
        writes.push(`composite ${roleId}`);
        composites.set(roleId, [...(composites.get(roleId) || []), ...list]);
      },
    },
    groups: {
      find: async ({ search }) => groups.filter((g) => g.name.includes(search)),
      listRealmRoleMappings: async ({ id }) => groupRoles.get(id) || [],
      addRealmRoleMappings: async ({ id, roles: list }) => {
        writes.push(`group ${id}`);
        groupRoles.set(id, [...(groupRoles.get(id) || []), ...list]);
      },
    },
  };
}

test('creates the role, maps the administrators group and the realm_admin composite', async () => {
  const kc = fakeKeycloak();
  await ensureStorageMirrorAdminRole(kc, { log: () => {} });
  assert.deepEqual(kc.writes, [`create ${STORAGE_MIRROR_ADMIN_ROLE}`, 'group g1', 'composite ra']);
  assert.equal(kc.groupRoles.get('g1')[0].name, STORAGE_MIRROR_ADMIN_ROLE);
  assert.equal(kc.composites.get('ra')[0].name, STORAGE_MIRROR_ADMIN_ROLE);
});

test('is idempotent', async () => {
  const kc = fakeKeycloak();
  await ensureStorageMirrorAdminRole(kc, { log: () => {} });
  kc.writes.length = 0;
  await ensureStorageMirrorAdminRole(kc, { log: () => {} });
  assert.deepEqual(kc.writes, []);
});

test('skips missing group and realm_admin without failing', async () => {
  const kc = fakeKeycloak({ groups: [{ id: 'g2', name: 'administrators-old' }], realmAdmin: false });
  await ensureStorageMirrorAdminRole(kc, { log: () => {} });
  assert.deepEqual(kc.writes, [`create ${STORAGE_MIRROR_ADMIN_ROLE}`]);
});
