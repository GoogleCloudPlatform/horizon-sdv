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

import json
import logging
import os
from pathlib import Path
from enum import Enum, auto
from google.cloud import resourcemanager_v3


USER_KEYWORD = "user:"


class Operation:
    """Operations on Google Cloud Platform (GCP) Class

    Class for executing operations on Google Cloud Platform.
    """

    def __init__(self, service, project_id, logger, output_dir, exec_user_id):
        """Operation Class Constructor

        Args:
            service (googleapiclient.discovery.Resource): A Resource object with methods for interacting with the service
            project_id (string): project id
            logger (logging.Logger): logger
            output_dir (dir): Path to directory where to store generated, output files
            exec_user_id (str): Id of user who is executing the script
        """
        self.service = service
        self.resource = f'projects/{project_id}'
        self.__log = logger
        self.out_dir = output_dir
        self.exec_user_id = exec_user_id

    class OperationsKey(Enum):
        """Operation Keys Enum

        All possible keys in JSON file with list of operations to perform.
        """
        OPERATION = "operation"
        USER = "user"
        ROLE = "role"
        ROLES = "roles"

    class Operations(Enum):
        """Possible operations' ids Enum

        All possible operations' ids that can be performed.
        """
        GET_USER = auto()
        GET_ALL_ROLES = auto()
        GET_ALL_ROLES_WITH_USERS = auto()
        GET_ROLES_WITH_USERS = auto()
        GET_ROLE_INFO = auto()
        SET_ROLE_TO_USER = auto()
        DELETE_ROLE_FROM_USER = auto()
        DELETE_ALL_ROLES_FROM_USER = auto()

    def __get_roles_list(self):
        """Get all GCP roles

        Lists every predefined Role that IAM supports, or every custom role that is defined for an organization or project.

        Returns:
            list: List of all possible GCP roles
        """

        request = self.service.roles().list()
        roles_ls = []

        while True:
            response = request.execute()
            for role in response.get('roles', []):
                roles_ls.append(role)
            request = self.service.roles().list_next(
                previous_request=request, previous_response=response)
            if request is None:
                break

        return roles_ls

    def __get_role_info(self, role):
        """Get info about specified role

        Args:
            role (string): Role id. Format info: without the "roles/" keyword.

        Returns:
            dict:   Information about given role. Output includes:
                    "name", "title", "description", "includedPermissions", "stage", "etag"
        """

        name = f"roles/{role}"
        request = self.service.roles().get(name=name)
        response = request.execute()
        print(type(response))
        return response

    def __get_users_by_roles(self):
        """Retrieve all roles and users that are assigned to them.

        Returns:
            dict:   Format:
                    {role: [user1, user2]}
        """

        users_by_roles_dict = {}
        client = resourcemanager_v3.ProjectsClient()
        policy = client.get_iam_policy(request={"resource": self.resource})

        for binding in policy.bindings:
            role = binding.role
            members = binding.members
            users_by_roles_dict[role] = []
            for member in members:
                if member.startswith(USER_KEYWORD):
                    users_by_roles_dict[role].append(member)
            # Only show roles with assigned users
            if not users_by_roles_dict[role]:
                del users_by_roles_dict[role]

        return users_by_roles_dict

    def get_user_and_assigned_roles(self, user):
        """Retrieve roles assigned to given user.

        Args:
            user (string or list of strings): User id.
                Possible values:
                    1. "*" - Retrieve all users and roles that are assigned to them.
                    2. "user_id" - retrieve only information about given user.
                    3. "[user1_id, user2_id]" - retrieve information about given users. Users shall be provided as a list.

        Raises:
            ValueError: Raised if provided argument `user` is not in correct type.

        Returns:
            dict: Format:
                        {user: [role1, role2]}
        """

        users_by_roles = self.__get_users_by_roles()
        users_and_roles_dict = {}

        if user == "*":
            # Retrieve all users
            for role, users in users_by_roles.items():
                for user in users:
                    if user.startswith(USER_KEYWORD):
                        if user not in users_and_roles_dict:
                            users_and_roles_dict[user] = []
                        users_and_roles_dict[user].append(role)
        elif isinstance(user, str):
            # Retrieve only one user
            for role, users in users_by_roles.items():
                for u in users:
                    if user in u:
                        if user not in users_and_roles_dict:
                            users_and_roles_dict[user] = []
                        users_and_roles_dict[user].append(role)
        elif isinstance(user, list):
            # Retrieve information about given users.
            for role, users in users_by_roles.items():
                for u in user:
                    if f"{USER_KEYWORD}{u}" in users:
                        if u not in users_and_roles_dict:
                            users_and_roles_dict[u] = []
                        users_and_roles_dict[u].append(role)
        else:
            raise ValueError("Invalid user parameter. Must be '*', a user ID (string), or a list of user IDs.")

        return users_and_roles_dict

    def __get_given_roles_and_asigned_users(self, roles_list):
        """Retrieve roles and users assigned to them.

        Retrieve roles listed in roles_list variable and users that are assigned to them.

        Args:
            roles_list (list): list of roles id. Format info: role id without the "roles/" keyword.

        Returns:
            dict: Format:
                        {
                            "role1": [user1, user2],
                            "role2": [user3]
                        }
        """

        chosen_roles_dict = {}
        client = resourcemanager_v3.ProjectsClient()
        policy = client.get_iam_policy(request={"resource": self.resource})

        for binding in policy.bindings:
            role = binding.role
            role = role.replace("roles/", "")
            if role in roles_list:
                chosen_roles_dict[role] = []
                members = binding.members
                for member in members:
                    if member.startswith(USER_KEYWORD):
                        chosen_roles_dict[role].append(member)

        return chosen_roles_dict

    def add_role_to_user(self, user, role):
        """Add role for given user.

        Args:
            user (string): user id
            role (string): role id. Format info: role id without the "roles/" keyword.

        Returns:
            bool: operation status - If operation is successful return True.
        """

        client = resourcemanager_v3.ProjectsClient()
        policy = client.get_iam_policy(request={"resource": self.resource})
        role = f"roles/{role}"

        for binding in policy.bindings:
            if binding.role == role and f"{USER_KEYWORD}{user}" in binding.members:
                self.__log.debug(f"User {user} already has the role {role}.")
                return True

        binding = policy.bindings.add()
        binding.role = role
        binding.members.append(f"{USER_KEYWORD}{user}")

        client.set_iam_policy(
            request={
                "resource": self.resource,
                "policy": policy
            }
        )

        self.__log.debug(f"Added role {role} to user {user} in project {self.resource}.")
        return True

    def __remove_role_from_user(self, user, role_id):
        """Removes a specific role from a user in a GCP.

        Args:
            user (string): user id
            role_id (string): role id. Format info: role id without the "roles/" keyword.
        """

        if user == self.exec_user_id:
            self.__log.info(f"User {user} executes the script. Their permissions will not be deleted.")
        else:
            client = resourcemanager_v3.ProjectsClient()
            policy = client.get_iam_policy(request={"resource": self.resource})
            role_id = f"roles/{role_id}"

            for binding in policy.bindings:
                if binding.role == role_id:
                    if f"{USER_KEYWORD}{user}" in binding.members:
                        binding.members.remove(f"{USER_KEYWORD}{user}")
                        self.__log.debug(f"Removed {user} from {role_id}")
                    # Only keep bindings that still have members
                    if not binding.members:
                        policy.bindings.remove(binding)

            # Update policy bindings
            client.set_iam_policy(
                request={
                    "resource": self.resource,
                    "policy": policy
                }
            )

            self.__log.debug(f"Updated IAM policy for project {self.resource}.")

    def remove_all_roles_from_user(self, user):
        """Removes all roles from a user in a Google Cloud project.

        Args:
            user (string): user id
        """

        if user == self.exec_user_id:
            self.__log.info(f"User {user} executes the script. Their permissions will not be deleted.")
        else:
            client = resourcemanager_v3.ProjectsClient()
            policy = client.get_iam_policy(request={"resource": self.resource})

            for binding in policy.bindings[:]:
                if f"{USER_KEYWORD}{user}" in binding.members:
                    binding.members.remove(f"{USER_KEYWORD}{user}")
                    self.__log.debug(f"Removed {user} form {binding.role}")
                # Only keep bindings that still have members
                if not binding.members:
                    policy.bindings.remove(binding)

            # Update policy bindings
            client.set_iam_policy(
                request={
                    "resource": self.resource,
                    "policy": policy
                }
            )

            self.__log.debug(f"Updated IAM policy for project {self.resource}.")

    def __save_data_to_json_file(self, out_file_name, data):
        """Save provided data into JSON file.

        Args:
            out_file_name (string): Name of output file.
            data (object): Data to be saved.
        """

        output_path = Path(self.out_dir)
        output_path.mkdir(parents=True, exist_ok=True)
        file_path = output_path / out_file_name

        # Ensure the folder exists
        os.makedirs(self.out_dir, exist_ok=True)
        with file_path.open("w+", encoding="utf-8") as file:
            json.dump(data, file, indent=4)
        self.__log.info(f"Data is saved in a file  '{file_path}'.")

    def operations_handler(self, operation, **kwargs):
        """Handling operations.

        Managing and handling provided operation.

        Args:
            operation (string): Operation id. One of the values listed in enum self.Operations
            kwargs (string):    Provide other parameters that go with given operation.
                                Key is one of the values listed in enum self.OperationsKey

        Returns:
            bool: status of operation - If operation is successful return True.
        """

        return_status = False

        if operation in self.Operations.__members__:
            self.__log.info(f"Handling operation {operation}")

            if operation == self.Operations.GET_USER.name:
                try:
                    users_and_their_roles_dict = self.get_user_and_assigned_roles(user=kwargs[self.OperationsKey.USER.value])
                except ValueError as e:
                    self.__log.error(f"There is a problem with operation {operation} \nError: {e}")
                    return_status = False
                else:
                    self.__save_data_to_json_file(out_file_name="User_and_assigned_roles.json",
                                                  data=users_and_their_roles_dict)
                    return_status = True

            elif operation == self.Operations.GET_ALL_ROLES.name:
                roles_ls = self.__get_roles_list()
                self.__save_data_to_json_file(out_file_name="Roles.json", data=roles_ls)
                return_status = True

            elif operation == self.Operations.GET_ALL_ROLES_WITH_USERS.name:
                users_by_roles_dict = self.__get_users_by_roles()
                self.__save_data_to_json_file(out_file_name="Users_by_roles.json", data=users_by_roles_dict)
                return_status = True

            elif operation == self.Operations.GET_ROLES_WITH_USERS.name:
                chosen_roles_dict = self.__get_given_roles_and_asigned_users(
                    roles_list=kwargs[self.OperationsKey.ROLES.value])
                self.__save_data_to_json_file(out_file_name="Chosen_roles_with_asigned_users.json", data=chosen_roles_dict)
                return_status = True

            elif operation == self.Operations.GET_ROLE_INFO.name:
                role_info = self.__get_role_info(role=kwargs[self.OperationsKey.ROLE.value])
                self.__save_data_to_json_file(out_file_name="Role_info.json", data=role_info)
                return_status = True

            elif operation == self.Operations.SET_ROLE_TO_USER.name:
                self.add_role_to_user(user=kwargs[self.OperationsKey.USER.value], role=kwargs[self.OperationsKey.ROLE.value])
                return_status = True

            elif operation == self.Operations.DELETE_ROLE_FROM_USER.name:
                self.__remove_role_from_user(user=kwargs[self.OperationsKey.USER.value],
                                             role_id=kwargs[self.OperationsKey.ROLE.value])
                return_status = True

            elif operation == self.Operations.DELETE_ALL_ROLES_FROM_USER.name:
                self.remove_all_roles_from_user(user=kwargs[self.OperationsKey.USER.value])
                return_status = True
        else:
            self.__log.error(f"There is no such operation as {operation}")

        return return_status

    def retrieve_operations_list_from_json(self, operations_file_path):
        """Load operations list from file.

        Handling json file which contains list of operations that shall be performed.

        Args:
            operations_file_path (string):  Path to Json file with mapping between GCP roles to Horizon roles.
                                            File structure:
                                                [
                                                    {"operation": "GET_ROLES_WITH_USERS", "roles": ["role1", "role2"]},
                                                    {"operation": "GET_USER", "user": "*"},
                                                    {"operation": "GET_ALL_ROLES"}
                                                ]

        Returns:
            bool: operation_result - return True if operation was done successfully. Otherwise return False.
        """

        self.__log.info("------")
        self.__log.info(f"I will look through the list of operations in the provided file: {operations_file_path}")

        try:
            with open(operations_file_path, "r") as file:
                operations_list = json.load(file)
        except FileNotFoundError as e:
            self.__log.error(f"No such file or directory: {operations_file_path}. Cannot perform operations.")
            return False
        except json.decoder.JSONDecodeError as e:
            self.__log.error(f"File {operations_file_path} is incorrect. Check formatting. Cannot perform operations.")
            return False

        success_score = 0
        for op in operations_list:
            operation_name = op[self.OperationsKey.OPERATION.value]
            kwargs = {
                key.value: op[key.value] for key in self.OperationsKey if key != self.OperationsKey.OPERATION and key.value in op
            }
            if self.operations_handler(operation_name, **kwargs):
                success_score += 1

        num_failed_op = len(operations_list) - success_score
        if num_failed_op:
            self.__log.error(f"There were: {num_failed_op} incorrect operations.")
        else:
            self.__log.info(f"All operations were successful.")

        return True
