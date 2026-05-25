package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/cs2demo/platform/internal/analyzer"
	"github.com/cs2demo/platform/internal/domain"
	"github.com/cs2demo/platform/internal/prokb"
)

func main() {
	apiBase := flag.String("api", "http://localhost:8089", "server base URL")
	demoID := flag.String("id", "", "demo id; empty = latest")
	out := flag.String("out", "/tmp/cs2demo-test/last_prompt.txt", "output file")
	flag.Parse()

	id := *demoID
	if id == "" {
		resp, err := http.Get(*apiBase + "/demos")
		if err != nil {
			log.Fatalf("list demos: %v", err)
		}
		defer resp.Body.Close()
		var listed struct {
			Demos []struct {
				ID        string `json:"id"`
				Status    string `json:"status"`
				CreatedAt string `json:"created_at"`
			} `json:"demos"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
			log.Fatalf("decode list: %v", err)
		}
		for _, d := range listed.Demos {
			if d.Status == "done" {
				id = d.ID
				break
			}
		}
		if id == "" {
			log.Fatal("no done demo found")
		}
	}

	body := mustGet(*apiBase + "/demos/" + id + "/stats")
	var stats domain.MatchStats
	if err := json.Unmarshal(body, &stats); err != nil {
		log.Fatalf("unmarshal stats: %v", err)
	}

	system, user := analyzer.DumpPrompt(stats, prokb.New())
	dump := fmt.Sprintf("=== demo_id: %s ===\n=== map:     %s ===\n=== rounds:  %d ===\n=== target:  %s [%s] ===\n\n=== SYSTEM PROMPT (len=%d) ===\n%s\n\n=== USER PROMPT (len=%d) ===\n%s\n",
		id, stats.Map, stats.RoundsTotal, stats.Target.Name, stats.Target.Team,
		len(system), system, len(user), user)
	if err := os.WriteFile(*out, []byte(dump), 0644); err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Printf("wrote %d bytes to %s\n", len(dump), *out)
	fmt.Printf("demo=%s map=%s rounds=%d target=%s system_len=%d user_len=%d\n",
		id, stats.Map, stats.RoundsTotal, stats.Target.Name, len(system), len(user))
}

func mustGet(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Fatalf("GET %s: %d %s", url, resp.StatusCode, string(b))
	}
	return b
}
