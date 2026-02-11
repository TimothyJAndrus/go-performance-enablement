# Development Environment Configuration

environment    = "dev"
aws_region     = "us-west-2"
partner_region = "us-east-1"

# Logging
log_level          = "DEBUG"
log_retention_days = 7

# Kafka Consumer
kafka_consumer_replicas = 2

# Additional tags
additional_tags = {
  Team        = "platform-engineering"
  CostCenter  = "development"
}
