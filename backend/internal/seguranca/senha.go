// Pacote seguranca contém hash de senhas (Argon2id) e geração de tokens.
package seguranca

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Parâmetros Argon2id (recomendação OWASP: m=19 MiB, t=2, p=1; usamos 64 MiB, t=3).
const (
	argonMemoria    = 64 * 1024
	argonIteracoes  = 3
	argonParalelo   = 2
	argonTamanhoSal = 16
	argonTamanhoHash = 32
)

var ErrSenhaFraca = errors.New("a senha deve ter pelo menos 8 caracteres")

func ValidarForcaSenha(senha string) error {
	if utf8.RuneCountInString(senha) < 8 {
		return ErrSenhaFraca
	}
	if utf8.RuneCountInString(senha) > 200 {
		return errors.New("a senha deve ter no máximo 200 caracteres")
	}
	return nil
}

// GerarHashSenha retorna o hash no formato PHC: $argon2id$v=19$m=...,t=...,p=...$sal$hash
func GerarHashSenha(senha string) (string, error) {
	sal := make([]byte, argonTamanhoSal)
	if _, err := rand.Read(sal); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(senha), sal, argonIteracoes, argonMemoria, argonParalelo, argonTamanhoHash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoria, argonIteracoes, argonParalelo,
		base64.RawStdEncoding.EncodeToString(sal), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerificarSenha(senha, hashArmazenado string) bool {
	partes := strings.Split(hashArmazenado, "$")
	if len(partes) != 6 || partes[1] != "argon2id" {
		return false
	}
	var memoria uint32
	var iteracoes uint32
	var paralelo uint8
	if _, err := fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &memoria, &iteracoes, &paralelo); err != nil {
		return false
	}
	sal, err := base64.RawStdEncoding.DecodeString(partes[4])
	if err != nil {
		return false
	}
	esperado, err := base64.RawStdEncoding.DecodeString(partes[5])
	if err != nil {
		return false
	}
	calculado := argon2.IDKey([]byte(senha), sal, iteracoes, memoria, paralelo, uint32(len(esperado)))
	return subtle.ConstantTimeCompare(calculado, esperado) == 1
}

// GerarToken cria um token aleatório de 32 bytes (URL-safe) e seu hash SHA-256.
// Somente o hash é armazenado no banco.
func GerarToken() (token string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

func HashToken(token string) string {
	soma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(soma[:])
}

// NomeAleatorio gera um identificador aleatório usado como nome interno de arquivos.
func NomeAleatorio() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
