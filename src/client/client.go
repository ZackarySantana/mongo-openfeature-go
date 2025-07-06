package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/zackarysantana/mongo-openfeature-go/src/flag"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func New(opts *Options) (*Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("validating client options: %w", err)
	}

	client := &Client{
		collection: opts.Client.Database(opts.Database).Collection(opts.Collection),
		maxTries:   opts.MaxTries,
		documentID: opts.DocumentID,
		logger:     opts.Logger,
	}

	return client, nil
}

type Client struct {
	collection *mongo.Collection
	maxTries   int
	documentID string

	logger *slog.Logger
}

func (c *Client) SetFlag(ctx context.Context, flagDefinition flag.Definition) error {
	var err error
	for i := 0; i < c.maxTries; i++ {
		err = c.setFlag(ctx, flagDefinition)
		if err == nil {
			return nil
		}
		c.logger.Error("error setting flag, retrying", slog.Int("attempt", i+1), slog.String("flagName", flagDefinition.FlagName), slog.Any("error", err))
	}

	return fmt.Errorf("setting flag %s after %d attempts: %w", flagDefinition.FlagName, c.maxTries, err)
}

func (c *Client) setFlag(ctx context.Context, flagDefinition flag.Definition) error {
	documentID := flagDefinition.FlagName
	var update any = flagDefinition
	if c.documentID != "" {
		documentID = c.documentID
		update = map[string]flag.Definition{
			flagDefinition.FlagName: flagDefinition,
		}
	}

	_, err := c.collection.UpdateByID(ctx, documentID, bson.M{
		"$set": update,
	}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetFlag(ctx context.Context, flagName string) (*flag.Definition, error) {
	var err error
	var result *flag.Definition
	for i := 0; i < c.maxTries; i++ {
		result, err = c.getFlag(ctx, flagName)
		if err == nil {
			return result, nil
		}
		c.logger.Error("error getting flag, retrying", slog.Int("attempt", i+1), slog.String("flagName", flagName), slog.Any("error", err))
	}

	return nil, fmt.Errorf("getting flag %s after %d attempts: %w", flagName, c.maxTries, err)
}

func (c *Client) getFlag(ctx context.Context, flagName string) (*flag.Definition, error) {
	if c.documentID != "" {
		return c.getFlagSingleDocument(ctx, flagName)
	}
	return c.getFlagMultiDocument(ctx, flagName)
}

func (c *Client) getFlagMultiDocument(ctx context.Context, flagName string) (*flag.Definition, error) {
	var result flag.Definition
	err := c.collection.FindOne(ctx, bson.M{"_id": flagName}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("flag not found")
		}
		return nil, err
	}

	return &result, nil
}

func (c *Client) getFlagSingleDocument(ctx context.Context, flagName string) (*flag.Definition, error) {
	var result map[string]flag.Definition
	err := c.collection.FindOne(ctx, bson.M{"_id": c.documentID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("document %s: %w", c.documentID, err)
		}
		return nil, fmt.Errorf("getting flag in document %s: %w", c.documentID, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("document %s is empty", c.documentID)
	}
	flag, ok := result[flagName]
	if !ok {
		return nil, fmt.Errorf("flag not found in document %s", c.documentID)
	}

	return &flag, nil
}

func (c *Client) DeleteFlag(ctx context.Context, flagName string) error {
	var err error
	for i := 0; i < c.maxTries; i++ {
		err = c.deleteFlag(ctx, flagName)
		if err == nil {
			return nil
		}
		c.logger.Error("error deleting flag, retrying", slog.Int("attempt", i+1), slog.String("flagName", flagName), slog.Any("error", err))
	}

	return fmt.Errorf("deleting flag %s after %d attempts: %w", flagName, c.maxTries, err)
}

func (c *Client) deleteFlag(ctx context.Context, flagName string) error {
	documentID := flagName
	if c.documentID != "" {
		documentID = c.documentID
	}

	result, err := c.collection.DeleteOne(ctx, bson.M{"_id": documentID})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errors.New("flag not found")
	}

	return nil
}

func (c *Client) GetAllFlags(ctx context.Context) (map[string]flag.Definition, error) {
	var err error
	var result map[string]flag.Definition
	for i := 0; i < c.maxTries; i++ {
		result, err = c.getAllFlags(ctx)
		if err == nil {
			return result, nil
		}
		if errors.Is(err, mongo.ErrNoDocuments) {
			return result, nil
		}
		c.logger.Error("error getting all flags, retrying", slog.Int("attempt", i+1), slog.Any("error", err))
	}

	return nil, fmt.Errorf("getting all flags after %d attempts: %w", c.maxTries, err)
}

func (c *Client) getAllFlags(ctx context.Context) (map[string]flag.Definition, error) {
	if c.documentID != "" {
		return c.getAllFlagsSingleDocument(ctx)
	}
	return c.getAllFlagsMultiDocument(ctx)
}

func (c *Client) getAllFlagsMultiDocument(ctx context.Context) (map[string]flag.Definition, error) {
	cursor, err := c.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("finding all flags: %w", err)
	}
	defer cursor.Close(ctx)

	flags := make(map[string]flag.Definition)
	for cursor.Next(ctx) {
		var flagDef flag.Definition
		if err := cursor.Decode(&flagDef); err != nil {
			return nil, fmt.Errorf("decoding flag: %w", err)
		}
		flags[flagDef.FlagName] = flagDef
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return flags, nil
}

func (c *Client) getAllFlagsSingleDocument(ctx context.Context) (map[string]flag.Definition, error) {
	var result struct {
		ID    any                        `bson:"_id"`
		Flags map[string]flag.Definition `bson:",inline"`
	}
	err := c.collection.FindOne(ctx, bson.M{"_id": c.documentID}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("document %s: %w", c.documentID, err)
		}
		return nil, fmt.Errorf("getting all flags in document %s: %w", c.documentID, err)
	}

	return result.Flags, nil
}
