package watchhandler

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (w *WatchHandler) changestream() error {
	var pipeline mongo.Pipeline
	if w.documentID != "" {
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"operationType":    bson.M{"$in": []string{"insert", "update ", "replace", "delete"}},
				"fullDocument._id": w.documentID,
			}}},
		}
	} else {
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"operationType": bson.M{"$in": []string{"insert", "update", "replace", "delete"}},
			}}},
		}
	}
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	ctx, cancel := context.WithCancel(w.ctx)
	defer cancel()

	cs, err := w.collection.Watch(ctx, pipeline, opts)
	if err != nil {
		return fmt.Errorf("starting change stream: %w", err)
	}
	defer cs.Close(context.WithoutCancel(w.ctx))

	for cs.Next(ctx) {
		if err := cs.Err(); err != nil {
			return fmt.Errorf("change stream error: %w", err)
		}
		var csEvent ChangeStreamEvent
		if err := cs.Decode(&csEvent); err != nil {
			return fmt.Errorf("decoding change stream document: %w", err)
		}
		fmt.Println(csEvent)
		if err := w.handleEvent(csEvent); err != nil {
			return fmt.Errorf("handling change stream event: %w", err)
		}
	}
	if err := cs.Err(); err != nil {
		return fmt.Errorf("error iterating change stream: %w", err)
	}
	if err := ctx.Err(); err != nil {
		if err == context.Canceled {
			w.logger.Info("watch cancelled", "documentID", w.documentID)
			return nil
		}
		return fmt.Errorf("context error: %w", err)
	}
	return nil
}
