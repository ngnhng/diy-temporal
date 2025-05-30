// Convenient wrapper to manage connections to NATS cluster for the SDK
package nats

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Client manages the connection to NATS and JetStream.
type Client struct {
	nc *nats.Conn
	js jetstream.JetStream
}

// NewClient creates a new NATS client, connects to the server,
// and initializes the JetStream context using the new API.
func NewClient(ctx context.Context) (*Client, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL // Default to localhost if not set
	}

	// Connect to NATS
	nc, err := nats.Connect(
		natsURL,
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			fmt.Printf("NATS disconnected: %v\n", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			fmt.Println("NATS connection closed")
		}),
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(5),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", natsURL, err)
	}

	// Create JetStream Context using the new API
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream instance: %w", err)
	}

	return &Client{
		nc,
		js,
	}, nil
}

// Conn returns the underlying NATS connection.
func (c *Client) Conn() *nats.Conn {
	return c.nc
}

// JetStream returns the JetStream instance.
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Close closes the NATS connection.
func (c *Client) Close() error {
	if c.nc != nil && !c.nc.IsClosed() {
		c.nc.Close()
	}

	return nil
}

func (c *Client) EnsureKV(ctx context.Context, cfg jetstream.KeyValueConfig) (jetstream.KeyValue, error) {
	_, err := c.js.KeyValue(ctx, cfg.Bucket)
	if err != nil {
		if err == jetstream.ErrBucketNotFound {
			kv, err := c.js.KeyValue(ctx, cfg.Bucket)
			if err != nil {
				return nil, fmt.Errorf("failed to create KV: %v, %w", cfg.Bucket, err)
			}
			return kv, nil
		}
		return nil, fmt.Errorf("failed to create KV: %v, %w", cfg.Bucket, err)
	}

	updatedKV, err := c.js.UpdateKeyValue(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to update KV: %v, %w", cfg.Bucket, err)
	}

	return updatedKV, err
}

// EnsureStream ensures that a stream with the given configuration exists.
// It creates the stream if it doesn't exist or updates it if it does.
func (c *Client) EnsureStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	stream, err := c.js.Stream(ctx, cfg.Name)
	if err != nil || stream == nil {
		if err == jetstream.ErrStreamNotFound {
			stream, err = c.js.CreateStream(ctx, cfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create stream %s: %w", cfg.Name, err)
			}
			return stream, nil
		}
		// Other error fetching stream info
		return nil, fmt.Errorf("failed to get stream %s info: %w", cfg.Name, err)
	}

	// Stream exists, update it (idempotent)
	// Note: UpdateStream might return specific errors if the update is incompatible.
	streamInfo, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info %s: %w", cfg.Name, err)
	}
	if streamInfo.Config.Retention == jetstream.WorkQueuePolicy {
		cfg.Retention = jetstream.WorkQueuePolicy
	} else {
		cfg.Retention = streamInfo.Config.Retention
	}

	updatedStream, err := c.js.UpdateStream(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to update stream %s: %w", cfg.Name, err)
	}
	return updatedStream, nil
}

// EnsureConsumer ensures that a consumer with the given configuration exists on the specified stream.
// It creates the consumer if it doesn't exist or updates it if it does.
func (c *Client) EnsureConsumer(ctx context.Context, streamName string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	stream, err := c.js.Stream(ctx, streamName)
	if err != nil || stream == nil {
		return nil, fmt.Errorf("failed to get stream %s for consumer creation: %w", streamName, err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, cfg)
	if err != nil || consumer == nil {
		return nil, fmt.Errorf("failed to create/update consumer %s on stream %s: %w", cfg.Name, streamName, err)
	}
	return consumer, nil
}

// PublishSync publishes a message synchronously to a subject and waits for acknowledgement.
func (c *Client) PublishSync(ctx context.Context, subj string, data []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	ack, err := c.js.Publish(ctx, subj, data, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to publish message to subject %s: %w", subj, err)
	}
	return ack, nil
}

// FetchMessages fetches a batch of messages from a consumer.
func (c *Client) FetchMessages(ctx context.Context, consumer jetstream.Consumer, batchSize int, opts ...jetstream.FetchOpt) (jetstream.MessageBatch, error) {
	msgs, err := consumer.Fetch(batchSize, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}
	return msgs, nil
}
