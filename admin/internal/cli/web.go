package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/rustybrownlee/ot-simulator/admin/internal/web"
)

// RunWeb starts the admin web dashboard server.
// Binds to --addr (default :8095) and serves the full admin dashboard.
func RunWeb(g Globals, args []string) {
	fs := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fs.String("addr", envOr("OTS_ADMIN_ADDR", ":8095"), "HTTP listen address for the admin dashboard")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	srv := web.New(*addr, toWebGlobals(g))
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "admin web: server error: %v\n", err)
		os.Exit(1)
	}
}

// toWebGlobals converts a cli.Globals to a web.Globals. The two types are
// structurally identical; this conversion exists to break the import cycle
// between the cli and web packages.
func toWebGlobals(g Globals) web.Globals {
	return web.Globals{
		DesignDir:  g.DesignDir,
		ConfigPath: g.ConfigPath,
		DBPath:     g.DBPath,
		APIAddr:    g.APIAddr,
		PlantPorts: g.PlantPorts,
	}
}

// envOr returns the value of environment variable name, or fallback if unset.
// Duplicated here to keep the cli package self-contained.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
