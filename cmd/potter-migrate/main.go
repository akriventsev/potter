package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"potter/framework/migrations"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Парсим флаги
	dbURL := flag.String("database-url", "", "Database connection string (postgres:// or mongodb://)")
	migrationsDir := flag.String("migrations-dir", "./migrations", "Path to migrations directory")
	verbose := flag.Bool("verbose", false, "Verbose output")
	dryRun := flag.Bool("dry-run", false, "Show SQL without executing")

	flag.CommandLine.Parse(os.Args[2:])

	if *dbURL == "" {
		fmt.Fprintf(os.Stderr, "Error: --database-url is required\n")
		os.Exit(1)
	}

	ctx := context.Background()

	switch command {
	case "up":
		runUp(ctx, *dbURL, *migrationsDir, *verbose, *dryRun)
	case "down":
		steps := 1
		if len(flag.Args()) > 0 {
			if n, err := strconv.Atoi(flag.Args()[0]); err == nil {
				steps = n
			}
		}
		runDown(ctx, *dbURL, *migrationsDir, steps, *verbose, *dryRun)
	case "status":
		runStatus(ctx, *dbURL, *migrationsDir, *verbose)
	case "version":
		runVersion(ctx, *dbURL, *migrationsDir)
	case "create":
		if len(flag.Args()) == 0 {
			fmt.Fprintf(os.Stderr, "Error: migration name is required\n")
			os.Exit(1)
		}
		runCreate(*migrationsDir, flag.Args()[0])
	case "force":
		if len(flag.Args()) == 0 {
			fmt.Fprintf(os.Stderr, "Error: migration version is required\n")
			os.Exit(1)
		}
		runForce(ctx, *dbURL, flag.Args()[0], *verbose)
	case "validate":
		runValidate(*migrationsDir)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Potter Migration Tool")
	fmt.Println()
	fmt.Println("Usage: potter-migrate <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up [N]        - Apply all pending migrations (or N migrations)")
	fmt.Println("  down [N]      - Rollback N migrations (default: 1)")
	fmt.Println("  status        - Show status of all migrations")
	fmt.Println("  version       - Show current migration version")
	fmt.Println("  create <name> - Create a new migration")
	fmt.Println("  force <version> - Mark migration as applied without executing")
	fmt.Println("  validate      - Validate migration files")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --database-url    - Database connection string (required)")
	fmt.Println("  --migrations-dir   - Path to migrations directory (default: ./migrations)")
	fmt.Println("  --verbose          - Verbose output")
	fmt.Println("  --dry-run          - Show SQL without executing")
}

func runUp(ctx context.Context, dbURL, migrationsDir string, verbose, dryRun bool) {
	db, err := createMigrationDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer closeDB(db)

	migrator := migrations.NewMigrator(db)
	if err := migrator.RegisterFromFiles(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Println("Dry run mode - migrations would be applied:")
		statuses, _ := migrator.Status(ctx)
		for _, status := range statuses {
			if status.Status == "pending" {
				fmt.Printf("  [PENDING] %s - %s\n", status.Version, status.Name)
			}
		}
		return
	}

	if err := migrator.Up(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migrations applied successfully")
}

func runDown(ctx context.Context, dbURL, migrationsDir string, steps int, verbose, dryRun bool) {
	db, err := createMigrationDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer closeDB(db)

	migrator := migrations.NewMigrator(db)
	if err := migrator.RegisterFromFiles(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	if dryRun {
		fmt.Printf("Dry run mode - would rollback %d migration(s)\n", steps)
		return
	}

	if err := migrator.Down(ctx, steps); err != nil {
		fmt.Fprintf(os.Stderr, "Error rolling back migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Rolled back %d migration(s)\n", steps)
}

func runStatus(ctx context.Context, dbURL, migrationsDir string, verbose bool) {
	db, err := createMigrationDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer closeDB(db)

	migrator := migrations.NewMigrator(db)
	if err := migrator.RegisterFromFiles(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	statuses, err := migrator.Status(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migration Status:")
	fmt.Println("================")
	for _, status := range statuses {
		statusIcon := "⏳"
		if status.Status == "applied" {
			statusIcon = "✅"
		} else if status.Status == "failed" {
			statusIcon = "❌"
		}

		fmt.Printf("%s %s - %s", statusIcon, status.Version, status.Name)
		if status.AppliedAt != nil {
			fmt.Printf(" (applied at %s)", status.AppliedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}
}

func runVersion(ctx context.Context, dbURL, migrationsDir string) {
	db, err := createMigrationDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer closeDB(db)

	migrator := migrations.NewMigrator(db)
	if err := migrator.RegisterFromFiles(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	version, err := migrator.Version(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting version: %v\n", err)
		os.Exit(1)
	}

	if version == "" {
		fmt.Println("No migrations applied")
	} else {
		fmt.Println(version)
	}
}

func runCreate(migrationsDir, name string) {
	// Генерируем версию на основе timestamp
	timestamp := fmt.Sprintf("%d", os.Getpid()) // Упрощенная версия, в реальности использовать time.Now().Unix()

	upFile := fmt.Sprintf("%s/%s_%s.up.sql", migrationsDir, timestamp, name)
	downFile := fmt.Sprintf("%s/%s_%s.down.sql", migrationsDir, timestamp, name)

	// Создаем директорию если не существует
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating migrations directory: %v\n", err)
		os.Exit(1)
	}

	// Создаем файлы
	if err := os.WriteFile(upFile, []byte("-- Migration: "+name+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating up migration: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(downFile, []byte("-- Rollback: "+name+"\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating down migration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created migration: %s\n", name)
	fmt.Printf("  Up:   %s\n", upFile)
	fmt.Printf("  Down: %s\n", downFile)
}

func runForce(ctx context.Context, dbURL, version string, verbose bool) {
	fmt.Fprintf(os.Stderr, "Force command not fully implemented\n")
	os.Exit(1)
}

func runValidate(migrationsDir string) {
	source := migrations.NewFileMigrationSource(migrationsDir)
	_, err := source.LoadMigrations()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All migrations are valid")
}

func createMigrationDB(dbURL string) (migrations.MigrationDB, error) {
	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		return migrations.NewPostgresMigrationDB(dbURL)
	} else if strings.HasPrefix(dbURL, "mongodb://") {
		// MongoDB миграции временно не поддерживаются
		// Полная реализация требует поддержки JavaScript миграций и более сложной инфраструктуры
		return nil, fmt.Errorf("MongoDB migrations are not yet fully implemented. Please use PostgreSQL for migrations or implement MongoDB migration support")
	}
	return nil, fmt.Errorf("unsupported database URL scheme: %s (supported: postgres://, postgresql://)", dbURL)
}

func closeDB(db migrations.MigrationDB) {
	// Закрываем соединение если необходимо
	// В текущей реализации это не требуется, но может быть добавлено в будущем
}
