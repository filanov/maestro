package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
)

type ServiceConfig struct {
	DBURL string `env:"MAESTRO_DB_URL,required"`

	GRPCHost string `env:"MAESTRO_GRPC_HOST" envDefault:"0.0.0.0"`
	GRPCPort int    `env:"MAESTRO_GRPC_PORT" envDefault:"9090"`

	RESTHost string `env:"MAESTRO_REST_HOST" envDefault:"0.0.0.0"`
	RESTPort int    `env:"MAESTRO_REST_PORT" envDefault:"8080"`

	HealthCheckInterval   time.Duration `env:"MAESTRO_HEALTH_CHECK_INTERVAL" envDefault:"1m"`
	HealthMissedThreshold int           `env:"MAESTRO_HEALTH_MISSED_THRESHOLD" envDefault:"3"`

	CleanerInterval      time.Duration `env:"MAESTRO_CLEANER_INTERVAL" envDefault:"1h"`
	RetentionDays        int           `env:"MAESTRO_RETENTION_DAYS" envDefault:"7"`
	MaxExecutionsPerTask int           `env:"MAESTRO_MAX_EXECUTIONS_PER_TASK" envDefault:"10"`

	MaxOutputSize int `env:"MAESTRO_MAX_OUTPUT_SIZE" envDefault:"1048576"`

	DBMaxOpenConns    int           `env:"MAESTRO_DB_MAX_OPEN_CONNS" envDefault:"100"`
	DBMaxIdleConns    int           `env:"MAESTRO_DB_MAX_IDLE_CONNS" envDefault:"10"`
	DBConnMaxLifetime time.Duration `env:"MAESTRO_DB_CONN_MAX_LIFETIME" envDefault:"5m"`
	DBConnMaxIdleTime time.Duration `env:"MAESTRO_DB_CONN_MAX_IDLE_TIME" envDefault:"5m"`

	LogLevel  string `env:"MAESTRO_LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"MAESTRO_LOG_FORMAT" envDefault:"json"`
}

func LoadServiceConfig() (*ServiceConfig, error) {
	cfg := &ServiceConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse service config: %w", err)
	}
	return cfg, nil
}

type AgentConfig struct {
	ServiceHost string `env:"MAESTRO_SERVICE_HOST,required"`
	ServicePort int    `env:"MAESTRO_SERVICE_PORT" envDefault:"9090"`

	ClusterID string `env:"MAESTRO_CLUSTER_ID,required"`
	Hostname  string `env:"MAESTRO_HOSTNAME" envDefault:"auto"`

	HeartbeatInterval time.Duration `env:"MAESTRO_HEARTBEAT_INTERVAL" envDefault:"1m"`
	TaskPollInterval  time.Duration `env:"MAESTRO_TASK_POLL_INTERVAL" envDefault:"1m"`
	DebugPollInterval time.Duration `env:"MAESTRO_DEBUG_POLL_INTERVAL" envDefault:"5s"`

	MaxOutputSize int `env:"MAESTRO_MAX_OUTPUT_SIZE" envDefault:"1048576"`

	LogLevel  string `env:"MAESTRO_LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"MAESTRO_LOG_FORMAT" envDefault:"json"`
}

func LoadAgentConfig() (*AgentConfig, error) {
	cfg := &AgentConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parse agent config: %w", err)
	}
	return cfg, nil
}
