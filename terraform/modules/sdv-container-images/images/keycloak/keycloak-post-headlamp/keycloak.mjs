// Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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

const config = {
  keycloak: {
    baseUrl: process.env.PLATFORM_URL + '/auth',
    username: process.env.KEYCLOAK_USERNAME,
    password: process.env.KEYCLOAK_PASSWORD,
    realm: {
      realm: 'horizon'
    },
    client: {
      clientId: 'oauth2-headlamp',
      adminUrl: process.env.DOMAIN + '/headlamp',
      redirectUris: [process.env.DOMAIN + '/headlamp/*'],
      protocol: 'openid-connect',
      publicClient: false
    },
    clientScope:{
      clientScopeName: 'groups'
    },
    rolesAndGroups: [
        'horizon-headlamp-administrators'
    ]
  }
}

const keycloakAdmin = new KcAdminClient({
  baseUrl: config.keycloak.baseUrl
});

// Check if DEBUG environment variable is set to 1.
const DEBUG = process.env.DEBUG === '1';

// Helper function that prints output to console only when DEBUG is 1.
function debugLog(...args) {
  if (DEBUG) console.log('[DEBUG]', ...args);
}

async function waitForKeycloak() {
  const opts = {
    retries: 100,
    minTimeout: 2000,
    factor: 1,
    onRetry: (err) => {console.info(`waiting for ${config.keycloak.baseUrl}...`, err.message)}
  };
  await retry(login, opts);
}

async function login()  {
  try {
    debugLog(`Attempt Keycloak login as`, config.keycloak.username);
    await keycloakAdmin.auth({
      'username': config.keycloak.username,
      'password': config.keycloak.password,
      'grantType': 'password',
      'clientId': 'admin-cli'
    });
    debugLog(`Keycloak login succeeded!`);
  } catch (err) {
    debugLog(`Keycloak authentication failed!`);
    debugLog(`Error message:`, err.message);
    debugLog(`Stack trace:`, err.stack);
    throw err;
  }
}

async function getRealm()  {
  try {;
    debugLog(`Fetching realm "${config.keycloak.realm.realm}" from Keycloak...`);
    let realm = await keycloakAdmin.realms.findOne({
      realm: config.keycloak.realm.realm,
    });
    if (!realm) {
      debugLog(`Realm "${config.keycloak.realm.realm}" not found!`);
    }
    debugLog(`Fetched realm: ${realm.realm}`);

    keycloakAdmin.setConfig({
      realmName: realm.realm,
    });
    debugLog(`Fetching keys for realm "${realm.realm}"...`);
    realm.keys = await keycloakAdmin.realms.getKeys({realm: realm.realm});
    debugLog(`Fetched ${realm.keys.keys?.length || 0} keys for realm "${realm.realm}".`);

    config.keycloak.realm = realm;
    debugLog(`Realm "${realm.realm}" successfully loaded and stored in config.`);
  } catch (err) {
    debugLog(`Error while fetching realm "${config.keycloak.realm.realm}!"`);
    debugLog(`Error Message:`, err.message)
    debugLog(`Stack Trace:`, err.stack);
    throw err;
  }
}

async function createClientIfRequired()  {
  try {
    debugLog(`Creating Client ${config.keycloak.client.clientId} if not existing already...`);
    let clients = await keycloakAdmin.clients.find();
    debugLog(`Clients fetched: ${clients.map(c => c.clientId).join(', ')}`);
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});

    if (client) {
      debugLog(`Client "${client.clientId}" already exists, updating client...`);
      console.info('updating %s client', config.keycloak.client.clientId);
      await keycloakAdmin.clients.update({id: client.id, realm: config.keycloak.realm.realm}, _.merge(client, config.keycloak.client));
    } else {
      console.info('creating %s client', config.keycloak.client.clientId);
      await keycloakAdmin.clients.create(config.keycloak.client);
      debugLog(`Client ${client} created successfully!`);
    }

    clients = await keycloakAdmin.clients.find();
    client = _.find(clients, {clientId: config.keycloak.client.clientId});

    config.keycloak.client = client;
    debugLog(`Client "${client.clientId}" successfully loaded and stored in config.`);
  } catch (err) {
    debugLog(`Error while updataing/creating Client ${config.keycloak.client.clientId}!`);
    debugLog(`Error Message:`, err.message);
    debugLog(`Error Stack:`, err.stack);
    throw err;
  }
}

