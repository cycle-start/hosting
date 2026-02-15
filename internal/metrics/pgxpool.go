package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterPgxPoolMetrics exposes pgx connection pool statistics as Prometheus gauges.
func RegisterPgxPoolMetrics(pool *pgxpool.Pool) {
	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "pgxpool_acquired_conns",
			Help: "Number of currently acquired connections in the pool",
		}, func() float64 {
			return float64(pool.Stat().AcquiredConns())
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "pgxpool_max_conns",
			Help: "Maximum number of connections in the pool",
		}, func() float64 {
			return float64(pool.Stat().MaxConns())
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "pgxpool_total_conns",
			Help: "Total number of connections in the pool",
		}, func() float64 {
			return float64(pool.Stat().TotalConns())
		}),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "pgxpool_idle_conns",
			Help: "Number of idle connections in the pool",
		}, func() float64 {
			return float64(pool.Stat().IdleConns())
		}),
	)
}
