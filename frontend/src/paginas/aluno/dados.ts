import { useQuery } from '@tanstack/react-query';
import { api } from '../../api/cliente';
import type { ArquivoTrabalho, EventoTrabalho, Parecer, ResumoATA, ResumoDeclaracao } from '../../api/tipos';

export interface PainelAlunoDados {
  aluno: { nome: string; email: string; matricula: string; curso: string };
  orientador: { vinculoId: string; id: string; nome: string; email: string; desde: string } | null;
  trabalho: {
    id: string; titulo: string; resumo: string; linkOverleaf: string; estado: string; submetidoEm: string | null;
    evento: EventoTrabalho | null; arquivos: Record<'artigo' | 'apresentacao', ArquivoTrabalho[]>; editavel: boolean;
  } | null;
  ata: ResumoATA | null;
  declaracao: ResumoDeclaracao;
  pareceres: Parecer[];
  proximosPassos: string[];
}

export function usePainelAluno() {
  return useQuery({
    queryKey: ['aluno', 'painel'],
    queryFn: () => api.get<PainelAlunoDados>('/aluno/painel'),
    refetchInterval: (q) => (q.state.data?.ata?.estado === 'gerando' ? 4000 : false),
  });
}
