// Package config 提供应用配置管理，基于 viper 库
// 支持从 .env 文件和环境变量读取配置，并提供必需字段验证
package config

import (
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config 保存应用的所有配置项
type Config struct {
	// 服务器配置
	Port string `mapstructure:"PORT"`

	// 数据库配置
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     string `mapstructure:"DB_PORT"`
	DBName     string `mapstructure:"DB_NAME"`

	// JWT 配置
	JWTSecret                    string `mapstructure:"JWT_SECRET"`
	ModelConfigEncryptionKey     string `mapstructure:"MODEL_CONFIG_ENCRYPTION_KEY"`
	ChatAIURL                    string `mapstructure:"CHAT_AI_URL"`
	ChatBackendURL               string `mapstructure:"CHAT_BACKEND_URL"`
	ModelAllowedPrivateEndpoints string `mapstructure:"MODEL_ALLOWED_PRIVATE_ENDPOINTS"`
	InternalAPISecret            string `mapstructure:"INTERNAL_API_SECRET"`
	QdrantURL                    string `mapstructure:"QDRANT_URL"`
	QdrantAPIKey                 string `mapstructure:"QDRANT_API_KEY"`

	// CORS 配置
	CORSAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`

	// 文件存储配置
	ChunkStoragePath string `mapstructure:"CHUNK_STORAGE_PATH"`
	FileStoragePath  string `mapstructure:"FILE_STORAGE_PATH"`

	// 日志和应用环境配置
	LogLevel string `mapstructure:"LOG_LEVEL"`
	AppEnv   string `mapstructure:"APP_ENV"`
}

// Global 是全局配置实例，在 Load() 后初始化
var Global *Config

// Load 加载配置，初始化 viper 并验证必需字段
// 按优先级从高到低读取配置：环境变量 > .env 文件 > 默认值
func Load() (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 从 .env 文件读取配置（如果存在）
	if err := loadEnvFile(v); err != nil {
		return nil, fmt.Errorf("加载 .env 文件失败: %w", err)
	}

	// 从环境变量读取（优先级高于 .env 文件）
	v.AutomaticEnv()

	// 绑定环境变量名（确保大小写兼容）
	bindEnvVariables(v)

	// 解析到结构体
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 解析存储路径为绝对路径
	cfg.resolveStoragePaths()

	// 验证必需字段
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	Global = &cfg
	return &cfg, nil
}

// setDefaults 设置配置项的默认值
func setDefaults(v *viper.Viper) {
	v.SetDefault("PORT", "9000")
	v.SetDefault("CHAT_AI_URL", "http://127.0.0.1:4111")
	v.SetDefault("CHAT_BACKEND_URL", "http://127.0.0.1:9000")
	v.SetDefault("QDRANT_URL", "http://127.0.0.1:6333")
	v.SetDefault("DB_USER", "root")
	v.SetDefault("DB_PASSWORD", "123456123456")
	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", "3306")
	v.SetDefault("DB_NAME", "zhixu")
	v.SetDefault("CHUNK_STORAGE_PATH", "./data")
	v.SetDefault("FILE_STORAGE_PATH", "./data")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("APP_ENV", "production")
}

// loadEnvFile 尝试从多个位置加载 .env 文件
func loadEnvFile(v *viper.Viper) error {
	if path := os.Getenv("ZHIXU_ENV_FILE"); path != "" {
		v.SetConfigFile(path)
		v.SetConfigType("env")
		return v.ReadInConfig()
	}

	// 可能的 .env 文件位置（按优先级）
	possiblePaths := []string{
		".env",                                    // 当前工作目录
		filepath.Join("..", ".env"),               // 上级目录（开发时从 cmd/server 运行）
		filepath.Join("..", "..", ".env"),         // 上两级目录
		filepath.Join(getExecutableDir(), ".env"), // 可执行文件所在目录
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			v.SetConfigFile(path)
			if err := v.ReadInConfig(); err != nil {
				// 如果文件存在但读取失败，返回错误
				return err
			}
			return nil
		}
	}

	// 没有找到 .env 文件是正常的，不返回错误
	return nil
}

// bindEnvVariables 显式绑定环境变量名
func bindEnvVariables(v *viper.Viper) {
	vars := []string{
		"PORT",
		"DB_USER", "DB_PASSWORD", "DB_HOST", "DB_PORT", "DB_NAME",
		"JWT_SECRET",
		"MODEL_CONFIG_ENCRYPTION_KEY",
		"CHAT_AI_URL", "CHAT_BACKEND_URL", "MODEL_ALLOWED_PRIVATE_ENDPOINTS", "INTERNAL_API_SECRET",
		"QDRANT_URL", "QDRANT_API_KEY",
		"CORS_ALLOWED_ORIGINS",
		"CHUNK_STORAGE_PATH", "FILE_STORAGE_PATH",
		"LOG_LEVEL", "APP_ENV",
	}
	for _, key := range vars {
		_ = v.BindEnv(key)
	}
}

// getExecutableDir 获取可执行文件所在目录
func getExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// resolveStoragePaths 将相对路径转换为绝对路径
func (c *Config) resolveStoragePaths() {
	c.ChunkStoragePath = resolvePath(c.ChunkStoragePath)
	c.FileStoragePath = resolvePath(c.FileStoragePath)
}

// resolvePath 解析路径为绝对路径
func resolvePath(path string) string {
	// 如果已经是绝对路径，直接返回
	if filepath.IsAbs(path) {
		return path
	}

	// 如果路径以 ./ 开头，基于当前工作目录解析
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, ".\\") {
		wd, err := os.Getwd()
		if err != nil {
			// 无法获取工作目录，返回原路径
			return path
		}
		return filepath.Join(wd, path)
	}

	// 其他相对路径，也基于当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	return filepath.Join(wd, path)
}

// validate 验证必需字段
// 如果必需字段为空，返回错误
func (c *Config) validate() error {
	if c.ModelConfigEncryptionKey != "" {
		key, err := base64.StdEncoding.DecodeString(c.ModelConfigEncryptionKey)
		if err != nil || len(key) != 32 {
			return fmt.Errorf("MODEL_CONFIG_ENCRYPTION_KEY 必须是 32 字节随机密钥的 Base64 编码")
		}
	}
	// JWT_SECRET 是必需字段
	if strings.TrimSpace(c.JWTSecret) == "" {
		return fmt.Errorf("必需配置项 JWT_SECRET 为空: JWT 密钥必须显式配置")
	}

	// 验证数据库必需字段
	if strings.TrimSpace(c.DBUser) == "" {
		return fmt.Errorf("必需配置项 DB_USER 为空")
	}
	if strings.TrimSpace(c.DBHost) == "" {
		return fmt.Errorf("必需配置项 DB_HOST 为空")
	}
	if strings.TrimSpace(c.DBPort) == "" {
		return fmt.Errorf("必需配置项 DB_PORT 为空")
	}
	if strings.TrimSpace(c.DBName) == "" {
		return fmt.Errorf("必需配置项 DB_NAME 为空")
	}

	return nil
}

// MustLoad 加载配置，如果失败则 panic
// 用于应用启动时确保配置正确加载
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		panic(fmt.Sprintf("配置加载失败: %v", err))
	}
	return cfg
}

// GetDSN 返回数据库连接字符串
func (c *Config) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser, c.DBPassword, net.JoinHostPort(c.DBHost, c.DBPort), c.DBName)
}

// GetCORSOrigins 返回解析后的 CORS 来源列表
func (c *Config) GetCORSOrigins() []string {
	if strings.TrimSpace(c.CORSAllowedOrigins) == "" {
		return nil
	}

	var origins []string
	for _, o := range strings.Split(c.CORSAllowedOrigins, ",") {
		v := strings.TrimSpace(o)
		if v != "" {
			origins = append(origins, v)
		}
	}
	return origins
}
