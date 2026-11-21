// Comando servidor: API REST + trabalhador da fila de documentos (monólito modular).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"wsi/internal/api"
	"wsi/internal/armazenamento"
	"wsi/internal/banco"
	"wsi/internal/configuracao"
	"wsi/internal/documentos"
	"wsi/internal/fila"
	"wsi/internal/latex"
	"wsi/internal/semente"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := executar(); err != nil {
		slog.Error("falha fatal", "erro", err)
		os.Exit(1)
	}
}

func executar() error {
	cfg, err := configuracao.Carregar()
	if err != nil {
		return err
	}
	ctx, cancelar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelar()

	pool, err := banco.Conectar(ctx, cfg.URLBanco)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := banco.Migrar(ctx, pool); err != nil {
		return err
	}
	if err := semente.Aplicar(ctx, pool, cfg.EmailAdminInicial); err != nil {
		return err
	}

	arm, err := armazenamento.Novo(cfg.DiretorioDados)
	if err != nil {
		return err
	}
	dirTemp, err := arm.DiretorioTemporario("latex")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dirTemp)

	f := fila.Nova(pool, cfg.ConcorrenciaFila)
	servico := &documentos.Servico{
		Pool:          pool,
		Armazenamento: arm,
		Compilador: &latex.Compilador{
			DiretorioTemplates: cfg.DiretorioTemplates,
			DiretorioTemp:      dirTemp,
			TempoLimite:        time.Duration(cfg.TempoLimiteLatexSegs) * time.Second,
		},
	}
	servico.RegistrarEm(f)
	ctxFila, pararFila := context.WithCancel(context.Background())
	if err := f.Iniciar(ctxFila); err != nil {
		pararFila()
		return err
	}

	openAPI := api.ArquivoOpenAPI(filepath.Join(filepath.Dir(cfg.DiretorioTemplates), "..", "openapi.yaml"))
	srv := &http.Server{
		Addr:              cfg.EnderecoHTTP,
		Handler:           api.Novo(cfg, pool, arm, f, openAPI).Rotas(),
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       5 * time.Minute, // uploads de até 30 MB em conexões lentas
		WriteTimeout:      10 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	erros := make(chan error, 1)
	go func() {
		slog.Info("servidor iniciado", "endereco", cfg.EnderecoHTTP, "concorrencia_fila", cfg.ConcorrenciaFila)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			erros <- err
		}
	}()
	select {
	case <-ctx.Done():
	case err := <-erros:
		pararFila()
		return err
	}
	slog.Info("encerrando...")
	desligar, cancelarDesligamento := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelarDesligamento()
	srv.Shutdown(desligar)
	pararFila()
	f.Aguardar()
	return nil
}
