# O que é este projeto

## Objetivo

Gerenciar de ponta a ponta os eventos de apresentação e defesa de TCC do IFMG Campus Ouro Branco. O ciclo vai do cadastro dos alunos até a publicação dos anais, com geração automática e consistente dos documentos institucionais (ATA e declaração anti-plágio).

## Escopo

1. Cadastro e autenticação (aluno livre e imediato; docente sujeito a aprovação; ADMIN inicial por credencial de instalação).
2. Vínculos de orientação: um orientador ativo por aluno, com histórico preservado.
3. Eventos, com sessões opcionais, apresentadores e ordem da programação.
4. Submissão do TCC: título, resumo, link do Overleaf, artigo e apresentação em PDF, com versões.
5. Banca, com orientador e quantidade variável de examinadores, critérios gerais e extras (0 a 100, sem pesos) e avaliação em tela única.
6. Geração automática da ATA (XeLaTeX em Docker) quando todas as avaliações terminam e o trabalho está associado a um evento.
7. Fluxo sequencial de assinaturas, independente para cada trabalho. Somente a última versão assinada é mantida.
8. Declaração anti-plágio: gerada pelo sistema, assinada externamente e enviada pelo aluno.
9. Consolidação: PDF único com as apresentações e PDF único com os artigos (anais), na ordem da programação.
10. Página pública do evento e auditoria das operações sensíveis.

## Perfis

- **ALUNO**: perfil exclusivo. Nunca acumula outro perfil.
- **DOCENTE**: une professor e orientador. As áreas “Minhas avaliações” e “Meus orientandos” coexistem.
- **ADMIN**: pode coexistir com DOCENTE. Nesse caso, o contexto é escolhido no login e pode ser trocado depois.
- **APRESENTADOR**: atribuição de um DOCENTE a um evento específico. Não é um perfil global.

## Tecnologias

| Camada | Tecnologia |
|---|---|
| Frontend | React 18 + TypeScript, Vite, Material UI 6, React Hook Form + Zod, TanStack Query |
| Backend | Go 1.23 (monólito modular), chi, pgx |
| Banco | PostgreSQL 16 (com a fila de documentos) |
| Documentos | XeLaTeX (TeX Live do Debian), qpdf e poppler-utils |
| Infraestrutura | Docker e Docker Compose (frontend servido por nginx) |
| Testes | go test, Vitest + Testing Library, Playwright |
| API | REST/JSON em `/api/v1`, documentada em OpenAPI (`backend/openapi.yaml`) |

Todo o sistema (interface, variáveis, tabelas, rotas e mensagens) está em português do Brasil. Não há infraestrutura de tradução.
