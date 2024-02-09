// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

var (
	minikubeTF = `terraform {
		required_providers {
		  minikube = {
			source = "scott-the-programmer/minikube"
			version = "0.3.6"
		  }
		}
	  }
	  
	  provider "minikube" {
		kubernetes_version = "v1.28.3"
	  }
	  
	  resource "minikube_cluster" "this" {
		driver       = "docker"
		cluster_name = "nevadito"
		addons = [
		  "default-storageclass",
		  "storage-provisioner"
		]
	  }`
	gcpTF = `terraform {
		required_providers {
		  google = {
			source  = "hashicorp/google"
			version = ">= 4.59.0"
		  }
	  
		  local = {
			source  = "hashicorp/local"
			version = "2.4.1"
		  }
		}
		required_version = ">= 1.5.4"
	  }
	  
	  terraform {
		backend "gcs" {
		  bucket = "hackaton-create"
		  prefix = "terraform/state"
		  # credentials = "$GOOGLE_CREDENTIALS"
		}
	  }
	  
	  data "google_client_config" "default" {}
	  
	  locals {
		cert       = base64decode(module.gkepublic.ca_certificate)
		kubeconfig = <<EOF
apiVersion: v1
kind: Config
current-context: "hackatontest"
preferences: {}
clusters:
- cluster:
    certificate-authority-data: ${module.gkepublic.ca_certificate}
    server: "https://${module.gkepublic.endpoint}"
  name: "hackatontest"
contexts:
- context:
    cluster: "hackatontest"
    user: "hackatontest"
  name: "hackatontest"
users:
- name: "hackatontest"
  user:
    token: ${data.google_client_config.default.access_token}
	  EOF
	  }
	  resource "local_file" "kubeconfig" {
		content  = local.kubeconfig
		filename = "kubeconfig"
	  }
	  
	  resource "google_compute_network" "this" {
		name                    = "hackatontest"
		project                 = "development-300207"
		auto_create_subnetworks = false
	  }
	  
	  resource "google_compute_subnetwork" "this" {
		project       = "development-300207"
		name          = format("pods-%s", "hackatontest")
		region        = "europe-west6"
		ip_cidr_range = "10.255.16.0/20"
		network       = google_compute_network.this.id
	  
		dynamic "secondary_ip_range" {
		  for_each = ["pods"]
		  content {
			range_name    = "pods"
			ip_cidr_range = "10.8.0.0/14"
		  }
		}
	  
		dynamic "secondary_ip_range" {
		  for_each = ["svcs"]
		  content {
			range_name    = "svcs"
			ip_cidr_range = "10.152.0.0/14"
		  }
		}
	  }
	  
	  
	  module "gkepublic" {
		source                     = "terraform-google-modules/kubernetes-engine/google"
		project_id                 = "development-300207"
		name                       = "hackatontest"
		region                     = "europe-west6"
		zones                      = ["europe-west6-a"]
		network                    = google_compute_network.this.name
		subnetwork                 = google_compute_subnetwork.this.name
		ip_range_pods              = "pods"
		ip_range_services          = "svcs"
		http_load_balancing        = false
		network_policy             = false
		horizontal_pod_autoscaling = true
		filestore_csi_driver       = false
	  
		node_pools = [
		  {
			name               = "default-node-pool"
			machine_type       = "e2-standard-4"
			node_locations     = "europe-west6-a"
			min_count          = 2
			max_count          = 3
			local_ssd_count    = 0
			spot               = false
			disk_size_gb       = 250
			disk_type          = "pd-ssd"
			image_type         = "COS_CONTAINERD"
			enable_gcfs        = false
			enable_gvnic       = false
			logging_variant    = "DEFAULT"
			auto_repair        = true
			auto_upgrade       = true
			service_account    = google_service_account.registry.email
			preemptible        = false
			initial_node_count = 2
		  },
		]
	  
		node_pools_oauth_scopes = {
		  all = [
			"https://www.googleapis.com/auth/logging.write",
			"https://www.googleapis.com/auth/monitoring",
		  ]
		}
	  
		node_pools_labels = {
		  all = {}
	  
		  default-node-pool = {
			default-node-pool = true
		  }
		}
	  
		node_pools_metadata = {
		  all = {}
	  
		  default-node-pool = {
			node-pool-metadata-custom-value = "my-node-pool"
		  }
		}
	  
		node_pools_taints = {
		  all = []
	  
		  default-node-pool = [
			{
			  key    = "default-node-pool"
			  value  = true
			  effect = "PREFER_NO_SCHEDULE"
			},
		  ]
		}
	  
		node_pools_tags = {
		  all = []
	  
		  default-node-pool = [
			"default-node-pool",
		  ]
		}
	  }
	  
	  resource "google_service_account" "this" {
		account_id   = format("%sgke", "hackatontest")
		display_name = format("%s GKE Service Account", "hackatontest")
		project      = "development-300207"
	  }
	  
	  resource "google_project_iam_custom_role" "this" {
		project     = "development-300207"
		role_id     = format("%sgke", "hackatontest")
		title       = format("%s GKE Okteto role", "hackatontest")
		description = "Permissions for okteto GKE"
		permissions = [
		  "cloudnotifications.activities.list",
		  "logging.logEntries.create",
		  "monitoring.timeSeries.create",
		  "monitoring.alertPolicies.get",
		  "monitoring.alertPolicies.list",
		  "monitoring.dashboards.get",
		  "monitoring.dashboards.list",
		  "monitoring.groups.get",
		  "monitoring.groups.list",
		  "monitoring.metricDescriptors.create",
		  "monitoring.metricDescriptors.get",
		  "monitoring.metricDescriptors.list",
		  "monitoring.monitoredResourceDescriptors.get",
		  "monitoring.monitoredResourceDescriptors.list",
		  "monitoring.notificationChannelDescriptors.get",
		  "monitoring.notificationChannelDescriptors.list",
		  "monitoring.notificationChannels.get",
		  "monitoring.notificationChannels.list",
		  "monitoring.publicWidgets.get",
		  "monitoring.publicWidgets.list",
		  "monitoring.services.get",
		  "monitoring.services.list",
		  "monitoring.slos.get",
		  "monitoring.slos.list",
		  "monitoring.timeSeries.list",
		  "monitoring.uptimeCheckConfigs.get",
		  "monitoring.uptimeCheckConfigs.list",
		  "resourcemanager.projects.get",
		  "stackdriver.projects.get",
		]
	  }
	  
	  
	  
	  resource "google_project_iam_binding" "binding_storage_role" {
		role    = google_project_iam_custom_role.this.id
		project = "development-300207"
		members = ["serviceAccount:${google_service_account.this.email}", "serviceAccount:${google_service_account.registry.email}"]
	  }
	  
	  
	  resource "google_service_account" "registry" {
		project      = "development-300207"
		account_id   = format("%sgkeregistry", "hackatontest")
		display_name = format("%s GKE Service Account with specials access", "hackatontest")
	  }
	  
	  resource "google_project_iam_custom_role" "registry" {
		project     = "development-300207"
		role_id     = format("%sgkeregistry", "hackatontest")
		title       = format("%s GKE Okteto role gcr access", "hackatontest")
		description = "Permissions to connect with gcr"
		permissions = [
		  "storage.objects.get",
		  "storage.objects.list",
		]
	  }
	  
	  resource "google_project_iam_binding" "binding_registry_role_to_registry" {
		project = "development-300207"
		role    = google_project_iam_custom_role.registry.id
		members = ["serviceAccount:${google_service_account.registry.email}"]
	  }`
)
