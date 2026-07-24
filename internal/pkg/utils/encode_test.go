package utils

import "testing"

func TestAESECBEncrypt(t *testing.T) {
	AESECBEncodeKeyInit("0123456789012345")
	encrypted, err := AESECBEncrypt("18700000002")
	if err != nil {
		t.Errorf("AESECBEncrypt() error = %v", err)
		return
	}
	t.Log(encrypted)
}

func TestAESECBDecrypt(t *testing.T) {
	AESECBEncodeKeyInit("0123456789012345")
	decrypted, err := AESECBDecrypt("POO1CwwIbM7YhDELBMwj0g==")
	if err != nil {
		t.Errorf("AESECBDecrypt() error = %v", err)
		return
	}
	t.Log(decrypted)
}
