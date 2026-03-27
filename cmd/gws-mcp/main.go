package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/orieg/gws-connector/internal/server"
)

func main() {
	useDotNames := flag.Bool("use-dot-names", false, "Use dot-separated tool names (gws.mail.search) instead of underscores")
	flag.Parse()

	stateDir := os.Getenv("GWS_STATE_DIR")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine home directory: %v\n", err)
			os.Exit(1)
		}
		stateDir = home + "/.claude/channels/gws"
	}

	clientID := os.Getenv("GWS_GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GWS_GOOGLE_CLIENT_SECRET")

	cfg := server.Config{
		StateDir:     stateDir,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UseDotNames:  *useDotNames,
	}

	srv := server.New(cfg)
	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
