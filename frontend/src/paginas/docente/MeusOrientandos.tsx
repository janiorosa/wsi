import { useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Autocomplete, Box, Button, Card, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Stack, Table, TableBody, TableCell, TableHead, TableRow, TextField,
} from '@mui/material';
import PersonAddIcon from '@mui/icons-material/PersonAdd';
import LinkIcon from '@mui/icons-material/Link';
import { api, mensagemDeErro } from '../../api/cliente';
import { Carregando, ErroCarregamento, EstadoChip, Titulo, Vazio } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { estadosTrabalho, rotulo } from '../../utilitarios/rotulos';
import type { Orientando } from './PainelDocente';

interface AlunoDisponivel { id: string; nome: string; email: string; matricula: string }

export function MeusOrientandos() {
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const [busca, setBusca] = useState('');
  const [vincular, setVincular] = useState(false);
  const [cadastrar, setCadastrar] = useState(false);
  const { data, isLoading, error, refetch } = useQuery({ queryKey: ['docente', 'orientandos', busca], queryFn: () => api.get<Orientando[]>(`/docente/orientandos?busca=${encodeURIComponent(busca)}`) });

  return (
    <>
      <Titulo
        titulo="Meus orientandos"
        acoes={
          <>
            <Button variant="contained" startIcon={<LinkIcon />} onClick={() => setVincular(true)}>Vincular aluno</Button>
            <Button variant="outlined" startIcon={<PersonAddIcon />} onClick={() => setCadastrar(true)}>Cadastrar aluno</Button>
          </>
        }
      />
      <TextField size="small" label="Pesquisar por nome ou matrícula" value={busca} onChange={(e) => setBusca(e.target.value)} sx={{ mb: 2, minWidth: 320 }} />
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (data.length === 0 ? <Vazio>Nenhum orientando. Use “Vincular aluno” para orientar alunos que ainda não têm orientador.</Vazio> : (
        <Card><Box sx={{ overflowX: 'auto' }}>
          <Table>
            <TableHead><TableRow><TableCell>Aluno</TableCell><TableCell>Trabalho</TableCell><TableCell>Evento</TableCell><TableCell>Pendências</TableCell><TableCell /></TableRow></TableHead>
            <TableBody>
              {data.map((o) => (
                <TableRow key={o.vinculoId} hover>
                  <TableCell><strong>{o.nome}</strong><Box component="span" sx={{ display: 'block', fontSize: 13, color: 'text.secondary' }}>{o.matricula || 'sem matrícula'} · desde {o.desde}</Box></TableCell>
                  <TableCell>{o.titulo || <em>não cadastrado</em>}{o.estadoTrabalho && <Box sx={{ mt: 0.5 }}><EstadoChip estado={o.estadoTrabalho} rotulo={rotulo(estadosTrabalho, o.estadoTrabalho)} /></Box>}</TableCell>
                  <TableCell>{o.evento ?? '—'}</TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                      {o.pendencias.length === 0 ? <Chip size="small" color="success" label="Sem pendências" /> : o.pendencias.map((p) => <Chip key={p} size="small" variant="outlined" color="warning" label={p} />)}
                    </Stack>
                  </TableCell>
                  <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                    {o.trabalhoId && <Button size="small" variant="contained" component={RouterLink} to={`/docente/trabalhos/${o.trabalhoId}`}>Abrir ficha</Button>}
                    <Button size="small" color="error" onClick={async () => {
                      const r = await confirmar({ titulo: 'Encerrar orientação', mensagem: `Encerrar a orientação de ${o.nome}? Histórico e trabalhos são preservados.`, perigoso: true, confirmar: 'Encerrar', pedirMotivo: 'Motivo (opcional, mas recomendado)' });
                      if (!r.confirmado) return;
                      try { await api.post(`/docente/vinculos/${o.vinculoId}/encerrar`, { motivo: r.motivo }); notificar('Orientação encerrada.'); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
                    }}>Encerrar</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box></Card>
      ))}
      <DialogoVincular aberto={vincular} aoFechar={(ok) => { setVincular(false); if (ok) void refetch(); }} />
      <DialogoCadastrar aberto={cadastrar} aoFechar={(criado) => { setCadastrar(false); if (criado) void refetch(); }} />
    </>
  );
}

function DialogoVincular({ aberto, aoFechar }: { aberto: boolean; aoFechar: (ok: boolean) => void }) {
  const notificar = useNotificar();
  const [busca, setBusca] = useState('');
  const [aluno, setAluno] = useState<AlunoDisponivel | null>(null);
  const { data } = useQuery({ queryKey: ['docente', 'disponiveis', busca], queryFn: () => api.get<AlunoDisponivel[]>(`/docente/alunos-disponiveis?busca=${encodeURIComponent(busca)}`), enabled: aberto });
  return (
    <Dialog open={aberto} onClose={() => aoFechar(false)} maxWidth="sm" fullWidth>
      <DialogTitle>Vincular aluno sem orientador</DialogTitle>
      <DialogContent>
        <Autocomplete sx={{ mt: 1 }} options={data ?? []} value={aluno} filterOptions={(x) => x} onChange={(_, v) => setAluno(v)} onInputChange={(_, v) => setBusca(v)}
          getOptionLabel={(o) => `${o.nome} — ${o.matricula || o.email}`} noOptionsText="Nenhum aluno sem orientador encontrado"
          renderInput={(p) => <TextField {...p} label="Buscar aluno por nome, matrícula ou e-mail" />} />
      </DialogContent>
      <DialogActions>
        <Button onClick={() => aoFechar(false)}>Cancelar</Button>
        <Button variant="contained" disabled={!aluno} onClick={async () => {
          try { await api.post('/docente/orientandos', { alunoId: aluno!.id }); notificar('Aluno vinculado.'); setAluno(null); aoFechar(true); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
        }}>Vincular</Button>
      </DialogActions>
    </Dialog>
  );
}

function DialogoCadastrar({ aberto, aoFechar }: { aberto: boolean; aoFechar: (criado: boolean) => void }) {
  const notificar = useNotificar();
  const inicial = { nome: '', email: '', telefone: '', identificador: '' };
  const [d, setD] = useState(inicial);
  const [erros, setErros] = useState<Record<string, string>>({});
  return (
    <Dialog open={aberto} onClose={() => aoFechar(false)} maxWidth="sm" fullWidth>
      <DialogTitle>Cadastrar aluno e vincular como orientando</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField label="Nome completo" value={d.nome} error={!!erros.nome} helperText={erros.nome} onChange={(e) => setD({ ...d, nome: e.target.value })} />
          <TextField label="E-mail" value={d.email} error={!!erros.email} helperText={erros.email} onChange={(e) => setD({ ...d, email: e.target.value })} />
          <TextField label="Telefone" value={d.telefone} onChange={(e) => setD({ ...d, telefone: e.target.value })} />
          <TextField required label="Matrícula" value={d.identificador} error={!!erros.identificador} helperText={erros.identificador ?? 'O aluno criará a senha em “Primeiro acesso” com nome completo, e-mail e matrícula.'} onChange={(e) => setD({ ...d, identificador: e.target.value })} />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => aoFechar(false)}>Cancelar</Button>
        <Button variant="contained" onClick={async () => {
          setErros({});
          try {
            await api.post('/docente/orientandos/novo', d);
            notificar('Aluno cadastrado e vinculado. Oriente o aluno a criar a senha em “Primeiro acesso”.');
            setD(inicial);
            aoFechar(true);
          } catch (e) {
            const campos = (e as { campos?: Record<string, string> }).campos;
            if (campos && Object.keys(campos).length) setErros(campos); else notificar(mensagemDeErro(e), 'error');
          }
        }}>Cadastrar</Button>
      </DialogActions>
    </Dialog>
  );
}
