# Variables for Go Performance Enablement Terraform Configuration

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be dev, staging, or prod."
  }
}

variable "aws_region" {
  description = "Primary AWS region"
  type        = string
  default     = "us-west-2"
}

variable "partner_region" {
  description = "Secondary/partner AWS region for cross-region replication"
  type        = string
  default     = "us-east-1"
}

variable "jwt_secret_key" {
  description = "JWT signing secret key"
  type        = string
  sensitive   = true
  default     = "change-me-in-production"
}

variable "log_level" {
  description = "Log level for Lambda functions"
  type        = string
  default     = "INFO"

  validation {
    condition     = contains(["DEBUG", "INFO", "WARN", "ERROR"], var.log_level)
    error_message = "Log level must be DEBUG, INFO, WARN, or ERROR."
  }
}

variable "log_retention_days" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 30

  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.log_retention_days)
    error_message = "Log retention must be a valid CloudWatch retention period."
  }
}

# Kafka Configuration
variable "kafka_bootstrap_servers" {
  description = "Confluent Kafka bootstrap servers"
  type        = string
  default     = ""
}

variable "kafka_api_key" {
  description = "Confluent Kafka API key"
  type        = string
  sensitive   = true
  default     = ""
}

variable "kafka_api_secret" {
  description = "Confluent Kafka API secret"
  type        = string
  sensitive   = true
  default     = ""
}

variable "schema_registry_url" {
  description = "Confluent Schema Registry URL"
  type        = string
  default     = ""
}

# EKS Configuration (for Kafka consumer)
variable "eks_cluster_name" {
  description = "EKS cluster name for Kafka consumer deployment"
  type        = string
  default     = ""
}

variable "kafka_consumer_replicas" {
  description = "Number of Kafka consumer pod replicas"
  type        = number
  default     = 3

  validation {
    condition     = var.kafka_consumer_replicas >= 1 && var.kafka_consumer_replicas <= 10
    error_message = "Kafka consumer replicas must be between 1 and 10."
  }
}

# Tags
variable "additional_tags" {
  description = "Additional tags to apply to all resources"
  type        = map(string)
  default     = {}
}
