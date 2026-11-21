package documentos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"wsi/internal/armazenamento"
	"wsi/internal/datas"
	"wsi/internal/fila"
	"wsi/internal/latex"
	"wsi/internal/notas"
	"wsi/internal/pdf"
	"wsi/internal/regras"
)

type Servico struct {
	Pool         *pgxpool.Pool
	Compilador   *latex.Compilador
	Armazenamento *armazenamento.Armazenamento
}

func (s *Servico) RegistrarEm(f *fila.Fila) {
	f.Registrar(fila.TipoDeclaracao, s.GerarDeclaracao)
	f.Registrar(fila.TipoATA, s.GerarATA)
	f.Registrar(fila.TipoApresentacoes, s.GerarPublicacao("apresentacoes", "apresentacao"))
	f.Registrar(fila.TipoAnais, s.GerarPublicacao("anais", "artigo"))
}

// ---------------------------------------------------------------------
// Declaração anti-plágio (entidade = aluno)
// ---------------------------------------------------------------------

func (s *Servico) GerarDeclaracao(ctx context.Context, t fila.Tarefa, progresso func(int)) (string, []string, error) {
	var nome, matricula string
	err := s.Pool.QueryRow(ctx, `SELECT u.nome_completo, COALESCE(pa.matricula, '')
		FROM usuarios u LEFT JOIN perfis_aluno pa ON pa.usuario_id = u.id WHERE u.id = $1`, t.EntidadeID).Scan(&nome, &matricula)
	if err != nil {
		return "", nil, fmt.Errorf("aluno não encontrado: %w", err)
	}
	cabecalho, err := CarregarCabecalho(ctx, s.Pool)
	if err != nil {
		return "", nil, err
	}
	cidade, err := CarregarCidade(ctx, s.Pool)
	if err != nil {
		return "", nil, err
	}
	// Data da declaração: data de emissão (geração) do documento, no fuso de São Paulo.
	dados, err := latex.EntradaDeclaracao{Cabecalho: cabecalho, NomeAluno: nome, Matricula: matricula, Cidade: cidade, Data: datas.Agora()}.Preparar()
	if err != nil {
		return "", nil, err
	}
	progresso(30)
	salvo, res, err := s.compilarESalvar(ctx, latex.DocumentoDeclaracao, dados, "declaracoes")
	if err != nil {
		return "", nil, err
	}
	progresso(90)
	var id string
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO documentos_gerados (tipo, entidade_id, versao, chave_armazenamento, sha256, tamanho, versao_template, gerado_por)
		VALUES ('declaracao_antiplagio', $1,
		        (SELECT COALESCE(max(versao), 0) + 1 FROM documentos_gerados WHERE tipo = 'declaracao_antiplagio' AND entidade_id = $1),
		        $2, $3, $4, $5, $6)
		RETURNING id::text`, t.EntidadeID, salvo.Chave, salvo.SHA256, salvo.Tamanho, latex.VersaoTemplateDeclaracao, t.SolicitadoPor).Scan(&id)
	if err != nil {
		s.Armazenamento.Remover(salvo.Chave)
		return "", nil, err
	}
	return id, res.Avisos, nil
}

// ---------------------------------------------------------------------
// ATA (entidade = ata)
// ---------------------------------------------------------------------

func (s *Servico) GerarATA(ctx context.Context, t fila.Tarefa, progresso func(int)) (string, []string, error) {
	resultado, avisos, err := s.gerarATA(ctx, t, progresso)
	if err != nil {
		s.Pool.Exec(context.Background(), `UPDATE atas SET erro_geracao = $2 WHERE id = $1 AND estado = 'gerando'`, t.EntidadeID, err.Error())
	}
	return resultado, avisos, err
}

type dadosATABanco struct {
	trabalhoID, estado, notaFinal string
	chaveOriginalAnterior         *string
	nomeAluno, matricula, titulo  string
	data                          time.Time
	horario                       string
	local                         string
}

func (s *Servico) gerarATA(ctx context.Context, t fila.Tarefa, progresso func(int)) (string, []string, error) {
	var d dadosATABanco
	// Data, horário e local: SEMPRE da sessão (se houver) ou do evento atualmente associado ao trabalho.
	err := s.Pool.QueryRow(ctx, `
		SELECT a.trabalho_id::text, a.estado, a.nota_final::text, a.chave_original,
		       u.nome_completo, COALESCE(pa.matricula, ''), tr.titulo,
		       COALESCE(se.data, ev.data_apresentacao),
		       to_char(COALESCE(se.horario, ev.horario_apresentacao), 'HH24:MI'),
		       COALESCE(se.local, ev.local)
		FROM atas a
		JOIN trabalhos tr ON tr.id = a.trabalho_id
		JOIN usuarios u ON u.id = tr.aluno_id
		LEFT JOIN perfis_aluno pa ON pa.usuario_id = u.id
		JOIN eventos ev ON ev.id = tr.evento_id
		LEFT JOIN sessoes_evento se ON se.id = tr.sessao_id
		WHERE a.id = $1`, t.EntidadeID).
		Scan(&d.trabalhoID, &d.estado, &d.notaFinal, &d.chaveOriginalAnterior, &d.nomeAluno, &d.matricula, &d.titulo, &d.data, &d.horario, &d.local)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, fmt.Errorf("ATA não encontrada ou trabalho sem evento associado")
	}
	if err != nil {
		return "", nil, err
	}
	if d.estado != "gerando" {
		return "", nil, fmt.Errorf("a ATA não está aguardando geração (estado: %s)", d.estado)
	}

	// Signatários congelados na criação da ATA.
	rows, err := s.Pool.Query(ctx, `SELECT tipo, u.nome_completo FROM assinaturas_ata aa JOIN usuarios u ON u.id = aa.signatario_id
		WHERE aa.ata_id = $1 ORDER BY aa.ordem`, t.EntidadeID)
	if err != nil {
		return "", nil, err
	}
	var orientador string
	var examinadores []string
	for rows.Next() {
		var tipo, nome string
		if err := rows.Scan(&tipo, &nome); err != nil {
			rows.Close()
			return "", nil, err
		}
		switch tipo {
		case "orientador":
			orientador = nome
		case "examinador":
			examinadores = append(examinadores, nome)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", nil, err
	}

	horario, err := time.Parse("15:04", d.horario)
	if err != nil {
		return "", nil, fmt.Errorf("horário inválido no evento: %s", d.horario)
	}
	nota, err := notas.DeTextoNumeric(d.notaFinal)
	if err != nil {
		return "", nil, fmt.Errorf("nota final inválida: %s", d.notaFinal)
	}
	cabecalho, err := CarregarCabecalho(ctx, s.Pool)
	if err != nil {
		return "", nil, err
	}
	entrada := latex.EntradaATA{
		Cabecalho:           cabecalho,
		DataApresentacao:    time.Date(d.data.Year(), d.data.Month(), d.data.Day(), 0, 0, 0, 0, datas.Fuso),
		HorarioApresentacao: time.Duration(horario.Hour())*time.Hour + time.Duration(horario.Minute())*time.Minute,
		Local:               d.local,
		NomeAluno:           d.nomeAluno,
		Matricula:           d.matricula,
		Titulo:              d.titulo,
		NomeOrientador:      orientador,
		Examinadores:        examinadores,
		NotaFinal:           nota.FormatarBR(),
		Aprovado:            notas.Aprovado(nota),
	}
	dados, err := entrada.Preparar()
	if err != nil {
		return "", nil, err
	}
	progresso(30)
	salvo, res, err := s.compilarESalvar(ctx, latex.DocumentoATA, dados, "atas-originais")
	if err != nil {
		return "", nil, err
	}
	progresso(90)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		s.Armazenamento.Remover(salvo.Chave)
		return "", nil, err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(ctx, `UPDATE atas SET estado = 'aguardando_assinaturas', chave_original = $2, sha256_original = $3,
		versao_template = $4, gerado_em = now(), erro_geracao = '', posicao_atual = 1
		WHERE id = $1 AND estado = 'gerando'`, t.EntidadeID, salvo.Chave, salvo.SHA256, latex.VersaoTemplateATA)
	if err == nil && tag.RowsAffected() != 1 {
		err = fmt.Errorf("a ATA mudou de estado durante a geração")
	}
	if err == nil {
		_, err = regras.RecalcularEstadoTrabalho(ctx, tx, d.trabalhoID, t.SolicitadoPor, "ATA gerada automaticamente")
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		s.Armazenamento.Remover(salvo.Chave)
		return "", nil, err
	}
	// Regeneração: o original anterior é substituído pelo novo.
	if d.chaveOriginalAnterior != nil && *d.chaveOriginalAnterior != salvo.Chave {
		s.Armazenamento.Remover(*d.chaveOriginalAnterior)
	}
	return t.EntidadeID, res.Avisos, nil
}


// ---------------------------------------------------------------------
// Publicações: apresentações consolidadas e anais (entidade = evento)
// ---------------------------------------------------------------------

type itemPublicacao struct {
	trabalhoID string
	arquivoID  *string
	chave      *string
	resultado  string
	detalhe    string
	paginas    *int
}

func (s *Servico) GerarPublicacao(tipoPublicacao, tipoArquivo string) fila.Processador {
	return func(ctx context.Context, t fila.Tarefa, progresso func(int)) (string, []string, error) {
		publicacaoID := t.Parametros["publicacao_id"]
		if publicacaoID == "" {
			return "", nil, fmt.Errorf("publicação não informada")
		}
		id, avisos, err := s.gerarPublicacao(ctx, t, publicacaoID, tipoPublicacao, tipoArquivo, progresso)
		if err != nil {
			s.Pool.Exec(context.Background(), `UPDATE publicacoes_evento SET estado = 'falhou', erro = $2
				WHERE id = $1 AND estado = 'em_preparacao'`, publicacaoID, err.Error())
		}
		return id, avisos, err
	}
}

func (s *Servico) gerarPublicacao(ctx context.Context, t fila.Tarefa, publicacaoID, tipoPublicacao, tipoArquivo string, progresso func(int)) (string, []string, error) {
	// Trabalhos do evento, na ordem da programação; nunca incluir atas, notas ou declarações:
	// somente o arquivo mais recente do tipo solicitado (artigo ou apresentação).
	rows, err := s.Pool.Query(ctx, `
		SELECT tr.id::text, af.id::text, af.chave_armazenamento
		FROM trabalhos tr
		LEFT JOIN LATERAL (
			SELECT id, chave_armazenamento FROM arquivos_trabalho
			WHERE trabalho_id = tr.id AND tipo = $2
			ORDER BY versao DESC LIMIT 1
		) af ON true
		WHERE tr.evento_id = $1 AND tr.estado <> 'cancelado'
		ORDER BY tr.ordem_programacao NULLS LAST, tr.titulo, tr.id`, t.EntidadeID, tipoArquivo)
	if err != nil {
		return "", nil, err
	}
	var itens []itemPublicacao
	for rows.Next() {
		var it itemPublicacao
		if err := rows.Scan(&it.trabalhoID, &it.arquivoID, &it.chave); err != nil {
			rows.Close()
			return "", nil, err
		}
		itens = append(itens, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(itens) == 0 {
		return "", nil, fmt.Errorf("o evento não possui trabalhos associados")
	}

	var entradas []string
	var avisos []string
	for i := range itens {
		it := &itens[i]
		if it.chave == nil {
			it.resultado, it.detalhe = "ausente", "arquivo não enviado"
			continue
		}
		caminho, err := s.Armazenamento.Caminho(*it.chave)
		if err == nil {
			err = pdf.Validar(ctx, caminho)
		}
		var paginas int
		if err == nil {
			paginas, err = pdf.ContarPaginas(ctx, caminho)
		}
		if err != nil {
			it.resultado, it.detalhe = "invalido", err.Error()
			avisos = append(avisos, fmt.Sprintf("trabalho %s: arquivo inválido", it.trabalhoID))
			continue
		}
		it.resultado = "incluido"
		it.paginas = &paginas
		entradas = append(entradas, caminho)
		progresso(10 + 60*(i+1)/len(itens))
	}
	if len(entradas) == 0 {
		if err := s.salvarItens(ctx, publicacaoID, itens); err != nil {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("nenhum arquivo elegível para consolidação")
	}

	dir, err := s.Armazenamento.DiretorioTemporario("publicacao")
	if err != nil {
		return "", nil, err
	}
	defer os.RemoveAll(dir)
	saida := filepath.Join(dir, "consolidado.pdf")
	if err := pdf.Juntar(ctx, entradas, saida); err != nil {
		return "", nil, err
	}
	if err := pdf.Validar(ctx, saida); err != nil {
		return "", nil, fmt.Errorf("PDF consolidado inválido: %w", err)
	}
	total, err := pdf.ContarPaginas(ctx, saida)
	if err != nil {
		return "", nil, err
	}
	soma := 0
	for _, it := range itens {
		if it.paginas != nil {
			soma += *it.paginas
		}
	}
	if total != soma {
		return "", nil, fmt.Errorf("o PDF consolidado tem %d páginas, mas a soma dos arquivos é %d", total, soma)
	}
	progresso(85)
	salvo, err := s.Armazenamento.SalvarDeArquivo("publicacoes", saida)
	if err != nil {
		return "", nil, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		s.Armazenamento.Remover(salvo.Chave)
		return "", nil, err
	}
	defer tx.Rollback(context.Background())
	if err := inserirItens(ctx, tx, publicacaoID, itens); err != nil {
		s.Armazenamento.Remover(salvo.Chave)
		return "", nil, err
	}
	tag, err := tx.Exec(ctx, `UPDATE publicacoes_evento SET estado = 'aguardando_revisao', chave_armazenamento = $2, sha256 = $3,
		tamanho = $4, total_paginas = $5, gerado_em = now(), erro = '' WHERE id = $1 AND estado = 'em_preparacao'`,
		publicacaoID, salvo.Chave, salvo.SHA256, salvo.Tamanho, total)
	if err == nil && tag.RowsAffected() != 1 {
		err = fmt.Errorf("publicação não está em preparação")
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		s.Armazenamento.Remover(salvo.Chave)
		return "", nil, err
	}
	_ = tipoPublicacao
	return publicacaoID, avisos, nil
}

func (s *Servico) salvarItens(ctx context.Context, publicacaoID string, itens []itemPublicacao) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err := inserirItens(ctx, tx, publicacaoID, itens); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func inserirItens(ctx context.Context, tx pgx.Tx, publicacaoID string, itens []itemPublicacao) error {
	if _, err := tx.Exec(ctx, `DELETE FROM itens_publicacao WHERE publicacao_id = $1`, publicacaoID); err != nil {
		return err
	}
	for i, it := range itens {
		if _, err := tx.Exec(ctx, `INSERT INTO itens_publicacao (publicacao_id, trabalho_id, arquivo_id, posicao, resultado, detalhe, paginas)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, publicacaoID, it.trabalhoID, it.arquivoID, i+1, it.resultado, it.detalhe, it.paginas); err != nil {
			return err
		}
	}
	return nil
}

// compilarESalvar compila em diretório temporário e move o PDF para o armazenamento persistente.
func (s *Servico) compilarESalvar(ctx context.Context, doc latex.Documento, dados any, categoria string) (armazenamento.ArquivoSalvo, latex.Resultado, error) {
	dir, err := s.Armazenamento.DiretorioTemporario("documento")
	if err != nil {
		return armazenamento.ArquivoSalvo{}, latex.Resultado{}, err
	}
	defer os.RemoveAll(dir)
	destino := filepath.Join(dir, "saida.pdf")
	res, err := s.Compilador.Compilar(ctx, doc, dados, destino)
	if err != nil {
		return armazenamento.ArquivoSalvo{}, latex.Resultado{}, err
	}
	salvo, err := s.Armazenamento.SalvarDeArquivo(categoria, destino)
	return salvo, res, err
}
