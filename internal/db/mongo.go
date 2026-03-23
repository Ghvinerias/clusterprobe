package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	mongoDriverName = "mongodb"
	mongoScope      = "clusterprobe/db/mongo"
)

// MongoClient wraps a mongo client.
type MongoClient struct {
	client   *mongo.Client
	database string
	tracer   otel.Tracer
}

// NewMongo creates a Mongo client with retry/backoff.
func NewMongo(ctx context.Context, uri, database string) (*MongoClient, error) {
	if strings.TrimSpace(uri) == "" {
		return nil, fmt.Errorf("mongo uri is required")
	}
	if strings.TrimSpace(database) == "" {
		return nil, fmt.Errorf("mongo database is required")
	}

	client, err := connectMongoWithRetry(ctx, uri)
	if err != nil {
		return nil, err
	}

	return &MongoClient{
		client:   client,
		database: database,
		tracer:   otel.Tracer(mongoScope),
	}, nil
}

func connectMongoWithRetry(ctx context.Context, uri string) (*mongo.Client, error) {
	backoff := 200 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		client, err := mongo.Connect(options.Client().ApplyURI(uri))
		if err != nil {
			lastErr = err
		} else if err := client.Ping(ctx, nil); err != nil {
			lastErr = err
			_ = client.Disconnect(ctx)
		} else {
			return client, nil
		}

		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("mongo connect canceled: %w", ctx.Err())
		}

		if attempt < 10 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("mongo connect canceled: %w", ctx.Err())
			case <-timer.C:
			}
			if backoff < 5*time.Second {
				backoff *= 2
			}
		}
	}

	return nil, fmt.Errorf("mongo connect failed after retries: %w", lastErr)
}

// Collection returns a collection handle.
func (c *MongoClient) Collection(name string) *mongo.Collection {
	return c.client.Database(c.database).Collection(name)
}

// InsertOne inserts a document.
func (c *MongoClient) InsertOne(ctx context.Context, collection string, doc any) (*mongo.InsertOneResult, error) {
	ctx, span := c.tracer.Start(ctx, "mongo.insert_one")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", mongoDriverName),
		attribute.String("db.name", c.database),
		attribute.String("db.mongodb.collection", collection),
	)

	result, err := c.Collection(collection).InsertOne(ctx, doc)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "insert one failed")
		return nil, fmt.Errorf("mongo insert one: %w", err)
	}

	return result, nil
}

// InsertMany inserts documents.
func (c *MongoClient) InsertMany(ctx context.Context, collection string, docs []any) (*mongo.InsertManyResult, error) {
	ctx, span := c.tracer.Start(ctx, "mongo.insert_many")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", mongoDriverName),
		attribute.String("db.name", c.database),
		attribute.String("db.mongodb.collection", collection),
	)

	result, err := c.Collection(collection).InsertMany(ctx, docs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "insert many failed")
		return nil, fmt.Errorf("mongo insert many: %w", err)
	}

	return result, nil
}

// Find queries for documents.
func (c *MongoClient) Find(ctx context.Context, collection string, filter any) (*mongo.Cursor, error) {
	ctx, span := c.tracer.Start(ctx, "mongo.find")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", mongoDriverName),
		attribute.String("db.name", c.database),
		attribute.String("db.mongodb.collection", collection),
	)

	cursor, err := c.Collection(collection).Find(ctx, filter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "find failed")
		return nil, fmt.Errorf("mongo find: %w", err)
	}

	return cursor, nil
}

// FindOne queries for a single document.
func (c *MongoClient) FindOne(ctx context.Context, collection string, filter any) (*mongo.SingleResult, error) {
	ctx, span := c.tracer.Start(ctx, "mongo.find_one")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", mongoDriverName),
		attribute.String("db.name", c.database),
		attribute.String("db.mongodb.collection", collection),
	)

	result := c.Collection(collection).FindOne(ctx, filter)
	if err := result.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "find one failed")
		return nil, fmt.Errorf("mongo find one: %w", err)
	}

	return result, nil
}

// UpdateOne updates a single document.
func (c *MongoClient) UpdateOne(ctx context.Context, collection string, filter any, update any) (*mongo.UpdateResult, error) {
	ctx, span := c.tracer.Start(ctx, "mongo.update_one")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", mongoDriverName),
		attribute.String("db.name", c.database),
		attribute.String("db.mongodb.collection", collection),
	)

	result, err := c.Collection(collection).UpdateOne(ctx, filter, update)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "update one failed")
		return nil, fmt.Errorf("mongo update one: %w", err)
	}

	return result, nil
}

// Close disconnects the client.
func (c *MongoClient) Close(ctx context.Context) error {
	return c.client.Disconnect(ctx)
}

func buildMongoFilter(id string) bson.D {
	return bson.D{{Key: "_id", Value: id}}
}
