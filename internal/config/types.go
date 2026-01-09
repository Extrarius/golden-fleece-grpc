package config

// ConfigLogger настройки логирования
type ConfigLogger struct {
	Level string `mapstructure:"level"`
}

// ConfigServer настройки сервера
type ConfigServer struct {
	UseReflection           bool `mapstructure:"use_reflection"`
	PortGRPC                int  `mapstructure:"port_grpc"`
	PortHTTP                int  `mapstructure:"port_http"`
	HTTPReadTimeout         int  `mapstructure:"http_read_timeout"`
	HTTPWriteTimeout        int  `mapstructure:"http_write_timeout"`
	HTTPIdleTimeout         int  `mapstructure:"http_idle_timeout"`
	HTTPReadHeaderTimeout   int  `mapstructure:"http_read_header_timeout"`
	GracefulShutdownTimeout int  `mapstructure:"graceful_shutdown_timeout"`
}

// ConfigGateway настройки HTTP Gateway
type ConfigGateway struct {
	CORSAllowedOrigins string `mapstructure:"cors_allowed_origins"`
	CORSMaxAge         int    `mapstructure:"cors_max_age"`
	RateLimitRPS       int    `mapstructure:"rate_limit_rps"`
	RateLimitBurst     int    `mapstructure:"rate_limit_burst"`
}

// ConfigSwagger настройки Swagger UI сервера
type ConfigSwagger struct {
	Port int `mapstructure:"port"`
}

// Config основная структура конфигурации
type Config struct {
	Logger  *ConfigLogger  `mapstructure:"logger"`
	Server  *ConfigServer  `mapstructure:"server"`
	Gateway *ConfigGateway `mapstructure:"gateway"`
	Swagger *ConfigSwagger `mapstructure:"swagger"`
}
