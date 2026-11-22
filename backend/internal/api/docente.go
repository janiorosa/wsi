package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"wsi/internal/notas"
	"wsi/internal/regras"
)

// ---------------------------------------------------------------------
// Painel: avaliação e orientação coexistem, sem seleção de "modo".
// ---------------------------------------------------------------------

func (s *Servidor) docentePainel(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	avaliacoes, err := s.consultarAvaliacoesDocente(ctx, ses.UsuarioID)
	if err != nil {
		return err
	}
	pendentes, concluidas := []itemAvaliacao{}, 0
	for _, a := range avaliacoes {
		if a.EstadoAvaliacao == "finalizada" {
			concluidas++
		} else {
			pendentes = append(pendentes, a)
		}
	}
	atas, err := s.consultarATAsDocente(ctx, ses.UsuarioID)
	if err != nil {
		return err
	}
	minhaVez, aguardando := []itemATADocente{}, 0
	for _, a := range atas {
		if a.MinhaVez {
			minhaVez = append(minhaVez, a)
		} else if a.Estado == "aguardando_assinaturas" {
			aguardando++
		}
	}
	orientandos, err := s.consultarOrientandos(ctx, ses.UsuarioID, "")
	if err != nil {
		return err
	}
	var eventosApresentador int
	var criteriosAtivos int
	if err := s.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM apresentadores_evento WHERE docente_id = $1),
		(SELECT count(*) FROM criterios_avaliacao WHERE ativo)`, ses.UsuarioID).Scan(&eventosApresentador, &criteriosAtivos); err != nil {
		return err
	}
	return responderOK(w, map[string]any{
		"avaliacoesPendentes":    pendentes,
		"avaliacoesConcluidas":   concluidas,
		"atasMinhaVez":           minhaVez,
		"atasAguardandoOutros":   aguardando,
		"orientandos":            orientandos,
		"eventosComoApresentador": eventosApresentador,
		"criteriosConfigurados":  criteriosAtivos > 0,
	})
}

// ---------------------------------------------------------------------
// Orientandos
// ---------------------------------------------------------------------

type itemOrientando struct {
	VinculoID          string   `json:"vinculoId"`
	AlunoID            string   `json:"alunoId"`
	Nome               string   `json:"nome"`
	Email              string   `json:"email"`
	Matricula          string   `json:"matricula"`
	Desde              string   `json:"desde"`
	TrabalhoID         *string  `json:"trabalhoId"`
	Titulo             *string  `json:"titulo"`
	EstadoTrabalho     *string  `json:"estadoTrabalho"`
	Evento             *string  `json:"evento"`
	TemArtigo          bool     `json:"temArtigo"`
	TemApresentacao    bool     `json:"temApresentacao"`
	DeclaracaoAssinada bool     `json:"declaracaoAssinada"`
	Pendencias         []string `json:"pendencias"`
}

func (s *Servidor) consultarOrientandos(ctx context.Context, docenteID, busca string) ([]itemOrientando, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT v.id::text, a.id::text, a.nome_completo, a.email, COALESCE(pa.matricula, ''), to_char(v.inicio AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY'),
		       t.id::text, t.titulo, t.estado, e.nome,
		       COALESCE(EXISTS(SELECT 1 FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'artigo'), false),
		       COALESCE(EXISTS(SELECT 1 FROM arquivos_trabalho WHERE trabalho_id = t.id AND tipo = 'apresentacao'), false),
		       EXISTS(SELECT 1 FROM declaracoes_assinadas WHERE aluno_id = a.id),
		       COALESCE((SELECT count(*) FROM membros_banca WHERE trabalho_id = t.id AND situacao = 'ativo' AND papel = 'examinador'), 0)
		FROM vinculos_orientacao v
		JOIN usuarios a ON a.id = v.aluno_id
		LEFT JOIN perfis_aluno pa ON pa.usuario_id = a.id
		LEFT JOIN LATERAL (SELECT * FROM trabalhos WHERE aluno_id = a.id AND estado <> 'cancelado' ORDER BY criado_em DESC LIMIT 1) t ON true
		LEFT JOIN eventos e ON e.id = t.evento_id
		WHERE v.docente_id = $1 AND v.situacao = 'ativo'
		  AND ($2 = '' OR lower(a.nome_completo) LIKE '%' || lower($2) || '%' OR pa.matricula LIKE '%' || $2 || '%')
		ORDER BY a.nome_completo`, docenteID, strings.TrimSpace(busca))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	itens := []itemOrientando{}
	for rows.Next() {
		var o itemOrientando
		var examinadores int
		if err := rows.Scan(&o.VinculoID, &o.AlunoID, &o.Nome, &o.Email, &o.Matricula, &o.Desde, &o.TrabalhoID, &o.Titulo, &o.EstadoTrabalho,
			&o.Evento, &o.TemArtigo, &o.TemApresentacao, &o.DeclaracaoAssinada, &examinadores); err != nil {
			return nil, err
		}
		o.Pendencias = []string{}
		if o.TrabalhoID == nil || o.Titulo == nil || vazio(*o.Titulo) {
			o.Pendencias = append(o.Pendencias, "Título/resumo não cadastrados")
		}
		if !o.TemArtigo {
			o.Pendencias = append(o.Pendencias, "Artigo não enviado")
		}
		if !o.TemApresentacao {
			o.Pendencias = append(o.Pendencias, "Apresentação não enviada")
		}
		if o.TrabalhoID != nil && o.Evento == nil {
			o.Pendencias = append(o.Pendencias, "Sem evento associado")
		}
		if o.TrabalhoID != nil && examinadores == 0 {
			o.Pendencias = append(o.Pendencias, "Banca não designada")
		}
		if !o.DeclaracaoAssinada {
			o.Pendencias = append(o.Pendencias, "Declaração anti-plágio assinada pendente")
		}
		itens = append(itens, o)
	}
	return itens, rows.Err()
}

