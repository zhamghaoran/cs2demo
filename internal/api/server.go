package api

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cs2demo/platform/internal/analyzer"
	"github.com/cs2demo/platform/internal/domain"
	"github.com/cs2demo/platform/internal/orchestrator"
	"github.com/cs2demo/platform/internal/prokb"
	"github.com/cs2demo/platform/internal/storage"
)

type Server struct {
	Store          *storage.Store
	Orch           *orchestrator.Orchestrator
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

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

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
			"note":          "暂无该玩家的已完成比赛数据",
		})
		return
	}
	c.JSON(http.StatusOK, trend)
}

// handlePlayers 列出所有曾出现过的玩家名（target.name 去重），便于前端补全。
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
	// 按场次降序
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Matches > out[j-1].Matches; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	c.JSON(http.StatusOK, gin.H{"players": out})
}

var _ = domain.StatusDone
