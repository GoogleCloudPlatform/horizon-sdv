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

import fs from 'fs/promises';
import _ from 'lodash';
import KcAdminClient from '@keycloak/keycloak-admin-client';
import retry from 'async-retry';

const REALM = 'horizon';

const HORIZON_API_CLIENT_ID = 'horizon-api';

// viewer has no client role: it is implicit for any signed-in realm user.
// admin is a composite of developer, so an admin token carries both roles.
const horizonApiClientRoles = [
  {
    name: 'developer',
    description: 'Horizon API: submit developer-level workflow templates and abort own workflows.',
  },
  {
    name: 'admin',
    description: 'Horizon API: all developer functions plus platform-wide and destructive functions.',
    composites: ['developer'],
  },
  {
    name: 'impersonate',
    description: 'Horizon API: lets a service account submit on behalf of a named user.',
  },
];

const clientRoleToGroup = {
  developer: ['developers'],
  admin: ['administrators'],
};

// Service accounts are not group members, so their client roles are granted directly.
// Grant admin only to a service account whose pipeline needs it.
const serviceAccountClientRoles = {
  'horizon-api-ci': ['developer'],
};

const HORIZON_API_CLIENT_SCOPE = 'horizon-api';

// Clients whose tokens are meant for Horizon API. Tokens issued to any other client
// do not get the horizon-api audience or the resource_access.horizon-api.roles claim.
const horizonApiTokenClients = ['horizon-api', 'horizon-dev-portal', 'horizon-api-ci'];

// The realm "roles" scope flattens every client's roles into one "roles" claim (see
// keycloak-post/keycloak.mjs). Other clients also define an admin role there, so
// Horizon reads its roles from resource_access.horizon-api.roles, added here.
function horizonApiClientScope() {
  return {
    name: HORIZON_API_CLIENT_SCOPE,
    description: 'Tokens intended for Horizon API: horizon-api audience and horizon-api client roles.',
    protocol: 'openid-connect',
    attributes: {
      'include.in.token.scope': 'false',
      'display.on.consent.screen': 'false',
    },
  };
}

function horizonApiClientScopeMappers() {
  return [
    {
      name: 'horizon-api audience',
      protocol: 'openid-connect',
      protocolMapper: 'oidc-audience-mapper',
      consentRequired: false,
      config: {
        'included.client.audience': HORIZON_API_CLIENT_ID,
        'id.token.claim': 'false',
        'access.token.claim': 'true',
        'introspection.token.claim': 'true',
      },
    },
    {
      name: 'horizon-api client roles',
      protocol: 'openid-connect',
      protocolMapper: 'oidc-usermodel-client-role-mapper',
      consentRequired: false,
      config: {
        'usermodel.clientRoleMapping.clientId': HORIZON_API_CLIENT_ID,
        'claim.name': `resource_access.${HORIZON_API_CLIENT_ID}.roles`,
        'jsonType.label': 'String',
        multivalued: 'true',
        'id.token.claim': 'false',
        'access.token.claim': 'true',
        'userinfo.token.claim': 'false',
        'introspection.token.claim': 'true',
      },
    },
  ];
}

const keycloakAdmin = new KcAdminClient({
  baseUrl: process.env.PLATFORM_URL + '/auth',
});

async function waitForKeycloak() {
  const opts = {
    retries: 100,
    minTimeout: 2000,
    factor: 1,
    onRetry: (err) => {
      console.info(
        `waiting for ${process.env.PLATFORM_URL}/auth...`,
        err.message,
      );
    },
  };
  await retry(login, opts);
}

async function login() {
  await keycloakAdmin.auth({
    username: process.env.KEYCLOAK_USERNAME,
    password: process.env.KEYCLOAK_PASSWORD,
    grantType: 'password',
    clientId: 'admin-cli',
  });
}

async function getRealm() {
  const realm = await keycloakAdmin.realms.findOne({ realm: REALM });
  if (!realm) {
    throw new Error(`realm "${REALM}" not found`);
  }
  keycloakAdmin.setConfig({ realmName: realm.realm });
}

function horizonPublicClient(domain) {
  return {
    clientId: 'horizon-api',
    name: 'Horizon API (CLI / humans)',
    protocol: 'openid-connect',
    publicClient: true,
    standardFlowEnabled: true,
    directAccessGrantsEnabled: false,
    attributes: {
      'pkce.code.challenge.method': 'S256',
      'oauth2.device.authorization.grant.enabled': 'true',
    },
    redirectUris: [
      `${domain}/horizon-api/oauth2/*`,
      'http://127.0.0.1:8080/*',
      'http://127.0.0.1:8400/*',
      'http://127.0.0.1:9250/*',
      'http://localhost:8080/*',
      'http://localhost:8400/*',
      'http://localhost:9250/*',
    ],
    webOrigins: [
      domain,
      'http://127.0.0.1:8080',
      'http://127.0.0.1:8400',
      'http://127.0.0.1:9250',
      'http://localhost:8080',
      'http://localhost:8400',
      'http://localhost:9250',
    ],
  };
}

