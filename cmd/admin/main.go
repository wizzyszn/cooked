// Command admin performs audited, operational account administration.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/wizzyszn/cooked/internal/config"
	"github.com/wizzyszn/cooked/internal/db"
	"github.com/wizzyszn/cooked/internal/user"
)

func main() {
	bootstrap := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	email := bootstrap.String("email", "", "existing account email")
	reason := bootstrap.String("reason", "initial administrator provisioning", "audit reason")
	if len(os.Args) < 2 || os.Args[1] != "bootstrap" {
		log.Fatal("usage: admin bootstrap --email EMAIL [--reason REASON]")
	}
	if err := bootstrap.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}
	if *email == "" {
		log.Fatal("--email is required")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	database, err := db.Connect(&cfg.Database, cfg.Server.Env)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	sqlDB, err := database.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	if err := user.NewRepository(database).BootstrapAdmin(context.Background(), *email, *reason); err != nil {
		log.Fatalf("bootstrap administrator: %v", err)
	}
	log.Printf("administrator role granted to %s; audit record written", *email)
}
