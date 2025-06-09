package auth

// PasswordHasher interface for password hashing operations
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hashedPassword string) error
}

// Argon2Hasher implements PasswordHasher using Argon2
type Argon2Hasher struct{}

func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{}
}

func (h *Argon2Hasher) HashPassword(password string) (string, error) {
	return HashPassword(password)
}

func (h *Argon2Hasher) VerifyPassword(password, hashedPassword string) error {
	return VerifyPassword(password, hashedPassword)
}
