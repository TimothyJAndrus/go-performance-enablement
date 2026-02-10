package awsutils

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// AWSClients holds all AWS service clients
type AWSClients struct {
	DynamoDB       *dynamodb.Client
	EventBridge    *eventbridge.Client
	SQS            *sqs.Client
	SecretsManager *secretsmanager.Client
	Config         aws.Config
}

// NewAWSClients creates a new set of AWS clients with the default configuration
func NewAWSClients(ctx context.Context) (*AWSClients, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRetryMaxAttempts(3),
		config.WithRetryMode(aws.RetryModeAdaptive),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AWSClients{
		DynamoDB:       dynamodb.NewFromConfig(cfg),
		EventBridge:    eventbridge.NewFromConfig(cfg),
		SQS:            sqs.NewFromConfig(cfg),
		SecretsManager: secretsmanager.NewFromConfig(cfg),
		Config:         cfg,
	}, nil
}

// NewAWSClientsWithRegion creates AWS clients for a specific region
func NewAWSClientsWithRegion(ctx context.Context, region string) (*AWSClients, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithRetryMaxAttempts(3),
		config.WithRetryMode(aws.RetryModeAdaptive),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for region %s: %w", region, err)
	}

	return &AWSClients{
		DynamoDB:       dynamodb.NewFromConfig(cfg),
		EventBridge:    eventbridge.NewFromConfig(cfg),
		SQS:            sqs.NewFromConfig(cfg),
		SecretsManager: secretsmanager.NewFromConfig(cfg),
		Config:         cfg,
	}, nil
}

// GetRegion returns the configured AWS region
func (c *AWSClients) GetRegion() string {
	return c.Config.Region
}

// WithTimeout creates a context with timeout for AWS operations
func WithTimeout(parent context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, duration)
}

// GetSecret retrieves a secret from AWS Secrets Manager
func (c *AWSClients) GetSecret(ctx context.Context, secretName string) (string, error) {
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	}

	result, err := c.SecretsManager.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	if result.SecretString != nil {
		return *result.SecretString, nil
	}

	return "", fmt.Errorf("secret %s has no string value", secretName)
}

// SendToDeadLetterQueue sends a failed message to DLQ
func (c *AWSClients) SendToDeadLetterQueue(ctx context.Context, queueURL, messageBody, errorMessage string) error {
	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(messageBody),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"ErrorMessage": {
				DataType:    aws.String("String"),
				StringValue: aws.String(errorMessage),
			},
			"FailureTimestamp": {
				DataType:    aws.String("String"),
				StringValue: aws.String(time.Now().Format(time.RFC3339)),
			},
		},
	}

	_, err := c.SQS.SendMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to send message to DLQ: %w", err)
	}

	return nil
}

// GetCurrentRegion returns the AWS region from environment or config
func GetCurrentRegion(ctx context.Context) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	return cfg.Region, nil
}
