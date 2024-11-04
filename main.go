package main

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type configs struct {
	Startup   string `json:"startup"`
	Readiness string `json:"readiness"`
	Liveness  string `json:"liveness"`
}

func getProbeDelay(probeEnv string) time.Duration {
	probeDelay, e := os.LookupEnv(probeEnv)
	if !e {
		return 0
	} else {
		probeDelay, _ := strconv.ParseInt(probeDelay, 10, 64)
		return time.Duration(probeDelay)
	}
}

func startupProbe(c *gin.Context) {
	time.Sleep(getProbeDelay("STARTUP_PROBE_DELAY") * time.Second)
	c.JSON(http.StatusOK, gin.H{"message": "startup"})
}

func readinessProbe(c *gin.Context) {
	time.Sleep(getProbeDelay("READINESS_PROBE_DELAY") * time.Second)
	c.JSON(http.StatusOK, gin.H{"message": "readiness"})
}

func livenessProbe(c *gin.Context) {
	time.Sleep(getProbeDelay("LIVENESS_PROBE_DELAY") * time.Second)
	c.JSON(http.StatusOK, gin.H{"message": "liveness"})
}

func postConfigs(c *gin.Context) {
	var newConfigs configs

	if err := c.BindJSON(&newConfigs); err != nil {
		return
	}

	os.Setenv("STARTUP_PROBE_DELAY", newConfigs.Startup)
	os.Setenv("READINESS_PROBE_DELAY", newConfigs.Readiness)
	os.Setenv("LIVENESS_PROBE_DELAY", newConfigs.Liveness)

	c.IndentedJSON(http.StatusCreated, newConfigs)
}

func delayRequest(c *gin.Context) {
  delay, _ := strconv.ParseInt(c.Param("seconds"),10,64)
  time.Sleep(time.Duration(delay) * time.Second)
  c.JSON(http.StatusOK, gin.H{"message": delay})
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Probes
	router.GET("/startup", startupProbe)
	router.GET("/readiness", readinessProbe)
	router.GET("/liveness", livenessProbe)
	// Config
	router.POST("/config", postConfigs)
  // Request delay test
  router.GET("/delay/:seconds", delayRequest)

	router.Run(":8080")
}

