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
      }
    ],
    client: {
      clientId: 'mtk-connect',
      adminUrl: process.env.DOMAIN + '/mtk-connect/saml/consume',
      redirectUris: [process.env.DOMAIN + '/mtk-connect/saml/consume'],
      protocol: 'saml',
      attributes: {
        'saml.assertion.signature': 'true'
      },
      fullScopeAllowed: false
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

async function createClientProtocolMappersIfRequired()  {
  try {
    let mappers = await keycloakAdmin.clients.listProtocolMappers({id: config.keycloak.client.id});
    let mappersToCreate = _.differenceWith(config.keycloak.mappers, mappers, (a, b) => {
      return a.name === b.name;
    });
    if (mappersToCreate.length > 0) {
      await Promise.all(mappersToCreate.map((mapper) => {
        console.info(`creating ${mapper.name} protocol mapper`);
        return keycloakAdmin.clients.addProtocolMapper({id: config.keycloak.client.id}, mapper)
      }));
    }
  } catch (err) {
    throw err
  }
}

async function generateSecretFiles()  {
  try {
    await fs.writeFile('idpCert.pem', '-----BEGIN CERTIFICATE-----\n' + _.find(config.keycloak.realm.keys.keys, {type: 'RSA', use: 'SIG'}).certificate + '\n-----END CERTIFICATE-----');
    await fs.writeFile('privateKey.pem', '-----BEGIN RSA PRIVATE KEY-----\n' + config.keycloak.client.attributes['saml.signing.private.key'] + '\n-----END RSA PRIVATE KEY-----');
  } catch (err) {
    throw err
  }
}

async function configureKeycloak()  {
  try {
    await waitForKeycloak();
    await getRealm();
    await createClientIfRequired();
    await createClientProtocolMappersIfRequired();
    await generateSecretFiles();
  } catch (err) {
    throw err
  }
}

configureKeycloak()
  .catch((err) => {
    console.error(err.message);
  });
