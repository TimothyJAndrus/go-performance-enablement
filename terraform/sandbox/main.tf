# Go Performance Enablement - Terraform Configuration
# Sandbox Environment

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Uncomment for remote state storage
  # backend "s3" {
  #   bucket         = "wgu-terraform-state"
  #   key            = "go-performance-enablement/sandbox/terraform.tfstate"
  #   region         = "us-west-2"
  #   dynamodb_table = "terraform-locks"
  #   encrypt        = true
  # }
}

# Primary Region
provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "go-performance-enablement"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# Secondary Region (for cross-region resources)
provider "aws" {
  alias  = "secondary"
  region = var.partner_region

  default_tags {
    tags = {
      Project     = "go-performance-enablement"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# Data sources
data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# Local values
locals {
  account_id = data.aws_caller_identity.current.account_id
  region     = data.aws_region.current.name

  lambda_functions = [
    "event-router",
    "stream-processor",
    "event-transformer",
    "health-checker",
    "authorizer"
  ]

  common_tags = {
    Project     = "go-performance-enablement"
    Environment = var.environment
  }
}

# DynamoDB Tables
resource "aws_dynamodb_table" "events" {
  name           = "events-${var.environment}"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "event_id"
  range_key      = "timestamp"

  attribute {
    name = "event_id"
    type = "S"
  }

  attribute {
    name = "timestamp"
    type = "N"
  }

  attribute {
    name = "event_type"
    type = "S"
  }

  global_secondary_index {
    name            = "event-type-index"
    hash_key        = "event_type"
    range_key       = "timestamp"
    projection_type = "ALL"
  }

  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"

  point_in_time_recovery {
    enabled = true
  }

  tags = local.common_tags
}

resource "aws_dynamodb_table" "customers" {
  name         = "customers-${var.environment}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "customer_id"

  attribute {
    name = "customer_id"
    type = "S"
  }

  stream_enabled   = true
  stream_view_type = "NEW_AND_OLD_IMAGES"

  point_in_time_recovery {
    enabled = true
  }

  tags = local.common_tags
}

# SQS Dead Letter Queue
resource "aws_sqs_queue" "event_dlq" {
  name                      = "event-dlq-${var.environment}"
  message_retention_seconds = 1209600 # 14 days

  tags = local.common_tags
}

# SQS Processing Queue
resource "aws_sqs_queue" "event_processing" {
  name                       = "event-processing-${var.environment}"
  visibility_timeout_seconds = 300
  message_retention_seconds  = 86400 # 1 day

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.event_dlq.arn
    maxReceiveCount     = 3
  })

  tags = local.common_tags
}

# EventBridge Event Bus
resource "aws_cloudwatch_event_bus" "eda" {
  name = "eda-event-bus-${var.environment}"

  tags = local.common_tags
}

# EventBridge Rule for CDC Events
resource "aws_cloudwatch_event_rule" "cdc_events" {
  name           = "cdc-events-rule-${var.environment}"
  event_bus_name = aws_cloudwatch_event_bus.eda.name

  event_pattern = jsonencode({
    source = ["qlik.cdc"]
  })

  tags = local.common_tags
}

# EventBridge Rule for Transformed Events
resource "aws_cloudwatch_event_rule" "transformed_events" {
  name           = "transformed-events-rule-${var.environment}"
  event_bus_name = aws_cloudwatch_event_bus.eda.name

  event_pattern = jsonencode({
    source = ["event-transformer"]
  })

  tags = local.common_tags
}

# Secrets Manager - JWT Secret
resource "aws_secretsmanager_secret" "jwt_secret" {
  name        = "jwt-secret-${var.environment}"
  description = "JWT signing secret for API authorizer"

  tags = local.common_tags
}

resource "aws_secretsmanager_secret_version" "jwt_secret" {
  secret_id = aws_secretsmanager_secret.jwt_secret.id
  secret_string = jsonencode({
    secret_key = var.jwt_secret_key
    issuer     = "go-performance-enablement"
    audience   = "api-gateway"
  })
}

# Secrets Manager - Kafka Credentials
resource "aws_secretsmanager_secret" "kafka_credentials" {
  name        = "kafka-credentials-${var.environment}"
  description = "Confluent Kafka credentials"

  tags = local.common_tags
}

# IAM Role for Lambda Functions
resource "aws_iam_role" "lambda_execution" {
  name = "lambda-execution-role-${var.environment}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = local.common_tags
}

# IAM Policy for Lambda Functions
resource "aws_iam_role_policy" "lambda_policy" {
  name = "lambda-policy-${var.environment}"
  role = aws_iam_role.lambda_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:*"
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem"
        ]
        Resource = [
          aws_dynamodb_table.events.arn,
          "${aws_dynamodb_table.events.arn}/index/*",
          aws_dynamodb_table.customers.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DescribeStream",
          "dynamodb:GetRecords",
          "dynamodb:GetShardIterator",
          "dynamodb:ListStreams"
        ]
        Resource = [
          "${aws_dynamodb_table.events.arn}/stream/*",
          "${aws_dynamodb_table.customers.arn}/stream/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "sqs:SendMessage",
          "sqs:ReceiveMessage",
          "sqs:DeleteMessage",
          "sqs:GetQueueAttributes"
        ]
        Resource = [
          aws_sqs_queue.event_dlq.arn,
          aws_sqs_queue.event_processing.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "events:PutEvents"
        ]
        Resource = aws_cloudwatch_event_bus.eda.arn
      },
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = [
          aws_secretsmanager_secret.jwt_secret.arn,
          aws_secretsmanager_secret.kafka_credentials.arn
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "xray:PutTraceSegments",
          "xray:PutTelemetryRecords"
        ]
        Resource = "*"
      }
    ]
  })
}

