package awsutils

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DynamoDBHelper provides helper methods for DynamoDB operations
type DynamoDBHelper struct {
	client    *dynamodb.Client
	tableName string
}

// NewDynamoDBHelper creates a new DynamoDB helper
func NewDynamoDBHelper(client *dynamodb.Client, tableName string) *DynamoDBHelper {
	return &DynamoDBHelper{
		client:    client,
		tableName: tableName,
	}
}

// PutItem stores an item in DynamoDB
func (h *DynamoDBHelper) PutItem(ctx context.Context, item interface{}) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	_, err = h.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(h.tableName),
		Item:      av,
	})

	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}

// GetItem retrieves an item from DynamoDB
func (h *DynamoDBHelper) GetItem(ctx context.Context, key map[string]types.AttributeValue, result interface{}) error {
	output, err := h.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(h.tableName),
		Key:       key,
	})

	if err != nil {
		return fmt.Errorf("failed to get item: %w", err)
	}

	if output.Item == nil {
		return fmt.Errorf("item not found")
	}

	err = attributevalue.UnmarshalMap(output.Item, result)
	if err != nil {
		return fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return nil
}

// UpdateItem updates an item in DynamoDB
func (h *DynamoDBHelper) UpdateItem(ctx context.Context, key map[string]types.AttributeValue, updateExpression string, expressionValues map[string]types.AttributeValue) error {
	_, err := h.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(h.tableName),
		Key:                       key,
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: expressionValues,
	})

	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	return nil
}

// DeleteItem deletes an item from DynamoDB
func (h *DynamoDBHelper) DeleteItem(ctx context.Context, key map[string]types.AttributeValue) error {
	_, err := h.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(h.tableName),
		Key:       key,
	})

	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}

// BatchWriteItems writes multiple items in a batch (up to 25 items)
func (h *DynamoDBHelper) BatchWriteItems(ctx context.Context, items []interface{}) error {
	const maxBatchSize = 25

	for i := 0; i < len(items); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		writeRequests := make([]types.WriteRequest, len(batch))

		for j, item := range batch {
			av, err := attributevalue.MarshalMap(item)
			if err != nil {
				return fmt.Errorf("failed to marshal item at index %d: %w", j, err)
			}

			writeRequests[j] = types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: av,
				},
			}
		}

		_, err := h.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				h.tableName: writeRequests,
			},
		})

		if err != nil {
			return fmt.Errorf("failed to batch write items: %w", err)
		}
	}

	return nil
}

// Query executes a query operation
func (h *DynamoDBHelper) Query(ctx context.Context, keyCondition string, expressionValues map[string]types.AttributeValue, results interface{}) error {
	output, err := h.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(h.tableName),
		KeyConditionExpression:    aws.String(keyCondition),
		ExpressionAttributeValues: expressionValues,
	})

	if err != nil {
		return fmt.Errorf("failed to query: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(output.Items, results)
	if err != nil {
		return fmt.Errorf("failed to unmarshal results: %w", err)
	}

	return nil
}
