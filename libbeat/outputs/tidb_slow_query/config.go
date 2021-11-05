package tidb_slow_query

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"io/ioutil"
	"time"
)

const (
	tlsKey = "slow_query_mysql_output"
)

type Config struct {
	// connections
	Host           string        `config:"host"`
	Port           int           `config:"port"`
	User           string        `config:"user"`
	Password       string        `config:"password"`
	Database       string        `config:"database"`
	Timeout        time.Duration `config:"timeout"`
	CAPath         string        `config:"ca_path"`
	ClientCertPath string        `config:"client_cert_path"`
	ClientKeyPath  string        `config:"client_key_path"`

	// retry
	MaxRetries int     `config:"max_retries"`
	Backoff    Backoff `config:"backoff"`

	// sql range partition
	Partition Partition `config:"partition"`
}

func (c Config) DSN() string {
	defaultConfig := mysql.NewConfig()

	defaultConfig.Net = "tcp"
	defaultConfig.User = c.User
	defaultConfig.Addr = fmt.Sprintf("%s:%d", c.Host, c.Port)
	defaultConfig.Passwd = c.Password
	defaultConfig.DBName = c.Database
	defaultConfig.ParseTime = true
	defaultConfig.Loc = time.UTC
	defaultConfig.Params = map[string]string{"charset": "utf8mb4"}
	defaultConfig.Collation = "utf8mb4_bin"
	defaultConfig.TLSConfig = tlsKey

	return defaultConfig.FormatDSN()
}

func (c Config) isMutualTLSEnabled() bool {
	if len(c.CAPath) > 0 && len(c.ClientCertPath) > 0 && len(c.ClientKeyPath) > 0 {
		return true
	}
	return false
}

func (c Config) registerTLSToDriver() error {
	if !c.isMutualTLSEnabled() {
		return fmt.Errorf("failed to enable tls: some of tls configs (ca, client key, or client cert) are missing")
	}
	// init ca
	rootCertPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(c.CAPath)
	if err != nil {
		return err
	}
	if ok := rootCertPool.AppendCertsFromPEM([]byte(ca)); !ok {
		return fmt.Errorf("parsing and appending certificates failed")
	}
	// init client cert and client key
	clientCert := make([]tls.Certificate, 0, 1)
	certs, err := tls.LoadX509KeyPair(c.ClientCertPath, c.ClientKeyPath)
	if err != nil {
		return err
	}
	clientCert = append(clientCert, certs)
	// register ca and cert to mysql driver
	return mysql.RegisterTLSConfig(tlsKey, &tls.Config{
		RootCAs:      rootCertPool,
		Certificates: clientCert,
	})
}

type Backoff struct {
	Init time.Duration `config:"init"`
	Max  time.Duration `config:"max"`
}

type Partition struct {
	Retention int `config:"retention"`
	RollStep  int `config:"roll_step"`
}

var defaultConfig = Config{
	Port:       4000,
	Timeout:    30 * time.Second,
	MaxRetries: 3,
	Backoff: Backoff{
		Init: 1 * time.Second,
		Max:  10 * time.Second,
	},
	Partition: Partition{
		Retention: 365,
		RollStep:  3,
	},
}
