# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import requests
import datetime
from dateutil.relativedelta import relativedelta
import base64
import json
from kubernetes import client, config
import argparse
import os

USERNAME = ""
KEY_VAL = ""
USER_ID = ""
URL_DOMAIN = ""
OLD_KEY_ID_LS = []
OLD_KEY_VAL = ''

KEY_DEL_TIME_DELTA = 2 # in days
NAMESPACE_MTK_CONNECT = "mtk-connect"
SECRET_NAME_MTK_CONNECT = "mtk-connect-apikey"
NAMESPACE_JENKINS = "jenkins"
SECRET_NAME_JENKINS = "jenkins-mtk-connect-apikey"

API_REQUEST_OPT = {
  "GET_VERSION": "https://URL_DOMAIN/mtk-connect/api/v1/config/version",
  "GET_CURRENT_USER": "https://URL_DOMAIN/mtk-connect/api/v1/users/USER_ID",
  "CREATE_KEY": "https://URL_DOMAIN/mtk-connect/api/v1/users/USER_ID/keys",
  "DELETE_KEY": "https://URL_DOMAIN/mtk-connect/api/v1/users/USER_ID/keys/",
  "GET_USER_DETAILS": "https://URL_DOMAIN/mtk-connect/api/v1/users?q=%7B%22username%22%3A%20%22mtk-connect-admin%22%7D"
}

def update_request_urls(upd_domain=False, upd_user_id=False):
  global API_REQUEST_OPT
  for key in API_REQUEST_OPT:
    if upd_domain:
      API_REQUEST_OPT[key] = API_REQUEST_OPT[key].replace("URL_DOMAIN", f"{URL_DOMAIN}")
    if upd_user_id:
      API_REQUEST_OPT[key] = API_REQUEST_OPT[key].replace("USER_ID", f"{USER_ID}")
  return True

def get_keys_id_ls_to_delete(key_list, current_key):
  """
  Searches the key_list and finds ale the keysthat are two days older than the current key (current_key variable),
  returns list of key's ids.
  """   
  key_id_ls = []
  threshold_date = 0

  # Get creation date of current_key
  try:
    for key in key_list:
      if key["key"][:8] == current_key[:8]:
        threshold_date = datetime.datetime.strptime(key["creationTime"], "%Y-%m-%dT%H:%M:%S.%fZ")
        
        # Calculate what is the threshold datefor keys deletion
        threshold_date -= relativedelta(days=KEY_DEL_TIME_DELTA)
        print(f"keys older or equal to will be deleted: {threshold_date.date()}\n")
        break
  except Exception as e:
    print(f"Exception occured when getting key id. \n\tException: {e}")

  # Create list of keys id that older or equal than threshold date 
  for key in key_list:
    key_creation_time = datetime.datetime.strptime(key["creationTime"], "%Y-%m-%dT%H:%M:%S.%fZ")
    if key_creation_time.date() <= threshold_date.date():
      key_id_ls.append(key["id"])

  return key_id_ls

