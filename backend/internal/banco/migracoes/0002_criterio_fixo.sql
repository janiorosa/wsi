-- Migração 0002: critério geral fixo "Nota".
-- O critério fixo existe sempre, está sempre ativo, não pode ser renomeado
-- e é aplicado a todos os trabalhos (portanto avaliado por todos os docentes da banca).
ALTER TABLE criterios_avaliacao ADD COLUMN fixo boolean NOT NULL DEFAULT false;
ALTER TABLE criterios_avaliacao ADD CONSTRAINT criterios_avaliacao_fixo_ativo CHECK (NOT fixo OR ativo);
CREATE UNIQUE INDEX criterios_avaliacao_fixo_unico ON criterios_avaliacao (fixo) WHERE fixo;

CREATE FUNCTION impedir_exclusao_criterio_fixo() RETURNS trigger AS $$
BEGIN
    IF OLD.fixo THEN
        RAISE EXCEPTION 'o critério fixo "Nota" não pode ser excluído' USING ERRCODE = 'check_violation';
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER criterios_avaliacao_fixo_sem_exclusao
    BEFORE DELETE ON criterios_avaliacao
    FOR EACH ROW EXECUTE FUNCTION impedir_exclusao_criterio_fixo();
