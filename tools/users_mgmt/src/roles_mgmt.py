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
from enum import Enum


class RolesMgmt:
    """Roles Management Class

    Class for managing Horizon, Google Cloud Platform roles.
    """

    def __init__(self, operation_manager, logger, exec_user_id):
        """Roles Management Class Constructor

        Args:
            operation_manager (operations.Operation): Operations class instance
            logger (logging.Logger): logger
            exec_user_id (str): Id of user who is executing the script
        """
        self.__horizon_roles_list = []
        self.__operation_mgmt = operation_manager
        self.__log = logger
        self.exec_user_id = exec_user_id

    class HorizonRolesKey(Enum):
        """Horizon Roles Keys Enum

        All possible keys in JSON map file between Horizon roles and GCP roles.
        """
        NAME = "name"
        GCP_ROLE = "gcp_roles"

    class UsersKey(Enum):
        """Users Keys Enum

        All possible keys in JSON map file between users and Horizon roles.
        """
        USER = "user"
        HORIZON_ROLES_ROLE = "horizon_roles"
        GROUP_H_ROLE = "group_role"
        USERS = "users"

    def get_horizon_roles_list(self):
        """Retrieve list of mapping between Horizon roles and GCP roles.

        Returns:
            List: Horizon roles with according GCP roles.
        """
        return self.__horizon_roles_list

    def horizon_roles_mgmt(self, horizon_roles_file_path):
        """Loads Horizon role mappings from a JSON file.

        This function processes a JSON file that defines Horizon roles and their corresponding GCP role mappings.
        The loaded role definitions are stored in the global variable `self.__horizon_roles_list` for use in the script.

        Args:
            horizon_roles_file_path (string):   Path to a JSON file containing mappings between GCP roles and Horizon roles.
                                                The file should have the following structure:
                                                [
                                                    {
                                                        "name": "horizon_role_name",
                                                        "gcp_roles": [
                                                            "roles/gcp_role_id",
                                                            "roles/another_gcp_role_id"
                                                        ]
                                                    }
                                                ]

        Returns:
            bool: operation_result - return True if operation was done successfully. Otherwise return False.
        """

        self.__log.info("------")
        self.__log.info(f"Loading Horizon roles definitions from the provided file: {horizon_roles_file_path}")
        operation_result = False

        try:
            with open(horizon_roles_file_path, "r") as file:
                self.__horizon_roles_list = json.load(file)
                operation_result = True
        except FileNotFoundError as e:
            self.__log.error(f"No such file or directory: {horizon_roles_file_path}. Cannot load Horizon roles mapping.")
            operation_result = False
        except json.decoder.JSONDecodeError as e:
            self.__log.error(f"File {horizon_roles_file_path} is incorrect. Check formatting. Cannot perform operations.")
            operation_result = False

        return operation_result

    def __check_single_user_configuration(self, user_conf):
        """Check provided configuration for single user.

        Check provided configuration for single user with configuration in GCP. User has assigned list of Horizon roles.

        Args:
            user_conf (dict): single dictionary entry from JSON file with user-horizon roles mapping.

        Returns:
            bool: comparison_result - True if configuration match, False if they do not match.
        """

        comparison_result = False
        user_id = user_conf[self.UsersKey.USER.value]
        for horizon_role in user_conf[self.UsersKey.HORIZON_ROLES_ROLE.value]:
            # Check if provided horizon role is correct
            if any(hr[self.HorizonRolesKey.NAME.value] == horizon_role for hr in self.__horizon_roles_list):
                expected_gcp_roles = next((hr[self.HorizonRolesKey.GCP_ROLE.value]
                                          for hr in self.__horizon_roles_list if hr[self.HorizonRolesKey.NAME.value] == horizon_role), None)

                if expected_gcp_roles:
                    self.__log.debug(
                        f"User {user_id} is expected to have assigned following GCP roles: {expected_gcp_roles}")

                    # Check if user has those GCP roles assigned
                    actual_user_gcp_roles_dict = self.__operation_mgmt.get_user_and_assigned_roles(user=user_id)

                    # User already has some roles assigned
                    if user_id in actual_user_gcp_roles_dict:
                        self.__log.debug(
                            f"User {user_id} already has following GCP roles: {actual_user_gcp_roles_dict[user_id]}")
                    else:
                        self.__log.debug(f"User {user_id} has no GCP roles assigned.")

                    # Compare expected GCP roles with actually assigned user's roles
                    expected_gcp_roles.sort()
                    actual_user_gcp_roles_dict[user_id].sort()

                    if actual_user_gcp_roles_dict[user_id] == expected_gcp_roles:
                        comparison_result = True
                    else:
                        comparison_result = False
                        # If the comparison is already showing configs don't match, no need to check further
                        break

            else:
                self.__log.error(f"Provided horizon role {horizon_role} is incorrect. Make it correct.")
                comparison_result = None

        return comparison_result

    def __check_users_group_configuration(self, group_conf):
        """Check provided configuration for group of users.

        Check provided configuration for group of users with configuration in GCP. Single Horizon role has assigned list of users.

        Args:
            user_conf (dict): single dictionary entry from JSON file with horizon role - users mapping.

        Returns:
            bool: comparison_result - True if configuration match, False if they do not match.
        """

        comparison_result = False
        horizon_role = group_conf[self.UsersKey.GROUP_H_ROLE.value]

        # Check if provided horizon role is correct
        if any(hr[self.HorizonRolesKey.NAME.value] == horizon_role for hr in self.__horizon_roles_list):
            expected_gcp_roles = next((hr[self.HorizonRolesKey.GCP_ROLE.value]
                                      for hr in self.__horizon_roles_list if hr[self.HorizonRolesKey.NAME.value] == horizon_role), None)

            if expected_gcp_roles:
                self.__log.debug(f"Users are expected to have assigned following GCP roles: {expected_gcp_roles}")

                for user_id in group_conf[self.UsersKey.USERS.value]:
                    # Check if user has those GCP roles assigned
                    actual_user_gcp_roles_dict = self.__operation_mgmt.get_user_and_assigned_roles(user=user_id)

                    # User already has some roles assigned
                    if user_id in actual_user_gcp_roles_dict:
                        self.__log.debug(
                            f"User {user_id} already has following GCP roles: {actual_user_gcp_roles_dict[user_id]}")
                    else:
                        self.__log.debug(f"User {user_id} has no GCP roles assigned.")

                    # Compare expected GCP roles with actually assigned user's roles
                    expected_gcp_roles.sort()
                    actual_user_gcp_roles_dict[user_id].sort()

                    if actual_user_gcp_roles_dict[user_id] == expected_gcp_roles:
                        comparison_result = True
                    else:
                        comparison_result = False
                        # If the comparison is already showing configs don't match, no need to check further
                        break
        else:
            self.__log.error(f"Provided horizon role {horizon_role} is incorrect. Make it correct.")
            comparison_result = None

        return comparison_result

    def check_users_configurations(self, users_list_file_path):
        """Compare users' roles configurtions in provided json file and in Google Cloud Platform.

        Args:
            users_list_file_path (string):  Path to Json file with mapping between GCP roles to Horizon roles.
                                            File structure:
                                            [
                                                {
                                                    "user": "user@email.com",
                                                    "horizon_roles": ["horizon_role_name"]
                                                },
                                                {
                                                    "group_role": "horizon_role_name",
                                                    "users":["user@accenture.com"]
                                                }
                                            ]

        Returns:
            bool: comparison_result - result of comparison. True if configuration match, False if they do not match.
        """

        self.__log.info("------")
        self.__log.info(f"Checking Users roles configuration from the provided file: {users_list_file_path}")

        comparison_result = False

        try:
            with open(users_list_file_path, "r") as file:
                users_conf_list = json.load(file)
        except FileNotFoundError as e:
            self.__log.error(f"No such file or directory: {users_list_file_path}. Users roles mapping cannot be performed.")
            raise
        except json.decoder.JSONDecodeError as e:
            self.__log.error(f"File {users_list_file_path} is incorrect. Check formatting. Cannot perform operations.")
            raise

        self.__log.debug(f"======")
        for conf_dict in users_conf_list:
            if self.UsersKey.USER.value in conf_dict:
                comparison_result = self.__check_single_user_configuration(user_conf=conf_dict)
            elif self.UsersKey.GROUP_H_ROLE.value in conf_dict:
                comparison_result = self.__check_users_group_configuration(group_conf=conf_dict)
            self.__log.debug(f"======")

            # If the comparison is already showing configs don't match, no need to check further
            if not comparison_result:
                break
        if comparison_result:
            self.__log.info(f"Configurations match.")
        else:
            self.__log.info(f"Configurations do NOT match.")

        return comparison_result

    def __asign_roles_to_single_user(self, user_conf, force_overwrite):
        """Asign roles to user according to configuration.

        Assign proper GCP roles to single user according to provided configuration.

        Args:
            user_conf (dict): single dictionary entry from JSON file with user-horizon roles mapping.
            force_overwrite (bool): if True - overwrite existing user's roles. If False - assign new roles as addition to existing ones.

        Returns:
            bool: operation_result - return True if operation was done successfully. Otherwise return False.
        """

        operation_result = False
        user_id = user_conf[self.UsersKey.USER.value]

        for horizon_role in user_conf[self.UsersKey.HORIZON_ROLES_ROLE.value]:
            # Check if provided horizon role is correct
            if any(hr[self.HorizonRolesKey.NAME.value] == horizon_role for hr in self.__horizon_roles_list):
                # Retreive GCP roles assigned to given Horizon role
                gcp_roles = next((hr[self.HorizonRolesKey.GCP_ROLE.value]
                                 for hr in self.__horizon_roles_list if hr[self.HorizonRolesKey.NAME.value] == horizon_role), None)

                if gcp_roles:
                    self.__log.debug(f"User: {user_id} will be now assigned with roles {gcp_roles}")

                    # Check if user has those GCP roles assigned
                    user_gcp_roles_dict = self.__operation_mgmt.get_user_and_assigned_roles(user=user_id)

                    # User already has some roles assigned
                    if user_id in user_gcp_roles_dict:
                        self.__log.debug(f"User: {user_id} already has gcp roles: {user_gcp_roles_dict[user_id]}")

                        # Check `force` flag. If it is True, overwrite user's existing roles.
                        if force_overwrite and self.exec_user_id != user_id:
                            self.__log.debug("User's roles will be overwritten.")
                            self.__operation_mgmt.remove_all_roles_from_user(user=user_id)
                        else:
                            self.__log.debug("User's roles will not be overwritten.")

                    else:
                        self.__log.debug(f"User: {user_id} has no gcp roles assigned.")

                    # Add roles to the user
                    for gcp_r in gcp_roles:
                        self.__operation_mgmt.add_role_to_user(user=user_id, role=gcp_r.replace("roles/", ""))

                    operation_result = True
            else:
                self.__log.error(f"Provided horizon role {horizon_role} is incorrect. Make it correct.")
                operation_result = False
        return operation_result

    def __asign_role_to_group_of_users(self, group_conf, force_overwrite):
        """Asign role to group of users according to configuration.

        Assign proper GCP roles to group of users according to provided configuration.

        Args:
            group_conf (dict): single dictionary entry from JSON file with horizon role - users mapping.
            force_overwrite (bool): if True - overwrite existing user's roles. If False - assign new roles as addition to existing ones.

        Returns:
            bool: operation_result - return True if operation was done successfully. Otherwise return False.
        """

        operation_result = False
        horizon_role = group_conf[self.UsersKey.GROUP_H_ROLE.value]

        # Check if provided horizon role is correct
        if any(hr[self.HorizonRolesKey.NAME.value] == horizon_role for hr in self.__horizon_roles_list):
            # Retreive GCP roles assigned to given Horizon role
            gcp_roles = next((hr[self.HorizonRolesKey.GCP_ROLE.value]
                             for hr in self.__horizon_roles_list if hr[self.HorizonRolesKey.NAME.value] == horizon_role), None)

            if gcp_roles:
                self.__log.debug(f"Users will be now assigned with roles {gcp_roles}")
                for user_id in group_conf[self.UsersKey.USERS.value]:

                    # Check if user has those GCP roles assigned
                    user_gcp_roles_dict = self.__operation_mgmt.get_user_and_assigned_roles(user=user_id)

                    # User already has some roles assigned
                    if user_id in user_gcp_roles_dict:
                        self.__log.debug(f"User: {user_id} already has gcp roles: {user_gcp_roles_dict[user_id]}")

                        # Check `force` flag. If it is True, overwrite user's existing roles.
                        if force_overwrite and self.exec_user_id != user_id:
                            self.__log.debug("User's roles will be overwritten.")
                            self.__operation_mgmt.remove_all_roles_from_user(user=user_id)
                        else:
                            self.__log.debug("User's roles will not be overwritten.")

                    else:
                        self.__log.debug(f"User: {user_id} has no gcp roles assigned.")

                    # Add roles to the user
                    for gcp_r in gcp_roles:
                        self.__operation_mgmt.add_role_to_user(user=user_id, role=gcp_r.replace("roles/", ""))

                    operation_result = True
        else:
            self.__log.error(f"Provided horizon role {horizon_role} is incorrect. Make it correct.")
            operation_result = False

        return operation_result

    def users_mgmt(self, users_list_file_path, force_overwrite=False):
        """Assign roles to users according to JSON configuration.

        Handling json file which contains list of users and assigned to them Horizon roles that they shall have.
        Asign roles as it is described in the provided file.


        Args:
            users_list_file_path (string):  Path to Json file with mapping between GCP roles to Horizon roles.
                                            File structure:
                                                [
                                                    {
                                                        "user": "user@email.com",
                                                        "horizon_roles": ["horizon_role_name"]
                                                    },
                                                    {
                                                        "group_role": "horizon_role_name",
                                                        "users":["user@accenture.com"]
                                                    }
                                                ]
            force_overwrite (bool): Flag used for users' roles mapping.
                                If flag is set than user's roles will be overwritten.
                                If flag is not set roles will be added to user with no ingeration to their existing roles.

        Returns:
            bool: operation_result - return True if operation was done successfully. Otherwise return False.
        """

        self.__log.info("------")
        self.__log.info(f"Loading users <-> horizon roles mapping list from the provided file: {users_list_file_path}")

        operation_result = False

        try:
            with open(users_list_file_path, "r") as file:
                users_conf_list = json.load(file)

            operation_result = True
        except FileNotFoundError as e:
            self.__log.error(f"No such file or directory: {users_list_file_path}. Users roles mapping cannot be performed.")
            operation_result = False
        except json.decoder.JSONDecodeError as e:
            self.__log.error(f"File {users_list_file_path} is incorrect. Check formatting. Cannot perform operations.")
            operation_result = False

        if operation_result:
            self.__log.debug(f"======")
            for conf_dict in users_conf_list:
                if self.UsersKey.USER.value in conf_dict:
                    operation_result = self.__asign_roles_to_single_user(
                        user_conf=conf_dict, force_overwrite=force_overwrite)
                elif self.UsersKey.GROUP_H_ROLE.value in conf_dict:
                    operation_result = self.__asign_role_to_group_of_users(
                        group_conf=conf_dict, force_overwrite=force_overwrite)
                self.__log.debug(f"======")

        if operation_result:
            self.__log.info(f"Mapping users <-> horizon roles was successful.")
        else:
            self.__log.error(f"Mapping users <-> horizon roles was NOT successful.")

        return operation_result
