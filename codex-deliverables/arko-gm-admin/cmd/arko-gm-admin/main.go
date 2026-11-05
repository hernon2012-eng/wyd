package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"arko-gm-admin/internal/access"
	"arko-gm-admin/internal/audit"
	"arko-gm-admin/internal/directory"
	"arko-gm-admin/internal/identity"
	"arko-gm-admin/internal/ratelimit"
	"arko-gm-admin/internal/server"
	"arko-gm-admin/internal/webui"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const requiredDatabaseName = "arko_admin"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: arko-gm-admin <serve|bootstrap>")
	}
	if args[0] == "serve" && len(args) != 1 {
		return errors.New("serve accepts no arguments")
	}
	if args[0] == "bootstrap" && (len(args) != 5 || args[1] != "--username" || args[3] != "--role") {
		return errors.New("usage: arko-gm-admin bootstrap --username <name> --role <player|moderator|admin>; read password from stdin")
	}
	if args[0] != "serve" && args[0] != "bootstrap" {
		return errors.New("usage: arko-gm-admin <serve|bootstrap>")
	}
	db, err := openDatabase()
	if err != nil {
		return err
	}
	defer db.Close()

	switch args[0] {
	case "serve":
		return serve(db)
	case "bootstrap":
		return bootstrap(db, args[2], identity.Role(args[4]))
	}
	return errors.New("unreachable command")
}

func openDatabase() (*sql.DB, error) {
	dsn := os.Getenv("ARKO_ADMIN_DATABASE_URL")
	if dsn == "" || os.Getenv("ARKO_ADMIN_DATABASE_NAME") != requiredDatabaseName {
		return nil, errors.New("ARKO_ADMIN_DATABASE_URL and ARKO_ADMIN_DATABASE_NAME=arko_admin are required")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil || config.Database != requiredDatabaseName {
		return nil, errors.New("database URL must target the dedicated arko_admin database")
	}
	if strings.EqualFold(config.Database, "wyd") || strings.EqualFold(config.Database, "wyd_dev") {
		return nil, errors.New("refusing to connect to a WYD database")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("could not initialize PostgreSQL connection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.New("could not connect to the configured ARKO admin database")
	}
	return db, nil
}

func bootstrap(db *sql.DB, username string, role identity.Role) error {
	passwordBytes, err := io.ReadAll(io.LimitReader(os.Stdin, 74))
	if err != nil {
		return errors.New("could not read password from stdin")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(passwordBytes), "\n"), "\r")
	if password == "" {
		return errors.New("password input is empty")
	}
	if len(password) > 72 {
		return errors.New("password exceeds the 72-byte limit")
	}
	store, err := identity.New(db)
	if err != nil {
		return errors.New("identity store is unavailable")
	}
	if _, err := store.CreateCredentials(context.Background(), username, password, role); err != nil {
		return errors.New("could not create administrative credentials; confirm the schema and input")
	}
	fmt.Println("Credentials created.")
	return nil
}

func serve(db *sql.DB) error {
	identityStore, err := identity.New(db)
	if err != nil {
		return errors.New("identity store is unavailable")
	}
	auditStore, err := audit.NewPostgres(db)
	if err != nil {
		return errors.New("audit store is unavailable")
	}
	authorizer, err := configuredAuthorizer()
	if err != nil {
		return err
	}
	limiter, err := ratelimit.New(ratelimit.Config{
		DefaultLimit: ratelimit.Limit{Requests: 120, Window: time.Minute},
		MaxKeys:      10_000,
		IdleTTL:      15 * time.Minute,
	})
	if err != nil {
		return errors.New("rate limiter configuration is invalid")
	}
	api, err := server.New(server.Dependencies{
		Identity:   server.IdentityAdapter{Store: identityStore, SessionTTL: 8 * time.Hour},
		Authorizer: server.AccessAdapter{Authorizer: authorizer},
		Directory:  server.DirectoryAdapter{Directory: directory.NewDemonstrationDirectory()},
		Audit:      server.AuditAdapter{Store: auditStore},
		Commands:   server.NoCommands{},
		Limiter:    limiter,
	})
	if err != nil {
		return errors.New("API dependencies are invalid")
	}
	handler := http.NewServeMux()
	handler.Handle("/api/", api.Handler())
	handler.Handle("/healthz", api.Handler())
	handler.Handle("/", webui.Handler())

	address := os.Getenv("ARKO_ADMIN_HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("HTTP listener must bind to a loopback IP; terminate TLS in a separate trusted proxy if remote access is needed")
	}

	httpServer := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	err = httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return errors.New("HTTP server stopped unexpectedly")
}

func configuredAuthorizer() (*access.Authorizer, error) {
	grants := make(map[access.Role][]access.Capability)
	for env, role := range map[string]access.Role{
		"ARKO_ADMIN_PLAYER_CAPABILITIES":    access.RolePlayer,
		"ARKO_ADMIN_MODERATOR_CAPABILITIES": access.RoleModerator,
		"ARKO_ADMIN_ADMIN_CAPABILITIES":     access.RoleAdmin,
	} {
		values := strings.Split(os.Getenv(env), ",")
		for _, raw := range values {
			value := strings.TrimSpace(raw)
			if value == "" {
				continue
			}
			if value != "directory.read" && value != "audit.read" {
				return nil, fmt.Errorf("%s contains an unsupported capability", env)
			}
			grants[role] = append(grants[role], access.Capability(value))
		}
	}
	return access.NewAuthorizer(grants)
}
