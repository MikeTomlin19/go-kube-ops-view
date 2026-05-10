package metrics

import (
	"context"
	"runtime"
	"time"
)

// Collector periodically collects and updates system metrics
type Collector struct {
	metrics   *Metrics
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewCollector creates a new metrics collector
func NewCollector(metrics *Metrics) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		metrics:   metrics,
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins collecting system metrics periodically
func (c *Collector) Start() {
	// Update application start time
	c.metrics.ApplicationStartTime.Set(float64(c.startTime.Unix()))

	// Start periodic collection
	ticker := time.NewTicker(15 * time.Second) // Collect every 15 seconds
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-ticker.C:
				c.collectSystemMetrics()
			}
		}
	}()
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	c.cancel()
}

// collectSystemMetrics collects and updates system-level metrics
func (c *Collector) collectSystemMetrics() {
	// Collect runtime metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Update memory usage (using heap in use as a reasonable approximation)
	c.metrics.MemoryUsage.Set(float64(memStats.HeapInuse))

	// Update goroutine count
	c.metrics.GoRoutinesActive.Set(float64(runtime.NumGoroutine()))

	// Update application uptime
	uptime := time.Since(c.startTime)
	c.metrics.ApplicationUptime.Set(uptime.Seconds())

	// Note: CPU usage is more complex to calculate accurately in Go
	// For now, we'll leave it at 0 and could implement it later using
	// system-specific calls or third-party libraries if needed
}

// UpdateConnectionMetrics updates connection-related metrics
func (c *Collector) UpdateConnectionMetrics(httpConnections, sseConnections int) {
	c.metrics.HTTPActiveConnections.Set(float64(httpConnections))
	c.metrics.SSEActiveConnections.Set(float64(sseConnections))
}

// UpdateClusterMetrics updates cluster availability metrics
func (c *Collector) UpdateClusterMetrics(totalClusters, availableClusters int) {
	c.metrics.ClustersTotal.Set(float64(totalClusters))
	c.metrics.ClustersAvailable.Set(float64(availableClusters))
}

// UpdateAuthMetrics updates authentication-related metrics
func (c *Collector) UpdateAuthMetrics(activeTokens, activeSessions int) {
	c.metrics.AuthTokensActive.Set(float64(activeTokens))
	c.metrics.AuthSessionsTotal.Set(float64(activeSessions))
}

// UpdateStorageMetrics updates storage connection pool metrics
func (c *Collector) UpdateStorageMetrics(poolSize int) {
	c.metrics.StorageConnectionPool.Set(float64(poolSize))
}
