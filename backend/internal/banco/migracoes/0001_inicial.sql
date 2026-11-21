-- Migração 0001: estrutura inicial do sistema WSI.
-- Todos os horários são armazenados como timestamptz; datas e horários de
-- apresentação são armazenados como date/time locais de America/Sao_Paulo.

-- =====================================================================
-- Usuários, perfis e sessões
-- =====================================================================

CREATE TABLE usuarios (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nome_completo               text NOT NULL CHECK (length(trim(nome_completo)) > 0),
    email                       text NOT NULL CHECK (email = lower(email)),
    telefone                    text NOT NULL DEFAULT '',
    identificador_institucional text NOT NULL DEFAULT '',
    tipo_identificador          text NOT NULL DEFAULT '' CHECK (tipo_identificador IN ('', 'SIAPE', 'MATRICULA')),
    hash_senha                  text,
    situacao                    text NOT NULL DEFAULT 'ativo'
                                CHECK (situacao IN ('ativo', 'aguardando_primeiro_acesso', 'bloqueado')),
    tentativas_falhas           integer NOT NULL DEFAULT 0,
    bloqueado_ate               timestamptz,
    dados_pendentes_conferencia boolean NOT NULL DEFAULT false,
    criado_em                   timestamptz NOT NULL DEFAULT now(),
    atualizado_em               timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX usuarios_email_unico ON usuarios (email);
CREATE INDEX usuarios_nome_idx ON usuarios (lower(nome_completo));

CREATE TABLE perfis_usuario (
    usuario_id    uuid NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    perfil        text NOT NULL CHECK (perfil IN ('ADMIN', 'DOCENTE', 'ALUNO')),
    concedido_por uuid REFERENCES usuarios(id),
    concedido_em  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usuario_id, perfil)
);

-- ALUNO é perfil exclusivo: não pode coexistir com nenhum outro perfil.
CREATE FUNCTION verificar_exclusividade_aluno() RETURNS trigger AS $$
BEGIN
    IF NEW.perfil = 'ALUNO' THEN
        IF EXISTS (SELECT 1 FROM perfis_usuario WHERE usuario_id = NEW.usuario_id AND perfil <> 'ALUNO') THEN
            RAISE EXCEPTION 'perfil ALUNO é exclusivo e não pode ser combinado com outros perfis'
                USING ERRCODE = 'check_violation';
        END IF;
    ELSE
        IF EXISTS (SELECT 1 FROM perfis_usuario WHERE usuario_id = NEW.usuario_id AND perfil = 'ALUNO') THEN
            RAISE EXCEPTION 'conta ALUNO não pode receber outros perfis'
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER perfis_usuario_exclusividade_aluno
    BEFORE INSERT OR UPDATE ON perfis_usuario
    FOR EACH ROW EXECUTE FUNCTION verificar_exclusividade_aluno();

CREATE TABLE perfis_docente (
    usuario_id          uuid PRIMARY KEY REFERENCES usuarios(id) ON DELETE CASCADE,
    siape               text NOT NULL DEFAULT '',
    situacao_aprovacao  text NOT NULL DEFAULT 'pendente'
                        CHECK (situacao_aprovacao IN ('pendente', 'aprovado', 'rejeitado')),
    aprovado_por        uuid REFERENCES usuarios(id),
    aprovado_em         timestamptz,
    motivo_rejeicao     text NOT NULL DEFAULT ''
);
CREATE INDEX perfis_docente_situacao_idx ON perfis_docente (situacao_aprovacao);

CREATE TABLE perfis_aluno (
    usuario_id  uuid PRIMARY KEY REFERENCES usuarios(id) ON DELETE CASCADE,
    matricula   text NOT NULL DEFAULT '',
    curso       text NOT NULL DEFAULT 'Sistemas de Informação'
);
CREATE UNIQUE INDEX perfis_aluno_matricula_unica ON perfis_aluno (matricula) WHERE matricula <> '';

