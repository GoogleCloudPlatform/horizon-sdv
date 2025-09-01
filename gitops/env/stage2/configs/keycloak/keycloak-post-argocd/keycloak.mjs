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
      clientId: 'argocd',
      adminUrl: process.env.DOMAIN + '/argocd',
      redirectUris: [process.env.DOMAIN + '/argocd/*'],
      protocol: 'openid-connect',
      publicClient: false
    },
    clientScope:{
      clientScopeName: 'groups'
    },
    rolesAndGroups: [
        'horizon-argocd-administrators'
    ]
  }
}

const keycloakAdmin = new KcAdminClient({
  baseUrl: config.keycloak.baseUrl
});

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
    await keycloakAdmin.auth({
      'username': config.keycloak.username,
      'password': config.keycloak.password,
      'grantType': 'password',
      'clientId': 'admin-cli'
    });
  } catch (err) {
    throw err
  }
}

async function getRealm()  {
  try {
    let realm = await keycloakAdmin.realms.findOne({
      realm: config.keycloak.realm.realm,
    });
    keycloakAdmin.setConfig({
      realmName: realm.realm,
    });
    realm.keys = await keycloakAdmin.realms.getKeys({realm: realm.realm});
    config.keycloak.realm = realm;
  } catch (err) {
    throw err
  }
}

async function createClientIfRequired()  {
  try {
    let clients = await keycloakAdmin.clients.find();
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});
    if (client) {
      console.info('updating %s client', config.keycloak.client.clientId);
      await keycloakAdmin.clients.update({id: client.id, realm: config.keycloak.realm.realm}, _.merge(client, config.keycloak.client));
    } else {
      console.info('creating %s client', config.keycloak.client.clientId);
      await keycloakAdmin.clients.create(config.keycloak.client);
    }
    clients = await keycloakAdmin.clients.find();
    client = _.find(clients, {clientId: config.keycloak.client.clientId});
    config.keycloak.client = client;
  } catch (err) {
    throw err
  }
}

async function generateSecretFiles()  {
  try {
    let clients = await keycloakAdmin.clients.find();
    let client = _.find(clients, {clientId: config.keycloak.client.clientId});

    if (client) {
      console.info('dumping %s client data into json file', config.keycloak.client.clientId);
      fs.writeFile('client-argocd.json', JSON.stringify(client));
    }

  } catch (err) {
    throw err
  }
}

async function addGroupsClientScopeToArgocdClientIfRequired() {
  const clientId = config.keycloak.client.clientId;
  const clientScopeName = config.keycloak.clientScope.clientScopeName;

  try {
    const clients = await keycloakAdmin.clients.find();
    const argocdClient = clients.find(client => client.clientId === clientId);

    if (!argocdClient) {
      console.error(`client "${clientId}" does not exist.`);
      return;
    }

    const clientScopes = await keycloakAdmin.clientScopes.find();
    const groupsScope = clientScopes.find(scope => scope.name === clientScopeName);
    
    if (!groupsScope) {
      console.error(`client scope "${clientScopeName}" does not exist.`);
      return;
    }

    const defaultScopes = await keycloakAdmin.clients.listDefaultClientScopes({ id: argocdClient.id });
    const isGroupsScopeAssigned = defaultScopes.some(scope => scope.id === groupsScope.id);
    
    if (isGroupsScopeAssigned) {
      console.info('"groups" client scope already exists in "argocd" client.');
    } else {
      console.log('adding "groups" client scope to "argocd" client.');
      await keycloakAdmin.clients.addDefaultClientScope({id: argocdClient.id, clientScopeId: groupsScope.id,});
    }
  } catch (err) {
    throw err;
  }
}

async function createArgocdRealmGroupsIfRequired() {
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
    await addGroupsClientScopeToArgocdClientIfRequired();
    await createArgocdRealmGroupsIfRequired();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });