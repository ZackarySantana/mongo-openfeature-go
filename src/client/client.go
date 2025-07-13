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
	var result struct {
		ID    any                        `bson:"_id"`
		Flags map[string]flag.Definition `bson:",inline"`
	}
	opts := options.FindOne().SetProjection(bson.M{flagName: 1})
	err := c.collection.FindOne(ctx, bson.M{"_id": c.documentID}, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("document %s: %w", c.documentID, err)
		}
		return nil, fmt.Errorf("getting flag in document %s: %w", c.documentID, err)
	}
	flag, ok := result.Flags[flagName]
	if !ok {
		return nil, fmt.Errorf("flag '%s' not found in document %s", flagName, c.documentID)
	}

	return &flag, nil
}

// PartialUpdateFlag performs an atomic partial update on a flag definition.
// The updates map should contain keys matching the BSON field names to be changed.
func (c *Client) PartialUpdateFlag(ctx context.Context, flagName string, updates map[string]any) error {
	var err error
	for i := 0; i < c.maxTries; i++ {
		err = c.partialUpdateFlag(ctx, flagName, updates)
		if err == nil {
			return nil
		}
		c.logger.Error("error partially updating flag, retrying", slog.Int("attempt", i+1), slog.String("flagName", flagName), slog.Any("error", err))
	}
	return fmt.Errorf("partially updating flag %s after %d attempts: %w", flagName, c.maxTries, err)
}

func (c *Client) partialUpdateFlag(ctx context.Context, flagName string, updates map[string]any) error {
	if c.documentID != "" {
		return c.partialUpdateFlagSingleDocument(ctx, flagName, updates)
	}
	return c.partialUpdateFlagMultiDocument(ctx, flagName, updates)
}

func (c *Client) partialUpdateFlagMultiDocument(ctx context.Context, flagName string, updates map[string]any) error {
	res, err := c.collection.UpdateOne(ctx,
		bson.M{"_id": flagName},
		bson.M{"$set": updates},
	)
	if err != nil {
		return fmt.Errorf("updating flag %s: %w", flagName, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("flag '%s' not found", flagName)
	}
	return nil
}

func (c *Client) partialUpdateFlagSingleDocument(ctx context.Context, flagName string, updates map[string]any) error {
	// In single-document mode, we need to prefix each update key with the flag name
	// to target the nested fields correctly. e.g., {"defaultValue": v} becomes {"my-flag.defaultValue": v}
	prefixedUpdates := make(bson.M)
	for key, value := range updates {
		prefixedKey := fmt.Sprintf("%s.%s", flagName, key)
		prefixedUpdates[prefixedKey] = value
	}

	res, err := c.collection.UpdateOne(ctx,
		bson.M{"_id": c.documentID},
		bson.M{"$set": prefixedUpdates},
	)
	if err != nil {
		return fmt.Errorf("updating flag %s in doc %s: %w", flagName, c.documentID, err)
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("document '%s' not found", c.documentID)
	}
	// Note: This doesn't guarantee the flag itself existed, only the parent doc.
	// The FlagExists check in the MCP tool is important for this reason.
	return nil
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
	if c.documentID != "" {
		return c.deleteFlagSingleDocument(ctx, flagName)
	}
	return c.deleteFlagMultiDocument(ctx, flagName)

}

func (c *Client) deleteFlagMultiDocument(ctx context.Context, flagName string) error {
	result, err := c.collection.DeleteOne(ctx, bson.M{"_id": flagName})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return errors.New("flag not found")
	}

	return nil
}

func (c *Client) deleteFlagSingleDocument(ctx context.Context, flagName string) error {
	_, err := c.collection.UpdateByID(ctx, c.documentID, bson.M{
		"$unset": bson.M{
			flagName: "",
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) FlagExists(ctx context.Context, flagName string) (bool, error) {
	var exists bool
	var err error
	for i := 0; i < c.maxTries; i++ {
		exists, err = c.flagExists(ctx, flagName)
		if err == nil {
			return exists, nil
		}
		c.logger.Error("error checking if flag exists, retrying", slog.Int("attempt", i+1), slog.String("flagName", flagName), slog.Any("error", err))
	}
	return false, fmt.Errorf("checking if flag %s exists after %d attempts: %w", flagName, c.maxTries, err)
}

func (c *Client) flagExists(ctx context.Context, flagName string) (bool, error) {
	if c.documentID != "" {
		return c.flagExistsSingleDocument(ctx, flagName)
	}
	return c.flagExistsMultiDocument(ctx, flagName)
}

func (c *Client) flagExistsMultiDocument(ctx context.Context, flagName string) (bool, error) {
	count, err := c.collection.CountDocuments(ctx, bson.M{"_id": flagName})
	if err != nil {
		return false, fmt.Errorf("counting document for flag %s: %w", flagName, err)
	}
	return count > 0, nil
}

func (c *Client) flagExistsSingleDocument(ctx context.Context, flagName string) (bool, error) {
	filter := bson.M{
		"_id":    c.documentID,
		flagName: bson.M{"$exists": true},
	}
	count, err := c.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("checking existence of flag %s in document %s: %w", flagName, c.documentID, err)
	}
	return count > 0, nil
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
