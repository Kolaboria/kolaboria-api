package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/syahrulfahmi/kolaboria-api/config"
)

type commandExecutor func(context.Context, string, []string, []string) error

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage(stderr)
		return nil
	}
	action := args[0]
	if _, err := migrationArguments(action, ""); err != nil {
		return err
	}
	flags := flag.NewFlagSet("db "+action, flag.ContinueOnError)
	flags.SetOutput(stderr)
	defaultEnvFile := os.Getenv("DB_ENV_FILE")
	if defaultEnvFile == "" {
		defaultEnvFile = ".env"
	}
	envFile := flags.String("env-file", defaultEnvFile, "database configuration file (default .env; empty selects OS environment only)")
	timeout := flags.Duration("timeout", 30*time.Second, "maximum migration command duration")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	var cfg *config.Config
	var err error
	if *envFile == "" {
		cfg, err = config.LoadFromEnvironment()
	} else {
		cfg, err = config.LoadFromFile(*envFile)
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	return runMigrationCommand(ctx, cfg.Database, action, os.Environ(), executeMigration)
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: go run ./cmd/db <migrate|version|down> [--env-file PATH] [--timeout DURATION]")
	fmt.Fprintln(writer, "Actions: migrate (apply pending migrations), version, down (revert one migration).")
}

func migrationURL(cfg config.DatabaseConfig) (string, error) {
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("DB_PORT must be a valid TCP port")
	}
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.Name) == "" || strings.TrimSpace(cfg.User) == "" {
		return "", fmt.Errorf("DB_HOST, DB_NAME, and DB_USER must be configured")
	}
	query := url.Values{}
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	query.Set("sslmode", sslMode)
	connectionURL := url.URL{
		Scheme:   "postgres",
		User:     url.User(cfg.User),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(port)),
		Path:     "/" + cfg.Name,
		RawQuery: query.Encode(),
	}
	return connectionURL.String(), nil
}

func migrationArguments(action, connectionURL string) ([]string, error) {
	args := []string{"-path", "./migrations", "-database", connectionURL}
	switch action {
	case "migrate":
		return append(args, "up"), nil
	case "version":
		return append(args, "version"), nil
	case "down":
		return append(args, "down", "1"), nil
	default:
		return nil, fmt.Errorf("unsupported action %q (choose migrate, version, or down)", action)
	}
}

func runMigrationCommand(ctx context.Context, cfg config.DatabaseConfig, action string, parentEnv []string, execute commandExecutor) error {
	connectionURL, err := migrationURL(cfg)
	if err != nil {
		return err
	}
	args, err := migrationArguments(action, connectionURL)
	if err != nil {
		return err
	}
	if err := execute(ctx, "migrate", args, migrationEnvironment(parentEnv, cfg.Password)); err != nil {
		return fmt.Errorf("migration command %q failed: %w", action, err)
	}
	return nil
}

func migrationEnvironment(parentEnv []string, password string) []string {
	environment := make([]string, 0, len(parentEnv)+1)
	for _, entry := range parentEnv {
		if !strings.HasPrefix(strings.ToUpper(entry), "PGPASSWORD=") {
			environment = append(environment, entry)
		}
	}
	return append(environment, "PGPASSWORD="+password)
}

func executeMigration(ctx context.Context, executable string, args, environment []string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = environment
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
