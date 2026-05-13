package modules

import "golang.org/x/crypto/bcrypt"

type EncryptionManager struct{}

func NewEncryptionManager() *EncryptionManager {
	return &EncryptionManager{}
}

func (e *EncryptionManager) HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashedPassword), nil
}

func (e *EncryptionManager) VerifyPasswordHash(passwordHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

func (e *EncryptionManager) IsPasswordStrong(password string) bool {
	if len(password) < 8 {
		return false
	}
	return true
}

var EncryptionManagerInstance = NewEncryptionManager()