# Lambda Function - Event Router
resource "aws_lambda_function" "event_router" {
  filename         = "${path.module}/../../bin/lambdas/event-router.zip"
  function_name    = "event-router-${var.environment}"
  role             = aws_iam_role.lambda_execution.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 128
  timeout          = 30

  environment {
    variables = {
      ENVIRONMENT    = var.environment
      AWS_REGION     = var.aws_region
      PARTNER_REGION = var.partner_region
      EVENT_BUS_NAME = aws_cloudwatch_event_bus.eda.name
      DLQ_URL        = aws_sqs_queue.event_dlq.url
      LOG_LEVEL      = var.log_level
    }
  }

  dead_letter_config {
    target_arn = aws_sqs_queue.event_dlq.arn
  }

  tracing_config {
    mode = "Active"
  }

  tags = local.common_tags
}

# Lambda Function - Stream Processor
resource "aws_lambda_function" "stream_processor" {
  filename         = "${path.module}/../../bin/lambdas/stream-processor.zip"
  function_name    = "stream-processor-${var.environment}"
  role             = aws_iam_role.lambda_execution.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 256
  timeout          = 60

  environment {
    variables = {
      ENVIRONMENT    = var.environment
      AWS_REGION     = var.aws_region
      PARTNER_REGION = var.partner_region
      EVENT_BUS_NAME = aws_cloudwatch_event_bus.eda.name
      DLQ_URL        = aws_sqs_queue.event_dlq.url
      LOG_LEVEL      = var.log_level
    }
  }

  dead_letter_config {
    target_arn = aws_sqs_queue.event_dlq.arn
  }

  tracing_config {
    mode = "Active"
  }

  tags = local.common_tags
}

# Lambda Event Source Mapping - DynamoDB Streams
resource "aws_lambda_event_source_mapping" "events_stream" {
  event_source_arn  = aws_dynamodb_table.events.stream_arn
  function_name     = aws_lambda_function.stream_processor.arn
  starting_position = "LATEST"
  batch_size        = 100

  filter_criteria {
    filter {
      pattern = jsonencode({
        eventName = ["INSERT", "MODIFY", "REMOVE"]
      })
    }
  }
}

# Lambda Function - Event Transformer
resource "aws_lambda_function" "event_transformer" {
  filename         = "${path.module}/../../bin/lambdas/event-transformer.zip"
  function_name    = "event-transformer-${var.environment}"
  role             = aws_iam_role.lambda_execution.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 512
  timeout          = 60

  environment {
    variables = {
      ENVIRONMENT    = var.environment
      EVENT_BUS_NAME = aws_cloudwatch_event_bus.eda.name
      DLQ_URL        = aws_sqs_queue.event_dlq.url
      LOG_LEVEL      = var.log_level
    }
  }

  dead_letter_config {
    target_arn = aws_sqs_queue.event_dlq.arn
  }

  tracing_config {
    mode = "Active"
  }

  tags = local.common_tags
}

# Lambda Function - Health Checker
resource "aws_lambda_function" "health_checker" {
  filename         = "${path.module}/../../bin/lambdas/health-checker.zip"
  function_name    = "health-checker-${var.environment}"
  role             = aws_iam_role.lambda_execution.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 256
  timeout          = 30

  environment {
    variables = {
      ENVIRONMENT    = var.environment
      AWS_REGION     = var.aws_region
      PARTNER_REGION = var.partner_region
      EVENT_BUS_NAME = aws_cloudwatch_event_bus.eda.name
      LOG_LEVEL      = var.log_level
    }
  }

  tracing_config {
    mode = "Active"
  }

  tags = local.common_tags
}

# CloudWatch Event Rule for Health Checker (every 5 minutes)
resource "aws_cloudwatch_event_rule" "health_check_schedule" {
  name                = "health-check-schedule-${var.environment}"
  description         = "Trigger health checker every 5 minutes"
  schedule_expression = "rate(5 minutes)"

  tags = local.common_tags
}

resource "aws_cloudwatch_event_target" "health_checker" {
  rule      = aws_cloudwatch_event_rule.health_check_schedule.name
  target_id = "health-checker"
  arn       = aws_lambda_function.health_checker.arn
}

resource "aws_lambda_permission" "health_checker" {
  statement_id  = "AllowEventBridgeInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.health_checker.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.health_check_schedule.arn
}

# Lambda Function - Authorizer
resource "aws_lambda_function" "authorizer" {
  filename         = "${path.module}/../../bin/lambdas/authorizer.zip"
  function_name    = "authorizer-${var.environment}"
  role             = aws_iam_role.lambda_execution.arn
  handler          = "bootstrap"
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  memory_size      = 128
  timeout          = 5

  environment {
    variables = {
      ENVIRONMENT       = var.environment
      JWT_SECRET_ARN    = aws_secretsmanager_secret.jwt_secret.arn
      LOG_LEVEL         = var.log_level
    }
  }

  tracing_config {
    mode = "Active"
  }

  tags = local.common_tags
}

# CloudWatch Log Groups for Lambda Functions
resource "aws_cloudwatch_log_group" "lambda_logs" {
  for_each = toset(local.lambda_functions)

  name              = "/aws/lambda/${each.value}-${var.environment}"
  retention_in_days = var.log_retention_days

  tags = local.common_tags
}
