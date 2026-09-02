package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server        SeverConfig
	Database      DatabaseConfig
	JWT           JWTConfig
	Brevo         BrevoConfig
	App           AppConfig
	GoogleOAuth   GoogleOAuthConfig
	ObjectStorage ObjectStorageConfig
	Worker        WorkerConfig
	Cook          CookConfig
}

type CookConfig struct {
	BaseXP                int
	PhotoXP               int
	FirstDishXP           int
	DailyRewardedSessions int
}

type ObjectStorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
}

func (c ObjectStorageConfig) Enabled() bool {
	return c.Endpoint != "" && c.AccessKey != "" && c.SecretKey != "" && c.Bucket != ""
}

type WorkerConfig struct {
	ID             string
	PollIntervalMS int
	BatchSize      int
}

type AppConfig struct {
	PublicURL string
}

type GoogleOAuthConfig struct {
	ClientID          string
	ClientSecret      string
	RedirectURL       string
	AllowedReturnURLs []string
}

func (c GoogleOAuthConfig) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != "" && len(c.AllowedReturnURLs) > 0
}

type JWTConfig struct {
	AccessSecret        string `mapstructure:"JWT_ACCESS_SECRET"`
	RefreshSecret       string `mapstructure:"JWT_REFRESH_SECRET"`
	AccessTTLMin        int    `mapstructure:"access_ttl"`
	RefreshTLLDay       int    `mapstructure:"refresh_ttl"`
	EmailVerifyTTLHours int    `mapstructure:"email_verify_ttl"`
}

type SeverConfig struct {
	Env            string   `mapstructure:"ENV"`
	Port           string   `mapstructure:"PORT"`
	ReadTimeOut    int      `mapstructure:"READ_TIME_OUT"`
	IdleTimeOut    int      `mapstructure:"IDLE_TIME_OUT"`
	WriteTimeOut   int      `mapstructure:"WRITE_TIME_OUT"`
	TrustedProxies []string `mapstructure:"TRUSTED_PROXIES"`
}

type DatabaseConfig struct {
	Host                  string `mapstructure:"DB_HOST"`
	Name                  string `mapstructure:"DB_NAME"`
	User                  string `mapstructure:"DB_USER"`
	Pass                  string `mapstructure:"DB_PASS"`
	Port                  string `mapstructure:"DB_PORT"`
	SSLMode               string `mapstructure:"DB_SSL_MODE"`
	MaxOpenConnection     int    `mapstructure:"DB_MAX_OPEN_CONNECTION"`
	MaxIdleConnection     int    `mapstructure:"DB_MAX_IDLE_CONNECTION"`
	MaxConnectionLifeTime int    `mapstructure:"DB_MAX_CONNECTION_LIFE_TIME"`
}

