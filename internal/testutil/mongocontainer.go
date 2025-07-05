package testutil

import (
	"context"
	"fmt"
	"os"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func CreateMongoContainer(ctx context.Context) (func(), error) {
	mongodbContainer, err := mongodb.Run(ctx, "mongo:8", testcontainers.CustomizeRequestOption(
		func(req *testcontainers.GenericContainerRequest) error {
			// req.Name = "edith-mongodb"
			// req.Reuse = true
			return nil
		},
	), mongodb.WithReplicaSet("rs0"))
	if err != nil {
		return nil, fmt.Errorf("failed to start mongodb container: %w", err)
	}
	cleanup := func() {
		fmt.Println("Terminating mongodb container...")
		if err := testcontainers.TerminateContainer(mongodbContainer); err != nil {
			fmt.Printf("failed to terminate mongodb container: %v", err)
		} else {
			fmt.Println("Terminated mongodb container")
		}
	}
	endpoint, err := mongodbContainer.ConnectionString(ctx)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to get mongodb container connection string: %w", err)
	}
	if err = os.Setenv("MONGODB_ENDPOINT", endpoint+"&directConnection=true"); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to set MONGO_ENDPOINT: %w", err)
	}
	return cleanup, nil
}
