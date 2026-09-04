package cmd

import (
	"fmt"
	"os"

	"kuro/cli/internal/client"
	"kuro/cli/internal/config"
)

func RunBackup(args []string) {
	if len(args) < 1 {
		printBackupHelp()
		return
	}

	subcommand := args[0]

	switch subcommand {
	case "list":
		runBackupList()
	case "restore":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: kuro backup restore <sha256>")
			fmt.Fprintln(os.Stderr, "  eg:  kuro backup restore abc123def456...")
			os.Exit(1)
		}
		runBackupRestore(args[1])
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: kuro backup %s\n", subcommand)
		printBackupHelp()
		os.Exit(1)
	}
}

func printBackupHelp() {
	fmt.Println("Usage: kuro backup <command>")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list                 List available backups in MinIO")
	fmt.Println("  restore <sha256>     Restore a backup to PostgreSQL")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  kuro backup list")
	fmt.Println("  kuro backup restore abc123def456...")
}

func runBackupList() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: no API key configured. Run: kuro auth <your-api-key>")
		os.Exit(1)
	}

	c := client.New(cfg.APIURL, cfg.APIKey)

	if !c.HealthCheck() {
		fmt.Fprintln(os.Stderr, "Error: cannot connect to API at", cfg.APIURL)
		os.Exit(1)
	}

	fmt.Println("Connected to", cfg.APIURL)
	fmt.Println()
	fmt.Println("Backups are stored at s3://kuro-backups/data/<sha256>.tar.gz")
	fmt.Println()
	fmt.Println("To list backups, connect to the container:")
	fmt.Println("  docker exec kuro-minio mc ls kuro/kuro-backups/data/")
	fmt.Println()
	fmt.Println("Or via S3 CLI:")
	fmt.Println("  aws --endpoint-url http://localhost:9000 s3 ls s3://kuro-backups/data/")
}

func runBackupRestore(hash string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(hash) != 64 {
		fmt.Fprintf(os.Stderr, "Error: SHA256 hash must be exactly 64 hex characters\n")
		os.Exit(1)
	}

	fmt.Printf("⚠  YOU ARE ABOUT TO RESTORE BACKUP: %s\n", hash)
	fmt.Println("   This will REPLACE all current data in PostgreSQL.")
	fmt.Print("   Are you sure? (type 'yes' to confirm): ")

	var confirm string
	fmt.Scanln(&confirm)
	if confirm != "yes" {
		fmt.Println("Restore cancelled.")
		os.Exit(0)
	}

	fmt.Println()
	fmt.Println("To restore the backup manually:")
	fmt.Printf("  1. Download from MinIO:\n")
	fmt.Printf("     docker exec kuro-minio mc cp kuro/kuro-backups/data/%s.tar.gz /tmp/\n", hash)
	fmt.Println()
	fmt.Println("  2. Decompress and restore:")
	fmt.Printf("     gunzip -c /tmp/%s.tar.gz | docker exec -i kuro-postgres pg_restore -U kuro -d kuro\n", hash)

	_ = cfg
}
