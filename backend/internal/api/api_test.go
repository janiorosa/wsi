package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"wsi/internal/armazenamento"
	"wsi/internal/banco"
	"wsi/internal/configuracao"
	"wsi/internal/documentos"
	"wsi/internal/fila"
	"wsi/internal/latex"
	"wsi/internal/pdf"
	"wsi/internal/seguranca"
)

// Testes de integração: exigem PostgreSQL (WSI_TESTE_BANCO_URL). O banco é LIMPO a cada teste.
// Executar com: docker compose --profile testes run --build --rm testes-backend

const senhaTeste = "senha-de-teste-123"

type ambiente struct {
	t        *testing.T
	pool     *pgxpool.Pool
	arm      *armazenamento.Armazenamento
	servidor *httptest.Server
	dirDados string
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	url := os.Getenv("WSI_TESTE_BANCO_URL")
	if url == "" {
		t.Skip("WSI_TESTE_BANCO_URL não definida: testes de integração ignorados")
	}
	ctx := context.Background()
	pool, err := banco.Conectar(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err := banco.Migrar(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE usuarios, eventos, criterios_avaliacao, configuracoes_institucionais CASCADE`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	arm, err := armazenamento.Novo(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := configuracao.Configuracao{CookieSeguro: false, TamanhoMaximoUpload: 10 << 20}
	srv := httptest.NewServer(Novo(cfg, pool, arm, fila.Nova(pool, 1), nil).Rotas())
	t.Cleanup(func() {
		srv.Close()
		pool.Close()
	})
	return &ambiente{t: t, pool: pool, arm: arm, servidor: srv, dirDados: dir}
}

func exigirQPDF(t *testing.T) {
	if _, err := exec.LookPath("qpdf"); err != nil {
		t.Skip("qpdf não disponível: teste executado no container de testes")
	}
}

// criarUsuario insere diretamente um usuário ativo com os perfis informados.
func (a *ambiente) criarUsuario(nome string, perfis ...string) (id, email string) {
	a.t.Helper()
	ctx := context.Background()
	email = strings.ToLower(strings.ReplaceAll(nome, " ", ".")) + "@teste.ifmg.edu.br"
	hash, _ := seguranca.GerarHashSenha(senhaTeste)
	d := dadosCadastro{Nome: nome, Email: email, Identificador: "0" + fmt.Sprint(time.Now().UnixNano()%10000000)}
	var err error
	for _, p := range perfis {
		if p == PerfilAluno {
			id, err = inserirAluno(ctx, a.pool, d, hash, nil)
			if err != nil {
				a.t.Fatal(err)
			}
			return id, email
		}
	}
	id, err = inserirUsuario(ctx, a.pool, d, "SIAPE", hash)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, p := range perfis {
		if p == PerfilDocente {
			a.pool.Exec(ctx, `INSERT INTO perfis_docente (usuario_id, situacao_aprovacao) VALUES ($1, 'aprovado')`, id)
		}
		if _, err := a.pool.Exec(ctx, `INSERT INTO perfis_usuario (usuario_id, perfil) VALUES ($1, $2)`, id, p); err != nil {
			a.t.Fatal(err)
		}
	}
	return id, email
}

type cliente struct {
	a    *ambiente
	http *http.Client
	csrf string
}

func (a *ambiente) cliente() *cliente {
	jar, _ := cookiejar.New(nil)
	return &cliente{a: a, http: &http.Client{Jar: jar}}
}

func (a *ambiente) entrar(email, contexto string) *cliente {
	a.t.Helper()
	c := a.cliente()
	status, corpo := c.req("POST", "/auth/entrar", map[string]string{"email": email, "senha": senhaTeste})
	if status != 200 {
		a.t.Fatalf("login de %s falhou: %d %v", email, status, corpo)
	}
	c.csrf, _ = m(corpo)["csrf"].(string)
	if contexto != "" && m(corpo)["contexto"] != contexto {
		status, corpo = c.req("POST", "/auth/contexto", map[string]string{"contexto": contexto})
		if status != 200 {
			a.t.Fatalf("troca de contexto falhou: %d %v", status, corpo)
		}
	}
	return c
}

func m(v any) map[string]any {
	r, _ := v.(map[string]any)
	return r
}

func lista(v any) []any {
	r, _ := v.([]any)
	return r
}

func (c *cliente) req(metodo, caminho string, corpo any) (int, any) {
	c.a.t.Helper()
	var leitor io.Reader
	if corpo != nil {
		b, _ := json.Marshal(corpo)
		leitor = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(metodo, c.a.servidor.URL+"/api/v1"+caminho, leitor)
	req.Header.Set("Content-Type", "application/json")
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	return c.executar(req)
}

func (c *cliente) executar(req *http.Request) (int, any) {
	resp, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var v any
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		json.Unmarshal(b, &v)
	} else {
		v = string(b)
	}
	return resp.StatusCode, v
}

func (c *cliente) ok(metodo, caminho string, corpo any) any {
	c.a.t.Helper()
	status, v := c.req(metodo, caminho, corpo)
	if status < 200 || status > 299 {
		c.a.t.Fatalf("%s %s: status %d: %v", metodo, caminho, status, v)
	}
	return v
}

func (c *cliente) enviar(caminho string, campos map[string]string, conteudo []byte) (int, any) {
	c.a.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range campos {
		mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("arquivo", "documento.pdf")
	fw.Write(conteudo)
	mw.Close()
	req, _ := http.NewRequest("POST", c.a.servidor.URL+"/api/v1"+caminho, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-CSRF-Token", c.csrf)
	return c.executar(req)
}

func (c *cliente) baixar(caminho string) (int, string) {
	req, _ := http.NewRequest("GET", c.a.servidor.URL+"/api/v1"+caminho, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		c.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// pdfMinimo gera um PDF A4 válido de uma página; "marca" torna cada arquivo único.
func pdfMinimo(marca string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	conteudo := "BT /F1 12 Tf 72 720 Td (" + marca + ") Tj ET"
	objetos := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595.28 841.89] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(conteudo), conteudo),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var posicoes []int
	for i, o := range objetos {
		posicoes = append(posicoes, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objetos)+1)
	for _, p := range posicoes {
		fmt.Fprintf(&b, "%010d 00000 n \n", p)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objetos)+1, xref)
	return b.Bytes()
}

// ---------------------------------------------------------------------
// Identidade, perfis e cadastro
// ---------------------------------------------------------------------

func TestAlunoCadastraEEntraSemAprovacao(t *testing.T) {
	a := novoAmbiente(t)
	c := a.cliente()
	status, corpo := c.req("POST", "/auth/cadastro/aluno", map[string]string{"nome": "Aluna Nova", "email": "aluna@exemplo.com",
		"identificador": "0123456", "senha": "senha-forte-1"})
	if status != 201 || m(corpo)["contexto"] != "ALUNO" {
		t.Fatalf("cadastro de aluno: %d %v", status, corpo)
	}
	c.csrf = m(corpo)["csrf"].(string)
	c.ok("GET", "/aluno/painel", nil)
	var hash string
	a.pool.QueryRow(context.Background(), `SELECT hash_senha FROM usuarios WHERE email = 'aluna@exemplo.com'`).Scan(&hash)
	if strings.Contains(hash, "senha-forte-1") || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("senha armazenada de forma insegura")
	}
}

func TestEmailInvalidoRejeitado(t *testing.T) {
	a := novoAmbiente(t)
	status, _ := a.cliente().req("POST", "/auth/cadastro/aluno", map[string]string{"nome": "Fulano", "email": "nao-e-email",
		"identificador": "1", "senha": "senha-forte-1"})
	if status != 400 {
		t.Fatalf("e-mail inválido deveria ser rejeitado, status %d", status)
	}
}

func TestAlunoEhPerfilExclusivo(t *testing.T) {
	a := novoAmbiente(t)
	alunoID, _ := a.criarUsuario("Aluno Exclusivo", PerfilAluno)
	_, adminEmail := a.criarUsuario("Admin Teste", PerfilAdmin)
	admin := a.entrar(adminEmail, "")
	for _, p := range []string{PerfilAdmin, PerfilDocente} {
		if status, _ := admin.req("POST", "/admin/usuarios/"+alunoID+"/perfis", map[string]string{"perfil": p}); status != 409 {
			t.Fatalf("ALUNO não pode receber %s (status %d)", p, status)
		}
	}
	if _, err := a.pool.Exec(context.Background(), `INSERT INTO perfis_usuario (usuario_id, perfil) VALUES ($1, 'ADMIN')`, alunoID); err == nil {
		t.Fatal("o banco deveria impedir perfil adicional para ALUNO")
	}
}

func TestDocentePendenteNaoAcessaFuncoesDocentes(t *testing.T) {
	a := novoAmbiente(t)
	c := a.cliente()
	status, corpo := c.req("POST", "/auth/cadastro/docente", map[string]string{"nome": "Docente Novo", "email": "docente.novo@ifmg.edu.br",
		"senha": "senha-forte-1"})
	if status != 201 || m(corpo)["situacaoDocente"] != "pendente" {
		t.Fatalf("cadastro docente: %d %v", status, corpo)
	}
	c.csrf = m(corpo)["csrf"].(string)
	if status, _ := c.req("GET", "/docente/painel", nil); status != 403 {
		t.Fatalf("docente pendente acessou área docente: %d", status)
	}
	var id string
	a.pool.QueryRow(context.Background(), `SELECT id::text FROM usuarios WHERE email = 'docente.novo@ifmg.edu.br'`).Scan(&id)
	_, adminEmail := a.criarUsuario("Admin Aprovador", PerfilAdmin)
	a.entrar(adminEmail, "").ok("POST", "/admin/docentes/"+id+"/aprovar", nil)
	d := a.cliente()
	_, corpo = d.req("POST", "/auth/entrar", map[string]string{"email": "docente.novo@ifmg.edu.br", "senha": "senha-forte-1"})
	d.csrf = m(corpo)["csrf"].(string)
	d.ok("GET", "/docente/painel", nil)
}

func TestAdminDocenteEscolheContexto(t *testing.T) {
	a := novoAmbiente(t)
	_, email := a.criarUsuario("Admin Docente", PerfilAdmin, PerfilDocente)
	c := a.cliente()
	_, corpo := c.req("POST", "/auth/entrar", map[string]string{"email": email, "senha": senhaTeste})
	if m(corpo)["precisaEscolherContexto"] != true {
		t.Fatalf("ADMIN+DOCENTE deveria escolher o contexto: %v", corpo)
	}
	c.csrf = m(corpo)["csrf"].(string)
	if status, _ := c.req("GET", "/admin/painel", nil); status != 403 {
		t.Fatal("sem contexto escolhido não deveria acessar a área ADMIN")
	}
	c.ok("POST", "/auth/contexto", map[string]string{"contexto": "ADMIN"})
	c.ok("GET", "/admin/painel", nil)
	if status, _ := c.req("GET", "/docente/painel", nil); status != 403 {
		t.Fatal("contexto ADMIN não deve exibir funções docentes")
	}
	c.ok("POST", "/auth/contexto", map[string]string{"contexto": "DOCENTE"})
	c.ok("GET", "/docente/painel", nil)
	var perfis int
	a.pool.QueryRow(context.Background(), `SELECT count(*) FROM perfis_usuario pu JOIN usuarios u ON u.id = pu.usuario_id WHERE u.email = $1`, email).Scan(&perfis)
	if perfis != 2 {
		t.Fatal("a escolha de contexto não pode alterar os perfis armazenados")
	}
}

func TestCSRFObrigatorio(t *testing.T) {
	a := novoAmbiente(t)
	_, email := a.criarUsuario("Aluno CSRF", PerfilAluno)
	c := a.entrar(email, "")
	c.csrf = ""
	if status, _ := c.req("PUT", "/aluno/trabalho", map[string]string{"titulo": "x", "resumo": "y"}); status != 403 {
		t.Fatalf("requisição sem CSRF deveria ser recusada: %d", status)
	}
}

// ---------------------------------------------------------------------
// Fluxo completo: orientação, banca, avaliação, ATA e assinaturas
// ---------------------------------------------------------------------

type cenario struct {
	a                          *ambiente
	admin                      *cliente
	eventoID                   string
	orientadorID, ex1ID, ex2ID string
	orientador, ex1, ex2, ex3  *cliente
}

func novoCenario(t *testing.T) *cenario {
	a := novoAmbiente(t)
	s := &cenario{a: a}
	_, adminEmail := a.criarUsuario("Administrador Geral", PerfilAdmin)
	s.admin = a.entrar(adminEmail, "")
	var e1, e2, e3, eo string
	s.orientadorID, eo = a.criarUsuario("Orientador Principal", PerfilDocente)
	s.ex1ID, e1 = a.criarUsuario("Examinador Um", PerfilDocente)
	s.ex2ID, e2 = a.criarUsuario("Examinador Dois", PerfilDocente)
	_, e3 = a.criarUsuario("Docente Externo", PerfilDocente)
	s.orientador, s.ex1, s.ex2, s.ex3 = a.entrar(eo, ""), a.entrar(e1, ""), a.entrar(e2, ""), a.entrar(e3, "")
	data := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	ev := s.admin.ok("POST", "/admin/eventos", map[string]any{"nome": "Workshop de TCC", "edicao": "2026", "numeroEdicao": 6, "dataApresentacao": data,
		"horarioApresentacao": "19:00", "local": "Salão de Convenções do Campus IFMG Ouro Branco", "estado": "aberto"})
	s.eventoID = m(ev)["id"].(string)
	s.admin.ok("POST", "/admin/criterios", map[string]any{"nome": "Critério A", "descricao": "Teste", "ativo": true, "ordem": 1})
	s.admin.ok("POST", "/admin/criterios", map[string]any{"nome": "Critério B", "descricao": "Teste", "ativo": true, "ordem": 2})
	return s
}

// novoTrabalho: aluno cadastra TCC, escolhe orientador; orientador associa evento e define banca.
func (s *cenario) novoTrabalho(nomeAluno string, examinadores ...string) (alunoCli *cliente, trabalhoID string) {
	_, email := s.a.criarUsuario(nomeAluno, PerfilAluno)
	alunoCli = s.a.entrar(email, "")
	alunoCli.ok("POST", "/aluno/orientador", map[string]any{"docenteId": s.orientadorID})
	trabalhoID = m(alunoCli.ok("PUT", "/aluno/trabalho", map[string]string{"titulo": "Trabalho de " + nomeAluno, "resumo": "Resumo"}))["id"].(string)
	s.orientador.ok("PUT", "/docente/trabalhos/"+trabalhoID+"/evento", map[string]string{"eventoId": s.eventoID})
	s.orientador.ok("PUT", "/docente/trabalhos/"+trabalhoID+"/banca", map[string]any{"examinadores": examinadores})
	return alunoCli, trabalhoID
}

func (s *cenario) avaliar(c *cliente, trabalhoID string, valores ...float64) any {
	tela := m(c.ok("GET", "/docente/avaliacoes/"+trabalhoID, nil))
	criterios := lista(tela["criterios"])
	if len(criterios) != len(valores) {
		s.a.t.Fatalf("esperados %d critérios, encontrados %d", len(valores), len(criterios))
	}
	var notasEnvio []map[string]any
	for i, cr := range criterios {
		notasEnvio = append(notasEnvio, map[string]any{"criterioId": m(cr)["id"], "nota": valores[i]})
	}
	return c.ok("PUT", "/docente/avaliacoes/"+trabalhoID, map[string]any{"notas": notasEnvio, "finalizar": true})
}

// simularGeracaoATA substitui o XeLaTeX (testado separadamente no pacote latex): grava um PDF como original.
func (s *cenario) simularGeracaoATA(trabalhoID string) string {
	ctx := context.Background()
	salvo, err := s.a.arm.Salvar("atas-originais", bytes.NewReader(pdfMinimo("ATA original")))
	if err != nil {
		s.a.t.Fatal(err)
	}
	var ataID string
	if err := s.a.pool.QueryRow(ctx, `UPDATE atas SET estado = 'aguardando_assinaturas', chave_original = $2, sha256_original = $3, gerado_em = now()
		WHERE trabalho_id = $1 AND estado = 'gerando' RETURNING id::text`, trabalhoID, salvo.Chave, salvo.SHA256).Scan(&ataID); err != nil {
		s.a.t.Fatalf("ATA não estava aguardando geração: %v", err)
	}
	return ataID
}

func TestAssociacaoSomenteAEventoFuturo(t *testing.T) {
	s := novoCenario(t)
	ontem := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	ev := s.admin.ok("POST", "/admin/eventos", map[string]string{"nome": "Evento Passado", "dataApresentacao": ontem,
		"horarioApresentacao": "10:00", "local": "Sala 1", "estado": "aberto"})
	passado := m(ev)["id"].(string)
	_, trabalhoID := s.novoTrabalho("Aluno Datas", s.ex1ID)
	if status, _ := s.orientador.req("PUT", "/docente/trabalhos/"+trabalhoID+"/evento", map[string]string{"eventoId": passado}); status != 409 {
		t.Fatalf("associação a evento passado deveria ser bloqueada: %d", status)
	}
	if status, _ := s.admin.req("POST", "/admin/trabalhos/"+trabalhoID+"/evento", map[string]string{"eventoId": passado, "justificativa": "curta"}); status != 400 {
		t.Fatal("exceção administrativa exige justificativa")
	}
	s.admin.ok("POST", "/admin/trabalhos/"+trabalhoID+"/evento", map[string]string{"eventoId": passado, "justificativa": "Defesa remarcada por motivo de saúde"})
}

func TestFluxoCompletoDeAvaliacaoEAssinaturas(t *testing.T) {
	exigirQPDF(t)
	s := novoCenario(t)
	aluno, trabalhoID := s.novoTrabalho("Aluna Fluxo", s.ex1ID, s.ex2ID)

	// Docente fora da banca não vê o trabalho; avaliador vê.
	if status, _ := s.ex3.req("GET", "/docente/trabalhos/"+trabalhoID, nil); status != 404 {
		t.Fatalf("docente sem vínculo acessou o trabalho: %d", status)
	}
	if len(lista(s.ex1.ok("GET", "/docente/avaliacoes", nil))) != 1 {
		t.Fatal("examinador deveria ver o trabalho destinado a ele")
	}
	if len(lista(s.ex3.ok("GET", "/docente/avaliacoes", nil))) != 0 {
		t.Fatal("docente fora da banca não deve ver avaliações")
	}

	// Critério extra aparece para todos os avaliadores deste trabalho.
	s.orientador.ok("POST", "/docente/trabalhos/"+trabalhoID+"/criterios", map[string]string{"nome": "Critério Extra", "descricao": "Somente aqui"})
	tela := m(s.ex1.ok("GET", "/docente/avaliacoes/"+trabalhoID, nil))
	if len(lista(tela["criterios"])) != 3 {
		t.Fatalf("critério extra deveria aparecer: %v", tela["criterios"])
	}
	// Escala 0–100 validada.
	cid := m(lista(tela["criterios"])[0])["id"]
	if status, _ := s.ex1.req("PUT", "/docente/avaliacoes/"+trabalhoID, map[string]any{"notas": []map[string]any{{"criterioId": cid, "nota": 101}}}); status != 400 {
		t.Fatal("nota acima de 100 deveria ser rejeitada")
	}

	s.avaliar(s.orientador, trabalhoID, 100, 90, 80) // 90,00
	s.avaliar(s.ex1, trabalhoID, 80, 70, 60)         // 70,00
	if status, _ := s.orientador.req("POST", "/docente/trabalhos/"+trabalhoID+"/ata", nil); status != 409 {
		t.Fatal("avaliação pendente deveria impedir a ATA")
	}
	resp := m(s.avaliar(s.ex2, trabalhoID, 90, 90, 90)) // 90,00
	if resp["ataId"] == nil {
		t.Fatalf("a ATA deveria ser colocada na fila automaticamente: %v", resp)
	}
	var notaFinal string
	a := s.a
	a.pool.QueryRow(context.Background(), `SELECT nota_final::text FROM atas WHERE trabalho_id = $1`, trabalhoID).Scan(&notaFinal)
	if notaFinal != "83.33" { // (90 + 70 + 90) / 3
		t.Fatalf("nota final incorreta: %s", notaFinal)
	}
	ataID := s.simularGeracaoATA(trabalhoID)

	// Ordem: aluno (1), orientador (2), examinadores (3, 4).
	if status, _ := s.orientador.baixar("/atas/" + ataID + "/documento"); status != 403 {
		t.Fatalf("orientador não pode baixar antes da sua vez: %d", status)
	}
	// ...mas a ATA original (em branco) fica disponível a qualquer momento para os docentes da banca.
	if status, conteudo := s.orientador.baixar("/atas/" + ataID + "/documento?versao=original"); status != 200 || !strings.Contains(conteudo, "ATA original") {
		t.Fatalf("orientador deveria baixar a ATA original a qualquer momento: %d", status)
	}
	if status, _ := s.ex2.baixar("/atas/" + ataID + "/documento?versao=assinada"); status != 404 {
		t.Fatalf("sem assinaturas ainda não há versão assinada: %d", status)
	}
	if status, _ := s.ex1.enviar("/atas/"+ataID+"/assinatura", map[string]string{"posicao": "1"}, pdfMinimo("ex1 adiantado")); status != 403 {
		t.Fatalf("signatário posterior não pode antecipar: %d", status)
	}
	if status, _ := aluno.baixar("/atas/" + ataID + "/documento"); status != 200 {
		t.Fatalf("aluno deveria baixar a ATA original: %d", status)
	}
	if status, v := aluno.enviar("/atas/"+ataID+"/assinatura", map[string]string{"posicao": "1"}, pdfMinimo("assinado aluno")); status != 200 {
		t.Fatalf("assinatura do aluno: %d %v", status, v)
	}
	// Reenvio/duplo clique não avança duas posições.
	if status, _ := aluno.enviar("/atas/"+ataID+"/assinatura", map[string]string{"posicao": "1"}, pdfMinimo("assinado aluno 2")); status == 200 {
		t.Fatal("reenvio não pode avançar o fluxo")
	}
	// Arquivo inválido: rejeitado e o signatário continua o mesmo.
	if status, _ := s.orientador.enviar("/atas/"+ataID+"/assinatura", map[string]string{"posicao": "2"}, []byte("não sou um PDF")); status != 400 {
		t.Fatal("arquivo não PDF deveria ser rejeitado")
	}
	_, conteudo := s.orientador.baixar("/atas/" + ataID + "/documento")
	if !strings.Contains(conteudo, "assinado aluno") {
		t.Fatal("o orientador deveria receber a última versão assinada")
	}
	// Examinador baixa a versão parcialmente assinada antes da sua vez; aluno não usa essas versões.
	if status, conteudo := s.ex2.baixar("/atas/" + ataID + "/documento?versao=assinada"); status != 200 || !strings.Contains(conteudo, "assinado aluno") {
		t.Fatalf("examinador deveria baixar a ATA com as assinaturas já coletadas: %d", status)
	}
	if status, _ := aluno.baixar("/atas/" + ataID + "/documento?versao=original"); status != 404 {
		t.Fatalf("aluno não usa o download de versões dos docentes: %d", status)
	}
	for i, c := range []*cliente{s.orientador, s.ex1, s.ex2} {
		if status, v := c.enviar("/atas/"+ataID+"/assinatura", map[string]string{"posicao": fmt.Sprint(i + 2)}, pdfMinimo(fmt.Sprintf("assinatura %d", i+2))); status != 200 {
			t.Fatalf("assinatura na posição %d: %d %v", i+2, status, v)
		}
	}
	// Somente a última versão assinada é mantida.
	arquivos, _ := filepath.Glob(filepath.Join(a.dirDados, "arquivos", "atas-assinadas", "*", "*.pdf"))
	if len(arquivos) != 1 {
		t.Fatalf("deveria existir somente a última versão assinada, existem %d", len(arquivos))
	}
	var registros int
	a.pool.QueryRow(context.Background(), `SELECT count(*) FROM assinaturas_ata WHERE ata_id = $1 AND estado = 'assinada' AND sha256 IS NOT NULL`, ataID).Scan(&registros)
	if registros != 4 {
		t.Fatalf("metadados de auditoria das 4 assinaturas deveriam permanecer, há %d", registros)
	}

	// ATA final: somente aluno e docentes participantes.
	for nome, c := range map[string]*cliente{"aluno": aluno, "orientador": s.orientador, "ex1": s.ex1, "ex2": s.ex2} {
		status, conteudo := c.baixar("/atas/" + ataID + "/documento")
		if status != 200 || !strings.Contains(conteudo, "assinatura 4") {
			t.Fatalf("%s deveria baixar a ATA final: %d", nome, status)
		}
	}
	outroAluno, _ := s.novoTrabalho("Outro Aluno", s.ex3ID())
	if status, _ := outroAluno.baixar("/atas/" + ataID + "/documento"); status != 404 {
		t.Fatal("outro aluno não pode acessar a ATA")
	}
	if status, _ := s.ex3.baixar("/atas/" + ataID + "/documento"); status != 404 {
		t.Fatal("docente fora da banca não pode acessar a ATA")
	}
	if status, _ := s.admin.baixar("/atas/" + ataID + "/documento"); status != 404 {
		t.Fatal("ADMIN não recebe acesso automático à ATA")
	}

	// Declaração assinada conclui a documentação.
	var estado string
	a.pool.QueryRow(context.Background(), `SELECT estado FROM trabalhos WHERE id = $1`, trabalhoID).Scan(&estado)
	if estado != "documentacao_pendente" {
		t.Fatalf("estado esperado documentacao_pendente, obtido %s", estado)
	}
	if status, v := aluno.enviar("/aluno/declaracao/assinada", nil, pdfMinimo("declaracao")); status != 201 {
		t.Fatalf("envio da declaração: %d %v", status, v)
	}
	a.pool.QueryRow(context.Background(), `SELECT estado FROM trabalhos WHERE id = $1`, trabalhoID).Scan(&estado)
	if estado != "concluido" {
		t.Fatalf("estado esperado concluido, obtido %s", estado)
	}
}

func (s *cenario) ex3ID() string {
	var id string
	s.a.pool.QueryRow(context.Background(), `SELECT id::text FROM usuarios WHERE nome_completo = 'Docente Externo'`).Scan(&id)
	return id
}

func TestFluxosDeAssinaturaIndependentesPorTrabalho(t *testing.T) {
	exigirQPDF(t)
	s := novoCenario(t)
	alunoA, trabA := s.novoTrabalho("Aluno A", s.ex1ID)
	alunoB, trabB := s.novoTrabalho("Aluno B", s.ex1ID)
	for _, tr := range []string{trabA, trabB} {
		s.avaliar(s.orientador, tr, 80, 80)
		s.avaliar(s.ex1, tr, 60, 60)
	}
	ataA, ataB := s.simularGeracaoATA(trabA), s.simularGeracaoATA(trabB)
	alunoA.enviar("/atas/"+ataA+"/assinatura", map[string]string{"posicao": "1"}, pdfMinimo("A1"))
	s.orientador.enviar("/atas/"+ataA+"/assinatura", map[string]string{"posicao": "2"}, pdfMinimo("A2"))
	// O examinador participa de dois processos: só é a vez dele no trabalho A.
	itens := lista(s.ex1.ok("GET", "/docente/atas", nil))
	vez := map[string]bool{}
	for _, it := range itens {
		vez[m(it)["ataId"].(string)] = m(it)["minhaVez"].(bool)
	}
	if !vez[ataA] || vez[ataB] {
		t.Fatalf("vez do examinador incorreta: %v", vez)
	}
	var posicaoB int
	s.a.pool.QueryRow(context.Background(), `SELECT posicao_atual FROM atas WHERE id = $1`, ataB).Scan(&posicaoB)
	if posicaoB != 1 {
		t.Fatal("assinaturas de um trabalho não podem alterar o fluxo de outro")
	}
	if status, _ := alunoA.baixar("/atas/" + ataB + "/documento"); status != 404 {
		t.Fatal("aluno não pode ver a ATA de outro aluno")
	}
	_ = alunoB
}

func TestApresentadorSemAcessoANotasEATAs(t *testing.T) {
	s := novoCenario(t)
	_, trabalhoID := s.novoTrabalho("Aluno Apresentado", s.ex1ID)
	s.admin.ok("POST", "/admin/eventos/"+s.eventoID+"/apresentadores", map[string]any{"docenteId": s.ex3ID()})
	ficha := m(s.ex3.ok("GET", "/docente/trabalhos/"+trabalhoID, nil))
	if _, temBanca := ficha["banca"]; temBanca {
		t.Fatal("apresentador não deve ver banca/notas")
	}
	if _, temATA := ficha["ata"]; temATA {
		t.Fatal("apresentador não deve ver a ATA")
	}
}

func TestAnaisSomenteArtigosNaOrdemDaProgramacao(t *testing.T) {
	exigirQPDF(t)
	s := novoCenario(t)
	alunoA, trabA := s.novoTrabalho("Aluno Primeiro", s.ex1ID)
	alunoB, trabB := s.novoTrabalho("Aluno Segundo", s.ex1ID)
	s.novoTrabalho("Aluno Sem Artigo", s.ex1ID)
	alunoA.enviar("/aluno/trabalho/arquivos/artigo", nil, pdfMinimo("artigo A v1"))
	alunoA.enviar("/aluno/trabalho/arquivos/artigo", nil, pdfMinimo("artigo A v2"))
	alunoB.enviar("/aluno/trabalho/arquivos/artigo", nil, pdfMinimo("artigo B"))
	alunoB.enviar("/aluno/declaracao/assinada", nil, pdfMinimo("DECLARACAO PRIVADA"))
	s.admin.ok("POST", "/admin/eventos/"+s.eventoID+"/apresentadores", map[string]any{"docenteId": s.ex3ID()})
	s.ex3.ok("PUT", "/apresentador/eventos/"+s.eventoID+"/programacao", map[string]any{"trabalhoIds": []string{trabB, trabA}})
	r := m(s.ex3.ok("POST", "/apresentador/eventos/"+s.eventoID+"/publicacoes", map[string]string{"tipo": "anais"}))
	r2 := m(s.ex3.ok("POST", "/apresentador/eventos/"+s.eventoID+"/publicacoes", map[string]string{"tipo": "anais"}))
	if r["tarefaId"] != r2["tarefaId"] {
		t.Fatal("duplo clique não pode criar duas tarefas")
	}
	// Votação: o mais votado aparece no editorial (páginas iniciais geradas em LaTeX).
	votacoes := lista(s.ex1.ok("GET", "/docente/premiacoes", nil))
	premiacaoID := m(lista(m(votacoes[0])["premiacoes"])[0])["id"].(string)
	s.ex1.ok("POST", "/docente/premiacoes/"+premiacaoID+"/votos", map[string]string{"trabalhoId": trabB})
	if _, err := exec.LookPath("xelatex"); err != nil {
		t.Skip("xelatex não disponível: teste executado no container de testes")
	}
	compilador := &latex.Compilador{DiretorioTemplates: filepath.Join("..", "..", "templates", "documentos"), DiretorioTemp: t.TempDir()}
	servico := &documentos.Servico{Pool: s.a.pool, Armazenamento: s.a.arm, Compilador: compilador}
	tarefa := fila.Tarefa{ID: r["tarefaId"].(string), Tipo: fila.TipoAnais, EntidadeID: s.eventoID,
		Parametros: map[string]string{"publicacao_id": r["publicacaoId"].(string)}}
	if _, _, err := servico.GerarPublicacao("anais", "artigo")(context.Background(), tarefa, func(int) {}); err != nil {
		t.Fatal(err)
	}
	painel := m(s.ex3.ok("GET", "/apresentador/eventos/"+s.eventoID, nil))
	pub := m(lista(painel["publicacoes"])[0])
	itens := lista(pub["itens"])
	if pub["estado"] != "aguardando_revisao" || len(itens) != 3 {
		t.Fatalf("publicação inesperada: %v", pub)
	}
	if m(itens[0])["aluno"] != "Aluno Segundo" || m(itens[1])["aluno"] != "Aluno Primeiro" || m(itens[2])["resultado"] != "ausente" {
		t.Fatalf("ordem ou resultado incorretos: %v", itens)
	}
	if m(itens[1])["versaoArquivo"].(float64) != 2 {
		t.Fatal("deveria usar a versão mais recente do artigo")
	}
	// Antes da publicação, nada é público.
	publico := m(s.a.cliente().ok("GET", "/publico/eventos/workshop-de-tcc-2026", nil))
	if len(lista(publico["publicacoes"])) != 0 {
		t.Fatal("documento não publicado apareceu na página pública")
	}
	s.ex3.ok("POST", "/apresentador/publicacoes/"+pub["id"].(string)+"/publicar", nil)
	publico = m(s.a.cliente().ok("GET", "/publico/eventos/workshop-de-tcc-2026", nil))
	if len(lista(publico["publicacoes"])) != 1 {
		t.Fatal("publicação deveria aparecer na página pública")
	}
	status, conteudo := s.a.cliente().baixar("/publico/publicacoes/" + pub["id"].(string) + "/arquivo")
	if status != 200 {
		t.Fatalf("download público dos anais: %d", status)
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		caminho := filepath.Join(t.TempDir(), "anais.pdf")
		os.WriteFile(caminho, []byte(conteudo), 0o600)
		texto, err := pdf.ExtrairTexto(context.Background(), caminho)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(texto, "DECLARACAO PRIVADA") || !strings.Contains(texto, "artigo A v2") || strings.Contains(texto, "artigo A v1") ||
			strings.Index(texto, "artigo B") > strings.Index(texto, "artigo A v2") {
			t.Fatalf("conteúdo ou ordem dos anais incorretos: %q", texto)
		}
		normalizado := strings.Join(strings.Fields(texto), " ")
		if !strings.Contains(normalizado, "ANAIS DO VI WORKSHOP") || !strings.Contains(normalizado, "EDITORIAL") ||
			!strings.Contains(normalizado, "v.6, n.1") || strings.Index(normalizado, "EDITORIAL") > strings.Index(normalizado, "artigo B") {
			t.Fatalf("páginas iniciais dos anais ausentes ou fora de ordem: %q", normalizado)
		}
	}
	var paginas int
	s.a.pool.QueryRow(context.Background(), `SELECT total_paginas FROM publicacoes_evento WHERE id = $1`, pub["id"]).Scan(&paginas)
	if paginas < 3+2 {
		t.Fatalf("anais deveriam ter as páginas iniciais + 2 artigos, têm %d páginas", paginas)
	}
}

func TestPrimeiroAcessoPorDadosERedefinicaoPeloAdmin(t *testing.T) {
	a := novoAmbiente(t)
	adminID, adminEmail := a.criarUsuario("Admin Redefinicao", PerfilAdmin)
	admin := a.entrar(adminEmail, "")
	r := m(admin.ok("POST", "/admin/usuarios", map[string]any{"tipo": "ALUNO", "nome": "Maria José da Silva",
		"email": "maria.jose@exemplo.com", "identificador": "0012345"}))
	alunaID := r["id"].(string)
	if _, temCodigo := r["acesso"]; temCodigo {
		t.Fatal("o cadastro não deve mais devolver código de acesso")
	}
	if status, _ := admin.req("POST", "/admin/usuarios", map[string]any{"tipo": "DOCENTE", "nome": "Docente Sem Siape", "email": "sem.siape@exemplo.com"}); status != 400 {
		t.Fatal("matrícula/SIAPE deve ser obrigatório no cadastro pelo ADMIN")
	}
	primeiro := func(nome, identificador string) int {
		status, _ := a.cliente().req("POST", "/auth/primeiro-acesso", map[string]string{"nome": nome, "email": "maria.jose@exemplo.com",
			"identificador": identificador, "senha": senhaTeste})
		return status
	}
	if primeiro("Maria José da Silva", "999") != 400 || primeiro("Outra Pessoa", "0012345") != 400 {
		t.Fatal("dados que não conferem devem ser recusados")
	}
	// Nome sem acento/maiúsculas e matrícula sem zeros à esquerda também conferem.
	if s := primeiro("maria jose  da silva", "12345"); s != 200 {
		t.Fatalf("primeiro acesso deveria funcionar: %d", s)
	}
	a.entrar("maria.jose@exemplo.com", "")
	if primeiro("Maria José da Silva", "0012345") != 409 {
		t.Fatal("primeiro acesso não pode ser repetido quando a conta já tem senha")
	}
	if status, _ := admin.req("POST", "/admin/usuarios/"+adminID+"/redefinir-acesso", nil); status != 400 {
		t.Fatal("ADMIN não deve redefinir o próprio acesso")
	}
	admin.ok("POST", "/admin/usuarios/"+alunaID+"/redefinir-acesso", nil)
	if status, v := a.cliente().req("POST", "/auth/entrar", map[string]string{"email": "maria.jose@exemplo.com", "senha": senhaTeste}); status != 401 ||
		m(m(v)["erro"])["codigo"] != "primeiro_acesso_pendente" {
		t.Fatalf("após redefinição a senha antiga não pode funcionar: %d %v", status, v)
	}
	if primeiro("Maria José da Silva", "0012345") != 200 {
		t.Fatal("após redefinição o primeiro acesso deve funcionar novamente")
	}
	a.entrar("maria.jose@exemplo.com", "")
}

func TestParecerVotacaoEDeclaracaoDoOrientando(t *testing.T) {
	s := novoCenario(t)
	aluno, trab := s.novoTrabalho("Aluna Parecer", s.ex1ID)
	_, trab2 := s.novoTrabalho("Aluno Votado Dois", s.ex1ID)
	_, trab3 := s.novoTrabalho("Aluno Votado Tres", s.ex1ID)

	// Parecer: salvo antes e separadamente da nota; visível ao aluno; somente membros da banca.
	s.ex1.ok("PUT", "/docente/avaliacoes/"+trab+"/parecer", map[string]string{"parecer": "Revise a seção 2 e amplie os resultados."})
	if status, _ := s.ex3.req("PUT", "/docente/avaliacoes/"+trab+"/parecer", map[string]string{"parecer": "intruso"}); status != 404 {
		t.Fatalf("docente fora da banca não pode registrar parecer: %d", status)
	}
	tela := m(s.ex1.ok("GET", "/docente/avaliacoes/"+trab, nil))
	if tela["parecer"] != "Revise a seção 2 e amplie os resultados." || tela["estado"] != "rascunho" || tela["podeEditarParecer"] != true {
		t.Fatalf("parecer não salvo corretamente: %v", tela)
	}
	pareceres := lista(m(aluno.ok("GET", "/aluno/painel", nil))["pareceres"])
	if len(pareceres) != 1 || m(pareceres[0])["avaliador"] != "Examinador Um" {
		t.Fatalf("aluno deveria ver o parecer do examinador: %v", pareceres)
	}

	// Votação: até dois trabalhos por premiação; votos podem ser retirados enquanto a votação estiver aberta.
	votacoes := lista(s.ex3.ok("GET", "/docente/premiacoes", nil))
	if len(votacoes) != 1 {
		t.Fatalf("deveria haver um evento em votação: %v", votacoes)
	}
	premiacoesEvento := lista(m(votacoes[0])["premiacoes"])
	if len(premiacoesEvento) != 1 || m(premiacoesEvento[0])["nome"] != "Prêmio Destaque: Reconhecendo a Excelência Acadêmica" {
		t.Fatalf("todo evento deveria ter a premiação padrão: %v", premiacoesEvento)
	}
	pid := m(premiacoesEvento[0])["id"].(string)
	s.ex3.ok("POST", "/docente/premiacoes/"+pid+"/votos", map[string]string{"trabalhoId": trab})
	s.ex3.ok("POST", "/docente/premiacoes/"+pid+"/votos", map[string]string{"trabalhoId": trab}) // repetido: sem efeito
	s.ex3.ok("POST", "/docente/premiacoes/"+pid+"/votos", map[string]string{"trabalhoId": trab2})
	if status, _ := s.ex3.req("POST", "/docente/premiacoes/"+pid+"/votos", map[string]string{"trabalhoId": trab3}); status != 409 {
		t.Fatalf("terceiro voto deveria ser recusado: %d", status)
	}
	s.ex3.ok("DELETE", "/docente/premiacoes/"+pid+"/votos/"+trab2, nil)
	s.ex3.ok("POST", "/docente/premiacoes/"+pid+"/votos", map[string]string{"trabalhoId": trab3})
	s.ex1.ok("POST", "/docente/premiacoes/"+pid+"/votos", map[string]string{"trabalhoId": trab})
	detalhe := m(s.admin.ok("GET", "/admin/eventos/"+s.eventoID, nil))
	resultado := m(lista(detalhe["premiacoes"])[0])
	vencedores := lista(resultado["vencedores"])
	if len(vencedores) != 1 || m(vencedores[0])["trabalhoId"] != trab || m(vencedores[0])["votos"].(float64) != 2 {
		t.Fatalf("vencedor incorreto: %v", resultado)
	}
	// Premiações extras cadastradas pelo ADMIN.
	s.admin.ok("POST", "/admin/eventos/"+s.eventoID+"/premiacoes", map[string]string{"nome": "Melhor Apresentação"})
	if status, _ := s.admin.req("POST", "/admin/eventos/"+s.eventoID+"/premiacoes", map[string]string{"nome": "Melhor Apresentação"}); status != 409 {
		t.Fatalf("premiação duplicada deveria ser recusada: %d", status)
	}
	// Evento publicado: votação encerrada.
	s.a.pool.Exec(context.Background(), `UPDATE eventos SET estado = 'publicado' WHERE id = $1`, s.eventoID)
	if status, _ := s.ex3.req("DELETE", "/docente/premiacoes/"+pid+"/votos/"+trab3, nil); status != 409 {
		t.Fatalf("votação encerrada não aceita alterações: %d", status)
	}

	// Declaração anti-plágio gerada: o orientador ativo também pode baixar.
	salvo, err := s.a.arm.Salvar("declaracoes", bytes.NewReader(pdfMinimo("declaracao gerada")))
	if err != nil {
		t.Fatal(err)
	}
	var alunoID, docID string
	s.a.pool.QueryRow(context.Background(), `SELECT aluno_id::text FROM trabalhos WHERE id = $1`, trab).Scan(&alunoID)
	if err := s.a.pool.QueryRow(context.Background(), `INSERT INTO documentos_gerados (tipo, entidade_id, versao, chave_armazenamento, sha256, tamanho, versao_template, gerado_por)
		VALUES ('declaracao_antiplagio', $1, 1, $2, $3, $4, 'teste', $1) RETURNING id::text`, alunoID, salvo.Chave, salvo.SHA256, salvo.Tamanho).Scan(&docID); err != nil {
		t.Fatal(err)
	}
	if status, _ := s.orientador.baixar("/documentos/" + docID); status != 200 {
		t.Fatalf("orientador deveria baixar a declaração gerada do orientando: %d", status)
	}
	if status, _ := s.ex1.baixar("/documentos/" + docID); status != 404 {
		t.Fatalf("examinador não pode baixar a declaração: %d", status)
	}
}
