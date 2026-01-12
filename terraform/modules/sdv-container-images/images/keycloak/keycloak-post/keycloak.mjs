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

'use strict';

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
      realm: 'horizon',
      displayName: "Horizon",
      enabled: true,
      loginWithEmailAllowed: false,
      duplicateEmailsAllowed: true,
      resetPasswordAllowed: true,
      bruteForceProtected: true,
      failureFactor: 6,
      passwordPolicy: "forceExpiredPasswordChange(75) and specialChars(1) and passwordHistory(24) and upperCase(1) and lowerCase(1) and length(8) and digits(1) and notUsername(undefined) and regexPattern(^(?!.*(.)\\1\\1\\1\\1).*)",
    },
    mappers: [
      {
        name:'X500 email',
        protocol:'saml',
        protocolMapper:'saml-user-property-mapper',
        consentRequired:false,
        config:
          {
            'attribute.nameformat':'urn:oasis:names:tc:SAML:2.0:attrname-format:uri',
            'user.attribute':'email',
            'friendly.name':'email',
            'attribute.name':'urn:oid:1.2.840.113549.1.9.1'
          }
      },
      {
        protocol: 'openid-connect',
        name: 'groups',
        protocolMapper: 'oidc-group-membership-mapper',
        config: {
          'claim.name': 'groups',
          'full.group.path': 'false',
          'id.token.claim': 'true',
          'access.token.claim': 'true',
          'userinfo.token.claim': 'true',
          'introspection.token.claim': 'true',
        }
      }
    ],
    adminUser: {
      username: process.env.HORIZON_ADMIN_USERNAME,
      password: process.env.HORIZON_ADMIN_PASSWORD
    },
    clientScope:{
      clientScopeName: 'groups'
    }
  }
};

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

async function createRealmIfRequired()  {
  try {
    let realm = await keycloakAdmin.realms.findOne({
      realm: config.keycloak.realm.realm,
    });
    if (realm) {
      console.info('updating %s realm', config.keycloak.realm.realm);
      await keycloakAdmin.realms.update({realm: realm.realm}, config.keycloak.realm);
    } else {
      console.info('creating %s realm', config.keycloak.realm.realm);
      await keycloakAdmin.realms.create(config.keycloak.realm);
    }
    realm = await keycloakAdmin.realms.findOne({
      realm: config.keycloak.realm.realm
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

async function createAdminUserIfRequired()  {
  try {
    const userCount = await keycloakAdmin.users.count({realm: config.keycloak.realm.realm});
    if (userCount === 0) {
      console.info(`creating ${config.keycloak.adminUser.username} user`);
      const user = await keycloakAdmin.users.create({
        username: config.keycloak.adminUser.username,
        enabled: true,
        requiredActions: [],
        realm: config.keycloak.realm.realm
      });
      const role = await keycloakAdmin.roles.findOneByName({name: 'realm_admin'});
      await keycloakAdmin.users.addRealmRoleMappings({
        id: user.id,
        realm: config.keycloak.realm.realm,
        roles: [{id: role.id, name: role.name}]
      });
      await keycloakAdmin.users.resetPassword({
        id: user.id,
        realm: config.keycloak.realm.realm,
        credential: {temporary: true, type: 'password', value: config.keycloak.adminUser.password}
      });
    }
  } catch (err) {
    throw err
  }
}

async function createRealmAdminRoleIfRequired() {
  const parentRoleName = 'realm_admin';
  try {
    let parentRole = await keycloakAdmin.roles.findOneByName({name: parentRoleName});
    if (parentRole) {
      console.info(`role ${parentRoleName} exists`);
    } else {
      console.info(`creating ${parentRoleName} role`);
      await keycloakAdmin.roles.create({name: parentRoleName});
      let clients = await keycloakAdmin.clients.find();
      let childRole = await keycloakAdmin.clients.findRole({id: _.find(clients, {clientId: 'realm-management'}).id, roleName: 'realm-admin'});
      parentRole = await keycloakAdmin.roles.findOneByName({name: parentRoleName});
      await keycloakAdmin.roles.createComposite({roleId: parentRole.id}, [childRole]);
    }
  } catch (err) {
    throw err;
  }
}

async function createGroupsClientScopeIfRequired() {
  const clientScopeName = config.keycloak.clientScope.clientScopeName;

  try {
    let clientScope = await keycloakAdmin.clientScopes.findOneByName({name: clientScopeName});
    if (clientScope) {
      console.info(`client scope ${clientScopeName} exists`);
    } else {
      console.info(`creating client scope ${clientScopeName}`);
      await keycloakAdmin.clientScopes.create({
        name: clientScopeName,
        description: 'Provides access to user group information.',
        protocol: 'openid-connect'
      });
    }
  } catch (err) {
    throw err;
  } 
}

async function createGroupsMapperIfRequired() {
  try {
    const clientScopeName  = config.keycloak.clientScope.clientScopeName;

    let clientScope   = await keycloakAdmin.clientScopes.findOneByName({ name: clientScopeName });
    if (!clientScope) {
      console.warn(`client scope ${clientScopeName} does not exist.`);
    }

    const existing = await keycloakAdmin.clientScopes.listProtocolMappers({ id: clientScope.id });
    if (existing.some(map => map.name === 'groups')) {
      console.info('"groups" mapper already exists.');
      return;
    }

    const groupsMapper = config.keycloak.mappers.find(map => map.name === 'groups');
    if (!groupsMapper) {
      console.warn(`"groups" mapper not found in configuration.`);
      return;
    }

    await keycloakAdmin.clientScopes.addProtocolMapper({ id: clientScope.id }, groupsMapper);
    console.info('"groups" mapper added successfully.');

  } catch (err) {
    throw err;
  }
}

async function configureKeycloak()  {
  try {
    await waitForKeycloak();
    await createRealmIfRequired();
    await createRealmAdminRoleIfRequired();
    await createAdminUserIfRequired();
    await createGroupsClientScopeIfRequired();
    await createGroupsMapperIfRequired();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });
