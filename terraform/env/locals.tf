# Copyright (c) 2026 Accenture, All Rights Reserved.
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

locals {

  # Password policies per secret (adjust lengths/policy per need)
  secret_password_specs = {
    # s5 -> argocd-admin-password-b64
    s5 = { length = 25, min_lower = 2, min_upper = 2, min_numeric = 2, min_special = 2 }

    # s6 -> jenkins-admin-password-b64
    s6 = { length = 25, min_lower = 2, min_upper = 2, min_numeric = 2, min_special = 2 }

    # s9 -> gerrit-admin-password-b64
    s9 = { length = 25, min_lower = 2, min_upper = 2, min_numeric = 2, min_special = 2 }

    # s12 -> grafana-admin-password-b64 
    s12 = { length = 25, min_lower = 2, min_upper = 2, min_numeric = 2, min_special = 2 }

    # s14 -> postgres-admin-password-b64
    s14 = { length = 25, min_lower = 2, min_upper = 2, min_numeric = 2, min_special = 2 }

    # s17 -> mcp-gateway-registry-admin-password-b64
    s17 = { length = 25, min_lower = 2, min_upper = 2, min_numeric = 2, min_special = 2 }
  }

  # Identify which keys require auto-generation
  ids_to_generate = setsubtract(keys(local.secret_password_specs), nonsensitive(keys(var.manual_secrets)))

  # Check if user has set values manually
  resolved_secret_values = {
    for k, spec in local.secret_password_specs : k =>
    lookup(var.manual_secrets, k, null) != null ? var.manual_secrets[k] : random_password.pw[k].result
  }

  password_policy_error = <<EOT
Password must be at least 12 characters long and include:
- At least one uppercase letter [A-Z]
- At least one lowercase letter [a-z]
- At least one number [0-9]
- At least one symbol [!@#$%^&* etc.]
- No whitespace characters
EOT

  sdv_gcp_common_secrets_map = {
    s4 = {
      secret_id        = "keycloak-idp-client-secret"
      value            = "dummy"
      use_github_value = false
      gke_access = [
        {
          ns = "keycloak"
          sa = "keycloak-sa"
        }
      ]
    }
    s5 = {
      secret_id        = "argocd-admin-password-b64"
      value            = base64encode(bcrypt(local.resolved_secret_values["s5"]))
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
      ]
    }
    s6 = {
      secret_id        = "jenkins-admin-password-b64"
      value            = base64encode(local.resolved_secret_values["s6"])
      use_github_value = false
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        },
      ]
    }
    s7 = {
      secret_id        = "keycloak-admin-password-b64"
      value            = base64encode(var.sdv_keycloak_admin_password)
      use_github_value = true
      gke_access = [
        {
          ns = "keycloak"
          sa = "keycloak-sa"
        },
      ]
    }
    # GCP secret name:  gerrit-admin-initial-password
    # WI to GKE at ns/gerrit/sa/gerrit-sa.
    s9 = {
      secret_id        = "gerrit-admin-password-b64"
      value            = base64encode(local.resolved_secret_values["s9"])
      use_github_value = false
      gke_access = [
        {
          ns = "gerrit"
          sa = "gerrit-sa"
        }
      ]
    }
    # GCP secret name:  gh-gerrit-admin-private-key
    # WI to GKE at ns/gerrit/sa/gerrit-sa.
    s10 = {
      secret_id        = "gerrit-admin-ssh-key-b64"
      value            = base64encode(module.gerrit_admin_key.private_key_openssh)
      use_github_value = false
      gke_access = [
        {
          ns = "gerrit"
          sa = "gerrit-sa"
        }
      ]
    }
    # GCP secret name:  gh-cuttlefish-vm-ssh-private-key
    # WI to GKE at ns/jenkins/sa/jenkins-sa.
    s11 = {
      secret_id        = "jenkins-cuttlefish-ssh-key-b64"
      value            = base64encode(module.cuttlefish_key.private_key_openssh)
      use_github_value = false
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s12 = {
      secret_id        = "grafana-admin-password-b64"
      value            = base64encode(local.resolved_secret_values["s12"])
      use_github_value = false
      gke_access = [
        {
          ns = "monitoring"
          sa = "monitoring-sa"
        },
      ]
    }
    s13 = {
      secret_id        = "keycloak-horizon-admin-password-b64"
      value            = base64encode(var.sdv_keycloak_horizon_admin_password)
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s14 = {
      secret_id        = "postgres-admin-password-b64"
      value            = base64encode(local.resolved_secret_values["s14"])
      use_github_value = false
      gke_access = [
        {
          ns = "keycloak"
          sa = "keycloak-sa"
        }
      ]
    }
    # GCP secret name:  gh_abfs_license_b64
    # WI to GKE at ns/jenkins/sa/jenkins-sa.
    s15 = {
      secret_id        = "jenkins-abfs-license-b64"
      value            = "dummy"
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s17 = {
      secret_id        = "mcp-gateway-registry-admin-password-b64"
      value            = base64encode(local.resolved_secret_values["s17"])
      use_github_value = true
      gke_access = [
        {
          ns = "mcp-gateway-registry"
          sa = "mcp-gateway-registry-sa"
        },
      ]
    }
  }
  sdv_gcp_github_app_secrets_map = {
    s1 = {
      secret_id        = "github-app-id-b64"
      value            = base64encode(var.sdv_github_app_id)
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s2 = {
      secret_id        = "github-app-installation-id-b64"
      value            = base64encode(var.sdv_github_app_install_id)
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s3 = {
      secret_id        = "github-app-private-key-b64"
      value            = base64encode(var.sdv_github_app_private_key)
      use_github_value = true
      gke_access = [
        {
          ns = "argocd"
          sa = "argocd-sa"
        },
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        }
      ]
    }
    s8 = {
      secret_id        = "github-app-private-key-pkcs8-b64"
      value            = base64encode(var.sdv_github_app_private_key_pkcs8)
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins"
        }
      ]
    }
  }
  sdv_gcp_github_pat_secrets_map = {
    s16 = {
      secret_id        = "github-pat-b64"
      value            = base64encode(var.sdv_github_pat)
      use_github_value = true
      gke_access = [
        {
          ns = "jenkins"
          sa = "jenkins-sa"
        },
        {
          ns = "argocd"
          sa = "argocd-sa"
        }
      ]
    }
  }
}