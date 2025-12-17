package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"log"
)

type Config struct {
	Port               string
	UserServiceAddr    string
	ProjectServiceAddr string
	PublicKeyPath      string
	PublicKey          *rsa.PublicKey
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}

	possiblePaths := []string{
		"../../keys/public.pem",
		"../keys/public.pem",
		"keys/public.pem",
		filepath.Join(filepath.Dir(execPath), "../../keys/public.pem"),
		filepath.Join(filepath.Dir(execPath), "../keys/public.pem"),
	}

	var publicKey *rsa.PublicKey
	var publicKeyPath string

	for _, path := range possiblePaths {
		if key, err := loadPublicKey(path); err == nil {
			publicKey = key
			publicKeyPath = path
			log.Printf("Loaded public key from: %s", path)
			break
		}
	}

	if publicKey == nil {
		log.Println("WARNING: No public key found, token validation will fail")
	}

	return &Config{
		Port:               getEnv("PORT", "8080"),
		UserServiceAddr:    getEnv("USER_SERVICE_ADDR", "localhost:50051"),
		ProjectServiceAddr: getEnv("PROJECT_SERVICE_ADDR", "localhost:50052"),
		PublicKeyPath:      publicKeyPath,
		PublicKey:          publicKey,
	}, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(keyData)
	if block == nil || block.Type != "PUBLIC KEY" && block.Type != "RSA PUBLIC KEY" {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		pub, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}

	return rsaPub, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