async function generateSecretFiles()  {
  try {
    debugLog('Generating secrets file (client-headlamp.json) with Client secret...');
    let clients = await keycloakAdmin.clients.find();
    debugLog(`Clients fetched: ${clients.map(c => c.clientId).join(', ')}`);
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});

    if (client) {
      debugLog(`Client ${client.clientId} found, attempting to fetch client data...`);
      console.info('dumping %s client data into json file', config.keycloak.client.clientId);
      fs.writeFile('client-headlamp.json', JSON.stringify(client));
    }

  } catch (err) {
    debugLog('Error while generating client data file!');
    debugLog(`Error Message:`, err.message);
    debugLog(`Stack Trace:`, err.stack);
    throw err
  }
}

async function addGroupsClientScopeToHeadlampClientIfRequired() {
  debugLog(`Adding "${config.keycloak.clientScope.clientScopeName}" client scope to "${config.keycloak.client.clientId}" client if not existing already...`);
  const clientId = config.keycloak.client.clientId;
  const clientScopeName = config.keycloak.clientScope.clientScopeName;

  try {
    const clients = await keycloakAdmin.clients.find();
    debugLog(`Clients fetched: ${clients.map(c => c.clientId).join(', ')}`);
    const headlampClient = clients.find(client => client.clientId === clientId);

    if (!headlampClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const clientScopes = await keycloakAdmin.clientScopes.find();
    debugLog(`Client scopes fetched: ${clientScopes.map(cs => cs.name).join(', ')}`);
    const groupsScope = clientScopes.find(scope => scope.name === clientScopeName);
    
    if (!groupsScope) {
      console.error(`client scope "${clientScopeName}" does not exist.`);
      return;
    }

    const defaultScopes = await keycloakAdmin.clients.listDefaultClientScopes({ id: headlampClient.id });
    debugLog(`Existing Client scopes for Client ${clientId}: ${defaultScopes.map(ds => ds.name).join(', ')}`);
    const isGroupsScopeAssigned = defaultScopes.some(scope => scope.id === groupsScope.id);
    
    if (isGroupsScopeAssigned) {
      console.info(`"${config.keycloak.clientScope.clientScopeName}" client scope already exists in "${config.keycloak.client.clientId}" client.`);
    } else {
      console.log(`adding "${config.keycloak.clientScope.clientScopeName}" client scope to "${config.keycloak.client.clientId}" client.`);
      await keycloakAdmin.clients.addDefaultClientScope({id: headlampClient.id, clientScopeId: groupsScope.id,});
      debugLog(`"${config.keycloak.clientScope.clientScopeName}" client scope successfully added to "${clientId}" client`);
    }
  } catch (err) {
    debugLog(`Error while adding "${config.keycloak.clientScope.clientScopeName}" client scope to "${clientId}" client`);
    debugLog(`Error Message:`, err.message);
    debugLog(`Stack Trace:`, err.stack);
    throw err;
  }
}

async function createHeadlampRealmGroupsIfRequired() {
  debugLog(`Creating Keycloak realm group "${config.keycloak.rolesAndGroups}" if not existing already...`);
  const realmGroupNames = config.keycloak.rolesAndGroups;

  for (const realmGroupName of realmGroupNames) {
    try {
      const existingGroups = await keycloakAdmin.groups.find({ search: realmGroupName });
      const matchedGroup = existingGroups.find(group => group.name === realmGroupName);

      if (matchedGroup) {
        console.info(`group "${realmGroupName}" already exists.`);
      } else {
        console.log(`creating group "${realmGroupName}".`);
        await keycloakAdmin.groups.create({ name: realmGroupName });
      }
    } catch (err) {
      debugLog(`Error while creating group ${config.keycloak.rolesAndGroups}`);
      debugLog(`Error Message:`, err.message);
      debugLog(`Stack Trace: `, err.stack);
      throw err;
    }
  }
}

async function configureKeycloak()  {
  try {
    await waitForKeycloak();
    await getRealm();
    await createClientIfRequired();
    await generateSecretFiles();
    await addGroupsClientScopeToHeadlampClientIfRequired();
    await createHeadlampRealmGroupsIfRequired();
  } catch (err) {
    debugLog(`Keycloak configuration failed.`);
    debugLog(`Error Message:`, err.message);
    debugLog(`Stack Trace:`, err.stack);
    throw err;
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });