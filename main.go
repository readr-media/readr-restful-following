package main

import (
	"flag"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/readr-media/readr-restful-following/config"
	"github.com/readr-media/readr-restful-following/internal/router"
	"github.com/readr-media/readr-restful-following/internal/rrsql"
	followingRouter "github.com/readr-media/readr-restful-following/pkg/following/router"
)

func setRoutes(rt *gin.Engine) {
	for _, h := range []router.RouterHandler{
		&followingRouter.Router,
		&followingRouter.PubsubRouter,
	} {
		h.SetRoutes(rt)
	}
}

func main() {

	var configFile string
	flag.StringVar(&configFile, "path", "", "Configuration file path.")
	flag.Parse()

	_, err := config.LoadConfig(configFile)
	if err != nil {
		panic(fmt.Errorf("Invalid application configuration: %s", err))
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// Set customed logger, specify routes skiped from logged
	r.Use(gin.LoggerWithWriter(gin.DefaultWriter, "/metrics"))

	// Include multiStatements=True for migration usage
	dbURI := fmt.Sprintf("%s:%s@tcp(%s)/memberdb?parseTime=true&charset=utf8mb4&multiStatements=true", config.Config.SQL.User, config.Config.SQL.Password, fmt.Sprintf("%s:%v", config.Config.SQL.Host, config.Config.SQL.Port))
	// Init Mysql connections
	rrsql.Connect(dbURI)

	setRoutes(r)

	// Implemented Prometheus metrics
	r.GET("/metrics", func() gin.HandlerFunc {
		return func(c *gin.Context) {
			promhttp.Handler().ServeHTTP(c.Writer, c.Request)
		}
	}())

	r.Run()

}
