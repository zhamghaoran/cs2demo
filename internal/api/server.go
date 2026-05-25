package api

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cs2demo/platform/internal/domain"
	"github.com/cs2demo/platform/internal/orchestrator"
	"github.com/cs2demo/platform/internal/storage"
)

type Server struct {
	Store          *storage.Store
	Orch           *orchestrator.Orchestrator
	MaxUploadBytes int64
	WebDir         string
}

func (s *Server) Router() *gin.Engine {
	r := gin.Default()
	r.MaxMultipartMemory = 32 << 20

	r.Use(func(c *gin.Context) {
		if c.Request.Method == http.MethodPost && s.MaxUploadBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.MaxUploadBytes)
		}
		c.Next()
	})

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	r.POST("/demos", s.handleUpload)
	r.GET("/demos", s.handleList)
	r.GET("/demos/:id", s.handleGet)
	r.GET("/demos/:id/report", s.handleReport)
	r.GET("/demos/:id/stats", s.handleStats)

	if s.WebDir != "" {
		r.GET("/", func(c *gin.Context) { c.File(filepath.Join(s.WebDir, "index.html")) })
		r.Static("/static", s.WebDir)
	}
	return r
}

func (s *Server) handleUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field: " + err.Error()})
		return
	}
	target := c.PostForm("player")
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing player field (target username)"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open upload: " + err.Error()})
		return
	}
	defer src.Close()

	id := uuid.NewString()
	path, err := s.Store.SaveUpload(id, file.Filename, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save upload: " + err.Error()})
		return
	}

	demo := domain.Demo{
		ID:         id,
		Filename:   file.Filename,
		FilePath:   path,
		TargetUser: target,
		Status:     domain.StatusQueued,
	}
	if err := s.Store.CreateDemo(c.Request.Context(), demo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create demo: " + err.Error()})
		return
	}

	s.Orch.Enqueue(orchestrator.Job{DemoID: id, FilePath: path, TargetUser: target})

	c.JSON(http.StatusAccepted, gin.H{
		"demo_id":  id,
		"status":   domain.StatusQueued,
		"poll_url": "/demos/" + id,
	})
}

func (s *Server) handleGet(c *gin.Context) {
	d, err := s.Store.GetDemo(c.Request.Context(), c.Param("id"))
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "demo not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

func (s *Server) handleList(c *gin.Context) {
	list, err := s.Store.ListDemos(c.Request.Context(), 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"demos": list})
}

func (s *Server) handleReport(c *gin.Context) {
	id := c.Param("id")
	d, err := s.Store.GetDemo(c.Request.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "demo not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if d.Status != domain.StatusDone {
		c.JSON(http.StatusAccepted, gin.H{"status": d.Status, "error": d.Error})
		return
	}
	r, ok, err := s.Store.GetReport(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"status": d.Status, "note": "report still generating"})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (s *Server) handleStats(c *gin.Context) {
	id := c.Param("id")
	st, ok, err := s.Store.GetStats(c.Request.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "demo not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusAccepted, gin.H{"note": "stats not ready"})
		return
	}
	c.JSON(http.StatusOK, st)
}
