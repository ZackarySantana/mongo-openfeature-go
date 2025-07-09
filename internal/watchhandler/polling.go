package watchhandler

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	pollingInterval = time.Second * 5
)

func (w *WatchHandler) polling() error {
	filter := bson.M{}
	if w.documentID != "" {
		filter["_id"] = w.documentID
	}

	ctx, cancel := context.WithCancel(w.ctx)
	defer cancel()

	// ticker
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var result bson.M
			err := w.collection.FindOne(ctx, filter).Decode(&result)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					w.logger.Debug("no document found", "documentID", w.documentID)
					continue
				}
				return fmt.Errorf("error finding document: %w", err)
			}

			w.handleEvent(ChangeStreamEvent{
				FullDocument: result,
			})
		case <-ctx.Done():
			w.logger.Info("polling cancelled", "documentID", w.documentID)
			return nil

		}
	}
}
