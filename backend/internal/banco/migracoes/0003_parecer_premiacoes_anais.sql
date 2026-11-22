-- Migração 0003: parecer descritivo da avaliação, premiações por evento com votação
-- dos docentes e número da edição (usado na capa dos anais).

-- Parecer: texto informativo do avaliador para o aluno corrigir o artigo.
-- É independente da nota: pode ser salvo antes dela e não entra no cálculo.
ALTER TABLE avaliacoes ADD COLUMN parecer text NOT NULL DEFAULT '';
ALTER TABLE avaliacoes ADD COLUMN parecer_atualizado_em timestamptz;

-- Número da edição do evento (ex.: 6 para o VI WSI): capa e folha de rosto dos anais.
ALTER TABLE eventos ADD COLUMN numero_edicao integer CHECK (numero_edicao BETWEEN 1 AND 3999);

CREATE TABLE premiacoes (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    evento_id   uuid NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    nome        text NOT NULL CHECK (length(trim(nome)) > 0),
    descricao   text NOT NULL DEFAULT '',
    ordem       integer NOT NULL DEFAULT 1,
    criado_por  uuid REFERENCES usuarios(id),
    criado_em   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (evento_id, nome)
);
CREATE INDEX premiacoes_evento_idx ON premiacoes (evento_id);

-- Cada docente vota em até dois trabalhos por premiação (limite verificado na API).
CREATE TABLE votos_premiacao (
    premiacao_id uuid NOT NULL REFERENCES premiacoes(id) ON DELETE CASCADE,
    docente_id   uuid NOT NULL REFERENCES usuarios(id),
    trabalho_id  uuid NOT NULL REFERENCES trabalhos(id),
    votado_em    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (premiacao_id, docente_id, trabalho_id)
);
CREATE INDEX votos_premiacao_trabalho_idx ON votos_premiacao (premiacao_id, trabalho_id);

-- Todo evento tem a premiação padrão.
INSERT INTO premiacoes (evento_id, nome, ordem)
SELECT id, 'Prêmio Destaque: Reconhecendo a Excelência Acadêmica', 1 FROM eventos;
