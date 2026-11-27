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

// Realm role required by Horizon API for mutating /v1/storage-mirror/* calls (MIRROR_ADMIN_ROLE).
// Horizon API reads realm roles from the access token claim realm_access.roles.
export const STORAGE_MIRROR_ADMIN_ROLE = 'horizon-storage-mirror-admin';

// Members of this group (created by keycloak-post, wave 4) get the role by default; Argo CD maps the same group to
// its admin role. The realm_admin composite gives it to the initial realm administrator as well.
export const STORAGE_MIRROR_ADMIN_GROUP = 'administrators';
export const REALM_ADMIN_ROLE = 'realm_admin';

/**
 * Creates the realm role and its default grants. Idempotent: every step checks before it writes.
 * @param {object} kc KcAdminClient with realmName already set.
 * @param {{log?: (msg: string) => void}} [opts]
 */
export async function ensureStorageMirrorAdminRole(kc, opts = {}) {
  const log = opts.log || ((msg) => console.info(msg));

  let role = await kc.roles.findOneByName({ name: STORAGE_MIRROR_ADMIN_ROLE });
  if (role) {
    log(`realm role ${STORAGE_MIRROR_ADMIN_ROLE} exists`);
  } else {
    log(`creating realm role ${STORAGE_MIRROR_ADMIN_ROLE}`);
    await kc.roles.create({
      name: STORAGE_MIRROR_ADMIN_ROLE,
      description: 'Create, sync and delete Storage Mirror mirrors through the Horizon API (/v1/storage-mirror).',
    });
    role = await kc.roles.findOneByName({ name: STORAGE_MIRROR_ADMIN_ROLE });
  }
  if (!role) {
    throw new Error(`realm role ${STORAGE_MIRROR_ADMIN_ROLE} could not be created`);
  }
  const ref = { id: role.id, name: role.name };

  const groups = await kc.groups.find({ search: STORAGE_MIRROR_ADMIN_GROUP });
  const group = (groups || []).find((g) => g.name === STORAGE_MIRROR_ADMIN_GROUP);
  if (!group) {
    log(`group ${STORAGE_MIRROR_ADMIN_GROUP} does not exist; skipping group mapping`);
  } else {
    const mapped = await kc.groups.listRealmRoleMappings({ id: group.id });
    if ((mapped || []).some((r) => r.name === STORAGE_MIRROR_ADMIN_ROLE)) {
      log(`group ${STORAGE_MIRROR_ADMIN_GROUP} already has ${STORAGE_MIRROR_ADMIN_ROLE}`);
    } else {
      await kc.groups.addRealmRoleMappings({ id: group.id, roles: [ref] });
      log(`mapped ${STORAGE_MIRROR_ADMIN_ROLE} to group ${STORAGE_MIRROR_ADMIN_GROUP}`);
    }
  }

  const realmAdmin = await kc.roles.findOneByName({ name: REALM_ADMIN_ROLE });
  if (!realmAdmin) {
    log(`realm role ${REALM_ADMIN_ROLE} does not exist; skipping composite`);
    return;
  }
  const composites = await kc.roles.getCompositeRoles({ id: realmAdmin.id });
  if ((composites || []).some((r) => r.name === STORAGE_MIRROR_ADMIN_ROLE)) {
    log(`${REALM_ADMIN_ROLE} already includes ${STORAGE_MIRROR_ADMIN_ROLE}`);
    return;
  }
  await kc.roles.createComposite({ roleId: realmAdmin.id }, [ref]);
  log(`added ${STORAGE_MIRROR_ADMIN_ROLE} to composite role ${REALM_ADMIN_ROLE}`);
}
