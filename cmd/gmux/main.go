package main

import (
	"fmt"
	"log"
	"os"

	"gmux/internal/config"
	"gmux/internal/tmux"
)

func usage() {
	fmt.Println("gmux - easy tmux frontend")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  gmux session                  Create next session 1..9 and attach")
	fmt.Println("  gmux <1..9>                   Attach session number")
	fmt.Println("  gmux start [name]             Boot/attach named managed session")
	fmt.Println("  gmux list                     List gmux sessions")
	fmt.Println("  gmux kill <1..9|all>          Kill one session or all")
	fmt.Println("  gmux rename <n> <name>        Rename session n label")
	fmt.Println("  gmux install                  Install tmux dependency")
	fmt.Println("  gmux doctor                   Show tmux binary/config paths")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if len(os.Args) < 2 {
		if err := tmux.Start(cfg, cfg.DefaultSession); err != nil {
			log.Fatal(err)
		}
		return
	}

	arg := os.Args[1]
	if n, ok := tmux.ParseNumericSession(arg); ok {
		if err := tmux.Start(cfg, n); err != nil {
			log.Fatal(err)
		}
		return
	}

	switch arg {
	case "session":
		next, err := tmux.NextNumericSession(cfg, 9)
		if err != nil {
			log.Fatal(err)
		}
		if err := tmux.Start(cfg, next); err != nil {
			log.Fatal(err)
		}
	case "start":
		target := cfg.DefaultSession
		if len(os.Args) > 2 {
			target = os.Args[2]
		}
		if err := tmux.Start(cfg, target); err != nil {
			log.Fatal(err)
		}
	case "list":
		if err := tmux.List(cfg); err != nil {
			log.Fatal(err)
		}
	case "kill":
		if len(os.Args) < 3 {
			log.Fatal("missing session name")
		}
		if err := tmux.Kill(cfg, os.Args[2]); err != nil {
			log.Fatal(err)
		}
	case "rename":
		if len(os.Args) < 4 {
			log.Fatal("usage: gmux rename <n> <name>")
		}
		if err := tmux.Rename(cfg, os.Args[2], os.Args[3:]); err != nil {
			log.Fatal(err)
		}
	case "doctor":
		if err := tmux.Doctor(cfg); err != nil {
			log.Fatal(err)
		}
	case "install":
		if err := tmux.Install(cfg); err != nil {
			log.Fatal(err)
		}
	case "statusline":
		current := ""
		if len(os.Args) > 2 {
			current = os.Args[2]
		}
		if err := tmux.StatusLine(cfg, current); err != nil {
			log.Fatal(err)
		}
	default:
		usage()
		os.Exit(1)
	}
}
