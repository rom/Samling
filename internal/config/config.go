// Package config loads the server configuration.
//
// Precedence (highest first): command line flags, environment variables
// (SAMLING_*), the YAML config file, built-in defaults. Secrets such as the
// database DSN or S3 keys can therefore be kept out of the YAML file and be
// injected through the environment instead.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rom/samling/internal/blob"
	"github.com/rom/samling/internal/crypto"
)

// Config mirrors the layout of samling.yaml.
type Config struct {
	OrgName string `yaml:"org_name"`
	DataDir string `yaml:"data_dir"`

	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
		TLS  struct {
			Cert string `yaml:"cert"`
			Key  string `yaml:"key"`
		} `yaml:"tls"`
		// InsecureHTTP serves plain HTTP. Only for development or behind a
		// TLS terminating reverse proxy on the same host.
		InsecureHTTP bool `yaml:"insecure_http"`
		// TrustedProxy trusts X-Forwarded-For / X-Forwarded-Proto.
		TrustedProxy bool `yaml:"trusted_proxy"`
	} `yaml:"server"`

	Database struct {
		Driver string `yaml:"driver"` // sqlite | postgres
		DSN    string `yaml:"dsn"`
	} `yaml:"database"`

	Storage struct {
		Backend string `yaml:"backend"` // fs | s3
		S3      struct {
			Endpoint  string `yaml:"endpoint"`
			Region    string `yaml:"region"`
			Bucket    string `yaml:"bucket"`
			AccessKey string `yaml:"access_key"`
			SecretKey string `yaml:"secret_key"`
			Prefix    string `yaml:"prefix"`
			Insecure  bool   `yaml:"insecure"`
		} `yaml:"s3"`
		MaxUploadMB int64 `yaml:"max_upload_mb"`
	} `yaml:"storage"`

	Security struct {
		MasterKeyFile     string `yaml:"master_key_file"`
		MinPasswordLength int    `yaml:"min_password_length"`
		SessionHours      int    `yaml:"session_hours"`
		Landlock          string `yaml:"landlock"` // on | off | strict
	} `yaml:"security"`
}

func defaults() *Config {
	c := &Config{OrgName: "Samling", DataDir: "./data"}
	c.Server.Port = 8443
	c.Database.Driver = "sqlite"
	c.Storage.Backend = "fs"
	c.Storage.S3.Region = "us-east-1"
	c.Storage.MaxUploadMB = 50
	c.Security.MinPasswordLength = 12
	c.Security.SessionHours = 24 * 14
	c.Security.Landlock = "on"
	return c
}

func envStr(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		*dst = v
	}
}

func envBool(key string, dst *bool) {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		v = strings.ToLower(v)
		*dst = v == "1" || v == "true" || v == "yes" || v == "on"
	}
}

func envInt[T int | int64](key string, dst *T) error {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: not a number: %q", key, v)
		}
		*dst = T(n)
	}
	return nil
}

// Load reads the YAML file at path (may be empty for defaults only), applies
// environment overrides and validates the result.
func Load(path string) (*Config, error) {
	c := defaults()
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		defer f.Close()
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true) // typos in the config must not pass silently
		if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
	}
	envStr("SAMLING_ORG_NAME", &c.OrgName)
	envStr("SAMLING_DATA_DIR", &c.DataDir)
	envStr("SAMLING_HOST", &c.Server.Host)
	envStr("SAMLING_TLS_CERT", &c.Server.TLS.Cert)
	envStr("SAMLING_TLS_KEY", &c.Server.TLS.Key)
	envBool("SAMLING_INSECURE_HTTP", &c.Server.InsecureHTTP)
	envBool("SAMLING_TRUSTED_PROXY", &c.Server.TrustedProxy)
	envStr("SAMLING_DB_DRIVER", &c.Database.Driver)
	envStr("SAMLING_DB_DSN", &c.Database.DSN)
	envStr("SAMLING_STORAGE_BACKEND", &c.Storage.Backend)
	envStr("SAMLING_S3_ENDPOINT", &c.Storage.S3.Endpoint)
	envStr("SAMLING_S3_REGION", &c.Storage.S3.Region)
	envStr("SAMLING_S3_BUCKET", &c.Storage.S3.Bucket)
	envStr("SAMLING_S3_ACCESS_KEY", &c.Storage.S3.AccessKey)
	envStr("SAMLING_S3_SECRET_KEY", &c.Storage.S3.SecretKey)
	envStr("SAMLING_S3_PREFIX", &c.Storage.S3.Prefix)
	envBool("SAMLING_S3_INSECURE", &c.Storage.S3.Insecure)
	envStr("SAMLING_MASTER_KEY_FILE", &c.Security.MasterKeyFile)
	envStr("SAMLING_LANDLOCK", &c.Security.Landlock)
	for key, dst := range map[string]*int{
		"SAMLING_PORT": &c.Server.Port, "SAMLING_MIN_PASSWORD_LENGTH": &c.Security.MinPasswordLength,
		"SAMLING_SESSION_HOURS": &c.Security.SessionHours,
	} {
		if err := envInt(key, dst); err != nil {
			return nil, err
		}
	}
	if err := envInt("SAMLING_MAX_UPLOAD_MB", &c.Storage.MaxUploadMB); err != nil {
		return nil, err
	}
	return c, nil
}

