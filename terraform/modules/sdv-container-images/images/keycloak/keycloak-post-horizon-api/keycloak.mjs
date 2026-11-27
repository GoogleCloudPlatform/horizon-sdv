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

import { ensureStorageMirrorAdminRole } from './storage-mirror-role.mjs';

const REALM = 'horizon';

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
  await ensureStorageMirrorAdminRole(keycloakAdmin);
  console.info('Horizon API Keycloak post-job finished.');
}

configureKeycloak().catch((err) => {
  console.error(err.message || err);
  process.exit(1);
});
