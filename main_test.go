package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestStartupProbe(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/startup", probeHandler(startupProbeDelayEnv, "startup"))

	req, _ := http.NewRequest("GET", "/startup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	expected := `{"message":"startup"}`
	if w.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, w.Body.String())
	}
}

func TestReadinessProbe(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/readiness", probeHandler(readinessProbeDelayEnv, "readiness"))

	req, _ := http.NewRequest("GET", "/readiness", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	expected := `{"message":"readiness"}`
	if w.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, w.Body.String())
	}
}

func TestLivenessProbe(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/liveness", probeHandler(livenessProbeDelayEnv, "liveness"))

	req, _ := http.NewRequest("GET", "/liveness", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	expected := `{"message":"liveness"}`
	if w.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, w.Body.String())
	}
}

func TestPostConfigs(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.POST("/config", postConfigs)

	body := `{"startup":"5","readiness":"10","liveness":"15"}`
	req, _ := http.NewRequest("POST", "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	expected := `{"startup":"5","readiness":"10","liveness":"15"}`
	if w.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, w.Body.String())
	}

	if os.Getenv("STARTUP_PROBE_DELAY") != "5" {
		t.Errorf("expected STARTUP_PROBE_DELAY to be %s, got %s", "5", os.Getenv("STARTUP_PROBE_DELAY"))
	}
	if os.Getenv("READINESS_PROBE_DELAY") != "10" {
		t.Errorf("expected READINESS_PROBE_DELAY to be %s, got %s", "10", os.Getenv("READINESS_PROBE_DELAY"))
	}
	if os.Getenv("LIVENESS_PROBE_DELAY") != "15" {
		t.Errorf("expected LIVENESS_PROBE_DELAY to be %s, got %s", "15", os.Getenv("LIVENESS_PROBE_DELAY"))
	}
}

func TestDelayRequest(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/delay/:seconds", delayRequest)

	req, _ := http.NewRequest("GET", "/delay/2", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	expected := `{"message":2}`
	if w.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, w.Body.String())
	}

	if duration < 2*time.Second {
		t.Errorf("expected delay of at least 2 seconds, got %v", duration)
	}
}

func TestGraceDelayRequest(t *testing.T) {
	// Initialize prometheus registry and metrics
	promRegistry := prometheus.NewRegistry()
	m = setMetrics(promRegistry)

	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.GET("/graceDelay/:seconds", graceDelayRequest)

	// Simulate a shutdown signal after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		inShutdown = true
	}()

	req, _ := http.NewRequest("GET", "/graceDelay/2", nil)
	w := httptest.NewRecorder()
	start := time.Now()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	expected := `{"message":1}`
	if w.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, w.Body.String())
	}

	if duration < 1*time.Second || duration > 2*time.Second {
		t.Errorf("expected delay of around 1 second, got %v", duration)
	}

	// Reset inShutdown for other tests
	inShutdown = false
}

func Test_main(t *testing.T) {
	tests := []struct {
		name string // description of this test case
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			main()
		})
	}
}
