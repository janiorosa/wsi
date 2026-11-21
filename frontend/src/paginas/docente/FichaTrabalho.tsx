import { useEffect, useState } from 'react';
import { Link as RouterLink, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Alert, Autocomplete, Box, Button, Chip, FormControlLabel, IconButton, Link, List, ListItem, ListItemText, MenuItem, Stack, Switch,
  Table, TableBody, TableCell, TableHead, TableRow, TextField, Tooltip, Typography,
} from '@mui/material';
import PictureAsPdfIcon from '@mui/icons-material/PictureAsPdf';
import DeleteIcon from '@mui/icons-material/Delete';
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward';
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward';
import DescriptionIcon from '@mui/icons-material/Description';
import { api, mensagemDeErro, urlApi } from '../../api/cliente';
import type { ArquivoTrabalho, DocenteResumo, Evento, EventoTrabalho, ResumoATA, ResumoDeclaracao, SessaoEvento } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { PainelAssinaturaATA } from '../../componentes/PainelAssinaturaATA';
import { AcompanharTarefa } from '../../componentes/AcompanharTarefa';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { estadosAvaliacao, estadosTrabalho, formatarTamanho, rotulo } from '../../utilitarios/rotulos';

interface Membro { docenteId: string; nome: string; papel: 'orientador' | 'examinador'; ordem: number; estadoAvaliacao: string; nota?: string | null }
interface Criterio { id: string; nome: string; descricao: string; origem: 'geral' | 'extra' }
interface Ficha {
  papeis: { orientador: boolean; banca: boolean; apresentador: boolean };
  trabalho: { id: string; titulo: string; resumo: string; linkOverleaf: string; estado: string; submetidoEm: string | null; orientador: string | null };
  aluno: { id: string; nome: string; email: string; matricula: string };
  evento: EventoTrabalho | null;
  arquivos: Record<'artigo' | 'apresentacao', ArquivoTrabalho[]>;
  banca?: Membro[];
  criterios?: Criterio[];
  criteriosBloqueados?: boolean;
  ata?: ResumoATA | null;
  notaFinalPrevia?: string;
  aprovadoPrevio?: boolean;
  podeGerarATA?: boolean;
  declaracao?: ResumoDeclaracao;
  historico?: { estadoAnterior: string; estadoNovo: string; ator: string | null; observacao: string; data: string }[];
}

