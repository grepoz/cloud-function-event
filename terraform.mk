# Updated targets in Makefile

TF_DIR = ./terraform

.PHONY: tf-init tf-plan tf-apply

# Initialize Terraform providers
tf-init:
	cd $(TF_DIR) && terraform init

# Preview changes before applying
tf-plan:
	cd $(TF_DIR) && terraform plan -var="project_id=$(GOOGLE_CLOUD_PROJECT)"

# Robust deployment using Terraform
deploy: tidy
	cd $(TF_DIR) && terraform apply -var="project_id=$(GOOGLE_CLOUD_PROJECT)" -auto-approve

# Legacy cleanup (optional)
# deploy-legacy:
# 	gcloud functions deploy bibently-functions --flags-file=deploy-config.yaml ...