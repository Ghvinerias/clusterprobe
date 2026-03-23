# Coverage Gap Report
Generated: 2026-03-23T13:07:08Z
Phase: 1
Threshold: 70%

## Summary
| Package | Current | Target | Gap | Uncovered Functions |
|---------|---------|--------|-----|---------------------|
| internal/chaos | 52.0% | 70% | 18.0% | 0 |
| internal/config | 85.7% | 70% | 0.0% | 0 |
| internal/db | 34.7% | 70% | 35.3% | 13 |
| internal/messaging | 47.2% | 70% | 22.8% | 10 |
| internal/telemetry | 51.1% | 70% | 18.9% | 4 |
| internal/workload | 61.7% | 70% | 8.3% | 5 |

## Per-Package Detail

### internal/chaos  (52.0% → needs 70%)

#### Partially covered functions (<70%)
**`internal/chaos/client.go:48` — `func (c *ChaosClient) Apply(ctx context.Context, spec ExperimentSpec) (string, error)`**
- Does: Apply creates a Chaos Mesh experiment and returns the created name.
- Dependencies: k8s dynamic client, unstructured resources, OTEL tracer
- Mock available: dynamic fake client in internal/chaos/client_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/chaos/client.go:111` — `func (c *ChaosClient) Status(ctx context.Context, id string) (ExperimentStatus, error)`**
- Does: Status returns the status of an experiment by name.
- Dependencies: k8s dynamic client, unstructured resources, OTEL tracer
- Mock available: dynamic fake client in internal/chaos/client_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/chaos/client.go:144` — `func (c *ChaosClient) Delete(ctx context.Context, id string) error`**
- Does: Delete removes an experiment by name.
- Dependencies: k8s dynamic client, unstructured resources, OTEL tracer
- Mock available: dynamic fake client in internal/chaos/client_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/chaos/client.go:193` — `func statusFromObject(obj *unstructured.Unstructured) ExperimentStatus`**
- Does: statusFromObject implementation in client.go
- Dependencies: k8s dynamic client, unstructured resources, OTEL tracer
- Mock available: dynamic fake client in internal/chaos/client_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised


### internal/config  (85.7% → needs 70%)

#### Partially covered functions (<70%)
**`internal/config/config.go:81` — `func (c Config) Validate() error`**
- Does: Validate ensures required configuration values are set and sane.
- Dependencies: env vars, strconv, string validation
- Mock available: unit tests in internal/config/config_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised


### internal/db  (34.7% → needs 70%)

#### Uncovered functions (0.0% coverage)
**`internal/db/mongo.go:31` — `func NewMongo(ctx context.Context, uri, database string) (*MongoClient, error)`**
- Does: NewMongo creates a Mongo client with retry/backoff.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:51` — `func connectMongoWithRetry(ctx context.Context, uri string) (*mongo.Client, error)`**
- Does: connectMongoWithRetry implementation in mongo.go
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: complex
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:87` — `func (c *MongoClient) Collection(name string) *mongo.Collection`**
- Does: Collection returns a collection handle.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:92` — `func (c *MongoClient) InsertOne(ctx context.Context, collection string, doc any) (*mongo.InsertOneResult, error)`**
- Does: InsertOne inserts a document.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:113` — `func (c *MongoClient) InsertMany(ctx context.Context, collection string, docs []any) (*mongo.InsertManyResult, error)`**
- Does: InsertMany inserts documents.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:134` — `func (c *MongoClient) Find(ctx context.Context, collection string, filter any) (*mongo.Cursor, error)`**
- Does: Find queries for documents.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:155` — `func (c *MongoClient) FindOne(ctx context.Context, collection string, filter any) (*mongo.SingleResult, error)`**
- Does: FindOne queries for a single document.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:176` — `func (c *MongoClient) UpdateOne( ctx context.Context, collection string, filter any, update any, ) (*mongo.UpdateResult, error)`**
- Does: UpdateOne updates a single document.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/mongo.go:202` — `func (c *MongoClient) Close(ctx context.Context) error`**
- Does: Close disconnects the client.
- Dependencies: mongo client, OTEL tracer
- Mock available: integration test only (testcontainers) in internal/db/mongo_test.go; no unit mock
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/postgres.go:43` — `func NewPostgres(ctx context.Context, dsn string, maxConns int32) (*PostgresClient, error)`**
- Does: NewPostgres creates a new Postgres client with retry/backoff.
- Dependencies: pgxpool/pgx, OTEL tracer
- Mock available: pgxmock pool in internal/db/postgres_test.go
- Complexity: complex
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/postgres.go:68` — `func connectPostgresWithRetry(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error)`**
- Does: connectPostgresWithRetry implementation in postgres.go
- Dependencies: pgxpool/pgx, OTEL tracer
- Mock available: pgxmock pool in internal/db/postgres_test.go
- Complexity: complex
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/postgres.go:104` — `func (c *PostgresClient) Ping(ctx context.Context) error`**
- Does: Ping verifies connectivity.
- Dependencies: pgxpool/pgx, OTEL tracer
- Mock available: pgxmock pool in internal/db/postgres_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/db/postgres.go:182` — `func (c *PostgresClient) Close()`**
- Does: Close releases pool resources.
- Dependencies: pgxpool/pgx, OTEL tracer
- Mock available: pgxmock pool in internal/db/postgres_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set


#### Partially covered functions (<70%)
**`internal/db/postgres.go:112` — `func (c *PostgresClient) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)`**
- Does: Exec executes a statement with an OTEL span.
- Dependencies: pgxpool/pgx, OTEL tracer
- Mock available: pgxmock pool in internal/db/postgres_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/postgres.go:132` — `func (c *PostgresClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)`**
- Does: Query executes a query with an OTEL span.
- Dependencies: pgxpool/pgx, OTEL tracer
- Mock available: pgxmock pool in internal/db/postgres_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:51` — `func connectRedisWithRetry(ctx context.Context, options *redis.Options) (*redis.Client, error)`**
- Does: connectRedisWithRetry implementation in redis.go
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:85` — `func (c *RedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error`**
- Does: Set sets a key with expiration.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:104` — `func (c *RedisClient) Get(ctx context.Context, key string) (string, error)`**
- Does: Get returns a value.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:124` — `func (c *RedisClient) Del(ctx context.Context, keys ...string) (int64, error)`**
- Does: Del deletes keys.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:144` — `func (c *RedisClient) Incr(ctx context.Context, key string) (int64, error)`**
- Does: Incr increments a key.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:164` — `func (c *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error)`**
- Does: Expire sets expiration.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:184` — `func (c *RedisClient) Publish(ctx context.Context, channel string, payload any) (int64, error)`**
- Does: Publish publishes a message.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:204` — `func (c *RedisClient) Subscribe(ctx context.Context, channel string) (*redis.PubSub, error)`**
- Does: Subscribe subscribes to a channel.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/db/redis.go:225` — `func (c *RedisClient) Close() error`**
- Does: Close closes the client.
- Dependencies: go-redis client, OTEL tracer
- Mock available: miniredis in internal/db/redis_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised


### internal/messaging  (47.2% → needs 70%)

#### Uncovered functions (0.0% coverage)
**`internal/messaging/rabbitmq.go:68` — `func (c realConn) Channel() (amqpChannel, error)`**
- Does: Channel implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:76` — `func (c realConn) NotifyClose(ch chan *amqp.Error) chan *amqp.Error`**
- Does: NotifyClose implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:80` — `func (c realConn) Close() error`**
- Does: Close implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:88` — `func NewProducer(ctx context.Context, url string) (*Producer, error)`**
- Does: NewProducer creates a producer and starts reconnect monitoring.
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: complex
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:340` — `func (e UnrecoverableError) Error() string`**
- Does: Error returns the wrapped error message.
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:345` — `func (e UnrecoverableError) Unwrap() error`**
- Does: Unwrap returns the wrapped error.
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:361` — `func NewConsumer(ctx context.Context, url string, prefetch int) (*Consumer, error)`**
- Does: NewConsumer creates a new RabbitMQ consumer.
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: complex
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:393` — `func (c *Consumer) Consume( ctx context.Context, queue string, handler func(context.Context, amqp.Delivery) error, ) error`**
- Does: Consume starts consuming messages from a queue and invokes handler for each delivery.
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:472` — `func (c *Consumer) consumeOnce(ctx context.Context) (<-chan amqp.Delivery, error)`**
- Does: consumeOnce implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/messaging/rabbitmq.go:558` — `func (c *Consumer) injectTraceContext(ctx context.Context, msg amqp.Delivery) context.Context`**
- Does: injectTraceContext implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set