CREATE TABLE tokens_conta (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id  uuid NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    hash_token  text NOT NULL UNIQUE,
    tipo        text NOT NULL CHECK (tipo IN ('instalacao', 'primeiro_acesso', 'redefinicao_senha')),
    expira_em   timestamptz NOT NULL,
    usado_em    timestamptz,
    criado_por  uuid REFERENCES usuarios(id),
    criado_em   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tokens_conta_usuario_idx ON tokens_conta (usuario_id);

CREATE TABLE sessoes (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    hash_token     text NOT NULL UNIQUE,
    usuario_id     uuid NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    contexto       text CHECK (contexto IN ('ADMIN', 'DOCENTE', 'ALUNO')),
    token_csrf     text NOT NULL,
    criada_em      timestamptz NOT NULL DEFAULT now(),
    expira_em      timestamptz NOT NULL,
    ultimo_uso_em  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessoes_usuario_idx ON sessoes (usuario_id);

CREATE TABLE registros_auditoria (
    id          bigserial PRIMARY KEY,
    ator_id     uuid REFERENCES usuarios(id),
    acao        text NOT NULL,
    entidade    text NOT NULL,
    entidade_id text NOT NULL DEFAULT '',
    dados       jsonb NOT NULL DEFAULT '{}'::jsonb,
    ip          text NOT NULL DEFAULT '',
    criado_em   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX registros_auditoria_criado_idx ON registros_auditoria (criado_em DESC);
CREATE INDEX registros_auditoria_entidade_idx ON registros_auditoria (entidade, entidade_id);

CREATE TABLE configuracoes_institucionais (
    chave  text PRIMARY KEY,
    valor  text NOT NULL,
    atualizado_por uuid REFERENCES usuarios(id),
    atualizado_em  timestamptz NOT NULL DEFAULT now()
);

-- =====================================================================
-- Orientação
-- =====================================================================

CREATE TABLE vinculos_orientacao (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aluno_id             uuid NOT NULL REFERENCES usuarios(id),
    docente_id           uuid NOT NULL REFERENCES usuarios(id),
    criado_por           uuid NOT NULL REFERENCES usuarios(id),
    inicio               timestamptz NOT NULL DEFAULT now(),
    fim                  timestamptz,
    situacao             text NOT NULL DEFAULT 'ativo' CHECK (situacao IN ('ativo', 'encerrado')),
    encerrado_por        uuid REFERENCES usuarios(id),
    motivo_encerramento  text NOT NULL DEFAULT '',
    CHECK (aluno_id <> docente_id)
);
-- Um único orientador principal ativo por aluno.
CREATE UNIQUE INDEX vinculos_orientacao_ativo_unico ON vinculos_orientacao (aluno_id) WHERE situacao = 'ativo';
CREATE INDEX vinculos_orientacao_docente_idx ON vinculos_orientacao (docente_id, situacao);

-- =====================================================================
-- Eventos
-- =====================================================================

CREATE TABLE eventos (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nome                  text NOT NULL CHECK (length(trim(nome)) > 0),
    edicao                text NOT NULL DEFAULT '',
    descricao             text NOT NULL DEFAULT '',
    data_apresentacao     date NOT NULL,
    horario_apresentacao  time NOT NULL,
    local                 text NOT NULL CHECK (length(trim(local)) > 0),
    campus                text NOT NULL DEFAULT 'Ouro Branco',
    estado                text NOT NULL DEFAULT 'rascunho'
                          CHECK (estado IN ('rascunho', 'aberto', 'em_preparacao', 'em_andamento', 'encerrado', 'publicado')),
    prazo_submissao       date,
    prazo_apresentacao    date,
    informacoes_publicas  text NOT NULL DEFAULT '',
    slug                  text NOT NULL UNIQUE,
    capa_chave            text,
    capa_mime             text,
    criado_por            uuid NOT NULL REFERENCES usuarios(id),
    criado_em             timestamptz NOT NULL DEFAULT now(),
    atualizado_em         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX eventos_data_idx ON eventos (data_apresentacao);
CREATE INDEX eventos_estado_idx ON eventos (estado);

-- Sessões opcionais de um evento (quando há várias datas/horários/locais).
CREATE TABLE sessoes_evento (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    evento_id  uuid NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    titulo     text NOT NULL,
    data       date NOT NULL,
    horario    time NOT NULL,
    local      text NOT NULL CHECK (length(trim(local)) > 0),
    ordem      integer NOT NULL DEFAULT 0
);
CREATE INDEX sessoes_evento_evento_idx ON sessoes_evento (evento_id);

CREATE TABLE apresentadores_evento (
    evento_id     uuid NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    docente_id    uuid NOT NULL REFERENCES usuarios(id),
    pode_publicar boolean NOT NULL DEFAULT true,
    atribuido_por uuid NOT NULL REFERENCES usuarios(id),
    atribuido_em  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (evento_id, docente_id)
);
CREATE INDEX apresentadores_evento_docente_idx ON apresentadores_evento (docente_id);

-- =====================================================================
-- Trabalhos e arquivos
-- =====================================================================

CREATE TABLE trabalhos (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aluno_id                 uuid NOT NULL REFERENCES usuarios(id),
    orientador_id            uuid REFERENCES usuarios(id),
    evento_id                uuid REFERENCES eventos(id),
    sessao_id                uuid REFERENCES sessoes_evento(id) ON DELETE SET NULL,
    ordem_programacao        integer,
    titulo                   text NOT NULL DEFAULT '',
    resumo                   text NOT NULL DEFAULT '',
    link_overleaf            text NOT NULL DEFAULT '',
    estado                   text NOT NULL DEFAULT 'rascunho'
                             CHECK (estado IN ('rascunho', 'submetido', 'aguardando_avaliadores', 'aguardando_avaliacoes',
                                               'avaliacoes_concluidas', 'ata_aguardando_geracao', 'ata_gerada',
                                               'assinaturas_em_andamento', 'ata_finalizada', 'documentacao_pendente',
                                               'concluido', 'cancelado')),
    submetido_em             timestamptz,
    excecao_evento_passado   text NOT NULL DEFAULT '',
    criado_em                timestamptz NOT NULL DEFAULT now(),
    atualizado_em            timestamptz NOT NULL DEFAULT now()
);
-- Um trabalho em andamento por aluno (cancelados/concluídos não contam).
CREATE UNIQUE INDEX trabalhos_aluno_ativo_unico ON trabalhos (aluno_id) WHERE estado NOT IN ('cancelado', 'concluido');
CREATE INDEX trabalhos_evento_idx ON trabalhos (evento_id, ordem_programacao);
CREATE INDEX trabalhos_orientador_idx ON trabalhos (orientador_id);
CREATE INDEX trabalhos_estado_idx ON trabalhos (estado);

CREATE TABLE arquivos_trabalho (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trabalho_id          uuid NOT NULL REFERENCES trabalhos(id),
    tipo                 text NOT NULL CHECK (tipo IN ('artigo', 'apresentacao')),
    nome_original        text NOT NULL,
    chave_armazenamento  text NOT NULL UNIQUE,
    mime                 text NOT NULL,
    tamanho              bigint NOT NULL,
    sha256               text NOT NULL,
    versao               integer NOT NULL,
    enviado_por          uuid NOT NULL REFERENCES usuarios(id),
    enviado_em           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trabalho_id, tipo, versao)
);

CREATE TABLE historico_estados_trabalho (
    id               bigserial PRIMARY KEY,
    trabalho_id      uuid NOT NULL REFERENCES trabalhos(id),
    estado_anterior  text NOT NULL,
    estado_novo      text NOT NULL,
    ator_id          uuid REFERENCES usuarios(id),
    observacao       text NOT NULL DEFAULT '',
    criado_em        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX historico_estados_trabalho_idx ON historico_estados_trabalho (trabalho_id, criado_em);

-- =====================================================================
-- Banca, critérios e avaliações (sem pesos)
-- =====================================================================

CREATE TABLE membros_banca (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trabalho_id  uuid NOT NULL REFERENCES trabalhos(id),
    docente_id   uuid NOT NULL REFERENCES usuarios(id),
    papel        text NOT NULL CHECK (papel IN ('orientador', 'examinador')),
    ordem        integer NOT NULL,
    obrigatorio  boolean NOT NULL DEFAULT true,
    situacao     text NOT NULL DEFAULT 'ativo' CHECK (situacao IN ('ativo', 'removido')),
    designado_por uuid NOT NULL REFERENCES usuarios(id),
    designado_em  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX membros_banca_docente_unico ON membros_banca (trabalho_id, docente_id) WHERE situacao = 'ativo';
CREATE INDEX membros_banca_docente_idx ON membros_banca (docente_id, situacao);

CREATE TABLE criterios_avaliacao (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nome         text NOT NULL CHECK (length(trim(nome)) > 0),
    descricao    text NOT NULL DEFAULT '',
    nota_minima  numeric(5,2) NOT NULL DEFAULT 0 CHECK (nota_minima = 0),
    nota_maxima  numeric(5,2) NOT NULL DEFAULT 100 CHECK (nota_maxima = 100),
    ativo        boolean NOT NULL DEFAULT true,
    ordem        integer NOT NULL DEFAULT 0,
    criado_em    timestamptz NOT NULL DEFAULT now()
);

-- Fotografia histórica dos critérios aplicados a cada trabalho.
CREATE TABLE criterios_trabalho (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trabalho_id        uuid NOT NULL REFERENCES trabalhos(id),
    criterio_geral_id  uuid REFERENCES criterios_avaliacao(id),
    origem             text NOT NULL CHECK (origem IN ('geral', 'extra')),
    nome               text NOT NULL,
    descricao          text NOT NULL DEFAULT '',
    nota_minima        numeric(5,2) NOT NULL DEFAULT 0 CHECK (nota_minima = 0),
    nota_maxima        numeric(5,2) NOT NULL DEFAULT 100 CHECK (nota_maxima = 100),
    ordem              integer NOT NULL,
    criado_por         uuid REFERENCES usuarios(id),
    criado_em          timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX criterios_trabalho_trabalho_idx ON criterios_trabalho (trabalho_id, ordem);
CREATE UNIQUE INDEX criterios_trabalho_geral_unico ON criterios_trabalho (trabalho_id, criterio_geral_id) WHERE criterio_geral_id IS NOT NULL;

CREATE TABLE avaliacoes (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trabalho_id     uuid NOT NULL REFERENCES trabalhos(id),
    avaliador_id    uuid NOT NULL REFERENCES usuarios(id),
    estado          text NOT NULL DEFAULT 'rascunho' CHECK (estado IN ('rascunho', 'finalizada')),
    nota_calculada  numeric(5,2) CHECK (nota_calculada BETWEEN 0 AND 100),
    enviada_em      timestamptz,
    versao_regra    text NOT NULL DEFAULT '',
    criado_em       timestamptz NOT NULL DEFAULT now(),
    atualizado_em   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (trabalho_id, avaliador_id)
);

CREATE TABLE notas_avaliacao (
    avaliacao_id         uuid NOT NULL REFERENCES avaliacoes(id) ON DELETE CASCADE,
    criterio_trabalho_id uuid NOT NULL REFERENCES criterios_trabalho(id),
    nota                 numeric(5,2) NOT NULL CHECK (nota BETWEEN 0 AND 100),
    comentario           text NOT NULL DEFAULT '',
    PRIMARY KEY (avaliacao_id, criterio_trabalho_id)
);

-- =====================================================================
-- ATA e assinaturas sequenciais (um fluxo independente por trabalho)
-- =====================================================================

CREATE TABLE atas (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    trabalho_id          uuid NOT NULL REFERENCES trabalhos(id),
    versao               integer NOT NULL,
    estado               text NOT NULL CHECK (estado IN ('gerando', 'aguardando_assinaturas', 'finalizada', 'cancelada')),
    nota_final           numeric(5,2) NOT NULL CHECK (nota_final BETWEEN 0 AND 100),
    versao_regra         text NOT NULL,
    versao_template      text NOT NULL DEFAULT '',
    chave_original       text,
    sha256_original      text,
    chave_assinado       text,
    sha256_assinado      text,
    posicao_atual        integer NOT NULL DEFAULT 1,
    erro_geracao         text NOT NULL DEFAULT '',
    gerado_por           uuid NOT NULL REFERENCES usuarios(id),
    gerado_em            timestamptz,
    criado_em            timestamptz NOT NULL DEFAULT now(),
    finalizada_em        timestamptz,
    motivo_cancelamento  text NOT NULL DEFAULT '',
    UNIQUE (trabalho_id, versao)
);
CREATE UNIQUE INDEX atas_ativa_unica ON atas (trabalho_id) WHERE estado <> 'cancelada';

CREATE TABLE assinaturas_ata (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    ata_id          uuid NOT NULL REFERENCES atas(id),
    signatario_id   uuid NOT NULL REFERENCES usuarios(id),
    tipo            text NOT NULL CHECK (tipo IN ('aluno', 'orientador', 'examinador')),
    rotulo          text NOT NULL,
    ordem           integer NOT NULL,
    estado          text NOT NULL DEFAULT 'pendente' CHECK (estado IN ('pendente', 'assinada')),
    enviado_em      timestamptz,
    sha256          text,
    observacao      text NOT NULL DEFAULT '',
    UNIQUE (ata_id, ordem),
    UNIQUE (ata_id, signatario_id)
);
CREATE INDEX assinaturas_ata_signatario_idx ON assinaturas_ata (signatario_id, estado);

-- =====================================================================
-- Declaração antiplágio
-- =====================================================================

CREATE TABLE documentos_gerados (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo                 text NOT NULL CHECK (tipo IN ('declaracao_antiplagio')),
    entidade_id          uuid NOT NULL,
    versao               integer NOT NULL,
    chave_armazenamento  text NOT NULL UNIQUE,
    sha256               text NOT NULL,
    tamanho              bigint NOT NULL,
    versao_template      text NOT NULL,
    gerado_por           uuid NOT NULL REFERENCES usuarios(id),
    gerado_em            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tipo, entidade_id, versao)
);

CREATE TABLE declaracoes_assinadas (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aluno_id             uuid NOT NULL REFERENCES usuarios(id),
    nome_original        text NOT NULL,
    chave_armazenamento  text NOT NULL UNIQUE,
    tamanho              bigint NOT NULL,
    sha256               text NOT NULL,
    versao               integer NOT NULL,
    enviado_por          uuid NOT NULL REFERENCES usuarios(id),
    enviado_em           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (aluno_id, versao)
);

-- =====================================================================
-- Fila assíncrona de documentos (persistida no PostgreSQL)
-- =====================================================================

CREATE TABLE tarefas_documentos (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo            text NOT NULL CHECK (tipo IN ('gerar_declaracao', 'gerar_ata', 'gerar_apresentacoes', 'gerar_anais')),
    entidade_id     uuid NOT NULL,
    parametros      jsonb NOT NULL DEFAULT '{}'::jsonb,
    estado          text NOT NULL DEFAULT 'pendente' CHECK (estado IN ('pendente', 'processando', 'concluida', 'falhou')),
    progresso       integer NOT NULL DEFAULT 0 CHECK (progresso BETWEEN 0 AND 100),
    tentativas      integer NOT NULL DEFAULT 0,
    erro            text NOT NULL DEFAULT '',
    avisos          text NOT NULL DEFAULT '',
    resultado_id    uuid,
    solicitado_por  uuid NOT NULL REFERENCES usuarios(id),
    criado_em       timestamptz NOT NULL DEFAULT now(),
    iniciado_em     timestamptz,
    concluido_em    timestamptz
);
-- Impede duplicação (duplo clique): uma tarefa ativa por tipo e entidade.
CREATE UNIQUE INDEX tarefas_documentos_ativa_unica ON tarefas_documentos (tipo, entidade_id)
    WHERE estado IN ('pendente', 'processando');
CREATE INDEX tarefas_documentos_estado_idx ON tarefas_documentos (estado, criado_em);

-- =====================================================================
-- Publicações do evento (anais e apresentações consolidadas)
-- =====================================================================

CREATE TABLE publicacoes_evento (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    evento_id            uuid NOT NULL REFERENCES eventos(id),
    tipo                 text NOT NULL CHECK (tipo IN ('anais', 'apresentacoes')),
    versao               integer NOT NULL,
    chave_armazenamento  text,
    sha256               text,
    tamanho              bigint,
    total_paginas        integer,
    estado               text NOT NULL CHECK (estado IN ('em_preparacao', 'aguardando_revisao', 'publicado', 'substituido', 'falhou')),
    erro                 text NOT NULL DEFAULT '',
    gerado_por           uuid NOT NULL REFERENCES usuarios(id),
    gerado_em            timestamptz,
    publicado_por        uuid REFERENCES usuarios(id),
    publicado_em         timestamptz,
    criado_em            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (evento_id, tipo, versao)
);
CREATE UNIQUE INDEX publicacoes_evento_publicada_unica ON publicacoes_evento (evento_id, tipo) WHERE estado = 'publicado';

CREATE TABLE itens_publicacao (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    publicacao_id   uuid NOT NULL REFERENCES publicacoes_evento(id) ON DELETE CASCADE,
    trabalho_id     uuid NOT NULL REFERENCES trabalhos(id),
    arquivo_id      uuid REFERENCES arquivos_trabalho(id),
    posicao         integer NOT NULL,
    resultado       text NOT NULL CHECK (resultado IN ('incluido', 'ausente', 'invalido')),
    detalhe         text NOT NULL DEFAULT '',
    paginas         integer
);
CREATE INDEX itens_publicacao_idx ON itens_publicacao (publicacao_id, posicao);
