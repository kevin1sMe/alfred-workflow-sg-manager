package config

import (
	"os"
	"testing"

	"github.com/keybase/go-keychain"
)

func TestConfigFileRW(t *testing.T) {
	cfg := &Config{
		FrpcTomlPath:    "/tmp/test.toml",
		SecurityGroupId: "sg-12345678",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save config failed: %v", err)
	}
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load config failed: %v", err)
	}
	if cfg2.FrpcTomlPath != cfg.FrpcTomlPath || cfg2.SecurityGroupId != cfg.SecurityGroupId {
		t.Errorf("Loaded config not match: got %+v, want %+v", cfg2, cfg)
	}
	// 清理
	path, _ := configFilePath()
	_ = os.Remove(path)
}

func TestKeychainRW(t *testing.T) {
	secretId := "test-secret-id-abcdefg"
	secretKey := "test-secret-key-1234567"
	if err := SaveSecretId(secretId); err != nil {
		t.Fatalf("SaveSecretId failed: %v", err)
	}
	if err := SaveSecretKey(secretKey); err != nil {
		t.Fatalf("SaveSecretKey failed: %v", err)
	}
	id, err := GetSecretId()
	if err != nil {
		t.Fatalf("GetSecretId failed: %v", err)
	}
	if id != secretId {
		t.Errorf("SecretId not match: got %s, want %s", id, secretId)
	}
	key, err := GetSecretKey()
	if err != nil {
		t.Fatalf("GetSecretKey failed: %v", err)
	}
	if key != secretKey {
		t.Errorf("SecretKey not match: got %s, want %s", key, secretKey)
	}
	// 清理
	_ = deleteKeychain(secretIdLabel)
	_ = deleteKeychain(secretKeyLabel)
}

func deleteKeychain(label string) error {
	return keychain.DeleteGenericPasswordItem(secretService, label)
}