func (s *Servidor) docenteListarOrientandos(w http.ResponseWriter, r *http.Request) error {
	itens, err := s.consultarOrientandos(r.Context(), sessaoDe(r).UsuarioID, r.URL.Query().Get("busca"))
	if err != nil {
		return err
	}
	return responderOK(w, itens)
}

// GET /docente/alunos-disponiveis?busca= — alunos sem orientador ativo.
func (s *Servidor) docenteAlunosDisponiveis(w http.ResponseWriter, r *http.Request) error {
	busca := strings.TrimSpace(r.URL.Query().Get("busca"))
	rows, err := s.pool.Query(r.Context(), `SELECT u.id::text, u.nome_completo, u.email, COALESCE(pa.matricula, '')
		FROM usuarios u JOIN perfis_usuario pu ON pu.usuario_id = u.id AND pu.perfil = 'ALUNO'
		LEFT JOIN perfis_aluno pa ON pa.usuario_id = u.id
		WHERE NOT EXISTS (SELECT 1 FROM vinculos_orientacao v WHERE v.aluno_id = u.id AND v.situacao = 'ativo')
		  AND u.situacao <> 'bloqueado'
		  AND ($1 = '' OR lower(u.nome_completo) LIKE '%' || lower($1) || '%' OR pa.matricula LIKE '%' || $1 || '%' OR u.email LIKE '%' || lower($1) || '%')
		ORDER BY u.nome_completo LIMIT 30`, busca)
	if err != nil {
		return err
	}
	defer rows.Close()
	type aluno struct {
		ID        string `json:"id"`
		Nome      string `json:"nome"`
		Email     string `json:"email"`
		Matricula string `json:"matricula"`
	}
	itens := []aluno{}
	for rows.Next() {
		var a aluno
		if err := rows.Scan(&a.ID, &a.Nome, &a.Email, &a.Matricula); err != nil {
			return err
		}
		itens = append(itens, a)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return responderOK(w, itens)
}

// POST /docente/orientandos {alunoId} — somente alunos sem orientador.
func (s *Servidor) docenteVincularAluno(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var req struct {
		AlunoID string `json:"alunoId"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	id, err := criarVinculo(r.Context(), tx, req.AlunoID, ses.UsuarioID, ses.UsuarioID, false)
	if err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "docente_vincular_aluno", "vinculo_orientacao", id, map[string]any{"aluno": req.AlunoID}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]string{"id": id})
	return nil
}

// POST /docente/orientandos/novo — cadastro simplificado do aluno + vínculo + código de primeiro acesso.
func (s *Servidor) docenteCadastrarAluno(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	var d dadosCadastro
	if err := lerJSON(r, &d); err != nil {
		return err
	}
	d.normalizar()
	// A matrícula é obrigatória: é com ela que o aluno fará o primeiro acesso.
	if err := d.validar(false, "a matrícula", true); err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	alunoID, err := inserirAluno(r.Context(), tx, d, "", ses.UsuarioID)
	if err != nil {
		return err
	}
	if _, err := criarVinculo(r.Context(), tx, alunoID, ses.UsuarioID, ses.UsuarioID, false); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "docente_cadastrar_aluno", "usuario", alunoID, nil); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]any{"id": alunoID})
	return nil
}

func (s *Servidor) docenteEncerrarVinculo(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	id := chi.URLParam(r, "id")
	var req struct {
		Motivo string `json:"motivo"`
	}
	lerJSON(r, &req)
	var docente string
	if err := s.pool.QueryRow(r.Context(), `SELECT docente_id::text FROM vinculos_orientacao WHERE id = $1`, id).Scan(&docente); err != nil || docente != ses.UsuarioID {
		return erroNaoEncontrado("Vínculo não encontrado.")
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err := encerrarVinculo(r.Context(), tx, id, ses.UsuarioID, strings.TrimSpace(req.Motivo)); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "docente_encerrar_vinculo", "vinculo_orientacao", id, map[string]any{"motivo": req.Motivo}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

func (s *Servidor) docenteEventosFuturos(w http.ResponseWriter, r *http.Request) error {
	eventos, err := listarEventos(r.Context(), s.pool, `WHERE e.estado NOT IN ('rascunho', 'encerrado', 'publicado')
		AND ((e.data_apresentacao + e.horario_apresentacao) > (now() AT TIME ZONE 'America/Sao_Paulo')
		     OR EXISTS (SELECT 1 FROM sessoes_evento se WHERE se.evento_id = e.id AND (se.data + se.horario) > (now() AT TIME ZONE 'America/Sao_Paulo')))
		ORDER BY e.data_apresentacao`)
	if err != nil {
		return err
	}
	resp := make([]map[string]any, 0, len(eventos))
	for _, e := range eventos {
		sessoes, err := listarSessoes(r.Context(), s.pool, e.ID)
		if err != nil {
			return err
		}
		resp = append(resp, map[string]any{"evento": e, "sessoes": sessoes})
	}
	return responderOK(w, resp)
}

// ---------------------------------------------------------------------
// Ficha do trabalho (visão do docente)
// ---------------------------------------------------------------------

type membroBanca struct {
	DocenteID       string  `json:"docenteId"`
	Nome            string  `json:"nome"`
	Papel           string  `json:"papel"`
	Ordem           int     `json:"ordem"`
	EstadoAvaliacao string  `json:"estadoAvaliacao"`
	Nota            *string `json:"nota,omitempty"`
}

func (s *Servidor) docenteFichaTrabalho(w http.ResponseWriter, r *http.Request) error {
	ses := sessaoDe(r)
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	acesso, err := acessoAoTrabalho(ctx, s.pool, ses, id)
	if err != nil {
		return err
	}
	if !acesso.algum() {
		return erroNaoEncontrado("Trabalho não encontrado.")
	}
	resp := map[string]any{"papeis": map[string]bool{"orientador": acesso.EhOrientador, "banca": acesso.EhBanca, "apresentador": acesso.EhApresentador}}
	var titulo, resumo, link, estado, alunoNome, alunoEmail, matricula string
	var submetido, orientador *string
	if err := s.pool.QueryRow(ctx, `SELECT t.titulo, t.resumo, t.link_overleaf, t.estado,
		to_char(t.submetido_em AT TIME ZONE 'America/Sao_Paulo', 'DD/MM/YYYY HH24:MI'), a.nome_completo, a.email, COALESCE(pa.matricula, ''), o.nome_completo
		FROM trabalhos t JOIN usuarios a ON a.id = t.aluno_id LEFT JOIN perfis_aluno pa ON pa.usuario_id = a.id
		LEFT JOIN usuarios o ON o.id = t.orientador_id WHERE t.id = $1`, id).
		Scan(&titulo, &resumo, &link, &estado, &submetido, &alunoNome, &alunoEmail, &matricula, &orientador); err != nil {
		return err
	}
	resp["trabalho"] = map[string]any{"id": id, "titulo": titulo, "resumo": resumo, "linkOverleaf": link, "estado": estado, "submetidoEm": submetido,
		"orientador": orientador}
	resp["aluno"] = map[string]any{"id": acesso.AlunoID, "nome": alunoNome, "email": alunoEmail, "matricula": matricula}
	evento, err := eventoDoTrabalho(ctx, s.pool, id)
	if err != nil {
		return err
	}
	resp["evento"] = evento
	if resp["arquivos"], err = listarArquivos(ctx, s.pool, id); err != nil {
		return err
	}
	if !acesso.EhOrientador && !acesso.EhBanca {
		// Apresentador: somente dados públicos e arquivos (sem notas, banca ou ATA).
		return responderOK(w, resp)
	}

	rows, err := s.pool.Query(ctx, `SELECT mb.docente_id::text, u.nome_completo, mb.papel, mb.ordem, COALESCE(a.estado, 'pendente'), a.nota_calculada::text
		FROM membros_banca mb JOIN usuarios u ON u.id = mb.docente_id
		LEFT JOIN avaliacoes a ON a.trabalho_id = mb.trabalho_id AND a.avaliador_id = mb.docente_id
		WHERE mb.trabalho_id = $1 AND mb.situacao = 'ativo' ORDER BY mb.ordem`, id)
	if err != nil {
		return err
	}
	banca := []membroBanca{}
	todasFinalizadas := true
	var notasAvaliadores []notas.Centesimos
	for rows.Next() {
		var m membroBanca
		if err := rows.Scan(&m.DocenteID, &m.Nome, &m.Papel, &m.Ordem, &m.EstadoAvaliacao, &m.Nota); err != nil {
			rows.Close()
			return err
		}
		if m.EstadoAvaliacao != "finalizada" {
			todasFinalizadas = false
			m.Nota = nil
		} else if m.Nota != nil {
			c, err := notas.DeTextoNumeric(*m.Nota)
			if err == nil {
				notasAvaliadores = append(notasAvaliadores, c)
				f := c.FormatarBR()
				m.Nota = &f
			}
		}
		// Notas individuais: visíveis ao orientador; o examinador vê somente a própria.
		if !acesso.EhOrientador && m.DocenteID != ses.UsuarioID {
			m.Nota = nil
		}
		banca = append(banca, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	resp["banca"] = banca
	criterios, bloqueados, err := listarCriteriosTrabalho(ctx, s.pool, id)
	if err != nil {
		return err
	}
	resp["criterios"] = criterios
	resp["criteriosBloqueados"] = bloqueados
	ata, err := resumoATA(ctx, s.pool, id, ses.UsuarioID)
	if err != nil {
		return err
	}
	resp["ata"] = ata
	if resp["pareceres"], err = listarPareceres(ctx, s.pool, id); err != nil {
		return err
	}

	if acesso.EhOrientador {
		examinadores := 0
		for _, m := range banca {
			if m.Papel == "examinador" {
				examinadores++
			}
		}
		if todasFinalizadas && examinadores > 0 && len(notasAvaliadores) == len(banca) {
			if final, err := notas.NotaTrabalho(notasAvaliadores); err == nil {
				resp["notaFinalPrevia"] = final.FormatarBR()
				resp["aprovadoPrevio"] = notas.Aprovado(final)
			}
		}
		resp["podeGerarATA"] = ata == nil && todasFinalizadas && examinadores > 0 && evento != nil
		dec, err := resumoDeclaracao(ctx, s.pool, acesso.AlunoID, true)
		if err != nil {
			return err
		}
		dec.TarefaAtivaID = nil
		resp["declaracao"] = dec
		if resp["historico"], err = listarHistorico(ctx, s.pool, id); err != nil {
			return err
		}
	}
	return responderOK(w, resp)
}

// exigirOrientador garante que o docente da sessão é o orientador do trabalho.
func (s *Servidor) exigirOrientador(r *http.Request, trabalhoID string) (acessoTrabalho, error) {
	acesso, err := acessoAoTrabalho(r.Context(), s.pool, sessaoDe(r), trabalhoID)
	if err != nil {
		return acesso, err
	}
	if !acesso.EhOrientador {
		if acesso.algum() {
			return acesso, erroProibido("Somente o orientador do trabalho pode realizar esta ação.")
		}
		return acesso, erroNaoEncontrado("Trabalho não encontrado.")
	}
	return acesso, nil
}

// PUT /docente/trabalhos/{id}/evento {eventoId, sessaoId} — somente eventos futuros.
func (s *Servidor) docenteAssociarEvento(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if _, err := s.exigirOrientador(r, id); err != nil {
		return err
	}
	var req struct {
		EventoID string `json:"eventoId"`
		SessaoID string `json:"sessaoId"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := associarEvento(r.Context(), tx, id, req.EventoID, req.SessaoID, ses.UsuarioID, true); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "associar_evento", "trabalho", id, map[string]any{"evento": req.EventoID, "sessao": req.SessaoID}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// PUT /docente/trabalhos/{id}/banca {examinadores: [docenteId, ...]} — na ordem desejada.
// O orientador integra a banca automaticamente. Quantidade variável de examinadores.
func (s *Servidor) docenteDefinirBanca(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	acesso, err := s.exigirOrientador(r, id)
	if err != nil {
		return err
	}
	var req struct {
		Examinadores []string `json:"examinadores"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	if len(req.Examinadores) == 0 {
		return erroCampo("examinadores", "Designe pelo menos um examinador.")
	}
	if len(req.Examinadores) > 10 {
		return erroCampo("examinadores", "Número de examinadores acima do permitido.")
	}
	vistos := map[string]bool{acesso.OrientadorID: true}
	for _, d := range req.Examinadores {
		if vistos[d] {
			return erroCampo("examinadores", "Um mesmo docente não pode aparecer duas vezes na banca (o orientador já faz parte dela).")
		}
		vistos[d] = true
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
	var possuiATA bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM atas WHERE trabalho_id = $1 AND estado <> 'cancelada')`, id).Scan(&possuiATA); err != nil {
		return err
	}
	if possuiATA {
		return erroConflito("A banca não pode ser alterada após a geração da ATA. Reinicie o processo da ATA, se necessário.")
	}
	for _, d := range req.Examinadores {
		ok, err := possuiPerfil(ctx, tx, d, PerfilDocente)
		if err != nil {
			return err
		}
		if !ok {
			return erroCampo("examinadores", "Todos os examinadores devem ser DOCENTES aprovados.")
		}
	}
	// Examinadores removidos: não podem ter avaliação finalizada.
	var finalizadosRemovidos int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM membros_banca mb JOIN avaliacoes a ON a.trabalho_id = mb.trabalho_id AND a.avaliador_id = mb.docente_id
		WHERE mb.trabalho_id = $1 AND mb.situacao = 'ativo' AND mb.papel = 'examinador' AND a.estado = 'finalizada'
		  AND NOT (mb.docente_id::text = ANY($2::text[]))`, id, req.Examinadores).Scan(&finalizadosRemovidos); err != nil {
		return err
	}
	if finalizadosRemovidos > 0 {
		return erroConflito("Um examinador que já finalizou a avaliação não pode ser removido. Reabra a avaliação antes.")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM avaliacoes a USING membros_banca mb
		WHERE mb.trabalho_id = $1 AND mb.situacao = 'ativo' AND mb.papel = 'examinador' AND a.trabalho_id = mb.trabalho_id
		  AND a.avaliador_id = mb.docente_id AND a.estado = 'rascunho' AND NOT (mb.docente_id::text = ANY($2::text[]))`, id, req.Examinadores); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE membros_banca SET situacao = 'removido' WHERE trabalho_id = $1 AND papel = 'examinador' AND situacao = 'ativo'
		AND NOT (docente_id::text = ANY($2::text[]))`, id, req.Examinadores); err != nil {
		return err
	}
	// Garante o orientador na banca (ordem 0).
	if _, err := tx.Exec(ctx, `INSERT INTO membros_banca (trabalho_id, docente_id, papel, ordem, obrigatorio, designado_por)
		SELECT $1, $2, 'orientador', 0, true, $2
		WHERE NOT EXISTS (SELECT 1 FROM membros_banca WHERE trabalho_id = $1 AND docente_id = $2 AND situacao = 'ativo')`, id, acesso.OrientadorID); err != nil {
		return err
	}
	for i, d := range req.Examinadores {
		tag, err := tx.Exec(ctx, `UPDATE membros_banca SET ordem = $3 WHERE trabalho_id = $1 AND docente_id = $2 AND situacao = 'ativo'`, id, d, i+1)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO membros_banca (trabalho_id, docente_id, papel, ordem, obrigatorio, designado_por)
				VALUES ($1, $2, 'examinador', $3, true, $4)`, id, d, i+1, ses.UsuarioID); err != nil {
				return err
			}
		}
	}
	if err := sincronizarCriteriosGerais(ctx, tx, id); err != nil {
		return err
	}
	if _, err := regras.RecalcularEstadoTrabalho(ctx, tx, id, ses.UsuarioID, "Banca designada"); err != nil {
		return err
	}
	if err := s.auditar(ctx, tx, r, "definir_banca", "trabalho", id, map[string]any{"examinadores": req.Examinadores}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}

