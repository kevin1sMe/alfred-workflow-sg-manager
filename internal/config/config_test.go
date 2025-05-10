package config

import (
	"testing"

	"github.com/keybase/go-keychain"
)

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
