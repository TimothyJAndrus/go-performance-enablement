# Terraform - Go Performance Enablement

Infrastructure as Code for deploying the Go Performance Enablement project to AWS.

## Prerequisites

- [Terraform](https://www.terraform.io/downloads) >= 1.5.0
- AWS CLI configured with appropriate credentials
- Lambda function ZIP files built (run `make build-lambdas` first)

## Resources Created

This Terraform configuration creates:

- **DynamoDB Tables**: `events`, `customers` with streams enabled
- **SQS Queues**: Dead letter queue, event processing queue
- **EventBridge**: Custom event bus with CDC and transformer rules
- **Secrets Manager**: JWT secret, Kafka credentials
- **Lambda Functions**: All 5 Lambda functions with IAM roles
- **CloudWatch**: Log groups with configurable retention

## Usage

### Initialize

```bash
cd terraform/sandbox
terraform init
```

### Plan (Dev)

```bash
terraform plan -var-file=dev.tfvars
```

### Apply (Dev)

```bash
terraform apply -var-file=dev.tfvars
```

### Plan (Prod)

```bash
terraform plan -var-file=prod.tfvars
```

### Apply (Prod)

```bash
# Requires manual approval
terraform apply -var-file=prod.tfvars
```

### Destroy

```bash
terraform destroy -var-file=dev.tfvars
```

## Configuration

### Required Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `environment` | Environment name (dev/staging/prod) | `dev` |
| `aws_region` | Primary AWS region | `us-west-2` |
| `partner_region` | Secondary region for cross-region | `us-east-1` |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `log_level` | Lambda log level | `INFO` |
| `log_retention_days` | CloudWatch log retention | `30` |
| `kafka_consumer_replicas` | Number of Kafka consumer pods | `3` |
| `jwt_secret_key` | JWT signing secret | - |

### Sensitive Variables

For production, set sensitive variables via environment:

```bash
export TF_VAR_jwt_secret_key="your-secret-key"
export TF_VAR_kafka_api_key="your-api-key"
export TF_VAR_kafka_api_secret="your-api-secret"
```

## Remote State

For team collaboration, uncomment the S3 backend in `main.tf`:

```hcl
backend "s3" {
  bucket         = "wgu-terraform-state"
  key            = "go-performance-enablement/sandbox/terraform.tfstate"
  region         = "us-west-2"
  dynamodb_table = "terraform-locks"
  encrypt        = true
}
```

## Outputs

After applying, key outputs include:

- Lambda function ARNs and names
- DynamoDB table names and ARNs
- SQS queue URLs
- EventBridge bus name
- Secrets Manager ARNs

View all outputs:

```bash
terraform output
```

## CI/CD Integration

This Terraform configuration is designed to work with:

- **GitHub Actions**: Automated plan on PR, apply on merge
- **Octopus Deploy**: Triggered deployment with approval gates

## Troubleshooting

### Lambda ZIP not found

Build Lambda functions first:

```bash
make build-lambdas
```

### Permission denied

Ensure AWS credentials have sufficient permissions:

```bash
aws sts get-caller-identity
```

### State lock

If state is locked, check for running pipelines or unlock:

```bash
terraform force-unlock LOCK_ID
```

## Security Notes

- Never commit `*.tfvars` files containing secrets
- Use AWS Secrets Manager or environment variables for sensitive data
- Review IAM policies follow least-privilege principle
- Enable encryption at rest for all data stores
