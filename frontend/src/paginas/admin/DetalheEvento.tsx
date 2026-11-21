import { useEffect, useState } from 'react';
import { Link as RouterLink, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Alert, Autocomplete, Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, IconButton,
  List, ListItem, ListItemText, Stack, Switch, Table, TableBody, TableCell, TableHead, TableRow, TextField, Tooltip, Typography,
} from '@mui/material';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward';
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import ImageIcon from '@mui/icons-material/Image';
import { api, enviarArquivo, mensagemDeErro } from '../../api/cliente';
import type { Evento, Publicacao, SessaoEvento } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { estadosEvento, estadosPublicacao, estadosTrabalho, formatarData, rotulo } from '../../utilitarios/rotulos';

interface TrabalhoProgramacao { id: string; titulo: string; aluno: string; orientador: string | null; estado: string; ordem: number | null; sessao: string | null; temArtigo: boolean; temApresentacao: boolean }
interface Detalhe {
  evento: Evento;
  sessoes: SessaoEvento[];
  trabalhos: TrabalhoProgramacao[];
  apresentadores: { docenteId: string; nome: string; podePublicar: boolean }[];
  publicacoes: Publicacao[];
}

export function DetalheEvento() {
  const { id } = useParams();
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const { data, isLoading, error, refetch } = useQuery({ queryKey: ['admin', 'evento', id], queryFn: () => api.get<Detalhe>(`/admin/eventos/${id}`) });
  const { data: usuariosDocentes } = useQuery({
    queryKey: ['admin', 'docentes-lista'],
    queryFn: () => api.get<{ itens: { id: string; nome: string }[] }>('/admin/usuarios?perfil=DOCENTE&tamanho=100'),
  });
  const [ordem, setOrdem] = useState<TrabalhoProgramacao[]>([]);
  const [sessaoEditando, setSessaoEditando] = useState<Partial<SessaoEvento> | null>(null);
  const [novoApresentador, setNovoApresentador] = useState<{ id: string; nome: string } | null>(null);
  const [excecao, setExcecao] = useState(false);

  useEffect(() => {
    if (data) setOrdem(data.trabalhos);
  }, [data]);

  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;
  const e = data.evento;
  const listaDocentes = usuariosDocentes?.itens ?? [];

  const executar = async (acao: () => Promise<unknown>, sucesso: string) => {
    try {
      await acao();
      notificar(sucesso);
      await refetch();
    } catch (err) {
      notificar(mensagemDeErro(err), 'error');
    }
  };
  const mover = (i: number, d: number) => {
    const n = [...ordem];
    const j = i + d;
    if (j < 0 || j >= n.length) return;
    [n[i], n[j]] = [n[j], n[i]];
    setOrdem(n);
  };
  const ordemAlterada = ordem.map((t) => t.id).join() !== data.trabalhos.map((t) => t.id).join();

  return (
    <>
      <Titulo
        titulo={`${e.nome} ${e.edicao}`}
        subtitulo={`${formatarData(e.dataApresentacao)} às ${e.horarioApresentacao} — ${e.local}`}
        acoes={
          <>
            <Button variant="contained" startIcon={<EditIcon />} component={RouterLink} to={`/admin/eventos/${id}/editar`}>Editar</Button>
            <Button startIcon={<OpenInNewIcon />} href={`/eventos/${e.slug}`} target="_blank">Página pública</Button>
            <Button component="label" startIcon={<ImageIcon />}>
              Capa
              <input hidden type="file" accept="image/png,image/jpeg" onChange={(ev) => {
                const f = ev.target.files?.[0];
                if (f) void executar(() => enviarArquivo(`/admin/eventos/${id}/capa`, f), 'Capa atualizada.');
              }} />
            </Button>
          </>
        }
      />
      <Stack direction="row" spacing={1} sx={{ mb: 3 }}>
        <EstadoChip estado={e.estado} rotulo={rotulo(estadosEvento, e.estado)} />
        <Chip size="small" label={e.futuro ? 'Apresentação futura' : 'Apresentação já ocorreu'} />
      </Stack>

      <Secao titulo="Trabalhos e programação" dica="A ordem definida aqui é usada na página pública, no PDF das apresentações e nos anais."
        acoes={
          <>
            <Button onClick={() => setExcecao(true)}>Associar trabalho (exceção)</Button>
            <Button variant="contained" disabled={!ordemAlterada} onClick={() => void executar(() => api.put(`/admin/eventos/${id}/programacao`, { trabalhoIds: ordem.map((t) => t.id) }), 'Programação salva.')}>Salvar ordem</Button>
          </>
        }>
        {ordem.length === 0 ? <Vazio>Nenhum trabalho associado. Os orientadores associam os trabalhos dos seus orientandos a eventos futuros.</Vazio> : (
          <Box sx={{ overflowX: 'auto' }}>
            <Table size="small">
              <TableHead><TableRow><TableCell>#</TableCell><TableCell>Trabalho</TableCell><TableCell>Aluno</TableCell><TableCell>Orientador</TableCell><TableCell>Arquivos</TableCell><TableCell>Situação</TableCell><TableCell align="right">Ordem</TableCell></TableRow></TableHead>
              <TableBody>
                {ordem.map((t, i) => (
                  <TableRow key={t.id}>
                    <TableCell>{i + 1}</TableCell>
                    <TableCell>{t.titulo || <em>sem título</em>}{t.sessao && <Typography variant="caption" display="block">{t.sessao}</Typography>}</TableCell>
                    <TableCell>{t.aluno}</TableCell>
                    <TableCell>{t.orientador ?? '—'}</TableCell>
                    <TableCell>
                      <Chip size="small" label="Artigo" color={t.temArtigo ? 'success' : 'default'} variant={t.temArtigo ? 'filled' : 'outlined'} sx={{ mr: 0.5 }} />
                      <Chip size="small" label="Apresentação" color={t.temApresentacao ? 'success' : 'default'} variant={t.temApresentacao ? 'filled' : 'outlined'} />
                    </TableCell>
                    <TableCell><EstadoChip estado={t.estado} rotulo={rotulo(estadosTrabalho, t.estado)} /></TableCell>
                    <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                      <IconButton size="small" aria-label="Mover para cima" disabled={i === 0} onClick={() => mover(i, -1)}><ArrowUpwardIcon fontSize="small" /></IconButton>
                      <IconButton size="small" aria-label="Mover para baixo" disabled={i === ordem.length - 1} onClick={() => mover(i, 1)}><ArrowDownwardIcon fontSize="small" /></IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
        )}
      </Secao>

      <Secao titulo="Apresentadores" dica="Docentes que geram e publicam o PDF das apresentações e os anais deste evento. Não recebem acesso a notas nem ATAs.">
        <List dense>
          {data.apresentadores.map((a) => (
            <ListItem key={a.docenteId} secondaryAction={
              <Stack direction="row" spacing={1} alignItems="center">
                <FormControlLabel control={<Switch checked={a.podePublicar} onChange={(_, v) => void executar(() => api.post(`/admin/eventos/${id}/apresentadores`, { docenteId: a.docenteId, podePublicar: v }), 'Permissão atualizada.')} />} label="Pode publicar" />
                <Tooltip title="Remover apresentador">
                  <IconButton aria-label="Remover apresentador" onClick={async () => {
                    const r = await confirmar({ titulo: 'Remover apresentador', mensagem: `Remover ${a.nome} como apresentador?`, perigoso: true, confirmar: 'Remover' });
                    if (r.confirmado) void executar(() => api.del(`/admin/eventos/${id}/apresentadores/${a.docenteId}`), 'Apresentador removido.');
                  }}><DeleteIcon /></IconButton>
                </Tooltip>
              </Stack>
            }>
              <ListItemText primary={a.nome} />
            </ListItem>
          ))}
        </List>
        {data.apresentadores.length === 0 && <Vazio>Nenhum apresentador designado.</Vazio>}
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mt: 1 }}>
          <Autocomplete sx={{ minWidth: 320 }} options={listaDocentes} getOptionLabel={(o) => o.nome} value={novoApresentador} onChange={(_, v) => setNovoApresentador(v)}
            renderInput={(p) => <TextField {...p} label="Docente" size="small" />} />
          <Button variant="contained" disabled={!novoApresentador} onClick={() => novoApresentador && void executar(() => api.post(`/admin/eventos/${id}/apresentadores`, { docenteId: novoApresentador.id }), 'Apresentador designado.').then(() => setNovoApresentador(null))}>
            Designar apresentador
          </Button>
        </Stack>
      </Secao>

      <Secao titulo="Sessões (opcional)" dica="Use sessões quando o evento ocorrer em mais de uma data, horário ou local. A ATA usará a sessão do trabalho."
        acoes={<Button onClick={() => setSessaoEditando({ titulo: '', data: e.dataApresentacao, horario: e.horarioApresentacao, local: e.local, ordem: data.sessoes.length + 1 })}>Nova sessão</Button>}>
        {data.sessoes.length === 0 ? <Vazio>Sem sessões: todos os trabalhos usam a data, o horário e o local do evento.</Vazio> : (
          <List dense>
            {data.sessoes.map((s) => (
              <ListItem key={s.id} secondaryAction={
                <>
                  <IconButton aria-label="Editar sessão" onClick={() => setSessaoEditando(s)}><EditIcon /></IconButton>
                  <IconButton aria-label="Remover sessão" onClick={async () => {
                    const r = await confirmar({ titulo: 'Remover sessão', mensagem: `Remover a sessão "${s.titulo}"?`, perigoso: true, confirmar: 'Remover' });
                    if (r.confirmado) void executar(() => api.del(`/admin/sessoes/${s.id}`), 'Sessão removida.');
                  }}><DeleteIcon /></IconButton>
                </>
              }>
                <ListItemText primary={s.titulo} secondary={`${formatarData(s.data)} às ${s.horario} — ${s.local}`} />
              </ListItem>
            ))}
          </List>
        )}
      </Secao>

      <Secao titulo="Documentos consolidados">
        {data.publicacoes.length === 0 ? <Vazio>Ainda não gerados pelo apresentador.</Vazio> : (
          <List dense>
            {data.publicacoes.map((p) => (
              <ListItem key={p.id}>
                <ListItemText primary={`${p.tipo === 'anais' ? 'Anais' : 'Apresentações'} — versão ${p.versao}`} secondary={`Gerado em ${p.geradoEm ?? '—'}${p.publicadoEm ? ` · publicado em ${p.publicadoEm}` : ''}`} />
                <EstadoChip estado={p.estado} rotulo={rotulo(estadosPublicacao, p.estado)} />
              </ListItem>
            ))}
          </List>
        )}
      </Secao>

      <DialogoSessao sessao={sessaoEditando} eventoId={id!} aoFechar={(salvou) => { setSessaoEditando(null); if (salvou) void refetch(); }} />
      <DialogoExcecao aberto={excecao} eventoId={id!} sessoes={data.sessoes} aoFechar={(salvou) => { setExcecao(false); if (salvou) void refetch(); }} />
    </>
  );
}

function DialogoSessao({ sessao, eventoId, aoFechar }: { sessao: Partial<SessaoEvento> | null; eventoId: string; aoFechar: (salvou: boolean) => void }) {
  const notificar = useNotificar();
  const [dados, setDados] = useState<Partial<SessaoEvento>>({});
  useEffect(() => setDados(sessao ?? {}), [sessao]);
  const salvar = async () => {
    try {
      if (dados.id) await api.put(`/admin/sessoes/${dados.id}`, dados);
      else await api.post(`/admin/eventos/${eventoId}/sessoes`, dados);
      notificar('Sessão salva.');
      aoFechar(true);
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <Dialog open={!!sessao} onClose={() => aoFechar(false)} maxWidth="sm" fullWidth>
      <DialogTitle>{dados.id ? 'Editar sessão' : 'Nova sessão'}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField label="Título" value={dados.titulo ?? ''} onChange={(e) => setDados({ ...dados, titulo: e.target.value })} />
          <TextField label="Data" type="date" InputLabelProps={{ shrink: true }} value={dados.data ?? ''} onChange={(e) => setDados({ ...dados, data: e.target.value })} />
          <TextField label="Horário" type="time" InputLabelProps={{ shrink: true }} value={dados.horario ?? ''} onChange={(e) => setDados({ ...dados, horario: e.target.value })} />
          <TextField label="Local" value={dados.local ?? ''} onChange={(e) => setDados({ ...dados, local: e.target.value })} />
          <TextField label="Ordem" type="number" value={dados.ordem ?? 0} onChange={(e) => setDados({ ...dados, ordem: Number(e.target.value) })} />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => aoFechar(false)}>Cancelar</Button>
        <Button variant="contained" onClick={() => void salvar()}>Salvar</Button>
      </DialogActions>
    </Dialog>
  );
}

interface TrabalhoAdmin { id: string; titulo: string; aluno: string; evento: string | null }

function DialogoExcecao({ aberto, eventoId, sessoes, aoFechar }: { aberto: boolean; eventoId: string; sessoes: SessaoEvento[]; aoFechar: (salvou: boolean) => void }) {
  const notificar = useNotificar();
  const [busca, setBusca] = useState('');
  const [trabalho, setTrabalho] = useState<TrabalhoAdmin | null>(null);
  const [sessaoId, setSessaoId] = useState('');
  const [justificativa, setJustificativa] = useState('');
  const { data } = useQuery({ queryKey: ['admin', 'trabalhos', busca], queryFn: () => api.get<TrabalhoAdmin[]>(`/admin/trabalhos?busca=${encodeURIComponent(busca)}`), enabled: aberto });
  const salvar = async () => {
    try {
      await api.post(`/admin/trabalhos/${trabalho!.id}/evento`, { eventoId, sessaoId, justificativa });
      notificar('Trabalho associado ao evento (exceção registrada na auditoria).');
      aoFechar(true);
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <Dialog open={aberto} onClose={() => aoFechar(false)} maxWidth="sm" fullWidth>
      <DialogTitle>Associar trabalho — exceção administrativa</DialogTitle>
      <DialogContent>
        <Alert severity="warning" sx={{ mb: 2 }}>Use somente quando o orientador não puder associar (ex.: evento já ocorrido). A justificativa fica registrada.</Alert>
        <Stack spacing={2}>
          <Autocomplete options={data ?? []} value={trabalho} onChange={(_, v) => setTrabalho(v)} onInputChange={(_, v) => setBusca(v)} filterOptions={(x) => x}
            getOptionLabel={(o) => `${o.aluno} — ${o.titulo || 'sem título'}${o.evento ? ` (${o.evento})` : ''}`}
            renderInput={(p) => <TextField {...p} label="Trabalho (busque por aluno ou título)" />} />
          {sessoes.length > 0 && (
            <TextField select SelectProps={{ native: true }} label="Sessão" value={sessaoId} onChange={(e) => setSessaoId(e.target.value)} InputLabelProps={{ shrink: true }}>
              <option value="">Selecione</option>
              {sessoes.map((s) => <option key={s.id} value={s.id}>{s.titulo}</option>)}
            </TextField>
          )}
          <TextField label="Justificativa" multiline minRows={2} value={justificativa} onChange={(e) => setJustificativa(e.target.value)} helperText="Mínimo de 10 caracteres." />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => aoFechar(false)}>Cancelar</Button>
        <Button variant="contained" disabled={!trabalho || justificativa.trim().length < 10} onClick={() => void salvar()}>Associar</Button>
      </DialogActions>
    </Dialog>
  );
}
