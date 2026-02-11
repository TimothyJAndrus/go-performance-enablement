# Production Environment Configuration

environment    = "prod"
aws_region     = "us-west-2"
partner_region = "us-east-1"

# Logging
log_level          = "INFO"
log_retention_days = 90

# Kafka Consumer
kafka_consumer_replicas = 3

# Additional tags
additional_tags = {
  Team        = "platform-engineering"
  CostCenter  = "production"
  Compliance  = "required"
}
