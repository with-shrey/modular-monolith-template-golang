// Package events provides a PostgreSQL-backed pub/sub EventBus built on Watermill.
//
// Delivery semantics:
//   - ConsumerGroup (default: <service>-consumer): messages are load-balanced across all
//     instances in the group — only one instance processes each message. Use this for
//     standard worker patterns.
//   - Without ConsumerGroup: every subscriber receives every message (broadcast).
//
// Handlers should be idempotent. On failure a message is Nacked and redelivered;
// the bus retries up to 3 times with exponential backoff before giving up.
//
// OTel context propagation: trace context is injected into message metadata on Publish
// and extracted in Subscribe, enabling end-to-end distributed tracing across services.
package events

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	watermillsql "github.com/ThreeDotsLabs/watermill-sql/v3/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/components/forwarder"
	"github.com/ThreeDotsLabs/watermill/message"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/ghuser/ghproject/pkg/config"
	"github.com/ghuser/ghproject/pkg/logger"
)

const (
	maxRetries      = 3
	retryBaseDelay  = time.Second
	shutdownTimeout = 30 * time.Second
	forwarderTopic  = "_forwarder_queue" // internal outbox topic for the Forwarder daemon
)

// EventBus is a PostgreSQL-backed pub/sub EventBus built on Watermill's SQL transport.
// It uses FOR UPDATE SKIP LOCKED under the hood for concurrent-safe delivery.
type EventBus struct {
	publisher    message.Publisher // either direct SQL publisher or forwarder-decorated
	subscriber   *watermillsql.Subscriber
	fwd          *forwarder.Forwarder // non-nil only when forwarder mode is enabled
	db           *sql.DB
	log          logger.Logger
	wg           sync.WaitGroup
	useForwarder bool
}

// NewEventBus opens a database connection from cfg.WatermillDatabaseURL and
// initializes a Watermill SQL publisher and subscriber. Schema tables are
// created automatically on first use.
//
// All instances with the same cfg.ServiceName share a ConsumerGroup, so each
// message is processed by exactly one instance (load-balanced, not broadcast).
func NewEventBus(cfg *config.Config, log logger.Logger) (*EventBus, error) {
	return newEventBus(cfg, log, false)
}

// NewEventBusWithForwarder creates an EventBus that uses the Forwarder pattern
// for at-least-once event delivery. Publish writes messages to a durable SQL
// queue; the Forwarder daemon (started with StartForwarder) asynchronously
// forwards them to the target topic. This guarantees no event loss if the
// process crashes after Publish returns.
//
// Call StartForwarder(ctx) after creating the bus to begin forwarding.
func NewEventBusWithForwarder(cfg *config.Config, log logger.Logger) (*EventBus, error) {
	return newEventBus(cfg, log, true)
}

func newEventBus(cfg *config.Config, log logger.Logger, useForwarder bool) (*EventBus, error) {
	db, err := sql.Open("pgx", cfg.DefinitionDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("events: open db: %w", err)
	}

	wlog := &slogAdapter{log: log}

	pub, err := watermillsql.NewPublisher(
		db,
		watermillsql.PublisherConfig{
			SchemaAdapter:        watermillsql.DefaultPostgreSQLSchema{},
			AutoInitializeSchema: true,
		},
		wlog,
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("events: new publisher: %w", err)
	}

	// In forwarder mode, wrap the publisher so messages are enveloped and
	// routed through the forwarder queue instead of published directly.
	var publisher message.Publisher = pub
	if useForwarder {
		publisher = forwarder.NewPublisher(pub, forwarder.PublisherConfig{
			ForwarderTopic: forwarderTopic,
		})
	}

	sub, err := watermillsql.NewSubscriber(
		db,
		watermillsql.SubscriberConfig{
			SchemaAdapter:    watermillsql.DefaultPostgreSQLSchema{},
			OffsetsAdapter:   watermillsql.DefaultPostgreSQLOffsetsAdapter{},
			InitializeSchema: true,
			ConsumerGroup:    cfg.ServiceName + "-consumer",
		},
		wlog,
	)
	if err != nil {
		_ = pub.Close()
		_ = db.Close()
		return nil, fmt.Errorf("events: new subscriber: %w", err)
	}

	return &EventBus{
		publisher:    publisher,
		subscriber:   sub,
		db:           db,
		log:          log,
		useForwarder: useForwarder,
	}, nil
}