// Finish fills in derived defaults and validates. Call it after flags have
// been applied.
func (c *Config) Finish() error {
	if c.Database.Driver == "sqlite" && c.Database.DSN == "" {
		c.Database.DSN = filepath.Join(c.DataDir, "samling.db")
	}
	if c.Security.MasterKeyFile == "" {
		c.Security.MasterKeyFile = filepath.Join(c.DataDir, "master.key")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port)
	}
	if c.Database.Driver != "sqlite" && c.Database.Driver != "postgres" {
		return errors.New("database.driver must be sqlite or postgres")
	}
	if c.Database.Driver == "postgres" && c.Database.DSN == "" {
		return errors.New("database.dsn is required for postgres")
	}
	if c.Storage.Backend != "fs" && c.Storage.Backend != "s3" {
		return errors.New("storage.backend must be fs or s3")
	}
	switch c.Security.Landlock {
	case "on", "off", "strict":
	default:
		return errors.New("security.landlock must be on, off or strict")
	}
	if (c.Server.TLS.Cert == "") != (c.Server.TLS.Key == "") {
		return errors.New("server.tls.cert and server.tls.key must be set together")
	}
	if c.Security.MinPasswordLength < 8 {
		return errors.New("security.min_password_length must be at least 8")
	}
	if c.Storage.MaxUploadMB < 1 || c.Security.SessionHours < 1 {
		return errors.New("storage.max_upload_mb and security.session_hours must be positive")
	}
	return nil
}

// Listen returns the host:port the server binds to.
func (c *Config) Listen() string {
	host := c.Server.Host
	if strings.Contains(host, ":") { // IPv6 literal
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(c.Server.Port)
}

// TLSEnabled reports whether the server terminates TLS itself.
func (c *Config) TLSEnabled() bool { return c.Server.TLS.Cert != "" && c.Server.TLS.Key != "" }

// LoadMasterKey reads the master key (base64, 32 bytes). When create is true
// and the file does not exist a new key is generated with mode 0600.
func (c *Config) LoadMasterKey(create bool) (*crypto.Keys, error) {
	path := c.Security.MasterKeyFile
	fromCred := false
	// systemd LoadCredential= exposes secrets in $CREDENTIALS_DIRECTORY.
	if dir := os.Getenv("CREDENTIALS_DIRECTORY"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "master.key")); err == nil {
			path, fromCred = filepath.Join(dir, "master.key"), true
		}
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && create {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		key := crypto.GenerateKey()
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)+"\n"), 0o600); err != nil {
			return nil, err
		}
		return crypto.NewKeys(key)
	}
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	if fi, err := os.Stat(path); err == nil && fi.Mode().Perm()&0o077 != 0 && !fromCred {
		return nil, fmt.Errorf("master key file %s must not be accessible by group/others (chmod 600)", path)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}
	return crypto.NewKeys(key)
}

// OpenBlob creates the configured blob back-end.
func (c *Config) OpenBlob() (blob.Backend, error) {
	if c.Storage.Backend == "s3" {
		s := c.Storage.S3
		return blob.NewS3(blob.S3Config{Endpoint: s.Endpoint, Region: s.Region, Bucket: s.Bucket,
			AccessKey: s.AccessKey, SecretKey: s.SecretKey, Prefix: s.Prefix, Insecure: s.Insecure})
	}
	return blob.NewFS(filepath.Join(c.DataDir, "blobs"))
}
