package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	startupProbeDelayEnv   = "STARTUP_PROBE_DELAY"
	readinessProbeDelayEnv = "READINESS_PROBE_DELAY"
	livenessProbeDelayEnv  = "LIVENESS_PROBE_DELAY"
)

var (
	inShutdown          bool = false
	m                   *metrics
	startupProbeDelay   time.Duration = 0
	readinessProbeDelay time.Duration = 0
	livenessProbeDelay  time.Duration = 0
)

type metrics struct {
	activeRequests prometheus.Gauge
	requestCounter *prometheus.CounterVec
}

type configs struct {
	Startup   string `json:"startup"`
	Readiness string `json:"readiness"`
	Liveness  string `json:"liveness"`
}

func setProbeDelay(probeEnv string, envValue string) time.Duration {
	os.Setenv(probeEnv, envValue)
	delay, err := strconv.ParseInt(envValue, 10, 8)
	if err != nil {
		log.Printf("Invalid delay value for %s: %v", probeEnv, err)
		return 0
	}
	return time.Duration(delay) * time.Second
}

func probeHandler(probeDelay time.Duration, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		time.Sleep(probeDelay)
		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}

func postConfigs(c *gin.Context) {
	m.activeRequests.Inc()
	defer m.activeRequests.Dec()

	var newConfigs configs
	if err := c.BindJSON(&newConfigs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Update the probe delay variables
	startupProbeDelay = setProbeDelay(startupProbeDelayEnv, newConfigs.Startup)
	readinessProbeDelay = setProbeDelay(readinessProbeDelayEnv, newConfigs.Readiness)
	livenessProbeDelay = setProbeDelay(livenessProbeDelayEnv, newConfigs.Liveness)

	c.JSON(http.StatusCreated, newConfigs)
	m.requestCounter.WithLabelValues("POST", "/config", strconv.Itoa(c.Writer.Status())).Inc()
}

func delayRequest(c *gin.Context) {
	m.activeRequests.Inc()
	defer m.activeRequests.Dec()

	delay, err := strconv.ParseInt(c.Param("seconds"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay value"})
		return
	}
	time.Sleep(time.Duration(delay) * time.Second)

	c.JSON(http.StatusOK, gin.H{"message": delay})
	m.requestCounter.WithLabelValues(c.Request.Method, c.FullPath(), strconv.Itoa(c.Writer.Status())).Inc()
}

func graceDelayRequest(c *gin.Context) {
	m.activeRequests.Inc()
	defer m.activeRequests.Dec()

	delay, err := strconv.ParseInt(c.Param("seconds"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay value"})
		return
	}
	var delayInc int64 = 0

	if delay > 0 {
		for delayInc < delay {
			delayInc++
			time.Sleep(1 * time.Second)

			if inShutdown {
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": delayInc})
	m.requestCounter.WithLabelValues(c.Request.Method, c.FullPath(), strconv.Itoa(c.Writer.Status())).Inc()
}

func setMetrics(promRegistry *prometheus.Registry) *metrics {
	metricList := &metrics{
		requestCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Number of HTTP requests",
		},
			[]string{"method", "endpoint", "statusCode"},
		),
		activeRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_requests",
			Help: "Number of active requests",
		}),
	}

	promRegistry.MustRegister(metricList.requestCounter)
	promRegistry.MustRegister(metricList.activeRequests)
	return metricList
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)
	promRegistry.MustRegister(version.NewCollector("prober"))

	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})))
	// Probes
	router.GET("/startup", probeHandler(startupProbeDelay, "startup"))
	router.GET("/readiness", probeHandler(readinessProbeDelay, "readiness"))
	router.GET("/liveness", probeHandler(livenessProbeDelay, "liveness"))
	// Config
	router.POST("/config", postConfigs)

	// Request Delay
	router.GET("/delay/:seconds", delayRequest)
	router.GET("/graceDelay/:seconds", graceDelayRequest)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	srvErrs := make(chan error, 1)
	go func() {
		srvErrs <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	shutdown := gracefulShutdown(srv)

	select {
	case err := <-srvErrs:
		shutdown(err)
	case sig := <-quit:
		shutdown(sig)
	}

	log.Println("Server exiting")
}

func gracefulShutdown(srv *http.Server) func(reason interface{}) {
	return func(reason interface{}) {
		inShutdown = true

		log.Println("Server shutdown: ", reason)

		ctx, cancel := context.WithTimeout(context.Background(), 260*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Println("Error to Gracefully shutdown server: ", err)
		}
	}
}
