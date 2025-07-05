package watchhandler

import "go.mongodb.org/mongo-driver/v2/bson"

type ChangeStreamEvent struct {
	FullDocument  bson.M `bson:"fullDocument"`
	OperationType string `bson:"operationType"`
}
