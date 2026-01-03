terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  type    = string
  default = "bibently-firebase"
}

variable "region" {
  type    = string
  default = "europe-west1"
}

# 1. Storage Bucket for source code
resource "google_storage_bucket" "function_bucket" {
  name     = "${var.project_id}-gcf-source"
  location = var.region
}

# 2. Archive source code (respects .gcloudignore via excluding folders)
data "archive_file" "source" {
  type        = "zip"
  output_path = "${path.module}/source.zip"
  source_dir  = "../"
  excludes    = [".git", ".idea", "test", "vendor", "terraform"]
}

resource "google_storage_bucket_object" "zip" {
  name   = "source.zip#${data.archive_file.source.output_md5}"
  bucket = google_storage_bucket.function_bucket.name
  source = data.archive_file.source.output_path
}

resource "google_cloudfunctions2_function" "function" {
  name        = "bibently-functions"
  location    = var.region
  description = "Event management service"

  build_config {
    runtime     = "go125"
    entry_point = "BibentlyFunctions"
    source {
      storage_source {
        bucket = google_storage_bucket.function_bucket.name
        object = google_storage_bucket_object.zip.name
      }
    }
  }

  service_config {
    max_instance_count = 1
    min_instance_count = 0
    available_memory   = "512MiB"
    timeout_seconds    = 10
    available_cpu      = "2"
    ingress_settings   = "ALLOW_ALL"
    all_traffic_on_latest_revision = true

    environment_variables = {
      APP_ENV               = "production"
      CORS_ALLOWED_ORIGIN   = "*"
      FIRESTORE_DATABASE_ID = "bibently-store"
      GOMEMLIMIT            = "460MiB"
      GOOGLE_CLOUD_PROJECT  = var.project_id
    }

    secret_volumes {
      mount_path = "/secrets"
      project_id = var.project_id
      secret     = "FIRESTORE_ADMIN_UID"
      versions {
        version = "latest"
        path    = "firestore-admin-uid"
      }
    }
  }
}

resource "google_cloud_run_service_iam_member" "public_access" {
  location = google_cloudfunctions2_function.function.location
  project  = google_cloudfunctions2_function.function.project
  service  = google_cloudfunctions2_function.function.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

output "function_url" {
  value = google_cloudfunctions2_function.function.url
}