// POST /docente/trabalhos/{id}/criterios {nome, descricao} — critério extra (0 a 100, sem peso).
func (s *Servidor) docenteCriarCriterioExtra(w http.ResponseWriter, r *http.Request) error {
	id := chi.URLParam(r, "id")
	if _, err := s.exigirOrientador(r, id); err != nil {
		return err
	}
	var req struct {
		Nome      string `json:"nome"`
		Descricao string `json:"descricao"`
	}
	if err := lerJSON(r, &req); err != nil {
		return err
	}
	req.Nome = strings.TrimSpace(req.Nome)
	if req.Nome == "" {
		return erroCampo("nome", "Informe o nome do critério.")
	}
	ses := sessaoDe(r)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := exigirCriteriosEditaveis(r.Context(), tx, id); err != nil {
		return err
	}
	if err := sincronizarCriteriosGerais(r.Context(), tx, id); err != nil {
		return err
	}
	var criterioID string
	if err := tx.QueryRow(r.Context(), `INSERT INTO criterios_trabalho (trabalho_id, origem, nome, descricao, ordem, criado_por)
		VALUES ($1, 'extra', $2, $3, (SELECT COALESCE(max(ordem), 0) + 1 FROM criterios_trabalho WHERE trabalho_id = $1), $4) RETURNING id::text`,
		id, req.Nome, strings.TrimSpace(req.Descricao), ses.UsuarioID).Scan(&criterioID); err != nil {
		return err
	}
	if err := s.auditar(r.Context(), tx, r, "criar_criterio_extra", "trabalho", id, map[string]any{"criterio": criterioID, "nome": req.Nome}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	responderJSON(w, http.StatusCreated, map[string]string{"id": criterioID})
	return nil
}

func (s *Servidor) docenteRemoverCriterioExtra(w http.ResponseWriter, r *http.Request) error {
	id, criterioID := chi.URLParam(r, "id"), chi.URLParam(r, "criterioId")
	if _, err := s.exigirOrientador(r, id); err != nil {
		return err
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := exigirCriteriosEditaveis(r.Context(), tx, id); err != nil {
		return err
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM notas_avaliacao WHERE criterio_trabalho_id = $1`, criterioID); err != nil {
		return err
	}
	tag, err := tx.Exec(r.Context(), `DELETE FROM criterios_trabalho WHERE id = $1 AND trabalho_id = $2 AND origem = 'extra'`, criterioID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return erroNaoEncontrado("Critério extra não encontrado.")
	}
	if err := s.auditar(r.Context(), tx, r, "remover_criterio_extra", "trabalho", id, map[string]any{"criterio": criterioID}); err != nil {
		return err
	}
	if err := tx.Commit(r.Context()); err != nil {
		return err
	}
	return responderOK(w, map[string]bool{"ok": true})
}
