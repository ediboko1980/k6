package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/loadimpact/speedboat/lib"
	"gopkg.in/tylerb/graceful.v1"
	"net/http"
	"strconv"
	"time"
)

type APIServer struct {
	Engine *lib.Engine
	Cancel context.CancelFunc

	Info lib.Info
}

// Run runs the API server.
// I'm not sure how idiomatic this is, probably not particularly...
func (s *APIServer) Run(ctx context.Context, addr string) {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(s.logRequestsMiddleware)
	router.Use(s.jsonErrorsMiddleware)

	v1 := router.Group("/v1")
	{
		v1.GET("/info", func(c *gin.Context) {
			c.JSON(200, s.Info)
		})
		v1.GET("/status", func(c *gin.Context) {
			c.JSON(200, s.Engine.Status)
		})
		v1.POST("/abort", func(c *gin.Context) {
			s.Cancel()
			c.JSON(202, gin.H{"success": true})
		})
		v1.POST("/scale", func(c *gin.Context) {
			vus, err := strconv.ParseInt(c.Query("vus"), 10, 64)
			if err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}

			if err := s.Engine.Scale(vus); err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			c.JSON(202, gin.H{"success": true})
		})
	}
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "Not Found"})
	})

	srv := graceful.Server{NoSignalHandling: true, Server: &http.Server{Addr: addr, Handler: router}}
	go srv.ListenAndServe()

	<-ctx.Done()
	srv.Stop(10 * time.Second)
	<-srv.StopChan()
}

func (s *APIServer) logRequestsMiddleware(c *gin.Context) {
	path := c.Request.URL.Path
	c.Next()
	log.WithField("status", c.Writer.Status()).Debugf("%s %s", c.Request.Method, path)
}

func (s *APIServer) jsonErrorsMiddleware(c *gin.Context) {
	c.Next()
	if c.Writer.Size() == 0 && len(c.Errors) > 0 {
		c.JSON(c.Writer.Status(), c.Errors)
	}
}