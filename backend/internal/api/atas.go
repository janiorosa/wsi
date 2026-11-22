package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"wsi/internal/fila"
	"wsi/internal/latex"
	"wsi/internal/notas"
	"wsi/internal/regras"
)

// iniciarGeracaoATA valida as pré-condições, congela nota final e ordem dos signatários
// (aluno, orientador, examinadores) e coloca a geração na fila. Cada ATA tem fluxo próprio.
func iniciarGeracaoATA(ctx context.Context, tx pgx.Tx, trabalhoID, atorID string) (ataID, tarefaID string, err error) {
	var alunoID, titulo, matricula string
	var orientadorID, eventoID *string
	err = tx.QueryRow(ctx, `SELECT t.aluno_id::text, t.orientador_id::text, t.evento_id::text, t.titulo, COALESCE(pa.matricula, '')
		FROM trabalhos t LEFT JOIN perfis_aluno pa ON pa.usuario_id = t.aluno_id WHERE t.id = $1 FOR UPDATE OF t`, trabalhoID).
		Scan(&alunoID, &orientadorID, &eventoID, &titulo, &matricula)
	if err != nil {
		return "", "", err
	}
	switch {
	case eventoID == nil:
		return "", "", erroConflito("Associe o trabalho a um evento antes de gerar a ATA: data, horário e local vêm do evento.")
	case orientadorID == nil:
		return "", "", erroConflito("O trabalho não possui orientador.")
	case vazio(titulo):
		return "", "", erroConflito("O trabalho não possui título.")
	case vazio(matricula):
		return "", "", erroConflito("O aluno ainda não informou a matrícula.")
	}
	var existe bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, trabalhoID).Scan(&existe); err != nil {
		return "", "", err
	}
	if existe {
		return "", "", erroConflito("Já existe uma ATA ativa para este trabalho.")
	}

	type membro struct {
		docenteID, papel, estado string
		nota                     *string
	}
	rows, err := tx.Query(ctx, `SELECT mb.docente_id::text, mb.papel, COALESCE(a.estado, 'pendente'), a.nota_calculada::text
		FROM membros_banca mb LEFT JOIN avaliacoes a ON a.trabalho_id = mb.trabalho_id AND a.avaliador_id = mb.docente_id
		WHERE mb.trabalho_id = $1 AND mb.situacao = 'ativo' AND mb.obrigatorio ORDER BY mb.ordem`, trabalhoID)
	if err != nil {
		return "", "", err
	}
	var membros []membro
	for rows.Next() {
		var m membro
		if err = rows.Scan(&m.docenteID, &m.papel, &m.estado, &m.nota); err != nil {
			rows.Close()
			return "", "", err
		}
		membros = append(membros, m)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return "", "", err
	}
	var notasAvaliadores []notas.Centesimos
	var examinadores []string
	temOrientador := false
	for _, m := range membros {
		if m.estado != "finalizada" || m.nota == nil {
			return "", "", erroConflito("Todas as avaliações obrigatórias devem estar concluídas antes da geração da ATA.")
		}
		c, errNota := notas.DeTextoNumeric(*m.nota)
		if errNota != nil {
			return "", "", errNota
		}
		notasAvaliadores = append(notasAvaliadores, c)
		switch m.papel {
		case "orientador":
			temOrientador = m.docenteID == *orientadorID
		case "examinador":
			examinadores = append(examinadores, m.docenteID)
		}
	}
	if !temOrientador || len(examinadores) == 0 {
		return "", "", erroConflito("A banca deve ter o orientador e pelo menos um examinador.")
	}
	notaFinal, err := notas.NotaTrabalho(notasAvaliadores)
	if err != nil {
		return "", "", err
	}

	if err = tx.QueryRow(ctx, `INSERT INTO atas (trabalho_id, versao, estado, nota_final, versao_regra, gerado_por)
		VALUES ($1, (SELECT COALESCE(max(versao), 0) + 1 FROM atas WHERE trabalho_id = $1), 'gerando', $2, $3, $4) RETURNING id::text`,
		trabalhoID, notaFinal.Decimal(), notas.VersaoRegra, atorID).Scan(&ataID); err != nil {
		return "", "", err
	}
	// Ordem congelada: 1 aluno, 2 orientador, 3.. examinadores na ordem da banca.
	inserir := func(signatario, tipo, rotulo string, ordem int) error {
		_, err := tx.Exec(ctx, `INSERT INTO assinaturas_ata (ata_id, signatario_id, tipo, rotulo, ordem) VALUES ($1, $2, $3, $4, $5)`,
			ataID, signatario, tipo, rotulo, ordem)
		return err
	}
	if err = inserir(alunoID, "aluno", latex.RotuloAluno, 1); err != nil {
		return "", "", err
	}
	if err = inserir(*orientadorID, "orientador", latex.RotuloOrientador, 2); err != nil {
		return "", "", err
	}
	for i, ex := range examinadores {
		if err = inserir(ex, "examinador", latex.RotuloExaminador(i+1), i+3); err != nil {
			return "", "", err
		}
	}
	if tarefaID, err = fila.Enfileirar(ctx, tx, fila.TipoATA, ataID, nil, atorID); err != nil {
		return "", "", err
	}
	if _, err = regras.RecalcularEstadoTrabalho(ctx, tx, trabalhoID, atorID, "ATA aguardando geração"); err != nil {
		return "", "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO registros_auditoria (ator_id, acao, entidade, entidade_id, dados)
		VALUES ($1, 'iniciar_geracao_ata', 'trabalho', $2, jsonb_build_object('ata', $3::text, 'nota_final', $4::text))`,
		atorID, trabalhoID, ataID, notaFinal.Decimal())
	return ataID, tarefaID, err
}

// POST /docente/trabalhos/{id}/ata — gerar (ou regenerar o original enquanto ninguém assinou).
func (s *Servidor) docenteGerarATA(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if _, err := s.exigirOrientador(r, id); err != nil {
		return err
	}
	ses := sessaoDe(r)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT id FROM trabalhos WHERE id = $1 FOR UPDATE`, id); err != nil {
		return err
	}
	var ataID, estado string
	var posicao int
	err = tx.QueryRow(ctx, `SELECT id::text, estado, posicao_atual FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada' FOR UPDATE`, id).
		Scan(&ataID, &estado, &posicao)
	var tarefaID string
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		ataID, tarefaID, err = iniciarGeracaoATA(ctx, tx, id, ses.UsuarioID)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	case estado == "gerando":
		// Nova tentativa (ex.: após falha); a fila ignora duplicatas ativas.
		if _, err := tx.Exec(ctx, `UPDATE atas SET erro_geracao = '' WHERE id = $1`, ataID); err != nil {
			return err
		}
		if tarefaID, err = fila.Enfileirar(ctx, tx, fila.TipoATA, ataID, nil, ses.UsuarioID); err != nil {
			return err
		}
	case estado == "aguardando_assinaturas" && posicao == 1:
		// Ninguém assinou ainda: regenera o original com os dados atuais do evento.
		if _, err := tx.Exec(ctx, `UPDATE atas SET estado = 'gerando', erro_geracao = '' WHERE id = $1`, ataID); err != nil {
			return err
		}
		if tarefaID, err = fila.Enfileirar(ctx, tx, fila.TipoATA, ataID, nil, ses.UsuarioID); err != nil {
			return err
		}
		if _, err := regras.RecalcularEstadoTrabalho(ctx, tx, id, ses.UsuarioID, "ATA original regenerada"); err != nil {
			return err
		}
	default:
		return erroConflito("O fluxo de assinaturas já foi iniciado. Para corrigir a ATA, use “Reiniciar processo da ATA”.")
	}
	if err := s.auditar(ctx, tx, r, "gerar_ata", "trabalho", id, map[string]any{"ata": ataID, "tarefa": tarefaID}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.fila.Notificar()
	responderJSON(w, http.StatusAccepted, map[string]string{"ataId": ataID, "tarefaId": tarefaID})
	return nil
}

