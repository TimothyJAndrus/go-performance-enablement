# Outputs for Go Performance Enablement Terraform Configuration

# DynamoDB
output "dynamodb_events_table_name" {
  description = "Name of the events DynamoDB table"
  value       = aws_dynamodb_table.events.name
}

output "dynamodb_events_table_arn" {
  description = "ARN of the events DynamoDB table"
  value       = aws_dynamodb_table.events.arn
}

output "dynamodb_events_stream_arn" {
  description = "ARN of the events DynamoDB stream"
  value       = aws_dynamodb_table.events.stream_arn
}

output "dynamodb_customers_table_name" {
  description = "Name of the customers DynamoDB table"
  value       = aws_dynamodb_table.customers.name
}

# SQS
output "sqs_dlq_url" {
  description = "URL of the dead letter queue"
  value       = aws_sqs_queue.event_dlq.url
}

output "sqs_dlq_arn" {
  description = "ARN of the dead letter queue"
  value       = aws_sqs_queue.event_dlq.arn
}

output "sqs_processing_queue_url" {
  description = "URL of the event processing queue"
  value       = aws_sqs_queue.event_processing.url
}

# EventBridge
output "eventbridge_bus_name" {
  description = "Name of the EventBridge event bus"
  value       = aws_cloudwatch_event_bus.eda.name
}

output "eventbridge_bus_arn" {
  description = "ARN of the EventBridge event bus"
  value       = aws_cloudwatch_event_bus.eda.arn
}

# Secrets Manager
output "jwt_secret_arn" {
  description = "ARN of the JWT secret"
  value       = aws_secretsmanager_secret.jwt_secret.arn
}

output "kafka_credentials_arn" {
  description = "ARN of the Kafka credentials secret"
  value       = aws_secretsmanager_secret.kafka_credentials.arn
}

# Lambda Functions
output "lambda_event_router_arn" {
  description = "ARN of the event router Lambda function"
  value       = aws_lambda_function.event_router.arn
}

output "lambda_event_router_name" {
  description = "Name of the event router Lambda function"
  value       = aws_lambda_function.event_router.function_name
}

output "lambda_stream_processor_arn" {
  description = "ARN of the stream processor Lambda function"
  value       = aws_lambda_function.stream_processor.arn
}

output "lambda_event_transformer_arn" {
  description = "ARN of the event transformer Lambda function"
  value       = aws_lambda_function.event_transformer.arn
}

output "lambda_health_checker_arn" {
  description = "ARN of the health checker Lambda function"
  value       = aws_lambda_function.health_checker.arn
}

output "lambda_authorizer_arn" {
  description = "ARN of the authorizer Lambda function"
  value       = aws_lambda_function.authorizer.arn
}

# IAM
output "lambda_execution_role_arn" {
  description = "ARN of the Lambda execution IAM role"
  value       = aws_iam_role.lambda_execution.arn
}

# Summary
output "deployment_summary" {
  description = "Summary of deployed resources"
  value = {
    environment    = var.environment
    region         = var.aws_region
    partner_region = var.partner_region
    lambda_functions = {
      event_router      = aws_lambda_function.event_router.function_name
      stream_processor  = aws_lambda_function.stream_processor.function_name
      event_transformer = aws_lambda_function.event_transformer.function_name
      health_checker    = aws_lambda_function.health_checker.function_name
      authorizer        = aws_lambda_function.authorizer.function_name
    }
    dynamodb_tables = {
      events    = aws_dynamodb_table.events.name
      customers = aws_dynamodb_table.customers.name
    }
    event_bus = aws_cloudwatch_event_bus.eda.name
  }
}
