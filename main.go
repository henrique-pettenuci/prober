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
)

const (
	startupProbeDelayEnv   = "STARTUP_PROBE_DELAY"
	readinessProbeDelayEnv = "READINESS_PROBE_DELAY"
	livenessProbeDelayEnv  = "LIVENESS_PROBE_DELAY"
)

var inShutdown bool = false

type configs struct {
	Startup   string `json:"startup"`
	Readiness string `json:"readiness"`
	Liveness  string `json:"liveness"`
}

func getProbeDelay(probeEnv string) time.Duration {
	probeDelay, exists := os.LookupEnv(probeEnv)
	if !exists {
		return 0
	}
	delay, err := strconv.ParseInt(probeDelay, 10, 64)
	if err != nil {
		log.Printf("Invalid delay value for %s: %v", probeEnv, err)
		return 0
	}
	return time.Duration(delay) * time.Second
}

func probeHandler(probeEnv string, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		time.Sleep(getProbeDelay(probeEnv))
		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}

func postConfigs(c *gin.Context) {
	var newConfigs configs

	if err := c.BindJSON(&newConfigs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	os.Setenv(startupProbeDelayEnv, newConfigs.Startup)
	os.Setenv(readinessProbeDelayEnv, newConfigs.Readiness)
	os.Setenv(livenessProbeDelayEnv, newConfigs.Liveness)

	c.JSON(http.StatusCreated, newConfigs)
}

func delayRequest(c *gin.Context) {
	delay, err := strconv.ParseInt(c.Param("seconds"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay value"})
		return
	}
	time.Sleep(time.Duration(delay) * time.Second)
	c.JSON(http.StatusOK, gin.H{"message": delay})
}

func graceDelayRequest(c *gin.Context) {
	delay, err := strconv.ParseInt(c.Param("seconds"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay value"})
		return
	}
	var delayInc int64 = 0

	if delay > 0 {
		for delayInc < delay {
			time.Sleep(1 * time.Second)

			if inShutdown {
				break
			}

			delayInc++
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": delayInc})
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	// Probes
	router.GET("/startup", probeHandler(startupProbeDelayEnv, "startup"))
	router.GET("/readiness", probeHandler(readinessProbeDelayEnv, "readiness"))
	router.GET("/liveness", probeHandler(livenessProbeDelayEnv, "liveness"))
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
			log.Println("Erros to Gracefully shutdown server: ", err)
		}
	}
}
