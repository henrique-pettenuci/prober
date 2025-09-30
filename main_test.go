package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"
)

func TestStartupProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/startup", probeHandler(0, "startup"))

	req, _ := http.NewRequest("GET", "/startup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":"startup"}`, w.Body.String())
}

func TestStartupProbeWithDelay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	delay := 100 * time.Millisecond
	router.GET("/startup", probeHandler(delay, "startup"))

	req, _ := http.NewRequest("GET", "/startup", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":"startup"}`, w.Body.String())
	assert.GreaterOrEqual(t, duration, delay)
}

func TestReadinessProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/readiness", probeHandler(0, "readiness"))

	req, _ := http.NewRequest("GET", "/readiness", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":"readiness"}`, w.Body.String())
}

func TestLivenessProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/liveness", probeHandler(0, "liveness"))

	req, _ := http.NewRequest("GET", "/liveness", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":"liveness"}`, w.Body.String())
}

func TestPostConfigs(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/config", postConfigs)

	body := `{"startup":"5","readiness":"10","liveness":"15"}`
	req, _ := http.NewRequest("POST", "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, `{"startup":"5","readiness":"10","liveness":"15"}`, w.Body.String())
	assert.Equal(t, "5", os.Getenv("STARTUP_PROBE_DELAY"))
	assert.Equal(t, "10", os.Getenv("READINESS_PROBE_DELAY"))
	assert.Equal(t, "15", os.Getenv("LIVENESS_PROBE_DELAY"))
	assert.Equal(t, 5*time.Second, startupProbeDelay)
	assert.Equal(t, 10*time.Second, readinessProbeDelay)
	assert.Equal(t, 15*time.Second, livenessProbeDelay)
}

func TestPostConfigsInvalidJSON(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/config", postConfigs)

	body := `{"startup":"5","readiness":"10","liveness":}`
	req, _ := http.NewRequest("POST", "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid JSON")
}

func TestDelayRequest(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	req, _ := http.NewRequest("GET", "/delay/1", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":1}`, w.Body.String())
	assert.GreaterOrEqual(t, duration, 1*time.Second)
}

func TestDelayRequestInvalidValue(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	req, _ := http.NewRequest("GET", "/delay/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid delay value")
}

func TestGraceDelayRequest(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/graceDelay/:seconds", graceDelayRequest)

	// Reset inShutdown before test
	inShutdown = false

	// Simulate a shutdown signal after 500ms
	go func() {
		time.Sleep(500 * time.Millisecond)
		inShutdown = true
	}()

	req, _ := http.NewRequest("GET", "/graceDelay/2", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":1}`, w.Body.String())
	assert.GreaterOrEqual(t, duration, 500*time.Millisecond)
	assert.Less(t, duration, 2*time.Second)

	// Reset inShutdown for other tests
	inShutdown = false
}

func TestGraceDelayRequestNoShutdown(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/graceDelay/:seconds", graceDelayRequest)

	// Reset inShutdown before test
	inShutdown = false

	req, _ := http.NewRequest("GET", "/graceDelay/1", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":1}`, w.Body.String())
	assert.GreaterOrEqual(t, duration, 1*time.Second)
}

func TestGraceDelayRequestInvalidValue(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/graceDelay/:seconds", graceDelayRequest)

	req, _ := http.NewRequest("GET", "/graceDelay/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid delay value")
}

func TestGraceDelayRequestZeroDelay(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/graceDelay/:seconds", graceDelayRequest)

	req, _ := http.NewRequest("GET", "/graceDelay/0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":0}`, w.Body.String())
}

func TestSetProbeDelay(t *testing.T) {
	tests := []struct {
		name        string
		probeEnv    string
		envValue    string
		expected    time.Duration
		expectError bool
	}{
		{
			name:     "valid delay value",
			probeEnv: "TEST_PROBE_DELAY",
			envValue: "5",
			expected: 5 * time.Second,
		},
		{
			name:     "zero delay value",
			probeEnv: "TEST_PROBE_DELAY",
			envValue: "0",
			expected: 0,
		},
		{
			name:     "invalid delay value",
			probeEnv: "TEST_PROBE_DELAY",
			envValue: "invalid",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setProbeDelay(tt.probeEnv, tt.envValue)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.envValue, os.Getenv(tt.probeEnv))
		})
	}
}

func TestSetMetrics(t *testing.T) {
	promRegistry := prometheus.NewRegistry()
	metrics := setMetrics(promRegistry)

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.requestCounter)
	assert.NotNil(t, metrics.activeRequests)
}

func TestMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	// Make a request to increment the counter so it appears in metrics
	router.GET("/test", func(c *gin.Context) {
		m.requestCounter.WithLabelValues("GET", "/test", "200").Inc()
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{})))

	// First make a test request to generate some metrics
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Now check the metrics endpoint
	req, _ = http.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "http_requests_total")
	assert.Contains(t, w.Body.String(), "active_requests")
}

func TestProbeHandlerFunction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	delay := 100 * time.Millisecond
	handler := probeHandler(delay, "test-message")
	router.GET("/test", handler)

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"message":"test-message"}`, w.Body.String())
	assert.GreaterOrEqual(t, duration, delay)
}

func TestGracefulShutdownFunction(t *testing.T) {
	// This test verifies the gracefulShutdown function returns a proper closure
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	srv := &http.Server{
		Addr:    ":0", // Use any available port
		Handler: router,
	}

	shutdownFunc := gracefulShutdown(srv)
	assert.NotNil(t, shutdownFunc)

	// Test that calling the shutdown function doesn't panic
	assert.NotPanics(t, func() {
		shutdownFunc("test shutdown")
	})
}
func TestPostConfigsWithInvalidDelayValues(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.POST("/config", postConfigs)

	body := `{"startup":"invalid","readiness":"10","liveness":"15"}`
	req, _ := http.NewRequest("POST", "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, `{"startup":"invalid","readiness":"10","liveness":"15"}`, w.Body.String())
	// Invalid values should result in 0 duration
	assert.Equal(t, time.Duration(0), startupProbeDelay)
	assert.Equal(t, 10*time.Second, readinessProbeDelay)
	assert.Equal(t, 15*time.Second, livenessProbeDelay)
}

func TestActiveRequestsMetric(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	// Start a request in a goroutine that will take some time
	done := make(chan bool)
	go func() {
		req, _ := http.NewRequest("GET", "/delay/1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		done <- true
	}()

	// Give the request time to start and increment the active requests counter
	time.Sleep(100 * time.Millisecond)

	// Check that active requests metric is greater than 0
	metricFamilies, err := promRegistry.Gather()
	assert.NoError(t, err)

	var activeRequestsValue float64
	for _, mf := range metricFamilies {
		if *mf.Name == "active_requests" {
			activeRequestsValue = *mf.Metric[0].Gauge.Value
			break
		}
	}

	// The active requests should be 1 while the request is processing
	assert.Equal(t, float64(1), activeRequestsValue)

	// Wait for the request to complete
	<-done

	// After completion, active requests should be back to 0
	metricFamilies, err = promRegistry.Gather()
	assert.NoError(t, err)

	for _, mf := range metricFamilies {
		if *mf.Name == "active_requests" {
			activeRequestsValue = *mf.Metric[0].Gauge.Value
			break
		}
	}
	assert.Equal(t, float64(0), activeRequestsValue)
}

func TestRequestCounterMetric(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	// Make a request
	req, _ := http.NewRequest("GET", "/delay/0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Check that request counter metric was incremented
	metricFamilies, err := promRegistry.Gather()
	assert.NoError(t, err)

	var requestCounterValue float64
	for _, mf := range metricFamilies {
		if *mf.Name == "http_requests_total" {
			for _, metric := range mf.Metric {
				// Find the metric with the right labels
				labels := make(map[string]string)
				for _, label := range metric.Label {
					labels[*label.Name] = *label.Value
				}
				if labels["method"] == "GET" && labels["endpoint"] == "/delay/:seconds" && labels["statusCode"] == "200" {
					requestCounterValue = *metric.Counter.Value
					break
				}
			}
			break
		}
	}
	assert.Equal(t, float64(1), requestCounterValue)
}

func TestEnvironmentVariableInitialization(t *testing.T) {
	// Test that environment variables are properly read on startup
	os.Setenv(startupProbeDelayEnv, "3")
	os.Setenv(readinessProbeDelayEnv, "6")
	os.Setenv(livenessProbeDelayEnv, "9")

	// Reset global variables to test initialization
	startupProbeDelay = 0
	readinessProbeDelay = 0
	livenessProbeDelay = 0

	// Simulate what would happen during initialization
	if val := os.Getenv(startupProbeDelayEnv); val != "" {
		startupProbeDelay = setProbeDelay(startupProbeDelayEnv, val)
	}
	if val := os.Getenv(readinessProbeDelayEnv); val != "" {
		readinessProbeDelay = setProbeDelay(readinessProbeDelayEnv, val)
	}
	if val := os.Getenv(livenessProbeDelayEnv); val != "" {
		livenessProbeDelay = setProbeDelay(livenessProbeDelayEnv, val)
	}

	assert.Equal(t, 3*time.Second, startupProbeDelay)
	assert.Equal(t, 6*time.Second, readinessProbeDelay)
	assert.Equal(t, 9*time.Second, livenessProbeDelay)

	// Clean up
	os.Unsetenv(startupProbeDelayEnv)
	os.Unsetenv(readinessProbeDelayEnv)
	os.Unsetenv(livenessProbeDelayEnv)
}

// Benchmark tests
func BenchmarkStartupProbe(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/startup", probeHandler(0, "startup"))

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/startup", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkDelayRequest(b *testing.B) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/delay/0", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkPostConfigs(b *testing.B) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.POST("/config", postConfigs)

	body := `{"startup":"1","readiness":"2","liveness":"3"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// Test for graceful shutdown error handling
func TestGracefulShutdownWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Create a server that's already closed to trigger an error
	srv := &http.Server{
		Addr:    ":0",
		Handler: router,
	}

	// Close the server before calling shutdown to trigger an error path
	srv.Close()

	shutdownFunc := gracefulShutdown(srv)

	// This should not panic even with a closed server
	assert.NotPanics(t, func() {
		shutdownFunc("test error shutdown")
	})
}

// Test concurrent requests to ensure thread safety
func TestConcurrentRequests(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.TestMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	const numRequests = 10
	var wg sync.WaitGroup
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", "/delay/0", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}

	wg.Wait()
}
