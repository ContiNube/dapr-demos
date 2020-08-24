package main

import (
	"context"
	"log"

	"net/http"
	"os"
	"strings"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/http"
	"github.com/pkg/errors"
)

var (
	logger     = log.New(os.Stdout, "", 0)
	address    = getEnvVar("ADDRESS", ":8080")
	pubSubName = getEnvVar("PUBSUB_NAME", "tweeter-pubsub")
	topicName  = getEnvVar("TOPIC_NAME", "tweets")
	client     dapr.Client
)

func main() {
	// create a Dapr service
	s := daprd.NewService(address)

	// create a Dapr client
	c, err := dapr.NewClient()
	if err != nil {
		logger.Fatalf("error creating Dapr client: %v", err)
	}
	client = c
	defer client.Close()

	// add some input binding handler
	if err := s.AddBindingInvocationHandler("tweets", tweetHandler); err != nil {
		logger.Fatalf("error adding binding handler: %v", err)
	}

	// start the service
	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("error starting service: %v", err)
	}
}

func tweetHandler(ctx context.Context, in *common.BindingEvent) (out []byte, err error) {
	//logger.Printf("Tweet - Metadata:%v", in.Metadata)
	if err := client.PublishEvent(ctx, pubSubName, topicName, in.Data); err != nil {
		return nil, errors.Wrapf(err, "error publishing to %s/%s", pubSubName, topicName)
	}
	return nil, nil
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}
