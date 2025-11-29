package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"potter/framework/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Парсим флаги
	dbURL := flag.String("database-url", "", "Database connection string (postgres://, mysql://, sqlite3://)")
	migrationsDir := flag.String("migrations-dir", "./migrations", "Path to migrations directory")
	dialect := flag.String("dialect", "", "Database dialect (postgres, mysql, sqlite3). Auto-detected from URL if not specified")
	verbose := flag.Bool("verbose", false, "Verbose output")
	dryRun := flag.Bool("dry-run", false, "Show what would be done without executing")

	flag.CommandLine.Parse(os.Args[2:])

	if *dbURL == "" && command != "create" && command != "validate" {
		fmt.Fprintf(os.Stderr, "Error: --database-url is required\n")
		os.Exit(1)
	}

	// Определяем диалект из URL если не указан
	if *dialect == "" && *dbURL != "" {
		*dialect = detectDialect(*dbURL)
	}

	// Устанавливаем диалект (SetDialect установит postgres по умолчанию, если не указан)
	if err := migrations.SetDialect(*dialect); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting dialect: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "up":
		steps := int64(0) // 0 означает применить все
		if len(flag.Args()) > 0 {
			if n, err := strconv.ParseInt(flag.Args()[0], 10, 64); err == nil {
				steps = n
			}
		}
		runUp(*dbURL, *migrationsDir, steps, *verbose, *dryRun)
	case "down":
		steps := int64(1)
		if len(flag.Args()) > 0 {
			if n, err := strconv.ParseInt(flag.Args()[0], 10, 64); err == nil {
				steps = n
			}
		}
		runDown(*dbURL, *migrationsDir, steps, *verbose, *dryRun)
	case "status":
		runStatus(*dbURL, *migrationsDir, *verbose)
	case "version":
		runVersion(*dbURL, *migrationsDir)
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
		runForce(*dbURL, flag.Args()[0], *verbose)
	case "validate":
		runValidate(*migrationsDir)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Potter Migration Tool (powered by goose)")
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
	fmt.Println("  --database-url    - Database connection string (required for most commands)")
	fmt.Println("  --migrations-dir   - Path to migrations directory (default: ./migrations)")
	fmt.Println("  --dialect          - Database dialect (postgres, mysql, sqlite3). Auto-detected from URL")
	fmt.Println("  --verbose          - Verbose output")
	fmt.Println("  --dry-run          - Show what would be done without executing")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  potter-migrate up --database-url postgres://user:pass@localhost/dbname")
	fmt.Println("  potter-migrate down 1 --database-url postgres://user:pass@localhost/dbname")
	fmt.Println("  potter-migrate status --database-url postgres://user:pass@localhost/dbname")
	fmt.Println("  potter-migrate create add_user_roles --migrations-dir ./migrations")
}

func detectDialect(dbURL string) string {
	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		return "postgres"
	}
	if strings.HasPrefix(dbURL, "mysql://") {
		return "mysql"
	}
	if strings.HasPrefix(dbURL, "sqlite3://") || strings.HasPrefix(dbURL, "sqlite://") {
		return "sqlite3"
	}
	// По умолчанию PostgreSQL
	return "postgres"
}