// StartForwarder starts the background Forwarder daemon that reads messages from
// the internal forwarder queue and publishes them to their target topics.
// Must only be called once on an EventBus created with NewEventBusWithForwarder.
func (q *EventBus) StartForwarder(ctx context.Context) error {
	if !q.useForwarder {
		return fmt.Errorf("events: StartForwarder called on non-forwarder EventBus")
	}
	if q.fwd != nil {
		return fmt.Errorf("events: forwarder already started")
	}

	wlog := &slogAdapter{log: q.log}

	// Separate subscriber for the forwarder to drain the outbox queue.
	fwdSub, err := watermillsql.NewSubscriber(
		q.db,
		watermillsql.SubscriberConfig{
			SchemaAdapter:    watermillsql.DefaultPostgreSQLSchema{},
			OffsetsAdapter:   watermillsql.DefaultPostgreSQLOffsetsAdapter{},
			InitializeSchema: true,
			ConsumerGroup:    "forwarder-consumer",
		},
		wlog,
	)
	if err != nil {
		return fmt.Errorf("events: new forwarder subscriber: %w", err)
	}

	// Separate publisher for final delivery to target topics.
	targetPub, err := watermillsql.NewPublisher(
		q.db,
		watermillsql.PublisherConfig{
			SchemaAdapter:        watermillsql.DefaultPostgreSQLSchema{},
			AutoInitializeSchema: true,
		},
		wlog,
	)
	if err != nil {
		_ = fwdSub.Close()
		return fmt.Errorf("events: new forwarder target publisher: %w", err)
	}

	fwd, err := forwarder.NewForwarder(fwdSub, targetPub, wlog, forwarder.Config{
		ForwarderTopic: forwarderTopic,
	})
	if err != nil {
		_ = targetPub.Close()
		_ = fwdSub.Close()
		return fmt.Errorf("events: create forwarder: %w", err)
	}

	q.fwd = fwd

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		q.log.InfoContext(ctx, "events: forwarder started")
		if err := fwd.Run(ctx); err != nil {
			q.log.ErrorContext(ctx, "events: forwarder stopped with error", "error", err)
		} else {
			q.log.InfoContext(ctx, "events: forwarder stopped")
		}
	}()

	// Wait until the forwarder router is running before returning.
	select {
	case <-fwd.Running():
	case <-ctx.Done():
		return fmt.Errorf("events: context cancelled waiting for forwarder: %w", ctx.Err())
	}

	return nil
}

// DB returns the underlying *sql.DB for transactional publishing.
// Use NewTxPublisher to publish within the same transaction as your business logic.
func (q *EventBus) DB() *sql.DB {
	return q.db
}

// NewTxPublisher returns a Publisher bound to the given *sql.Tx.
// All Publish calls on the returned publisher execute within that transaction,
// enabling atomic "save data + publish event" semantics.
//
// If forwarder mode is enabled, messages are wrapped as forwarder envelopes so
// the background Forwarder daemon picks them up and delivers them to the real topic.
//
// AutoInitializeSchema is false — tables are guaranteed to exist after EventBus startup.
func (q *EventBus) NewTxPublisher(tx *sql.Tx) (message.Publisher, error) {
	wlog := &slogAdapter{log: q.log}
	pub, err := watermillsql.NewPublisher(
		tx,
		watermillsql.PublisherConfig{
			SchemaAdapter:        watermillsql.DefaultPostgreSQLSchema{},
			AutoInitializeSchema: false,
		},
		wlog,
	)
	if err != nil {
		return nil, fmt.Errorf("events: new tx publisher: %w", err)
	}
	if q.useForwarder {
		return forwarder.NewPublisher(pub, forwarder.PublisherConfig{
			ForwarderTopic: forwarderTopic,
		}), nil
	}
	return pub, nil
}

// Publish sends one or more messages to the given topic.
// OTel trace context from ctx is injected into each message's metadata so
// the receiving subscriber can restore the trace and continue the span tree.
func (q *EventBus) Publish(ctx context.Context, topic string, msgs ...*message.Message) error {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	for _, msg := range msgs {
		for k, v := range carrier {
			msg.Metadata.Set(k, v)
		}
	}
	if err := q.publisher.Publish(topic, msgs...); err != nil { //nolint:contextcheck
		return fmt.Errorf("events: publish to %s: %w", topic, err)
	}
	return nil
}