// POST /docente/trabalhos/{id}/ata/reiniciar {motivo, gerarNova} — operação explícita, autorizada e auditada.
func (s *Servidor) docenteReiniciarATA(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if _, err := s.exigirOrientador(r, id); err != nil {
		return err
	}
	var req struct {
		Motivo    string `json:"motivo"`
		GerarNova bool   `json:"gerarNova"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if len(strings.TrimSpace(req.Motivo)) < 5 {
		return erroCampo("motivo", "Informe o motivo do reinício.")
	}
	ses := sessaoDe(r)
	ctx := r.Context()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var ataID, estado string
	var chaveAssinado, chaveOriginal *string
	err = tx.QueryRow(ctx, `UPDATE atas SET estado = 'cancelada', motivo_cancelamento = $2
		WHERE trabalho_id = $1 AND estado <> 'cancelada'
		RETURNING id::text, (SELECT estado FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada'), chave_assinado, chave_original`,
		id, strings.TrimSpace(req.Motivo)).Scan(&ataID, &estado, &chaveAssinado, &chaveOriginal)
	if errors.Is(err, pgx.ErrNoRows) {
		return erroNaoEncontrado("Não há ATA ativa para reiniciar.")
	}
	if err != nil {
		return err
	}
	// Tarefas pendentes dessa ATA deixam de ser processadas.
	if _, err := tx.Exec(ctx, `UPDATE tarefas_documentos SET estado = 'falhou', erro = 'ATA cancelada', concluido_em = now()
		WHERE tipo = 'gerar_ata' AND entidade_id = $1 AND estado = 'pendente'`, ataID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE atas SET chave_assinado = NULL WHERE id = $1`, ataID); err != nil {
		return err
	}
	resposta := map[string]string{"ataCancelada": ataID}
	if _, err := regras.RecalcularEstadoTrabalho(ctx, tx, id, ses.UsuarioID, "Processo da ATA reiniciado: "+req.Motivo); err != nil {
		return err
	}
	if req.GerarNova {
		novaID, tarefaID, err := iniciarGeracaoATA(ctx, tx, id, ses.UsuarioID)
		if err != nil {
			return err
		}
		resposta["ataId"], resposta["tarefaId"] = novaID, tarefaID
	}
	if err := s.auditar(ctx, tx, r, "reiniciar_ata", "trabalho", id,
		map[string]any{"ata": ataID, "estadoAnterior": estado, "motivo": req.Motivo, "gerarNova": req.GerarNova}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	// Versões assinadas não são mantidas; o original da ATA cancelada também é descartado.
	if chaveAssinado != nil {
		s.arm.Remover(*chaveAssinado)
	}
	if chaveOriginal != nil {
		s.arm.Remover(*chaveOriginal)
	}
	s.fila.Notificar()
	return responderOK(w, resposta)
}

// ---------------------------------------------------------------------
// ATAs para assinar (DOCENTE): uma linha por processo independente.
// ---------------------------------------------------------------------

type itemATADocente struct {
	ATAID        string `json:"ataId"`
	TrabalhoID   string `json:"trabalhoId"`
	Titulo       string `json:"titulo"`
	Aluno        string `json:"aluno"`
	Evento       string `json:"evento"`
	Estado       string `json:"estado"`
	PosicaoAtual int    `json:"posicaoAtual"`
	MinhaOrdem   int    `json:"minhaOrdem"`
	MeuRotulo    string `json:"meuRotulo"`
	JaAssinei    bool   `json:"jaAssinei"`
	MinhaVez     bool   `json:"minhaVez"`
	Aguardando   string `json:"aguardando"`
	Total        int    `json:"totalSignatarios"`
}

func (s *Servidor) consultarATAsDocente(ctx context.Context, docenteID string) ([]itemATADocente, error) {
	rows, err := s.pool.Query(ctx, `SELECT a.id::text, t.id::text, t.titulo, al.nome_completo, e.nome, a.estado, a.posicao_atual,
		aa.ordem, aa.rotulo, aa.estado = 'assinada',
		COALESCE((SELECT u.nome_completo || ' (' || x.rotulo || ')' FROM assinaturas_ata x JOIN usuarios u ON u.id = x.signatario_id
		          WHERE x.ata_id = a.id AND x.ordem = a.posicao_atual), ''),
		(SELECT count(*) FROM assinaturas_ata WHERE ata_id = a.id)
		FROM assinaturas_ata aa JOIN atas a ON a.id = aa.ata_id JOIN trabalhos t ON t.id = a.trabalho_id
		JOIN usuarios al ON al.id = t.aluno_id JOIN eventos e ON e.id = t.evento_id
		WHERE aa.signatario_id = $1 AND a.estado IN ('aguardando_assinaturas', 'finalizada')
		ORDER BY (a.estado = 'aguardando_assinaturas' AND a.posicao_atual = aa.ordem) DESC, a.estado, a.criado_em DESC`, docenteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []itemATADocente{}
	for rows.Next() {
		var i itemATADocente
		if err := rows.Scan(&i.ATAID, &i.TrabalhoID, &i.Titulo, &i.Aluno, &i.Evento, &i.Estado, &i.PosicaoAtual, &i.MinhaOrdem, &i.MeuRotulo,
			&i.JaAssinei, &i.Aguardando, &i.Total); err != nil {
			return nil, err
		}
		i.MinhaVez = i.Estado == "aguardando_assinaturas" && i.PosicaoAtual == i.MinhaOrdem
		if i.Estado == "finalizada" {
			i.Aguardando = ""
		}
		itens = append(itens, i)
	}
	return itens, rows.Err()
}

func (s *Servidor) docenteListarATAs(w http.ResponseWriter, r *http.Request) error {
	itens, err := s.consultarATAsDocente(r.Context(), sessaoDe(r).UsuarioID)
	if err != nil {
		return err
	}
	return responderOK(w, itens)
}

// ---------------------------------------------------------------------
// Acesso à ATA: somente signatários (aluno avaliado e docentes da banca).
// ADMIN e APRESENTADOR não têm acesso por esses perfis.
// ---------------------------------------------------------------------

type dadosATAAcesso struct {
	ID, TrabalhoID, Estado, AlunoNome string
	Posicao, MinhaOrdem, Total         int
	ChaveOriginal, ChaveAssinado       *string
}

func (s *Servidor) carregarATAParaSignatario(ctx context.Context, ses *Sessao, ataID string) (dadosATAAcesso, error) {
	var d dadosATAAcesso
	if ses.Contexto != PerfilAluno && ses.Contexto != PerfilDocente {
		return d, erroNaoEncontrado("ATA não encontrada.")
	}
	err := s.pool.QueryRow(ctx, `SELECT a.id::text, a.trabalho_id::text, a.estado, u.nome_completo, a.posicao_atual,
		COALESCE((SELECT ordem FROM assinaturas_ata WHERE ata_id = a.id AND signatario_id = $2), 0),
		(SELECT count(*) FROM assinaturas_ata WHERE ata_id = a.id), a.chave_original, a.chave_assinado
		FROM atas a JOIN trabalhos t ON t.id = a.trabalho_id JOIN usuarios u ON u.id = t.aluno_id
		WHERE a.id = $1 AND a.estado <> 'cancelada'`, ataID, ses.UsuarioID).
		Scan(&d.ID, &d.TrabalhoID, &d.Estado, &d.AlunoNome, &d.Posicao, &d.MinhaOrdem, &d.Total, &d.ChaveOriginal, &d.ChaveAssinado)
	if err != nil || d.MinhaOrdem == 0 {
		return d, erroNaoEncontrado("ATA não encontrada.")
	}
	// O aluno assina somente como aluno (ordem 1); docentes, nas demais posições.
	if (ses.Contexto == PerfilAluno) != (d.MinhaOrdem == 1) {
		return d, erroNaoEncontrado("ATA não encontrada.")
	}
	return d, nil
}

func (s *Servidor) obterATA(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	d, err := s.carregarATAParaSignatario(r.Context(), ses, chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	resumo, err := resumoATA(r.Context(), s.pool, d.TrabalhoID, ses.UsuarioID)
	if err != nil {
		return err
	}
	return responderOK(w, resumo)
}

// GET /atas/{id}/documento — durante o fluxo, somente o signatário da vez baixa a versão atual
// (original para o aluno; última assinada para os demais). Finalizada: todos os participantes.
func (s *Servidor) baixarDocumentoATA(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	d, err := s.carregarATAParaSignatario(r.Context(), ses, chi.URLParam(r, "id"))
	if err != nil {
		return err
	}
	var chave *string
	nome := nomeDownload("ATA_TCC", d.AlunoNome)
	versao := r.URL.Query().Get("versao")
	switch {
	// Docentes da banca (ordem >= 2): original em branco ou versão atual assinada, a qualquer momento.
	case versao == "original":
		if d.MinhaOrdem < 2 || d.Estado == "gerando" {
			return erroNaoEncontrado("Documento da ATA indisponível.")
		}
		chave = d.ChaveOriginal
		nome = nomeDownload("ATA_TCC", d.AlunoNome, "original")
	case versao == "assinada":
		if d.MinhaOrdem < 2 {
			return erroNaoEncontrado("Documento da ATA indisponível.")
		}
		chave = d.ChaveAssinado
		assinadas := d.Posicao - 1
		if d.Estado == "finalizada" {
			assinadas = d.Total
		}
		nome = nomeDownload("ATA_TCC", d.AlunoNome, "assinaturas", strconv.Itoa(assinadas)+"_de_"+strconv.Itoa(d.Total))
	case d.Estado == "finalizada":
		chave = d.ChaveAssinado
		nome = nomeDownload("ATA_TCC", d.AlunoNome, "final")
	case d.Estado == "aguardando_assinaturas" && d.Posicao == d.MinhaOrdem:
		if d.Posicao == 1 {
			chave = d.ChaveOriginal
		} else {
			chave = d.ChaveAssinado
			nome = nomeDownload("ATA_TCC", d.AlunoNome, "assinaturas", strconv.Itoa(d.Posicao-1)+"_de_"+strconv.Itoa(d.Total))
		}
	case d.Estado == "aguardando_assinaturas":
		return erroProibido("Ainda não é a sua vez de assinar esta ATA.")
	default:
		return erroConflito("A ATA ainda está sendo preparada.")
	}
	if chave == nil {
		return erroNaoEncontrado("Documento da ATA indisponível.")
	}
	s.auditar(r.Context(), s.pool, r, "baixar_ata", "ata", d.ID, map[string]any{"estado": d.Estado, "posicao": d.Posicao, "versao": versao})
	return s.enviarArquivo(w, r, *chave, nome, "application/pdf", false)
}

// POST /atas/{id}/assinatura (multipart: arquivo, posicao) — avança exatamente uma posição.
func (s *Servidor) enviarAssinaturaATA(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	ataID := chi.URLParam(r, "id")
	if _, err := s.carregarATAParaSignatario(ctx, ses, ataID); err != nil {
		return err
	}
	rec, err := s.receberPDF(w, r)
	if err != nil {
		return err
	}
	defer rec.Limpar()
	posicaoEnviada, err := strconv.Atoi(r.FormValue("posicao"))
	if err != nil {
		return erroCampo("posicao", "Posição do fluxo não informada. Recarregue a página.")
	}
	salvo, err := s.arm.SalvarDeArquivo("atas-assinadas", rec.Caminho)
	if err != nil {
		return err
	}
	var chaveAnterior *string
	var finalizada bool
	err = func() error {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(context.Background())
		// Bloqueio da linha da ATA: requisições concorrentes ou duplo clique são serializados
		// e somente a primeira avança a posição.
		var trabalhoID, estado string
		var posicao, total int
		if err := tx.QueryRow(ctx, `SELECT trabalho_id::text, estado, posicao_atual, chave_assinado FROM atas WHERE id = $1 FOR UPDATE`, ataID).
			Scan(&trabalhoID, &estado, &posicao, &chaveAnterior); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM assinaturas_ata WHERE ata_id = $1`, ataID).Scan(&total); err != nil {
			return err
		}
		if estado != "aguardando_assinaturas" {
			return erroConflito("Esta ATA não está aguardando assinaturas.")
		}
		if posicao != posicaoEnviada {
			return erroConflito("O fluxo já avançou. Recarregue a página para ver a situação atual.")
		}
		var signatario, assinaturaID string
		if err := tx.QueryRow(ctx, `SELECT signatario_id::text, id::text FROM assinaturas_ata WHERE ata_id = $1 AND ordem = $2`, ataID, posicao).
			Scan(&signatario, &assinaturaID); err != nil {
			return err
		}
		if signatario != ses.UsuarioID {
			return erroProibido("Ainda não é a sua vez de assinar esta ATA.")
		}
		if _, err := tx.Exec(ctx, `UPDATE assinaturas_ata SET estado = 'assinada', enviado_em = now(), sha256 = $2 WHERE id = $1`,
			assinaturaID, salvo.SHA256); err != nil {
			return err
		}
		finalizada = posicao >= total
		if _, err := tx.Exec(ctx, `UPDATE atas SET chave_assinado = $2, sha256_assinado = $3, posicao_atual = posicao_atual + 1,
			estado = CASE WHEN $4 THEN 'finalizada' ELSE estado END, finalizada_em = CASE WHEN $4 THEN now() ELSE NULL END
			WHERE id = $1`, ataID, salvo.Chave, salvo.SHA256, finalizada); err != nil {
			return err
		}
		if _, err := regras.RecalcularEstadoTrabalho(ctx, tx, trabalhoID, ses.UsuarioID, "ATA assinada (posição "+strconv.Itoa(posicao)+")"); err != nil {
			return err
		}
		if err := s.auditar(ctx, tx, r, "assinar_ata", "ata", ataID,
			map[string]any{"posicao": posicao, "sha256": salvo.SHA256, "nomeOriginal": rec.NomeOriginal, "finalizada": finalizada}); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}()
	if err != nil {
		s.arm.Remover(salvo.Chave)
		return err
	}
	// Regra simplificada: somente a última versão assinada é mantida.
	if chaveAnterior != nil && *chaveAnterior != salvo.Chave {
		s.arm.Remover(*chaveAnterior)
	}
	return responderOK(w, map[string]any{"ok": true, "finalizada": finalizada})
}
