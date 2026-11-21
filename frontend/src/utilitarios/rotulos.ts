// Rótulos em português para os estados do sistema.
export const estadosTrabalho: Record<string, string> = {
  rascunho: 'Rascunho',
  submetido: 'Submetido',
  aguardando_avaliadores: 'Aguardando avaliadores',
  aguardando_avaliacoes: 'Aguardando avaliações',
  avaliacoes_concluidas: 'Avaliações concluídas',
  ata_aguardando_geracao: 'ATA aguardando geração',
  ata_gerada: 'ATA gerada',
  assinaturas_em_andamento: 'Assinaturas em andamento',
  ata_finalizada: 'ATA finalizada',
  documentacao_pendente: 'Documentação pendente',
  concluido: 'Concluído',
  cancelado: 'Cancelado',
};

export const estadosEvento: Record<string, string> = {
  rascunho: 'Rascunho',
  aberto: 'Aberto para submissões',
  em_preparacao: 'Em preparação',
  em_andamento: 'Em andamento',
  encerrado: 'Encerrado',
  publicado: 'Publicado',
};

export const estadosPublicacao: Record<string, string> = {
  em_preparacao: 'Em preparação',
  aguardando_revisao: 'Aguardando revisão',
  publicado: 'Publicado',
  substituido: 'Substituído',
  falhou: 'Falhou',
};

export const estadosTarefa: Record<string, string> = {
  pendente: 'Na fila',
  processando: 'Processando',
  concluida: 'Concluída',
  falhou: 'Falhou',
};

export const tiposTarefa: Record<string, string> = {
  gerar_declaracao: 'Declaração anti-plágio',
  gerar_ata: 'ATA',
  gerar_apresentacoes: 'PDF das apresentações',
  gerar_anais: 'Anais do evento',
};

export const estadosAvaliacao: Record<string, string> = {
  pendente: 'Pendente',
  rascunho: 'Rascunho salvo',
  finalizada: 'Finalizada',
};

export type Tom = 'default' | 'primary' | 'success' | 'warning' | 'error' | 'info';

export function tomDoEstado(estado: string): Tom {
  if (['concluido', 'finalizada', 'publicado', 'concluida', 'aprovado', 'ativo', 'incluido'].includes(estado)) return 'success';
  if (['falhou', 'cancelado', 'rejeitado', 'invalido', 'bloqueado'].includes(estado)) return 'error';
  if (['documentacao_pendente', 'aguardando_revisao', 'pendente', 'ausente', 'aguardando_primeiro_acesso'].includes(estado)) return 'warning';
  if (['rascunho', 'substituido', 'encerrado'].includes(estado)) return 'default';
  return 'info';
}

export function rotulo(mapa: Record<string, string>, chave: string | null | undefined): string {
  if (!chave) return '—';
  return mapa[chave] ?? chave;
}

export function formatarData(iso: string | null | undefined): string {
  if (!iso) return '—';
  const [a, m, d] = iso.split('-');
  return d && m && a ? `${d}/${m}/${a}` : iso;
}

export function formatarTamanho(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1).replace('.', ',')} MB`;
}