// Subscribe registers handler to process messages from topic asynchronously.
// The handler receives a context with the publisher's OTel trace restored from
// message metadata, enabling distributed tracing across service boundaries.
//
// Ack/Nack is managed by the bus:
//   - handler returns nil   → Ack (message consumed)
//   - handler returns error → retried up to 3× with exponential backoff (1s, 2s, 4s)
//   - all retries exhausted → Nack + error forwarded to the returned channel
//
// The returned error channel is buffered (capacity 100). Callers must drain it:
//
//	errCh, err := bus.Subscribe(ctx, topic, handler)
//	go func() { for err := range errCh { log.ErrorContext(ctx, "subscriber error", "error", err) } }()
//
// All in-flight handlers complete before Close() returns.
func (q *EventBus) Subscribe(ctx context.Context, topic string, handler func(context.Context, *message.Message) error) (<-chan error, error) {
	ch, err := q.subscriber.Subscribe(ctx, topic)
	if err != nil {
		return nil, fmt.Errorf("events: subscribe to %s: %w", topic, err)
	}

	errCh := make(chan error, 100)
	propagator := otel.GetTextMapPropagator()

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		defer close(errCh)

		for msg := range ch {
			// Restore the publisher's trace context from message metadata.
			carrier := propagation.MapCarrier{}
			for k, v := range msg.Metadata {
				carrier[k] = v
			}
			msgCtx := propagator.Extract(ctx, carrier)

			if err := retryWithBackoff(msgCtx, msg, handler, maxRetries, retryBaseDelay, q.log); err != nil {
				msg.Nack()
				select {
				case errCh <- err:
				default:
					q.log.ErrorContext(msgCtx, "events: error channel full, dropping error",
						"error", err, "topic", topic)
				}
			} else {
				msg.Ack()
			}
		}
	}()

	return errCh, nil
}

// retryWithBackoff calls handler up to maxRetries times with exponential backoff.
// Returns nil on first success; returns the last error after all retries exhaust.
func retryWithBackoff(
	ctx context.Context,
	msg *message.Message,
	handler func(context.Context, *message.Message) error,
	maxRetries int,
	baseDelay time.Duration,
	log logger.Logger,
) error {
	delay := baseDelay
	var err error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err = handler(ctx, msg); err == nil {
			return nil
		}
		if attempt < maxRetries {
			log.WarnContext(ctx, "events: handler failed, retrying",
				"attempt", attempt,
				"max_retries", maxRetries,
				"next_delay", delay,
				"error", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	return fmt.Errorf("events: handler failed after %d retries: %w", maxRetries, err)
}

// Ping checks the EventBus database connection health.
func (q *EventBus) Ping(ctx context.Context) error {
	if err := q.db.PingContext(ctx); err != nil {
		return fmt.Errorf("events: ping db: %w", err)
	}
	return nil
}

// Close gracefully shuts down the EventBus.
// Shutdown order: stop subscriber → stop forwarder (if running) → wait for
// in-flight handlers (30 s max) → close publisher → close database connection.
func (q *EventBus) Close() error {
	if err := q.subscriber.Close(); err != nil {
		return fmt.Errorf("events: close subscriber: %w", err)
	}

	if q.fwd != nil {
		if err := q.fwd.Close(); err != nil {
			return fmt.Errorf("events: close forwarder: %w", err)
		}
	}

	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		q.log.Error("events: timed out waiting for in-flight handlers to complete")
	}

	if err := q.publisher.Close(); err != nil {
		return fmt.Errorf("events: close publisher: %w", err)
	}
	return q.db.Close()
}

// slogAdapter bridges logger.Logger to watermill.LoggerAdapter.
type slogAdapter struct{ log logger.Logger }

func (a *slogAdapter) Error(msg string, err error, fields watermill.LogFields) {
	a.log.Error(msg, append(fieldsToArgs(fields), "error", err)...)
}
func (a *slogAdapter) Info(msg string, fields watermill.LogFields) {
	a.log.Info(msg, fieldsToArgs(fields)...)
}
func (a *slogAdapter) Debug(msg string, fields watermill.LogFields) {
	a.log.Debug(msg, fieldsToArgs(fields)...)
}
func (a *slogAdapter) Trace(msg string, fields watermill.LogFields) {
	a.log.Debug(msg, fieldsToArgs(fields)...)
}
func (a *slogAdapter) With(fields watermill.LogFields) watermill.LoggerAdapter {
	return &slogAdapter{log: a.log.With(fieldsToArgs(fields)...)}
}

func fieldsToArgs(fields watermill.LogFields) []any {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return args
}
