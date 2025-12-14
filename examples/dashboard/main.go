package main

import (
	"context"
	"log"

	"github.com/dkoosis/fo/dashboard"
)

func main() {
	dash := dashboard.New("Example Suite")
	dash.AddTask("Docs", "list", "bash", "-c", "ls docs")
	dash.AddTask("Echo", "say", "bash", "-c", "echo hello from dashboard")

	if _, err := dash.Run(context.Background()); err != nil {
		log.Fatalf("suite failed: %v", err)
	}
}
