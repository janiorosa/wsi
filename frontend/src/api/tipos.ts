export type Perfil = 'ADMIN' | 'DOCENTE' | 'ALUNO';

export interface Sessao {
  autenticado: boolean;
  usuario?: { id: string; nome: string; email: string };
  perfis: Perfil[];
  contexto: Perfil | '';
  precisaEscolherContexto: boolean;
  situacaoDocente: '' | 'pendente' | 'aprovado' | 'rejeitado';
  ehApresentador: boolean;
  csrf?: string;
}

export interface Pagina<T> {
  itens: T[];
  total: number;
  pagina: number;
  tamanho: number;
}

export interface Evento {
  id: string;
  nome: string;
  edicao: string;
  numeroEdicao: number | null;
  descricao: string;
  dataApresentacao: string;
  horarioApresentacao: string;
  local: string;
  campus: string;
  estado: string;
  prazoSubmissao: string | null;
  prazoApresentacao: string | null;
  informacoesPublicas: string;
  slug: string;
  temCapa: boolean;
  futuro: boolean;
  totalTrabalhos: number;
}

export interface SessaoEvento {
  id: string;
  titulo: string;
  data: string;
  horario: string;
  local: string;
  ordem: number;
}

export interface ArquivoTrabalho {
  id: string;
  tipo: 'artigo' | 'apresentacao';
  nomeOriginal: string;
  versao: number;
  tamanho: number;
  sha256: string;
  enviadoEm: string;
  enviadoPor: string;
}

export interface EventoTrabalho {
  id: string;
  nome: string;
  data: string;
  horario: string;
  local: string;
  sessaoId: string | null;
  sessao: string | null;
  futuro: boolean;
}

export interface Signatario {
  ordem: number;
  rotulo: string;
  nome: string;
  estado: 'pendente' | 'assinada';
  enviadoEm: string | null;
  souEu: boolean;
}

export interface ResumoATA {
  id: string;
  versao: number;
  estado: 'gerando' | 'aguardando_assinaturas' | 'finalizada';
  posicaoAtual: number;
  minhaVez: boolean;
  souSignatario: boolean;
  situacao: string;
  erroGeracao: string;
  geradaEm: string | null;
  signatarios: Signatario[];
  podeBaixar: boolean;
  podeBaixarOriginal: boolean;
  podeBaixarAssinada: boolean;
  assinaturasColetadas: number;
}

export interface Parecer {
  avaliador: string;
  papel: 'orientador' | 'examinador';
  parecer: string;
  atualizadoEm: string | null;
}

export interface TrabalhoPremiado {
  trabalhoId: string;
  titulo: string;
  aluno: string;
  orientador: string;
  votos: number;
}

export interface ResultadoPremiacao {
  id: string;
  nome: string;
  descricao: string;
  ordem: number;
  votantes: number;
  ranking: TrabalhoPremiado[];
  vencedores: TrabalhoPremiado[];
}

export interface ResumoDeclaracao {
  geradaId: string | null;
  geradaEm: string | null;
  assinadaId: string | null;
  assinadaEm: string | null;
  assinadaNomeOriginal: string | null;
  tarefaAtivaId: string | null;
}

export interface Tarefa {
  id: string;
  tipo: string;
  estado: 'pendente' | 'processando' | 'concluida' | 'falhou';
  progresso: number;
  erro: string;
  avisos: string;
  resultadoId: string | null;
  criadoEm: string;
}

export interface Publicacao {
  id: string;
  tipo: 'anais' | 'apresentacoes';
  versao: number;
  estado: 'em_preparacao' | 'aguardando_revisao' | 'publicado' | 'substituido' | 'falhou';
  totalPaginas: number | null;
  erro?: string;
  geradoEm: string | null;
  geradoPor?: string;
  publicadoEm: string | null;
  publicadoPor?: string | null;
  itens?: { posicao: number; titulo: string; aluno: string; resultado: 'incluido' | 'ausente' | 'invalido'; detalhe: string; paginas: number | null; versaoArquivo: number | null }[];
}

export interface DocenteResumo {
  id: string;
  nome: string;
}
