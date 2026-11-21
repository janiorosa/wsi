package seguranca

import (
	"strings"
	"testing"
)

func TestHashSenhaNaoArmazenaTextoPuro(t *testing.T) {
	hash, err := GerarHashSenha("senha-secreta-123")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hash, "senha-secreta-123") {
		t.Fatal("hash contém a senha em texto puro")
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash não é Argon2id: %s", hash)
	}
	if !VerificarSenha("senha-secreta-123", hash) {
		t.Fatal("senha correta não foi aceita")
	}
	if VerificarSenha("senha-errada", hash) {
		t.Fatal("senha errada foi aceita")
	}
}

func TestHashesDiferentesParaMesmaSenha(t *testing.T) {
	a, _ := GerarHashSenha("mesma-senha")
	b, _ := GerarHashSenha("mesma-senha")
	if a == b {
		t.Fatal("hashes deveriam usar sais diferentes")
	}
}

func TestForcaSenha(t *testing.T) {
	if ValidarForcaSenha("curta") == nil {
		t.Fatal("senha curta deveria ser rejeitada")
	}
	if ValidarForcaSenha("suficiente") != nil {
		t.Fatal("senha de 10 caracteres deveria ser aceita")
	}
}

func TestTokenArmazenaSomenteHash(t *testing.T) {
	token, hash, err := GerarToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == hash || HashToken(token) != hash {
		t.Fatal("hash do token inconsistente")
	}
}
