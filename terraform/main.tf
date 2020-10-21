variable "project" {}
variable "credentials_file" {
  default = "./credentials.json"
}

provider "google" {
  credentials = file(var.credentials_file)
  project     = var.project
  region      = "us-central1"
}

resource "google_service_account" "snmpcollect" {
  account_id   = "snmpcollect"
  display_name = "SNMP Collect"
}

resource "google_project_iam_custom_role" "snmpcollect" {
  role_id     = "snmpCollect"
  title       = "SNMP Collect"
  description = "Role for SNMP Collect"
  permissions = ["monitoring.metricDescriptors.create",
    "monitoring.metricDescriptors.get",
    "monitoring.metricDescriptors.list",
    "monitoring.timeSeries.create"]
}

resource "google_project_iam_member" "snmpcollect" {
  project = google_service_account.snmpcollect.project
  role    = google_project_iam_custom_role.snmpcollect.id
  member  = "serviceAccount:${google_service_account.snmpcollect.email}"
}