export function FichaTrabalho() {
  const { id } = useParams();
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const [tarefaATA, setTarefaATA] = useState<string | null>(null);
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['docente', 'ficha', id],
    queryFn: () => api.get<Ficha>(`/docente/trabalhos/${id}`),
    refetchInterval: (q) => (q.state.data?.ata?.estado === 'gerando' ? 3000 : false),
  });

  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;
  const orientador = data.papeis.orientador;

  const executar = async (acao: () => Promise<unknown>, sucesso: string) => {
    try {
      await acao();
      notificar(sucesso);
      await refetch();
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };

  return (
    <>
      <Titulo
        titulo={data.trabalho.titulo || 'Trabalho sem título'}
        subtitulo={<>{data.aluno.nome} · matrícula {data.aluno.matricula || '—'} · {data.aluno.email}</>}
        acoes={
          <Stack direction="row" spacing={1} alignItems="center">
            <EstadoChip estado={data.trabalho.estado} rotulo={rotulo(estadosTrabalho, data.trabalho.estado)} />
            {orientador && <Chip size="small" label="Você é o orientador" color="primary" variant="outlined" />}
            {!orientador && data.papeis.banca && <Chip size="small" label="Você é examinador" color="primary" variant="outlined" />}
            {data.papeis.apresentador && <Chip size="small" label="Apresentador do evento" variant="outlined" />}
          </Stack>
        }
      />

      <Secao titulo="Resumo e arquivos">
        <Typography sx={{ whiteSpace: 'pre-line', mb: 2 }}>{data.trabalho.resumo || <em>Resumo não cadastrado.</em>}</Typography>
        {data.trabalho.linkOverleaf && <Typography sx={{ mb: 2 }}>Overleaf: <Link href={data.trabalho.linkOverleaf} target="_blank" rel="noopener">{data.trabalho.linkOverleaf}</Link></Typography>}
        <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
          {(['artigo', 'apresentacao'] as const).map((tipo) => (
            <Box key={tipo}>
              <Typography fontWeight={700}>{tipo === 'artigo' ? 'Artigo' : 'Apresentação'}</Typography>
              {data.arquivos[tipo].length === 0 ? <Vazio>Não enviado.</Vazio> : (
                <List dense>
                  {data.arquivos[tipo].map((a, i) => (
                    <ListItem key={a.id} disableGutters>
                      <Button size="small" startIcon={<PictureAsPdfIcon />} href={urlApi(`/arquivos/${a.id}?visualizar=1`)} target="_blank" rel="noopener" variant={i === 0 ? 'contained' : 'text'}>
                        Versão {a.versao}{i === 0 ? ' (atual)' : ''}
                      </Button>
                      <Typography variant="caption" color="text.secondary" sx={{ ml: 1 }}>{a.enviadoEm} · {formatarTamanho(a.tamanho)}</Typography>
                    </ListItem>
                  ))}
                </List>
              )}
            </Box>
          ))}
        </Box>
      </Secao>

      {(orientador || data.evento) && (
        <Secao titulo="Evento da apresentação" dica="A data, o horário e o local impressos na ATA vêm sempre deste evento (ou da sessão), nunca da data de geração do PDF.">
          {data.evento ? (
            <Typography sx={{ mb: orientador ? 2 : 0 }}>
              <strong>{data.evento.nome}</strong> — {data.evento.data} às {data.evento.horario} — {data.evento.local}{data.evento.sessao ? ` (${data.evento.sessao})` : ''}
            </Typography>
          ) : <Alert severity="info" sx={{ mb: 2 }}>Trabalho ainda não associado a um evento.</Alert>}
          {orientador && !data.ata && <AssociarEvento trabalhoId={data.trabalho.id} atual={data.evento} aoSalvar={() => void refetch()} />}
        </Secao>
      )}

      {data.banca && (
        <Secao titulo="Banca avaliadora" dica="O orientador integra a banca. Configuração usual: orientador + dois examinadores.">
          <Table size="small" sx={{ mb: 2 }}>
            <TableHead><TableRow><TableCell>Membro</TableCell><TableCell>Papel na ATA</TableCell><TableCell>Avaliação</TableCell>{orientador && <TableCell>Nota</TableCell>}<TableCell /></TableRow></TableHead>
            <TableBody>
              {data.banca.map((m) => (
                <TableRow key={m.docenteId}>
                  <TableCell>{m.nome}</TableCell>
                  <TableCell>{m.papel === 'orientador' ? 'Professor Orientador' : `Examinador ${m.ordem}`}</TableCell>
                  <TableCell><EstadoChip estado={m.estadoAvaliacao} rotulo={rotulo(estadosAvaliacao, m.estadoAvaliacao)} /></TableCell>
                  {orientador && <TableCell>{m.nota ?? '—'}</TableCell>}
                  <TableCell align="right">
                    {orientador && m.estadoAvaliacao === 'finalizada' && !data.ata && (
                      <Button size="small" onClick={async () => {
                        const r = await confirmar({ titulo: 'Reabrir avaliação', mensagem: `Reabrir a avaliação de ${m.nome}? O docente poderá alterar as notas.`, pedirMotivo: 'Motivo da reabertura', confirmar: 'Reabrir' });
                        if (r.confirmado) void executar(() => api.post(`/docente/trabalhos/${id}/avaliacoes/${m.docenteId}/reabrir`, { motivo: r.motivo }), 'Avaliação reaberta.');
                      }}>Reabrir</Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          {orientador && data.notaFinalPrevia && (
            <Alert severity={data.aprovadoPrevio ? 'success' : 'error'} sx={{ mb: 2 }}>
              Nota final (média simples dos avaliadores): <strong>{data.notaFinalPrevia}</strong> — <strong>{data.aprovadoPrevio ? 'Aprovado' : 'Reprovado'}</strong> (mínimo para aprovação: 60,00)
            </Alert>
          )}
          {orientador && !data.ata && <DefinirBanca trabalhoId={data.trabalho.id} banca={data.banca} aoSalvar={() => void refetch()} />}
          {data.papeis.banca && (
            <Button sx={{ mt: 2 }} variant="contained" component={RouterLink} to={`/docente/avaliar/${data.trabalho.id}`}>AVALIAR este trabalho</Button>
          )}
        </Secao>
      )}

      {data.criterios && (
        <Secao titulo="Critérios de avaliação" dica="Todos de 0 a 100, sem pesos. Critérios extras valem só para este trabalho e aparecem para todos os avaliadores.">
          {data.criterios.length === 0 ? <Vazio>Os critérios gerais são aplicados quando a banca é definida.</Vazio> : (
            <List dense>
              {data.criterios.map((c) => (
                <ListItem key={c.id} secondaryAction={orientador && c.origem === 'extra' && !data.criteriosBloqueados ? (
                  <Tooltip title="Remover critério extra"><IconButton aria-label="Remover critério extra" onClick={async () => {
                    const r = await confirmar({ titulo: 'Remover critério extra', mensagem: `Remover "${c.nome}"?`, perigoso: true, confirmar: 'Remover' });
                    if (r.confirmado) void executar(() => api.del(`/docente/trabalhos/${id}/criterios/${c.id}`), 'Critério removido.');
                  }}><DeleteIcon /></IconButton></Tooltip>
                ) : undefined}>
                  <ListItemText primary={<>{c.nome} <Chip size="small" label={c.origem === 'extra' ? 'extra' : 'geral'} variant="outlined" sx={{ ml: 1 }} /></>} secondary={c.descricao} />
                </ListItem>
              ))}
            </List>
          )}
          {data.criteriosBloqueados && <Alert severity="info">Critérios congelados: já existe avaliação finalizada.</Alert>}
          {orientador && !data.criteriosBloqueados && <NovoCriterioExtra trabalhoId={data.trabalho.id} aoSalvar={() => void refetch()} />}
        </Secao>
      )}

      {(data.ata || orientador) && data.banca && (
        <Secao titulo="ATA de defesa" dica="Ordem de assinatura: aluno, orientador e examinadores. Cada trabalho tem seu próprio fluxo.">
          {tarefaATA && <AcompanharTarefa tarefaId={tarefaATA} texto="Gerando a ATA..." aoConcluir={() => { setTarefaATA(null); void refetch(); }} />}
          {data.ata ? <PainelAssinaturaATA ata={data.ata} aoAtualizar={() => void refetch()} /> : (
            data.podeGerarATA ? (
              <Button variant="contained" size="large" startIcon={<DescriptionIcon />} onClick={async () => {
                try { const r = await api.post<{ tarefaId: string }>(`/docente/trabalhos/${id}/ata`); setTarefaATA(r.tarefaId); notificar('Geração da ATA iniciada.'); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
              }}>GERAR ATA</Button>
            ) : <Vazio>A ATA será gerada automaticamente quando todas as avaliações da banca estiverem concluídas e o trabalho estiver associado a um evento.</Vazio>
          )}
          {orientador && data.ata && (
            <Stack direction="row" spacing={2} sx={{ mt: 3 }}>
              {(data.ata.estado === 'gerando' || (data.ata.estado === 'aguardando_assinaturas' && data.ata.posicaoAtual === 1)) && (
                <Button variant="outlined" onClick={async () => {
                  const r = await confirmar({ titulo: 'Gerar novamente o PDF da ATA', mensagem: 'O PDF original será gerado de novo com os dados atuais do evento. Ninguém assinou ainda.' });
                  if (!r.confirmado) return;
                  try { const resp = await api.post<{ tarefaId: string }>(`/docente/trabalhos/${id}/ata`); setTarefaATA(resp.tarefaId); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
                }}>Gerar PDF novamente</Button>
              )}
              <ReiniciarATA trabalhoId={data.trabalho.id} aoConcluir={(t) => { if (t) setTarefaATA(t); void refetch(); }} />
            </Stack>
          )}
        </Secao>
      )}

      {orientador && data.declaracao && (
        <Secao titulo="Declaração anti-plágio do orientando">
          {data.declaracao.assinadaId ? (
            <Button variant="outlined" startIcon={<PictureAsPdfIcon />} href={urlApi(`/declaracoes-assinadas/${data.declaracao.assinadaId}`)}>
              Baixar declaração assinada (enviada em {data.declaracao.assinadaEm})
            </Button>
          ) : <Alert severity="warning">O aluno ainda não enviou a declaração assinada.</Alert>}
        </Secao>
      )}

      {orientador && data.historico && (
        <Secao titulo="Histórico do trabalho">
          <List dense>
            {data.historico.map((h, i) => (
              <ListItem key={i} disableGutters>
                <ListItemText primary={`${rotulo(estadosTrabalho, h.estadoAnterior) === '—' ? 'Criado' : rotulo(estadosTrabalho, h.estadoAnterior)} → ${rotulo(estadosTrabalho, h.estadoNovo)}`} secondary={`${h.data}${h.ator ? ` · ${h.ator}` : ''}${h.observacao ? ` · ${h.observacao}` : ''}`} />
              </ListItem>
            ))}
          </List>
        </Secao>
      )}
    </>
  );
}

function AssociarEvento({ trabalhoId, atual, aoSalvar }: { trabalhoId: string; atual: EventoTrabalho | null; aoSalvar: () => void }) {
  const notificar = useNotificar();
  const { data } = useQuery({ queryKey: ['docente', 'eventos-futuros'], queryFn: () => api.get<{ evento: Evento; sessoes: SessaoEvento[] }[]>('/docente/eventos-futuros') });
  const [eventoId, setEventoId] = useState(atual?.id ?? '');
  const [sessaoId, setSessaoId] = useState(atual?.sessaoId ?? '');
  const escolhido = data?.find((d) => d.evento.id === eventoId);
  return (
    <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems={{ md: 'center' }}>
      <TextField select size="small" label="Evento futuro" value={eventoId} onChange={(e) => { setEventoId(e.target.value); setSessaoId(''); }} sx={{ minWidth: 320 }}>
        {(data ?? []).length === 0 && <MenuItem value="" disabled>Nenhum evento futuro aberto</MenuItem>}
        {data?.map((d) => <MenuItem key={d.evento.id} value={d.evento.id}>{d.evento.nome} {d.evento.edicao} — {d.evento.dataApresentacao.split('-').reverse().join('/')}</MenuItem>)}
      </TextField>
      {escolhido && escolhido.sessoes.length > 0 && (
        <TextField select size="small" label="Sessão" value={sessaoId} onChange={(e) => setSessaoId(e.target.value)} sx={{ minWidth: 240 }}>
          {escolhido.sessoes.map((s) => <MenuItem key={s.id} value={s.id}>{s.titulo} — {s.data.split('-').reverse().join('/')} {s.horario}</MenuItem>)}
        </TextField>
      )}
      <Button variant="contained" disabled={!eventoId} onClick={async () => {
        try { await api.put(`/docente/trabalhos/${trabalhoId}/evento`, { eventoId, sessaoId }); notificar('Trabalho associado ao evento.'); aoSalvar(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
      }}>{atual ? 'Alterar evento' : 'Associar ao evento'}</Button>
    </Stack>
  );
}

function DefinirBanca({ trabalhoId, banca, aoSalvar }: { trabalhoId: string; banca: Membro[]; aoSalvar: () => void }) {
  const notificar = useNotificar();
  const { data: docentes } = useQuery({ queryKey: ['docente', 'docentes'], queryFn: () => api.get<DocenteResumo[]>('/docente/docentes') });
  const orientadorId = banca.find((m) => m.papel === 'orientador')?.docenteId;
  const [escolhidos, setEscolhidos] = useState<DocenteResumo[]>([]);
  useEffect(() => {
    setEscolhidos(banca.filter((m) => m.papel === 'examinador').map((m) => ({ id: m.docenteId, nome: m.nome })));
  }, [banca]);
  const mover = (i: number, d: number) => {
    const n = [...escolhidos];
    const j = i + d;
    if (j < 0 || j >= n.length) return;
    [n[i], n[j]] = [n[j], n[i]];
    setEscolhidos(n);
  };
  return (
    <Box>
      <Typography fontWeight={700} sx={{ mb: 1 }}>Designar examinadores (na ordem: Examinador 1, 2, ...)</Typography>
      <Autocomplete multiple options={(docentes ?? []).filter((d) => d.id !== orientadorId)} value={escolhidos} onChange={(_, v) => setEscolhidos(v)}
        isOptionEqualToValue={(a, b) => a.id === b.id} getOptionLabel={(o) => o.nome} filterSelectedOptions
        renderInput={(p) => <TextField {...p} label="Docentes examinadores" placeholder="Digite o nome" />} />
      {escolhidos.length > 1 && (
        <List dense>
          {escolhidos.map((d, i) => (
            <ListItem key={d.id} secondaryAction={
              <>
                <IconButton size="small" aria-label="Subir" disabled={i === 0} onClick={() => mover(i, -1)}><ArrowUpwardIcon fontSize="small" /></IconButton>
                <IconButton size="small" aria-label="Descer" disabled={i === escolhidos.length - 1} onClick={() => mover(i, 1)}><ArrowDownwardIcon fontSize="small" /></IconButton>
              </>
            }>
              <ListItemText primary={`Examinador ${i + 1}: ${d.nome}`} />
            </ListItem>
          ))}
        </List>
      )}
      <Button sx={{ mt: 1 }} variant="contained" disabled={escolhidos.length === 0} onClick={async () => {
        try { await api.put(`/docente/trabalhos/${trabalhoId}/banca`, { examinadores: escolhidos.map((d) => d.id) }); notificar('Banca salva.'); aoSalvar(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
      }}>Salvar banca</Button>
    </Box>
  );
}

function NovoCriterioExtra({ trabalhoId, aoSalvar }: { trabalhoId: string; aoSalvar: () => void }) {
  const notificar = useNotificar();
  const [nome, setNome] = useState('');
  const [descricao, setDescricao] = useState('');
  return (
    <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mt: 2 }}>
      <TextField size="small" label="Nome do critério extra" value={nome} onChange={(e) => setNome(e.target.value)} />
      <TextField size="small" label="Descrição" value={descricao} onChange={(e) => setDescricao(e.target.value)} sx={{ flexGrow: 1 }} />
      <Button variant="outlined" disabled={!nome.trim()} onClick={async () => {
        try { await api.post(`/docente/trabalhos/${trabalhoId}/criterios`, { nome, descricao }); notificar('Critério extra adicionado.'); setNome(''); setDescricao(''); aoSalvar(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
      }}>Adicionar critério (0 a 100)</Button>
    </Stack>
  );
}

function ReiniciarATA({ trabalhoId, aoConcluir }: { trabalhoId: string; aoConcluir: (tarefa: string | null) => void }) {
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const [gerarNova, setGerarNova] = useState(true);
  return (
    <Stack direction="row" spacing={1} alignItems="center">
      <Button color="error" variant="outlined" onClick={async () => {
        const r = await confirmar({
          titulo: 'Reiniciar processo da ATA',
          mensagem: <>A ATA atual e as assinaturas já enviadas serão canceladas (os metadados ficam na auditoria). {gerarNova ? 'Uma nova ATA será gerada com a banca e as notas atuais.' : 'Você poderá corrigir a banca ou reabrir avaliações antes de gerar outra.'}</>,
          perigoso: true, confirmar: 'Reiniciar', pedirMotivo: 'Motivo do reinício',
        });
        if (!r.confirmado) return;
        try {
          const resp = await api.post<{ tarefaId?: string }>(`/docente/trabalhos/${trabalhoId}/ata/reiniciar`, { motivo: r.motivo, gerarNova });
          notificar('Processo da ATA reiniciado.');
          aoConcluir(resp.tarefaId ?? null);
        } catch (e) { notificar(mensagemDeErro(e), 'error'); }
      }}>Reiniciar processo da ATA</Button>
      <FormControlLabel control={<Switch size="small" checked={gerarNova} onChange={(_, v) => setGerarNova(v)} />} label="gerar nova em seguida" />
    </Stack>
  );
}
