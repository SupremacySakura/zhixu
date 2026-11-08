package config

import (
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestModelEncryptionKeyConfiguration(t *testing.T) {
	for _, key := range []string{"", base64.StdEncoding.EncodeToString(make([]byte, 32))} {
		cfg := Config{JWTSecret: "0123456789abcdef0123456789abcdef", DBUser: "root", DBHost: "localhost", DBPort: "3306", DBName: "zhixu", ModelConfigEncryptionKey: key}
		require.NoError(t, cfg.validate())
	}
	cfg := Config{ModelConfigEncryptionKey: "invalid-secret"}
	err := cfg.validate()
	require.Error(t, err)
	require.NotContains(t, err.Error(), "invalid-secret")
	v := viper.New()
	bindEnvVariables(v)
	t.Setenv("MODEL_CONFIG_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	var loaded Config
	require.NoError(t, v.Unmarshal(&loaded))
	require.NotEmpty(t, loaded.ModelConfigEncryptionKey)
}

func TestJWTSecretValidation(t *testing.T) {
	base := Config{JWTSecret: "0123456789abcdef0123456789abcdef", DBUser: "root", DBHost: "localhost", DBPort: "3306", DBName: "zhixu"}

	// 空值、已知占位值（含历史文档中的长占位符）、31 字符边界均应被拒绝
	for _, secret := range []string{"", "your_secret_key", "YOUR_SECRET_KEY", "changeme", "0123456789abcdef0123456789abcde", "REPLACE_WITH_32_PLUS_CHAR_RANDOM_SECRET"} {
		cfg := base
		cfg.JWTSecret = secret
		require.Error(t, cfg.validate(), "secret %q should be rejected", secret)
	}

	// 32 字符边界应通过
	cfg := base
	require.NoError(t, cfg.validate())
}

func TestGetTrustedProxies(t *testing.T) {
	// 空值返回 nil（调用方转成不信任任何代理）
	require.Nil(t, (&Config{TrustedProxies: ""}).GetTrustedProxies())
	// 显式 none 表示不信任任何代理（空串会被 viper 忽略，无法表达禁用）
	require.Empty(t, (&Config{TrustedProxies: "none"}).GetTrustedProxies())
	require.Empty(t, (&Config{TrustedProxies: " NONE "}).GetTrustedProxies())
	// 正常解析 CIDR 列表并去空白
	require.Equal(t, []string{"172.28.0.0/16", "127.0.0.1"},
		(&Config{TrustedProxies: "172.28.0.0/16, 127.0.0.1"}).GetTrustedProxies())
}

func TestLoadEnvFileUsesExplicitDevelopmentFile(t *testing.T) {
	dir := t.TempDir()
	development := filepath.Join(dir, ".env.development")
	require.NoError(t, os.WriteFile(development, []byte("DB_HOST=127.0.0.1\nDB_NAME=zhixu\nPORT=9100\n"), 0600))
	t.Setenv("ZHIXU_ENV_FILE", development)

	v := viper.New()
	require.NoError(t, loadEnvFile(v))
	require.Equal(t, "127.0.0.1", v.GetString("DB_HOST"))
	require.Equal(t, "zhixu", v.GetString("DB_NAME"))
	require.Equal(t, "9100", v.GetString("PORT"))
}

func TestExplicitMissingEnvFileDoesNotFallBackToDockerEnv(t *testing.T) {
	t.Setenv("ZHIXU_ENV_FILE", filepath.Join(t.TempDir(), ".env.development"))
	require.Error(t, loadEnvFile(viper.New()))
}

func TestDatabaseDefaultUsesZhixu(t *testing.T) {
	v := viper.New()
	setDefaults(v)
	require.Equal(t, "zhixu", v.GetString("DB_NAME"))
}

func TestGetDSNFormatsDatabaseAddress(t *testing.T) {
	for _, scenario := range []struct {
		host, port, address string
	}{
		{"127.0.0.1", "3306", "127.0.0.1:3306"},
		{"localhost", "3306", "localhost:3306"},
		{"mysql", "3306", "mysql:3306"},
		{"::1", "3306", "[::1]:3306"},
		{"2001:db8::1", "3307", "[2001:db8::1]:3307"},
	} {
		t.Run(scenario.host, func(t *testing.T) {
			cfg := Config{
				DBUser: "testuser", DBPassword: "testpassword",
				DBHost: scenario.host, DBPort: scenario.port, DBName: "zhixu",
			}
			dsn := cfg.GetDSN()
			require.Equal(t, "testuser:testpassword@tcp("+scenario.address+")/zhixu?charset=utf8mb4&parseTime=True&loc=Local", dsn)

			parsed, err := mysqlDriver.ParseDSN(dsn)
			require.NoError(t, err)
			require.Equal(t, scenario.address, parsed.Addr)
			host, port, err := net.SplitHostPort(parsed.Addr)
			require.NoError(t, err)
			require.Equal(t, scenario.host, host)
			require.Equal(t, scenario.port, port)
		})
	}
}