func openDB(dbURL string) (*sql.DB, error) {
	// Определяем драйвер из URL
	var driver string
	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		driver = "pgx"
	} else if strings.HasPrefix(dbURL, "mysql://") {
		driver = "mysql"
		// Убираем префикс mysql:// для драйвера
		dbURL = strings.TrimPrefix(dbURL, "mysql://")
	} else if strings.HasPrefix(dbURL, "sqlite3://") {
		driver = "sqlite3"
		dbURL = strings.TrimPrefix(dbURL, "sqlite3://")
	} else if strings.HasPrefix(dbURL, "sqlite://") {
		driver = "sqlite3"
		dbURL = strings.TrimPrefix(dbURL, "sqlite://")
	} else {
		// По умолчанию PostgreSQL
		driver = "pgx"
	}

	db, err := sql.Open(driver, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func runUp(dbURL, migrationsDir string, steps int64, verbose, dryRun bool) {
	db, err := openDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if dryRun {
		fmt.Println("Dry run mode - migrations would be applied:")
		statuses, err := migrations.GetMigrationStatus(db, migrationsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
			os.Exit(1)
		}
		pendingCount := 0
		for _, status := range statuses {
			if status.Status == "pending" {
				if steps == 0 || int64(pendingCount) < steps {
					fmt.Printf("  [PENDING] %d - %s\n", status.Version, status.Name)
					pendingCount++
				}
			}
		}
		if steps > 0 && int64(pendingCount) > steps {
			fmt.Printf("  ... and %d more migration(s) would be skipped\n", int64(pendingCount)-steps)
		}
		return
	}

	if steps > 0 {
		fmt.Printf("Applying %d migration(s)...\n", steps)
		if err := migrations.RunMigrationsLimited(db, migrationsDir, steps); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Applied %d migration(s) successfully\n", steps)
	} else {
		if err := migrations.RunMigrations(db, migrationsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error applying migrations: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations applied successfully")
	}
}

func runDown(dbURL, migrationsDir string, steps int64, verbose, dryRun bool) {
	db, err := openDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if dryRun {
		fmt.Printf("Dry run mode - would rollback %d migration(s)\n", steps)
		return
	}

	if steps == 1 {
		if err := migrations.RollbackMigration(db, migrationsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error rolling back migration: %v\n", err)
			os.Exit(1)
		}
	} else {
		if err := migrations.RollbackMigrations(db, migrationsDir, steps); err != nil {
			fmt.Fprintf(os.Stderr, "Error rolling back migrations: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Rolled back %d migration(s)\n", steps)
}

func runStatus(dbURL, migrationsDir string, verbose bool) {
	db, err := openDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	statuses, err := migrations.GetMigrationStatus(db, migrationsDir)
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
		}

		fmt.Printf("%s %d - %s", statusIcon, status.Version, status.Name)
		if status.AppliedAt != nil {
			fmt.Printf(" (applied at %s)", status.AppliedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Println()
	}
}

func runVersion(dbURL, migrationsDir string) {
	db, err := openDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	version, err := migrations.GetCurrentVersion(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting version: %v\n", err)
		os.Exit(1)
	}

	if version == 0 {
		fmt.Println("No migrations applied")
	} else {
		fmt.Println(version)
	}
}

func runCreate(migrationsDir, name string) {
	if err := migrations.CreateMigration(migrationsDir, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating migration: %v\n", err)
		os.Exit(1)
	}
}

func runForce(dbURL, version string, verbose bool) {
	db, err := openDB(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Конвертируем строковую версию в int64
	versionInt, err := strconv.ParseInt(version, 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid version '%s', must be a number: %v\n", version, err)
		os.Exit(1)
	}

	// Получаем текущую версию для подтверждения
	currentVersion, err := migrations.GetCurrentVersion(db)
	if err != nil {
		// Если таблица не существует, это нормально - мы создадим её
		currentVersion = 0
	}

	// Устанавливаем версию
	if err := migrations.SetVersion(db, versionInt); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting version: %v\n", err)
		os.Exit(1)
	}

	if verbose {
		fmt.Printf("Current version was: %d\n", currentVersion)
	}
	fmt.Printf("Version set to: %d\n", versionInt)
	fmt.Println("WARNING: This command does not execute migration SQL. Use with caution!")
}

func runValidate(migrationsDir string) {
	// Проверяем что директория существует
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Validation failed: migrations directory does not exist: %s\n", migrationsDir)
		os.Exit(1)
	}

	// Проверяем что есть файлы миграций
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}

	migrationFiles := 0
	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(file.Name(), ".sql") || strings.HasSuffix(file.Name(), ".go")) {
			migrationFiles++
		}
	}

	if migrationFiles == 0 {
		fmt.Fprintf(os.Stderr, "Validation failed: no migration files found in %s\n", migrationsDir)
		os.Exit(1)
	}

	fmt.Printf("Validation passed: found %d migration file(s)\n", migrationFiles)
}
