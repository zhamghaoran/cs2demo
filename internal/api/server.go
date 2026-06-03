package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cs2demo/platform/internal/analyzer"
	"github.com/cs2demo/platform/internal/domain"
	"github.com/cs2demo/platform/internal/orchestrator"
	"github.com/cs2demo/platform/internal/parser"
	"github.com/cs2demo/platform/internal/prokb"
	"github.com/cs2demo/platform/internal/storage"
)

type Server struct {
	Store          *storage.Store
	Orch           *orchestrator.Orchestrator
	Parser         *parser.Parser
	KB             prokb.KB
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

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	r.POST("/demos/inspect", s.handleInspectUpload)
	r.POST("/demos", s.handleUpload)
	r.GET("/demos", s.handleList)
	r.GET("/demos/:id", s.handleGet)
	r.GET("/demos/:id/report", s.handleReport)
	r.GET("/demos/:id/stats", s.handleStats)
	r.GET("/trends", s.handleTrends)
	r.GET("/players", s.handlePlayers)

	if s.WebDir != "" {
		r.GET("/", func(c *gin.Context) { c.File(filepath.Join(s.WebDir, "index.html")) })
		r.Static("/static", s.WebDir)
	}
	return r
}

func (s *Server) handleUpload(c *gin.Context) {
	target := strings.TrimSpace(c.PostForm("player"))
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing player field (target username)"})
		return
	}

	id := uuid.NewString()
	filename := ""
	path := ""
	uploadID := strings.TrimSpace(c.PostForm("upload_id"))
	if uploadID != "" {
		if _, err := uuid.Parse(uploadID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid upload_id"})
			return
		}
		var err error
		path, err = s.Store.UploadPath(uploadID)
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "upload_id not found; choose the demo file again"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "load upload: " + err.Error()})
			return
		}
		filename = c.PostForm("filename")
		if filename == "" {
			filename = uploadID + ".dem"
		}
	} else {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field or upload_id: " + err.Error()})
			return
		}
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "open upload: " + err.Error()})
			return
		}
		defer src.Close()
		path, err = s.Store.SaveUpload(id, file.Filename, src)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save upload: " + err.Error()})
			return
		}
		filename = file.Filename
	}

	demo := domain.Demo{
		ID:         id,
		Filename:   filename,
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

func (s *Server) handleInspectUpload(c *gin.Context) {
	if s.Parser == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "parser not configured"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field: " + err.Error()})
		return
	}
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open upload: " + err.Error()})
		return
	}
	defer src.Close()

	uploadID := uuid.NewString()
	path, err := s.Store.SaveUpload(uploadID, file.Filename, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save upload: " + err.Error()})
		return
	}
	players, err := s.Parser.ListPlayers(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "inspect demo: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"upload_id": uploadID,
		"filename":  file.Filename,
		"players":   players,
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

func (s *Server) handleTrends(c *gin.Context) {
	player := c.Query("player")
	if player == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing player query param"})
		return
	}
	rows, err := s.Store.ListAllDoneStats(c.Request.Context(), 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if s.KB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "knowledge base not configured"})
		return
	}
	trend := analyzer.BuildTrend(player, rows, s.KB)
	if trend.MatchesCount == 0 {
		c.JSON(http.StatusOK, gin.H{
			"player":        player,
			"matches_count": 0,
			"note":          "no completed matches found for this player yet",
		})
		return
	}
	c.JSON(http.StatusOK, trend)
}

func (s *Server) handlePlayers(c *gin.Context) {
	rows, err := s.Store.ListAllDoneStats(c.Request.Context(), 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	seen := map[string]int{}
	for _, r := range rows {
		if r.Stats.Target.Name != "" {
			seen[r.Stats.Target.Name]++
		}
	}
	type entry struct {
		Name    string `json:"name"`
		Matches int    `json:"matches"`
	}
	out := make([]entry, 0, len(seen))
	for name, n := range seen {
		out = append(out, entry{Name: name, Matches: n})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Matches > out[j-1].Matches; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	c.JSON(http.StatusOK, gin.H{"players": out})
}

var _ = domain.StatusDone