/** Confidential client: client_credentials via service account; use client_id + client_secret from Keycloak Admin. */
function horizonCiClient() {
  return {
    clientId: 'horizon-api-ci',
    name: 'Horizon API CI/CD',
    protocol: 'openid-connect',
    publicClient: false,
    standardFlowEnabled: false,
    directAccessGrantsEnabled: false,
    serviceAccountsEnabled: true,
    // Lets Keycloak return refresh_token + refresh_expires_in on client_credentials (renew via refresh_token grant).
    attributes: {
      'use.refresh.tokens': 'true',
    },
  };
}

/** Browser SPA at /developer-portal (PKCE + optional password grant). */
function horizonDevPortalClient(domain) {
  return {
    clientId: 'horizon-dev-portal',
    name: 'Horizon Developer Portal',
    protocol: 'openid-connect',
    publicClient: true,
    standardFlowEnabled: true,
    directAccessGrantsEnabled: true,
    attributes: {
      'pkce.code.challenge.method': 'S256',
    },
    redirectUris: [`${domain}/developer-portal/*`],
    webOrigins: [domain],
  };
}

async function upsertClient(desired) {
  const clients = await keycloakAdmin.clients.find();
  const found = _.find(clients, { clientId: desired.clientId });
  const merged = _.merge({}, found, desired);
  if (merged.clientId === 'horizon-api-ci' && merged.attributes) {
    merged.attributes = _.omit(merged.attributes, 'oauth2.token.exchange.grant.enabled');
  }
  if (found) {
    console.info('updating client %s', desired.clientId);
    await keycloakAdmin.clients.update(
      { id: found.id, realm: REALM },
      merged,
    );
  } else {
    console.info('creating client %s', desired.clientId);
    await keycloakAdmin.clients.create(desired);
  }
}

async function writeHorizonCiClientSecretForJenkins() {
  const clients = await keycloakAdmin.clients.find();
  const client = _.find(clients, { clientId: 'horizon-api-ci' });
  if (!client?.id) {
    throw new Error('horizon-api-ci client not found');
  }
  let cred = await keycloakAdmin.clients.getClientSecret({ id: client.id });
  if (!cred?.value) {
    cred = await keycloakAdmin.clients.generateNewClientSecret({
      id: client.id,
    });
  }
  if (!cred?.value) {
    throw new Error('horizon-api-ci client has no client secret');
  }
  await fs.writeFile('horizon-api-ci-client-secret', cred.value, 'utf8');
}

async function findClient(clientId) {
  const client = (await keycloakAdmin.clients.find({ clientId }))[0];
  if (!client?.id) {
    throw new Error(`client ${clientId} not found`);
  }
  return client;
}

async function findHorizonApiRole(client, roleName) {
  const role = await keycloakAdmin.clients.findRole({ id: client.id, roleName });
  if (!role) {
    throw new Error(`client role ${roleName} not found on ${client.clientId}`);
  }
  return role;
}

async function ensureHorizonApiClientRoles() {
  const client = await findClient(HORIZON_API_CLIENT_ID);
  const existing = await keycloakAdmin.clients.listRoles({ id: client.id });

  for (const { name, description } of horizonApiClientRoles) {
    if (existing.some((role) => role.name === name)) {
      console.info('client role %s already exists on %s', name, client.clientId);
      continue;
    }
    await keycloakAdmin.clients.createRole({ id: client.id, name, description });
    console.info('created client role %s on %s', name, client.clientId);
  }

  for (const { name, composites = [] } of horizonApiClientRoles) {
    if (composites.length === 0) {
      continue;
    }
    const role = await findHorizonApiRole(client, name);
    const current = await keycloakAdmin.roles.getCompositeRoles({ id: role.id });
    const missing = [];
    for (const childName of composites) {
      if (!current.some((child) => child.name === childName && child.containerId === client.id)) {
        missing.push(await findHorizonApiRole(client, childName));
      }
    }
    if (missing.length === 0) {
      console.info('client role %s already includes %s', name, composites.join(', '));
      continue;
    }
    await keycloakAdmin.roles.createComposite({ roleId: role.id }, missing);
    console.info('client role %s now includes %s', name, missing.map((r) => r.name).join(', '));
  }
}