type BrevoConfig struct {
	SenderEmail string `mapstructure:"BREVO_SENDER_EMAIL"`
	SenderName  string `mapstructure:"BREVO_SENDER_NAME"`
	APIKey      string `mapstructure:"BREVO_API_KEY"`
	BaseUrl     string `mapstructure:"BREVO_BASE_URL"`
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
	cfg := &Config{
		Server: SeverConfig{
			Env:            viper.GetString("ENV"),
			Port:           viper.GetString("PORT"),
			IdleTimeOut:    viper.GetInt("IDLE_TIME_OUT"),
			WriteTimeOut:   viper.GetInt("WRITE_TIME_OUT"),
			ReadTimeOut:    viper.GetInt("READ_TIME_OUT"),
			TrustedProxies: splitCSV(viper.GetString("TRUSTED_PROXIES")),
		},
		Database: DatabaseConfig{
			Host:                  viper.GetString("DB_HOST"),
			Name:                  viper.GetString("DB_NAME"),
			User:                  viper.GetString("DB_USER"),
			Pass:                  viper.GetString("DB_PASS"),
			Port:                  viper.GetString("DB_PORT"),
			SSLMode:               viper.GetString("DB_SSL_MODE"),
			MaxOpenConnection:     viper.GetInt("DB_MAX_OPEN_CONNECTION"),
			MaxIdleConnection:     viper.GetInt("DB_MAX_IDLE_CONNECTION"),
			MaxConnectionLifeTime: viper.GetInt("DB_MAX_CONNECTION_LIFE_TIME"),
		},
		JWT: JWTConfig{
			AccessSecret:        firstNonEmpty(viper.GetString("JWT_ACCESS_SECRET"), viper.GetString("JWT_SECRET")),
			RefreshSecret:       firstNonEmpty(viper.GetString("JWT_REFRESH_SECRET"), viper.GetString("JWT_SECRET")),
			AccessTTLMin:        firstNonZero(viper.GetInt("JWT_ACCESS_TTL"), viper.GetInt("JWT_EXPIRY_MINUTES")),
			RefreshTLLDay:       firstNonZero(viper.GetInt("JWT_REFRESH_TTL"), viper.GetInt("JWT_REFRESH_TOKEN_EXPIRY_DAYS")),
			EmailVerifyTTLHours: viper.GetInt("JWT_EMAIL_VERIFY_TTL"),
		},
		Brevo: BrevoConfig{
			SenderEmail: viper.GetString("BREVO_SENDER_EMAIL"),
			SenderName:  viper.GetString("BREVO_SENDER_NAME"),
			APIKey:      viper.GetString("BREVO_API_KEY"),
			BaseUrl:     viper.GetString("BREVO_BASE_URL"),
		},
		App: AppConfig{
			PublicURL: viper.GetString("APP_PUBLIC_URL"),
		},
		GoogleOAuth: GoogleOAuthConfig{
			ClientID: viper.GetString("GOOGLE_OAUTH_CLIENT_ID"), ClientSecret: viper.GetString("GOOGLE_OAUTH_CLIENT_SECRET"),
			RedirectURL: viper.GetString("GOOGLE_OAUTH_REDIRECT_URL"), AllowedReturnURLs: splitCSV(viper.GetString("GOOGLE_OAUTH_ALLOWED_RETURN_URLS")),
		},
		ObjectStorage: ObjectStorageConfig{
			Endpoint: viper.GetString("S3_ENDPOINT"), AccessKey: viper.GetString("S3_ACCESS_KEY"),
			SecretKey: viper.GetString("S3_SECRET_KEY"), Bucket: viper.GetString("S3_BUCKET"),
			Region: viper.GetString("S3_REGION"), UseSSL: viper.GetBool("S3_USE_SSL"),
		},
		Worker: WorkerConfig{
			ID: viper.GetString("WORKER_ID"), PollIntervalMS: viper.GetInt("WORKER_POLL_INTERVAL_MS"), BatchSize: viper.GetInt("WORKER_BATCH_SIZE"),
		},
		Cook: CookConfig{BaseXP: viper.GetInt("COOK_BASE_XP"), PhotoXP: viper.GetInt("COOK_PHOTO_XP"), FirstDishXP: viper.GetInt("COOK_FIRST_DISH_XP"), DailyRewardedSessions: viper.GetInt("COOK_DAILY_REWARDED_SESSIONS")},
	}
	setDefaultConfigs(cfg)
	err := validateConfigs(cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func setDefaultConfigs(cfg *Config) {
	if cfg.Cook.BaseXP == 0 {
		cfg.Cook.BaseXP = 50
	}
	if cfg.Cook.PhotoXP == 0 {
		cfg.Cook.PhotoXP = 10
	}
	if cfg.Cook.FirstDishXP == 0 {
		cfg.Cook.FirstDishXP = 25
	}
	if cfg.Cook.DailyRewardedSessions == 0 {
		cfg.Cook.DailyRewardedSessions = 5
	}
	if cfg.Server.Env == "" {
		cfg.Server.Env = "development"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.WriteTimeOut == 0 {
		cfg.Server.WriteTimeOut = 15
	}
	if cfg.Server.ReadTimeOut == 0 {
		cfg.Server.ReadTimeOut = 15
	}
	if cfg.Server.IdleTimeOut == 0 {
		cfg.Server.IdleTimeOut = 60
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}

	if cfg.Database.Port == "" {
		cfg.Database.Port = "8560"
	}
	if cfg.Database.MaxOpenConnection == 0 {
		cfg.Database.MaxOpenConnection = 100
	}
	if cfg.Database.MaxIdleConnection == 0 {
		cfg.Database.MaxIdleConnection = 10
	}
	if cfg.Database.MaxConnectionLifeTime == 0 {
		cfg.Database.MaxConnectionLifeTime = 1
	}
	if cfg.JWT.AccessTTLMin == 0 {
		cfg.JWT.AccessTTLMin = 15
	}
	if cfg.JWT.RefreshTLLDay == 0 {
		cfg.JWT.RefreshTLLDay = 14
	}
	if cfg.JWT.EmailVerifyTTLHours == 0 {
		cfg.JWT.EmailVerifyTTLHours = 24
	}
	if cfg.App.PublicURL == "" {
		cfg.App.PublicURL = "http://localhost:" + cfg.Server.Port
	}
	if cfg.Worker.ID == "" {
		cfg.Worker.ID = "cooked-worker"
	}
	if cfg.Worker.PollIntervalMS == 0 {
		cfg.Worker.PollIntervalMS = 1000
	}
	if cfg.Worker.BatchSize == 0 {
		cfg.Worker.BatchSize = 20
	}

}

func validateConfigs(cfg *Config) error {

	if cfg.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if cfg.Database.Name == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if cfg.Database.Pass == "" {
		return fmt.Errorf("DB_PASS is required")
	}

	if cfg.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}
	if cfg.JWT.AccessSecret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	}
	if cfg.JWT.RefreshSecret == "" {
		return fmt.Errorf("JWT_REFRESH_SECRET is required")
	}
	if cfg.Cook.BaseXP < 1 || cfg.Cook.PhotoXP < 1 || cfg.Cook.FirstDishXP < 1 || cfg.Cook.DailyRewardedSessions < 1 {
		return fmt.Errorf("Cook reward values must be positive")
	}
	return nil
}

func (b BrevoConfig) Enabled() bool {
	return b.APIKey != "" && b.BaseUrl != "" && b.SenderEmail != "" && b.SenderName != ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstNonZero(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s", d.Host, d.User, d.Pass, d.Name, d.Port, d.SSLMode)
}