#### Partially covered functions (<70%)
**`internal/messaging/rabbitmq.go:169` — `func (p *Producer) reconnect(ctx context.Context)`**
- Does: reconnect implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/messaging/rabbitmq.go:230` — `func (p *Producer) DeclareTopology(ctx context.Context) error`**
- Does: DeclareTopology sets up exchanges, queues, and bindings for ClusterProbe.
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/messaging/rabbitmq.go:541` — `func (c *Consumer) reconnect(ctx context.Context)`**
- Does: reconnect implementation in rabbitmq.go
- Dependencies: amqp091-go, OTEL propagator/tracer
- Mock available: mockConn/mockChannel in internal/messaging/rabbitmq_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised


### internal/telemetry  (51.1% → needs 70%)

#### Uncovered functions (0.0% coverage)
**`internal/telemetry/otel.go:111` — `func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool`**
- Does: Enabled implementation in otel.go
- Dependencies: OTEL SDK, slog handlers, OTLP exporters
- Mock available: OTLP stub in internal/telemetry/otel_test.go
- Complexity: simple
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/telemetry/otel.go:115` — `func (h *fanoutHandler) Handle(ctx context.Context, record slog.Record) error`**
- Does: Handle implementation in otel.go
- Dependencies: OTEL SDK, slog handlers, OTLP exporters
- Mock available: OTLP stub in internal/telemetry/otel_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/telemetry/otel.go:133` — `func (h *fanoutHandler) WithGroup(name string) slog.Handler`**
- Does: WithGroup implementation in otel.go
- Dependencies: OTEL SDK, slog handlers, OTLP exporters
- Mock available: OTLP stub in internal/telemetry/otel_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/telemetry/otel.go:140` — `func injectTraceID(ctx context.Context, record slog.Record) slog.Record`**
- Does: injectTraceID implementation in otel.go
- Dependencies: OTEL SDK, slog handlers, OTLP exporters
- Mock available: OTLP stub in internal/telemetry/otel_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set


### internal/workload  (61.7% → needs 70%)

#### Uncovered functions (0.0% coverage)
**`internal/workload/types.go:30` — `func (p LoadProfile) Validate() error`**
- Does: Validate validates the load profile.
- Dependencies: pure validation logic
- Mock available: no tests yet for validation in internal/workload/workload_test.go
- Complexity: complex
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/workload/types.go:62` — `func (r ScenarioRequest) Validate() error`**
- Does: Validate validates the scenario request.
- Dependencies: pure validation logic
- Mock available: no tests yet for validation in internal/workload/workload_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/workload/types.go:82` — `func (r ScenarioResponse) Validate() error`**
- Does: Validate validates the scenario response.
- Dependencies: pure validation logic
- Mock available: no tests yet for validation in internal/workload/workload_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/workload/types.go:109` — `func (r ChaosExperimentRequest) Validate() error`**
- Does: Validate validates the chaos experiment request.
- Dependencies: pure validation logic
- Mock available: no tests yet for validation in internal/workload/workload_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set