async function mapHorizonApiClientRolesToGroups() {
  const client = await findClient(HORIZON_API_CLIENT_ID);
  const allGroups = await keycloakAdmin.groups.find();

  for (const [roleName, groupNames] of Object.entries(clientRoleToGroup)) {
    const role = await findHorizonApiRole(client, roleName);
    for (const groupName of groupNames) {
      const group = allGroups.find((g) => g.name === groupName);
      if (!group) {
        console.warn('group %s does not exist; client role %s not mapped', groupName, roleName);
        continue;
      }
      const mapped = await keycloakAdmin.groups.listClientRoleMappings({
        id: group.id,
        clientUniqueId: client.id,
      });
      if (mapped.some((r) => r.name === roleName)) {
        console.info('client role %s already mapped to group %s', roleName, groupName);
        continue;
      }
      await keycloakAdmin.groups.addClientRoleMappings({
        id: group.id,
        clientUniqueId: client.id,
        roles: [{ id: role.id, name: role.name }],
      });
      console.info('mapped client role %s to group %s', roleName, groupName);
    }
  }
}

async function grantServiceAccountClientRoles() {
  const horizonApi = await findClient(HORIZON_API_CLIENT_ID);

  for (const [serviceClientId, roleNames] of Object.entries(serviceAccountClientRoles)) {
    const serviceClient = await findClient(serviceClientId);
    const user = await keycloakAdmin.clients.getServiceAccountUser({ id: serviceClient.id });
    const mapped = await keycloakAdmin.users.listClientRoleMappings({
      id: user.id,
      clientUniqueId: horizonApi.id,
    });
    for (const roleName of roleNames) {
      if (mapped.some((r) => r.name === roleName)) {
        console.info('service account of %s already has client role %s', serviceClientId, roleName);
        continue;
      }
      const role = await findHorizonApiRole(horizonApi, roleName);
      await keycloakAdmin.users.addClientRoleMappings({
        id: user.id,
        clientUniqueId: horizonApi.id,
        roles: [{ id: role.id, name: role.name }],
      });
      console.info('granted client role %s to service account of %s', roleName, serviceClientId);
    }
  }
}

async function ensureHorizonApiClientScope() {
  const desired = horizonApiClientScope();
  let scope = await keycloakAdmin.clientScopes.findOneByName({ name: desired.name });
  if (scope) {
    console.info('updating client scope %s', desired.name);
    await keycloakAdmin.clientScopes.update({ id: scope.id }, _.merge({}, scope, desired));
  } else {
    console.info('creating client scope %s', desired.name);
    await keycloakAdmin.clientScopes.create(desired);
    scope = await keycloakAdmin.clientScopes.findOneByName({ name: desired.name });
  }

  const existing = await keycloakAdmin.clientScopes.listProtocolMappers({ id: scope.id });
  for (const mapper of horizonApiClientScopeMappers()) {
    const found = existing.find((m) => m.name === mapper.name);
    if (found) {
      await keycloakAdmin.clientScopes.updateProtocolMapper(
        { id: scope.id, mapperId: found.id },
        { ...mapper, id: found.id },
      );
      console.info('updated mapper %s in client scope %s', mapper.name, desired.name);
    } else {
      await keycloakAdmin.clientScopes.addProtocolMapper({ id: scope.id }, mapper);
      console.info('created mapper %s in client scope %s', mapper.name, desired.name);
    }
  }
  return scope;
}

async function attachHorizonApiClientScope(scope) {
  for (const clientId of horizonApiTokenClients) {
    const client = await findClient(clientId);
    const defaults = await keycloakAdmin.clients.listDefaultClientScopes({ id: client.id });
    if (defaults.some((s) => s.id === scope.id)) {
      console.info('client scope %s already a default scope of %s', scope.name, clientId);
      continue;
    }
    await keycloakAdmin.clients.addDefaultClientScope({ id: client.id, clientScopeId: scope.id });
    console.info('added client scope %s as a default scope of %s', scope.name, clientId);
  }
}

async function configureKeycloak() {
  const domain = (process.env.DOMAIN || '').replace(/\/$/, '');
  if (!domain) {
    throw new Error('DOMAIN is required');
  }

  await waitForKeycloak();
  await getRealm();
  await upsertClient(horizonPublicClient(domain));
  await upsertClient(horizonDevPortalClient(domain));
  await upsertClient(horizonCiClient());
  await writeHorizonCiClientSecretForJenkins();
  await ensureHorizonApiClientRoles();
  await mapHorizonApiClientRolesToGroups();
  await grantServiceAccountClientRoles();
  const scope = await ensureHorizonApiClientScope();
  await attachHorizonApiClientScope(scope);
  console.info('Horizon API Keycloak post-job finished.');
}

configureKeycloak().catch((err) => {
  console.error(err.message || err);
  process.exit(1);
});
