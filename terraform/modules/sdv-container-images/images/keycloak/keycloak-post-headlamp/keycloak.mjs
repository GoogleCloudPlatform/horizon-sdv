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
    ClientRoles: [
      'administrators'
    ]
  }
};

const clientRoleToGroup = {
  administrators: ['administrators']
};

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
    throw err
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
    throw err
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
    throw err
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

async function DisableFullScopeIfRequired() {
  const clientId = config.keycloak.client.clientId;

  try {
    const clients = await keycloakAdmin.clients.find();
    const headlampClient = clients.find(client => client.clientId === clientId);

    if (!headlampClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    if (headlampClient.fullScopeAllowed === false) {
      console.info(`"Full scope allowed" is already disabled for client "${clientId}".`);
    } else {
      console.log(`disabling "Full scope allowed" for client "${clientId}".`);
      await keycloakAdmin.clients.update(
        { id: headlampClient.id, realm: config.keycloak.realm.realm },
        { ...headlampClient, fullScopeAllowed: false }
      );
    }
  } catch (err) {
    throw err;
  }
}

async function createHeadlampClientRolesIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.ClientRoles;

  try {
    const clients = await keycloakAdmin.clients.find();
    const headlampClient = clients.find(client => client.clientId === clientId);

    if (!headlampClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const existingRoles = await keycloakAdmin.clients.listRoles({ id: headlampClient.id });

    for (const roleName of clientRoleNames) {
      const roleExists = existingRoles.some(role => role.name === roleName);
      if (roleExists) {
        console.info(`client role "${roleName}" already exists for "${clientId}".`);
        continue;
      }

      await keycloakAdmin.clients.createRole({id: headlampClient.id, name: roleName});
      console.log(`client role "${roleName}" created for client "${clientId}".`);
    }
  } catch (err) {
    throw err;
  }
}

async function mapHeadlampClientRolesToGroupsIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientRoleNames = config.keycloak.ClientRoles;

  try {
    const clients = await keycloakAdmin.clients.find();
    const headlampClient = clients.find(client => client.clientId === clientId);
    
    if (!headlampClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const allGroups = await keycloakAdmin.groups.find();

    for (const clientRoleName of clientRoleNames) {
      const clientRole = await keycloakAdmin.clients.findRole({id: headlampClient.id, roleName: clientRoleName});

      if (!clientRole) {
        console.warn(`client role "${clientRoleName}" does not exist in "${clientId}".`);
        continue;
      }

      const groupNames = clientRoleToGroup[clientRoleName];
      if (!groupNames) {
        console.warn(`no group mapping defined for client role "${clientRoleName}".`);
        continue;
      }

      const names = Array.isArray(groupNames) ? groupNames : [groupNames];
      for (const groupName of names) {
        const group = allGroups.find(g => g.name === groupName);

        if (!group) {
          console.warn(`group "${groupName}" does not exist.`);
          continue;
        }

        const mappedRoles = await keycloakAdmin.groups.listClientRoleMappings({id: group.id, clientUniqueId: headlampClient.id});
        const alreadyMapped = mappedRoles.some(role => role.name === clientRole.name);

        if (alreadyMapped) {
          console.info(`client role "${clientRoleName}" is already mapped to group "${groupName}".`);
          continue;
        }

        await keycloakAdmin.groups.addClientRoleMappings({
          id: group.id,
          clientUniqueId: headlampClient.id,
          roles: [{
            id: clientRole.id,
            name: clientRole.name
          }]
        });
        console.log(`client role "${clientRoleName}" mapped to group "${groupName}".`);
      }
    }
  } catch (err) {
    throw err;
  }
}

async function configureKeycloak()  {
  try {
    await waitForKeycloak();
    await getRealm();
    await createClientIfRequired();
    await generateSecretFiles();
    await DisableFullScopeIfRequired();
    await createHeadlampClientRolesIfRequired();
    await mapHeadlampClientRolesToGroupsIfRequired();
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