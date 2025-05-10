package config

import (
	"errors"
	"log"
	"os"

	"github.com/keybase/go-keychain"
)

const (
	configDirName  = ".alfred-frp-sg"
	configFileName = "config.json"
	secretService  = "Alfred_TencentCloud_FRP_Keys"
	secretIdLabel  = "TENCENTCLOUD_FRP_SECRET_ID"
	secretKeyLabel = "TENCENTCLOUD_FRP_SECRET_KEY"
)

type Config struct {
	FrpcTomlPath    string `json:"frpc_toml_path"`
	SecurityGroupId string `json:"security_group_id"`
	Region          string `json:"region"`
	LogPath         string `json:"log_path"`
	SecretId        string `json:"secret_id,omitempty"`
	SecretKey       string `json:"secret_key,omitempty"`
}

func Load() (*Config, error) {
	cfg := Config{
		FrpcTomlPath:    os.Getenv("FRPC_TOML_PATH"),
		SecurityGroupId: os.Getenv("SECURITY_GROUP_ID"),
		Region:          os.Getenv("REGION"),
		LogPath:         os.Getenv("LOG_PATH"),
		SecretId:        os.Getenv("SECRET_ID"),
		SecretKey:       os.Getenv("SECRET_KEY"),
	}
	log.Println("load config:", cfg)
	if cfg.FrpcTomlPath == "" || cfg.SecurityGroupId == "" || cfg.Region == "" || cfg.LogPath == "" {
		return nil, errors.New("FRPC_TOML_PATH, SECURITY_GROUP_ID, REGION, LOG_PATH 这些环境变量必须全部设置")
	}
	return &cfg, nil
}

func SaveSecretId(secretId string) error {
	return saveKeychain(secretIdLabel, secretId)
}

func SaveSecretKey(secretKey string) error {
	return saveKeychain(secretKeyLabel, secretKey)
}

func GetSecretId() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.SecretId != "" {
		return cfg.SecretId, nil
	}
	return getKeychain(secretIdLabel)
}

func GetSecretKey() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	if cfg.SecretKey != "" {
		return cfg.SecretKey, nil
	}
	return getKeychain(secretKeyLabel)
}

func saveKeychain(label, value string) error {
	item := keychain.NewGenericPassword(secretService, label, label, []byte(value), "")
	item.SetSynchronizable(keychain.SynchronizableNo)
	item.SetAccessible(keychain.AccessibleWhenUnlocked)
	_ = keychain.DeleteGenericPasswordItem(secretService, label)
	return keychain.AddItem(item)
}

func getKeychain(label string) (string, error) {
	q := keychain.NewItem()
	q.SetSecClass(keychain.SecClassGenericPassword)
	q.SetService(secretService)
	q.SetAccount(label)
	q.SetMatchLimit(keychain.MatchLimitOne)
	q.SetReturnData(true)
	results, err := keychain.QueryItem(q)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", errors.New("not found")
	}
	return string(results[0].Data), nil
}