def perform_api_request(operation=API_REQUEST_OPT["GET_VERSION"], is_delete_key_id=False):
  """
  Sends request to api. Possible actions are listed in API_REQUEST_OPT. 
  Returns result flag: True if operation was succesfull.
  """
  global KEY_VAL, OLD_KEY_VAL, OLD_KEY_ID_LS, USER_ID
  result = False

  try:
    if operation == "GET_VERSION":
      print("\nGet version of MTK Connect")
      response_api = requests.get(API_REQUEST_OPT["GET_VERSION"], auth=(USERNAME, KEY_VAL))

    elif operation == "GET_CURRENT_USER":
      # Gets user data and stores value of the old key id, which will be used to delete the key
      print("\nGet current user data")
      response_api = requests.get(API_REQUEST_OPT["GET_CURRENT_USER"], auth=(USERNAME, KEY_VAL))
      if is_delete_key_id and (response_api.status_code // 100 == 2):
        # OLD_KEY_ID = get_key_id(response_api.json()["data"]["keys"], OLD_KEY_VAL)
        OLD_KEY_ID_LS = get_keys_id_ls_to_delete(response_api.json()["data"]["keys"], KEY_VAL)
        print(f"----\nKeys for deletion:\n{OLD_KEY_ID_LS}\n----")

    elif operation == "CREATE_KEY":
      print(f"\nCreate key for user id {USER_ID}")
      # New key settings: name, expiration date.
      # Current settings: name is empty, expiration date is set to 1 month after creation date (UTC+00:00 Timezone)
      key_expiration_date = datetime.datetime.now(tz=datetime.timezone.utc) + relativedelta(months=1)
      key_create_request_body = {
        "name": "",
        "expiryTime": f"{key_expiration_date.strftime('%Y-%m-%dT%H:%M:%SZ')}"
      }
      response_api = requests.post(API_REQUEST_OPT["CREATE_KEY"], auth=(USERNAME, KEY_VAL), json=key_create_request_body)
      if response_api.status_code // 100 == 2:
        OLD_KEY_VAL = KEY_VAL
        KEY_VAL = response_api.json()["data"]["key"]

      print(f"Thoreticaly key created. Is it saved? Key: {KEY_VAL}, old key: {OLD_KEY_VAL}")

    elif operation == "DELETE_KEY":
      print(f"\nDeleting keys id: {OLD_KEY_ID_LS}")
      for delete_key_id in OLD_KEY_ID_LS:
        response_api = requests.delete(API_REQUEST_OPT["DELETE_KEY"]+str(delete_key_id), auth=(USERNAME, KEY_VAL))
      else:
        print("There are no keys to delete.")

    elif operation == "GET_USER_DETAILS":
      print("\nRetreiving detail info for mtk-connect-admin user")
      response_api = requests.get(API_REQUEST_OPT["GET_USER_DETAILS"], auth=(USERNAME, KEY_VAL))
      if response_api.status_code // 100 == 2:
        USER_ID = response_api.json()["data"][0]["id"]

    else:
      raise KeyError(f"Operation '{operation}' not found.")

  except Exception as e:
    print(f"Exception occured when requesting response from API. \n\t{e}")
  else:
    reponse_category = response_api.status_code // 100
    if reponse_category == 2:
      print(f"Operation done succesfully!")
      result = True
    elif reponse_category == 3:
      print(f"Redirection!")
    elif reponse_category == 4:
      print(f"Client Error!")
    elif reponse_category == 5:
      print(f"Server Error!")
    else:
      print(f"There was a problem!")

    print(f"Status code: {response_api.status_code} \nResponse from API: \n\t{response_api.text}")
    print("Request to API finished")
    return result

def retrieve_secret_value(secret_key):
  """
  Retrieves the value of a specific key from a Kubernetes Secret.
  """
  secret_value = os.getenv(secret_key).strip()
  if secret_value:
    print(f"Retrieved secret value from environment variable: {secret_value}")
  else:
    print("Failed to retrieve secret value from environment variable.")
  return secret_value

def update_secret_value(secret_name, namespace, new_value, key="password"):
  """
  Updates the value of a specific key in a Kubernetes Secret.
  Returns result flag: True if operation was succesfull.
  """
  result = False
  print("\nUpdating secret value.")
  try:
    config.load_incluster_config()
    print("Using in-cluster configuration.")
  except config.ConfigException:
    config.load_kube_config()
    print("Using kubeconfig file.")
  v1 = client.CoreV1Api()
  
  try:
    # Fetch the existing secret
    secret = v1.read_namespaced_secret(name=secret_name, namespace=namespace)
    
    # Update the value of the specified key
    if secret.data is None:
      secret.data = {}  # Ensure `data` exists
    
    # Encode the new value to base64
    secret.data[key] = base64.b64encode(new_value.encode()).decode()
    
    # Update the secret in Kubernetes
    v1.replace_namespaced_secret(name=secret_name, namespace=namespace, body=secret)
    print(f"Updated key '{key}' in secret '{secret_name}' with new value.{secret}")
  except client.exceptions.ApiException as e:
    if e.status == 404:
        print(f"Secret '{secret_name}' not found in namespace '{namespace}'.")
    else:
        raise e
  else:
    result = True
  finally:
    return result

if __name__ == "__main__":
  print("Script start")

  operation_result = False

  parser = argparse.ArgumentParser(description="Run the script with parameters. Required parameters: domain")
  parser.add_argument("--api-domain", type=str, required=True, help="API domain")
  args = parser.parse_args()
  URL_DOMAIN = vars(args)["api_domain"]
  USERNAME = retrieve_secret_value("MTK_KEY_UPD_USERNAME")
  KEY_VAL = retrieve_secret_value("MTK_KEY_UPD_PASSWORD")

  if (USERNAME and KEY_VAL):
    operation_result = update_request_urls(upd_domain=True)

  if operation_result:
    operation_result = perform_api_request(operation="GET_USER_DETAILS")

  if operation_result:
    operation_result = update_request_urls(upd_user_id=True)

  if operation_result:
    operation_result = perform_api_request(operation="CREATE_KEY")

  if operation_result:
    operation_result = update_secret_value(SECRET_NAME_MTK_CONNECT, NAMESPACE_MTK_CONNECT, KEY_VAL)
    
  if operation_result:
    operation_result = update_secret_value(SECRET_NAME_JENKINS, NAMESPACE_JENKINS, KEY_VAL)

  if operation_result:
    operation_result = perform_api_request(operation="GET_CURRENT_USER", is_delete_key_id=True)

  if operation_result and OLD_KEY_ID_LS:
    operation_result = perform_api_request(operation="DELETE_KEY")

  if operation_result:
    operation_result = perform_api_request(operation="GET_CURRENT_USER")

  print("Script end")
