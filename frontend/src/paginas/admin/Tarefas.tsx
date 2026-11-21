import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Card, LinearProgress, MenuItem, Table, TableBody, TableCell, TableHead, TablePagination, TableRow, TextField, Typography } from '@mui/material';
import { api, mensagemDeErro } from '../../api/cliente';
import type { Pagina } from '../../api/tipos';
import { Carregando, EstadoChip, Titulo, Vazio } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { estadosTarefa, rotulo, tiposTarefa } from '../../utilitarios/rotulos';

interface TarefaAdmin { id: string; tipo: string; estado: string; progresso: number; tentativas: number; erro: string; avisos: string; solicitante: string; criadoEm: string; iniciadoEm: string | null; concluidoEm: string | null }

export function Tarefas() {
  const [parametros, setParametros] = useSearchParams();
  const estado = parametros.get('estado') ?? '';
  const [pagina, setPagina] = useState(0);
  const notificar = useNotificar();
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin', 'tarefas', estado, pagina],
    queryFn: () => api.get<Pagina<TarefaAdmin>>(`/admin/tarefas?estado=${estado}&pagina=${pagina + 1}&tamanho=25`),
    refetchInterval: 4000,
  });
  return (
    <>
      <Titulo titulo="Fila de documentos" subtitulo="Geração automática de declarações, ATAs, apresentações consolidadas e anais (XeLaTeX/qpdf no servidor)." />
      <TextField select size="small" label="Estado" value={estado} onChange={(e) => { setParametros(e.target.value ? { estado: e.target.value } : {}); setPagina(0); }} sx={{ mb: 2, minWidth: 200 }}>
        <MenuItem value="">Todos</MenuItem>
        {Object.entries(estadosTarefa).map(([v, r]) => <MenuItem key={v} value={v}>{r}</MenuItem>)}
      </TextField>
      {isLoading ? <Carregando /> : !data?.itens.length ? <Vazio>Nenhuma tarefa.</Vazio> : (
        <Card>
          <Box sx={{ overflowX: 'auto' }}>
            <Table size="small">
              <TableHead><TableRow><TableCell>Documento</TableCell><TableCell>Solicitante</TableCell><TableCell>Criada</TableCell><TableCell>Concluída</TableCell><TableCell>Estado</TableCell><TableCell /></TableRow></TableHead>
              <TableBody>
                {data.itens.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell>{rotulo(tiposTarefa, t.tipo)}<Typography variant="caption" display="block" color="text.secondary">tentativas: {t.tentativas}</Typography></TableCell>
                    <TableCell>{t.solicitante}</TableCell>
                    <TableCell>{t.criadoEm}</TableCell>
                    <TableCell>{t.concluidoEm ?? '—'}</TableCell>
                    <TableCell sx={{ minWidth: 200 }}>
                      <EstadoChip estado={t.estado} rotulo={rotulo(estadosTarefa, t.estado)} />
                      {t.estado === 'processando' && <LinearProgress variant="determinate" value={t.progresso} sx={{ mt: 1 }} />}
                      {t.erro && <Typography variant="caption" color="error" display="block" sx={{ mt: 0.5 }}>{t.erro}</Typography>}
                      {t.avisos && <Typography variant="caption" color="warning.main" display="block">{t.avisos}</Typography>}
                    </TableCell>
                    <TableCell>
                      {t.estado === 'falhou' && <Button size="small" variant="outlined" onClick={async () => {
                        try { await api.post(`/admin/tarefas/${t.id}/reprocessar`); notificar('Tarefa recolocada na fila.'); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
                      }}>Reprocessar</Button>}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
          <TablePagination component="div" count={data.total} page={pagina} rowsPerPage={25} rowsPerPageOptions={[25]} onPageChange={(_, p) => setPagina(p)} labelDisplayedRows={({ from, to, count }) => `${from}–${to} de ${count}`} />
        </Card>
      )}
    </>
  );
}
