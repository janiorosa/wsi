// Pacote armazenamento guarda arquivos privados no volume persistente.
// Os caminhos físicos nunca são enviados ao frontend: o banco guarda apenas
// uma chave relativa ("categoria/ab/abcdef....pdf").
package armazenamento

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"wsi/internal/seguranca"
)

type Armazenamento struct {
	raiz string
}

func Novo(raiz string) (*Armazenamento, error) {
	dir := filepath.Join(raiz, "arquivos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(raiz, "temporarios"), 0o750); err != nil {
		return nil, err
	}
	return &Armazenamento{raiz: raiz}, nil
}

// DiretorioTemporario cria um diretório isolado para uploads e compilações.
func (a *Armazenamento) DiretorioTemporario(prefixo string) (string, error) {
	return os.MkdirTemp(filepath.Join(a.raiz, "temporarios"), prefixo+"-")
}

// Caminho converte uma chave em caminho físico, recusando chaves que escapem da raiz.
func (a *Armazenamento) Caminho(chave string) (string, error) {
	if chave == "" || strings.Contains(chave, "..") || filepath.IsAbs(chave) {
		return "", errors.New("chave de armazenamento inválida")
	}
	return filepath.Join(a.raiz, "arquivos", filepath.FromSlash(chave)), nil
}

type ArquivoSalvo struct {
	Chave   string
	SHA256  string
	Tamanho int64
}

// SalvarDeArquivo copia um arquivo local para o armazenamento com nome aleatório.
func (a *Armazenamento) SalvarDeArquivo(categoria, origem string) (ArquivoSalvo, error) {
	f, err := os.Open(origem)
	if err != nil {
		return ArquivoSalvo{}, err
	}
	defer f.Close()
	return a.Salvar(categoria, f)
}

// Salvar grava o conteúdo com nome interno aleatório e calcula SHA-256.
func (a *Armazenamento) Salvar(categoria string, r io.Reader) (ArquivoSalvo, error) {
	nome, err := seguranca.NomeAleatorio()
	if err != nil {
		return ArquivoSalvo{}, err
	}
	chave := fmt.Sprintf("%s/%s/%s.pdf", categoria, nome[:2], nome)
	destino, err := a.Caminho(chave)
	if err != nil {
		return ArquivoSalvo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o750); err != nil {
		return ArquivoSalvo{}, err
	}
	tmp := destino + ".parcial"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return ArquivoSalvo{}, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), r)
	if errSync := f.Sync(); err == nil {
		err = errSync
	}
	if errClose := f.Close(); err == nil {
		err = errClose
	}
	if err != nil {
		os.Remove(tmp)
		return ArquivoSalvo{}, err
	}
	if err := os.Rename(tmp, destino); err != nil {
		os.Remove(tmp)
		return ArquivoSalvo{}, err
	}
	return ArquivoSalvo{Chave: chave, SHA256: hex.EncodeToString(h.Sum(nil)), Tamanho: n}, nil
}

func (a *Armazenamento) Abrir(chave string) (*os.File, error) {
	c, err := a.Caminho(chave)
	if err != nil {
		return nil, err
	}
	return os.Open(c)
}

// Remover apaga um arquivo (usado apenas para a versão assinada anterior da ATA,
// conforme a regra simplificada de versões, e para arquivos órfãos).
func (a *Armazenamento) Remover(chave string) error {
	c, err := a.Caminho(chave)
	if err != nil {
		return err
	}
	if err := os.Remove(c); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
