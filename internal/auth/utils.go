package auth

import "golang.org/x/crypto/bcrypt"

func hashPassword(password string) (string, error) {
	// Cost factor (can be adjusted; 12 is a solid modern balance between speed and security)
	cost := 12

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
