package receiptpipeline

const (
	defaultQueueBufferSize = 1
	defaultWorkerCount     = 1
)

// Config contains only values required to configure the receipt pipeline.
//
// Runtime dependencies such as queues, workers, processors, and loggers are
// created by the pipeline constructor. Keeping dependencies out of Config makes
// it safe to load this struct from environment variables, YAML, JSON, or tests.
type Config struct {
	Queue   QueueConfig
	Workers WorkerConfig
}

// QueueConfig controls in-memory queue capacity.
//
// Single and batch queues are intentionally separated. Single-receipt requests
// usually need lower latency, while batch jobs are more throughput-oriented.
type QueueConfig struct {
	SingleBuffer int
	BatchBuffer  int
}

// WorkerConfig controls how many workers process each queue.
//
// Single and batch worker counts are independent because these workloads have
// different latency, throughput, and resource-consumption profiles.
type WorkerConfig struct {
	SingleCount int
	BatchCount  int
}

func (c Config) normalized() Config {
	c.Queue = c.Queue.normalized()
	c.Workers = c.Workers.normalized()

	return c
}

func (c QueueConfig) normalized() QueueConfig {
	if c.SingleBuffer < 1 {
		c.SingleBuffer = defaultQueueBufferSize
	}

	if c.BatchBuffer < 1 {
		c.BatchBuffer = defaultQueueBufferSize
	}

	return c
}

func (c WorkerConfig) normalized() WorkerConfig {
	if c.SingleCount < 1 {
		c.SingleCount = defaultWorkerCount
	}

	if c.BatchCount < 1 {
		c.BatchCount = defaultWorkerCount
	}

	return c
}
