package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

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
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, configDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func Load() (*Config, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func SaveSecretId(secretId string) error {
	return saveKeychain(secretIdLabel, secretId)
}

func SaveSecretKey(secretKey string) error {
	return saveKeychain(secretKeyLabel, secretKey)
}

func GetSecretId() (string, error) {
	return getKeychain(secretIdLabel)
}

func GetSecretKey() (string, error) {
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