**`internal/workload/types.go:130` — `func (r ChaosExperimentResponse) Validate() error`**
- Does: Validate validates the chaos experiment response.
- Dependencies: pure validation logic
- Mock available: no tests yet for validation in internal/workload/workload_test.go
- Complexity: medium
- Suggested test: cover success + error branches; assert returned errors wrap context and span status is set


#### Partially covered functions (<70%)
**`internal/workload/db_read.go:81` — `func randomDuration(maxMs int64) (time.Duration, error)`**
- Does: randomDuration implementation in db_read.go
- Dependencies: Store interfaces (Rows/Row), OTEL metric
- Mock available: mockStore/mockRow/mockRows in internal/workload/workload_test.go
- Complexity: medium
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/workload/db_write.go:20` — `func (g *DBWriteGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error)`**
- Does: Execute writes load_events rows in batches for the configured duration.
- Dependencies: Store interface, OTEL tracer
- Mock available: mockStore in internal/workload/workload_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised

**`internal/workload/mixed.go:20` — `func (g *MixedGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error)`**
- Does: Execute runs CPU, DB write, and DB read workloads based on ratio.
- Dependencies: CPU/DB generators, Store interface
- Mock available: mockStore in internal/workload/workload_test.go
- Complexity: complex
- Missing branches: error paths, input validation, or context cancellation
- Suggested test: add table-driven cases for error/edge paths not currently exercised


## Test writing guidance for coverage-fixer-agent
- Use testify/assert and testify/require (already in go.mod).
- Table-driven tests with t.Run for all non-trivial functions.
- For Postgres: pgxmock v2 (already used in internal/db/postgres_test.go).
- For Redis: miniredis/v2 (already used in internal/db/redis_test.go).
- For MongoDB: prefer unit-level abstractions or mongo/mtest; integration tests are build-tagged and do not count toward coverage.
- For RabbitMQ: use existing mockConn/mockChannel in internal/messaging/rabbitmq_test.go; avoid real AMQP dial.
- For OTEL: use go.opentelemetry.io/otel/trace/noop or in-process OTLP stub.
- For k8s dynamic client (chaos): use k8s.io/client-go/dynamic/fake (already used in internal/chaos/client_test.go).
- Do not test unexported functions directly; test them through exported APIs when possible.
- Each new unit test file should remain in the package (no _test package) to access helpers if needed.
- If a test relies only on mocks, add //go:build !integration where appropriate.
