package tidb_slow_query

import (
	"context"
	"github.com/elastic/beats/v7/libbeat/common/backoff"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"time"
)

type backoffClient struct {
	client *client

	done    chan struct{}
	backoff backoff.Backoff
}

func newBackoffClient(client *client, init, max time.Duration) *backoffClient {
	done := make(chan struct{})
	bkf := backoff.NewEqualJitterBackoff(done, init, max)
	return &backoffClient{
		client:  client,
		done:    done,
		backoff: bkf,
	}
}

// Connect is called by pipeline.netClientWorker.run()
func (b *backoffClient) Connect() error {
	err := b.client.Connect()
	if err != nil {
		// give the client a chance to promote an internal error to a network error.
		b.backoff.Wait()
	} else {
		b.backoff.Reset()
	}
	return err
}

func (b *backoffClient) Close() error {
	err := b.client.Close()
	close(b.done)
	return err
}

// Publish does following:
//   - close the wrapped client on error
//   - wait for a period of time before retrying
func (b *backoffClient) Publish(ctx context.Context, batch publisher.Batch) error {
	err := b.client.Publish(ctx, batch)
	if err != nil {
		b.client.Close()
		b.backoff.Wait()
	} else {
		b.backoff.Reset()
	}
	return err
}

func (b *backoffClient) String() string {
	return b.client.String()
}
