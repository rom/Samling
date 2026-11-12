// Command samling is the Samling server and its administration CLI.
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // embedded time zone database (iCalendar import inside the sandbox)

	"github.com/rom/samling/internal/app"
	"github.com/rom/samling/internal/blob"
	"github.com/rom/samling/internal/config"
	"github.com/rom/samling/internal/crypto"
	"github.com/rom/samling/internal/db"
	"github.com/rom/samling/internal/sandbox"
	"github.com/rom/samling/web"
)

var version = "0.1.0-mvp"

const usage = `Samling - club management server

Usage: samling <command> [flags]

Commands:
  serve            run the server
  admin create     create an account (default role: admin)
  admin passwd     set a new password for an account (--reset-mfa also removes two-step verification)
  backup           write an encrypted backup (database + files)
  restore          restore a backup into an empty installation
  decrypt          decrypt an encrypted export (.enc)
  genkey           print a new master key
  check-config     validate the configuration and exit
  version          print the version

Common flags:
  --config FILE    YAML configuration file (default: $SAMLING_CONFIG or ./samling.yaml if present)

Passphrases for backup/restore/decrypt are read from $SAMLING_BACKUP_PASSPHRASE,
--passphrase-file, or prompted for on standard input.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	if cmd == "admin" && len(args) > 0 {
		cmd, args = "admin "+args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "admin create":
		err = adminCreate(args)
	case "admin passwd":
		err = adminPasswd(args)
	case "backup":
		err = backup(args)
	case "restore":
		err = restore(args)
	case "decrypt":
		err = decrypt(args)
	case "genkey":
		fmt.Println(base64.StdEncoding.EncodeToString(crypto.GenerateKey()))
	case "check-config":
		var cfg *config.Config
		if cfg, err = loadConfig(flag.NewFlagSet(cmd, flag.ExitOnError), args); err == nil {
			fmt.Printf("configuration OK (listen %s, database %s, storage %s)\n", cfg.Listen(), cfg.Database.Driver, cfg.Storage.Backend)
		}
	case "version":
		fmt.Println("samling", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// loadConfig parses flags (fs may have extra flags registered by the caller)
// and loads the configuration: flags > environment > YAML file > defaults.
func loadConfig(fs *flag.FlagSet, args []string) (*config.Config, error) {
	path := fs.String("config", "", "YAML configuration file")
	port := fs.Int("port", 0, "port to listen on (overrides server.port)")
	host := fs.String("host", "", "address to listen on (overrides server.host)")
	dataDir := fs.String("data-dir", "", "data directory (overrides data_dir)")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if *path == "" {
		*path = os.Getenv("SAMLING_CONFIG")
	}
	if *path == "" {
		if _, err := os.Stat("samling.yaml"); err == nil {
			*path = "samling.yaml"
		}
	}
	cfg, err := config.Load(*path)
	if err != nil {
		return nil, err
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *host != "" {
		cfg.Server.Host = *host
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	return cfg, cfg.Finish()
}

type runtime struct {
	cfg   *config.Config
	keys  *crypto.Keys
	db    *db.DB
	blobs *blob.Store
}

func open(cfg *config.Config) (*runtime, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return nil, err
	}
	d, err := db.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		return nil, err
	}
	// A master key is only generated for a brand new installation. If data
	// exists but the key is missing, generating a new key would silently make
	// the encrypted fields and files unreadable - so refuse instead.
	keys, err := cfg.LoadMasterKey(false)
	if errors.Is(err, os.ErrNotExist) {
		var users int
		if err := d.Row(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
			return nil, err
		}
		if users > 0 {
			return nil, fmt.Errorf("the master key (%s) is missing but the database contains data; restore the key instead of generating a new one", cfg.Security.MasterKeyFile)
		}
		keys, err = cfg.LoadMasterKey(true)
	}
	if err != nil {
		return nil, err
	}
	backend, err := cfg.OpenBlob()
	if err != nil {
		return nil, err
	}
	return &runtime{cfg, keys, d, blob.NewStore(backend, keys.Blob)}, nil
}

func (rt *runtime) app(log *slog.Logger) (*app.App, error) {
	return app.New(rt.cfg, rt.db, rt.blobs, rt.keys, web.Static(), log)
}

// certReloader re-reads the certificate when the file changes, so renewed
// (Let's Encrypt) certificates are picked up without a restart.
type certReloader struct {
	mu        sync.Mutex
	cert, key string
	loaded    *tls.Certificate
	mod       time.Time
	checked   time.Time
}

func (c *certReloader) get(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded != nil && time.Since(c.checked) < time.Minute {
		return c.loaded, nil
	}
	c.checked = time.Now()
	fi, err := os.Stat(c.cert)
	if err == nil && (c.loaded == nil || fi.ModTime().After(c.mod)) {
		pair, err := tls.LoadX509KeyPair(c.cert, c.key)
		if err == nil {
			c.loaded, c.mod = &pair, fi.ModTime()
		} else if c.loaded == nil {
			return nil, err
		}
	}
	if c.loaded == nil {
		return nil, fmt.Errorf("no certificate: %v", err)
	}
	return c.loaded, nil
}

func serve(args []string) error {
	cfg, err := loadConfig(flag.NewFlagSet("serve", flag.ExitOnError), args)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if !cfg.TLSEnabled() && !cfg.Server.InsecureHTTP {
		return errors.New("no TLS certificate configured (server.tls.cert/key). Run scripts/gen-selfsigned-cert.sh or " +
			"scripts/letsencrypt.sh, or set server.insecure_http: true when a reverse proxy terminates TLS")
	}
	rt, err := open(cfg)
	if err != nil {
		return err
	}
	defer rt.db.Close()
	a, err := rt.app(log)
	if err != nil {
		return err
	}
	var users int
	if err := rt.db.Row(context.Background(), `SELECT COUNT(*) FROM users`).Scan(&users); err == nil && users == 0 {
		log.Warn("no accounts exist yet: create the first administrator with 'samling admin create --username <name>'")
	}

	srv := &http.Server{
		Handler:           a,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Minute, // bounds slow uploads; responses (event streams) are not limited
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	var reloader *certReloader
	if cfg.TLSEnabled() {
		reloader = &certReloader{cert: cfg.Server.TLS.Cert, key: cfg.Server.TLS.Key}
		if _, err := reloader.get(nil); err != nil {
			return fmt.Errorf("TLS certificate: %w", err)
		}
		// TLS 1.3 only. Its cipher suites are all AEAD with forward secrecy,
		// so there is nothing further to configure.
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: reloader.get}
	}
	ln, err := net.Listen("tcp", cfg.Listen())
	if err != nil {
		return err
	}
	// Everything that needs wide access is done: lock the process down.
	status, err := sandbox.Apply(cfg)
	if err != nil {
		return fmt.Errorf("landlock: %w (set security.landlock: off to run without it)", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go a.PurgeLoop(ctx)
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
		_ = srv.Close() // long-lived event streams
	}()
	log.Info("samling started", "version", version, "listen", ln.Addr().String(), "tls", cfg.TLSEnabled(),
		"database", cfg.Database.Driver, "storage", cfg.Storage.Backend, "landlock", status)
	if cfg.TLSEnabled() {
		err = srv.ServeTLS(ln, "", "")
	} else {
		log.Warn("serving plain HTTP: only acceptable behind a TLS terminating reverse proxy or for local development")
		err = srv.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// secret reads a secret from a file, an environment variable or stdin.
func secret(file, envVar, prompt string) (string, error) {
	if file != "" {
		b, err := os.ReadFile(file)
		return strings.TrimRight(string(b), "\r\n"), err
	}
	if v := os.Getenv(envVar); envVar != "" && v != "" {
		return v, nil
	}
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func adminCreate(args []string) error {
	fs := flag.NewFlagSet("admin create", flag.ExitOnError)
	username := fs.String("username", "", "login name (required)")
	name := fs.String("name", "", "display name")
	email := fs.String("email", "", "e-mail address")
	role := fs.String("role", app.RoleAdmin, "role: admin, leader or member")
	pwFile := fs.String("password-file", "", "read the password from this file instead of generating one")
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	if *username == "" {
		return errors.New("--username is required")
	}
	rt, err := open(cfg)
	if err != nil {
		return err
	}
	defer rt.db.Close()
	a, err := rt.app(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		return err
	}
	pw, generated := "", false
	if *pwFile != "" {
		if pw, err = secret(*pwFile, "", ""); err != nil {
			return err
		}
	} else {
		pw, generated = crypto.Token(15), true
	}
	if _, err := a.CreateUser(context.Background(), *username, *name, *email, pw, []string{*role}, true); err != nil {
		return err
	}
	fmt.Printf("Created account %q with role %q.\n", *username, *role)
	if generated {
		fmt.Printf("Temporary password: %s\n", pw)
	}
	fmt.Println("The password must be changed at first sign-in.")
	return nil
}

func adminPasswd(args []string) error {
	fs := flag.NewFlagSet("admin passwd", flag.ExitOnError)
	username := fs.String("username", "", "login name (required)")
	pwFile := fs.String("password-file", "", "read the password from this file instead of generating one")
	resetMFA := fs.Bool("reset-mfa", false, "also remove two-step verification (lost authenticator)")
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	rt, err := open(cfg)
	if err != nil {
		return err
	}
	defer rt.db.Close()
	a, err := rt.app(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		return err
	}
	var id string
	err = rt.db.Row(context.Background(), `SELECT id FROM users WHERE username=? AND deleted_at=0`, strings.ToLower(*username)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("no account named %q", *username)
	} else if err != nil {
		return err
	}
	pw, generated := "", false
	if *pwFile != "" {
		if pw, err = secret(*pwFile, "", ""); err != nil {
			return err
		}
	} else {
		pw, generated = crypto.Token(15), true
	}
	if err := a.SetPassword(context.Background(), id, pw, true); err != nil {
		return err
	}
	// Recovery path for a locked-out administrator.
	if err := rt.db.Exec(context.Background(), `UPDATE users SET disabled=0 WHERE id=?`, id); err != nil {
		return err
	}
	if *resetMFA {
		if err := rt.db.Exec(context.Background(), `UPDATE users SET totp_enabled=0, totp_secret_enc='', totp_last_step=0 WHERE id=?`, id); err != nil {
			return err
		}
		fmt.Println("Two-step verification was removed for the account.")
	}
	fmt.Printf("Password for %q was reset; all sessions were signed out.\n", *username)
	if generated {
		fmt.Printf("Temporary password: %s\n", pw)
	}
	return nil
}

func backup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("out", "", "output file (required)")
	ppFile := fs.String("passphrase-file", "", "file containing the passphrase")
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	if *out == "" {
		return errors.New("--out is required")
	}
	pass, err := secret(*ppFile, "SAMLING_BACKUP_PASSPHRASE", "Backup passphrase (min 12 characters): ")
	if err != nil {
		return err
	}
	if app.CheckPassphrase(pass) != nil {
		return errors.New("the passphrase must be at least 12 characters")
	}
	rt, err := open(cfg)
	if err != nil {
		return err
	}
	defer rt.db.Close()
	f, err := os.OpenFile(*out+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(*out + ".tmp")
	if err := app.WriteBackup(context.Background(), rt.db, rt.blobs, rt.keys, cfg.OrgName, f, pass); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(*out+".tmp", *out); err != nil {
		return err
	}
	fmt.Println("backup written to", *out)
	return nil
}

func restore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	in := fs.String("in", "", "backup file (required)")
	ppFile := fs.String("passphrase-file", "", "file containing the passphrase")
	cfg, err := loadConfig(fs, args)
	if err != nil {
		return err
	}
	if *in == "" {
		return errors.New("--in is required")
	}
	pass, err := secret(*ppFile, "SAMLING_BACKUP_PASSPHRASE", "Backup passphrase: ")
	if err != nil {
		return err
	}
	f, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer f.Close()
	var d *db.DB
	var masterKey string
	rows, blobs, err := app.Restore(context.Background(), f, pass, func(m app.Manifest) (*db.DB, *blob.Store, error) {
		// The restored data is bound to the master key inside the backup, so
		// that key is used directly (never whatever key happens to be
		// configured) and installed once the restore has succeeded. Refuse to
		// continue if a different key is already in place.
		if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
			return nil, nil, err
		}
		masterKey = m.MasterKey
		if existing, err := os.ReadFile(cfg.Security.MasterKeyFile); err == nil && strings.TrimSpace(string(existing)) != m.MasterKey {
			return nil, nil, fmt.Errorf("%s already exists and differs from the key in the backup; move it away first", cfg.Security.MasterKeyFile)
		}
		raw, err := base64.StdEncoding.DecodeString(m.MasterKey)
		if err != nil {
			return nil, nil, fmt.Errorf("master key in backup: %w", err)
		}
		keys, err := crypto.NewKeys(raw)
		if err != nil {
			return nil, nil, err
		}
		if d, err = db.Open(cfg.Database.Driver, cfg.Database.DSN); err != nil {
			return nil, nil, err
		}
		backend, err := cfg.OpenBlob()
		if err != nil {
			return nil, nil, err
		}
		return d, blob.NewStore(backend, keys.Blob), nil
	})
	if d != nil {
		defer d.Close()
	}
	if err != nil {
		return err
	}
	keyFile := cfg.Security.MasterKeyFile
	if _, statErr := os.Stat(keyFile); errors.Is(statErr, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(keyFile, []byte(masterKey+"\n"), 0o600); err != nil {
			return fmt.Errorf("data restored, but the master key could not be written to %s: %w", keyFile, err)
		}
		fmt.Println("master key from the backup installed at", keyFile)
	}
	fmt.Printf("restored %d rows and %d files\n", rows, blobs)
	return nil
}

func decrypt(args []string) error {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	in := fs.String("in", "", "encrypted file (required)")
	out := fs.String("out", "", "output file (required)")
	ppFile := fs.String("passphrase-file", "", "file containing the passphrase")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" {
		return errors.New("--in and --out are required")
	}
	pass, err := secret(*ppFile, "SAMLING_BACKUP_PASSPHRASE", "Passphrase: ")
	if err != nil {
		return err
	}
	f, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := crypto.NewPassphraseReader(f, pass)
	if err != nil {
		return err
	}
	// Decrypt fully before creating the output so a wrong passphrase or a
	// tampered file never leaves partial plaintext behind.
	o, err := os.OpenFile(*out+".tmp", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(*out + ".tmp")
	if _, err := io.Copy(o, r); err != nil {
		o.Close()
		return err
	}
	if err := o.Close(); err != nil {
		return err
	}
	return os.Rename(*out+".tmp", *out)
}
