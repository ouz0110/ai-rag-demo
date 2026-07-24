package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

var keyBytes = []byte("F19737400616888F")

func AESECBEncodeKeyInit(key string) {
	if key == "" {
		return
	}
	keyBytes = []byte(key)
}

func AESECBEncodeKey() string {
	return base64.StdEncoding.EncodeToString(keyBytes)
}

func pkcs7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func pkcs7UnPadding(data []byte) []byte {
	return data[:len(data)-int(data[len(data)-1])]
}

func AESECBEncrypt(plainText string) (string, error) {
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	plainTextBytes := []byte(plainText)
	plainTextBytes = pkcs7Padding(plainTextBytes, aes.BlockSize)

	ciphertext := make([]byte, len(plainTextBytes))
	for i := 0; i < len(plainTextBytes); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], plainTextBytes[i:i+aes.BlockSize])
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func AESECBDecrypt(encrypted string) (string, error) {
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	encryptedBytes, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	decrypted := make([]byte, len(encryptedBytes))
	for i := 0; i < len(encryptedBytes); i += aes.BlockSize {
		block.Decrypt(decrypted[i:i+aes.BlockSize], encryptedBytes[i:i+aes.BlockSize])
	}

	return string(pkcs7UnPadding(decrypted)), nil
}

func Md5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	cipher := h.Sum(nil)
	return hex.EncodeToString(cipher)
}

func AESECBEncryptWithKey(key, plainText string) (string, error) {
	block, err := aes.NewCipher([]byte(Md5(key)))
	if err != nil {
		return "", err
	}

	plainTextBytes := []byte(plainText)
	plainTextBytes = pkcs7Padding(plainTextBytes, aes.BlockSize)

	ciphertext := make([]byte, len(plainTextBytes))
	for i := 0; i < len(plainTextBytes); i += aes.BlockSize {
		block.Encrypt(ciphertext[i:i+aes.BlockSize], plainTextBytes[i:i+aes.BlockSize])
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func AESECBDecryptWithKey(key, encrypted string) (string, error) {
	block, err := aes.NewCipher([]byte(Md5(key)))
	if err != nil {
		return "", err
	}

	encryptedBytes, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	decrypted := make([]byte, len(encryptedBytes))
	for i := 0; i < len(encryptedBytes); i += aes.BlockSize {
		block.Decrypt(decrypted[i:i+aes.BlockSize], encryptedBytes[i:i+aes.BlockSize])
	}

	return string(pkcs7UnPadding(decrypted)), nil
}

// HashPassword 用于混淆密码: password + salt + openid
func HashPassword(password, salt, openid string) ([]byte, error) {
	return HashString(password, salt, openid)
}

// HashString 用于混淆字符串
func HashString(s ...string) ([]byte, error) {
	h := sha1.New()
	for _, str := range s {
		n, err := io.WriteString(h, str)
		if err != nil || n < 1 {
			return nil, errors.New("string invalid")
		}
	}
	return h.Sum(nil), nil
}
