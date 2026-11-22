import { useState } from 'react';
import { Alert, Box, Button, Card, CardContent, Chip, IconButton, Stack, Table, TableBody, TableCell, TableHead, TableRow, TextField, Tooltip, Typography } from '@mui/material';
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import { api, mensagemDeErro } from '../api/cliente';
import type { ResultadoPremiacao } from '../api/tipos';
import { useNotificar } from './Notificacoes';
import { useConfirmar } from './Confirmacao';
import { Vazio } from './Comuns';

// Resultado das premiações de um evento (votos dos docentes). Com eventoId, o ADMIN também cadastra,
// renomeia e remove premiações; sem eventoId, somente leitura (apresentador).
export function ResultadoPremiacoes({ premiacoes, eventoId, aoAlterar }: { premiacoes: ResultadoPremiacao[]; eventoId?: string; aoAlterar?: () => void }) {
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const [nome, setNome] = useState('');
  const [descricao, setDescricao] = useState('');
  const [editando, setEditando] = useState<{ id: string; nome: string; descricao: string } | null>(null);
  const gerenciar = !!eventoId;

  const executar = async (acao: () => Promise<unknown>, sucesso: string) => {
    try {
      await acao();
      notificar(sucesso);
      aoAlterar?.();
      return true;
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
      return false;
    }
  };

  return (
    <Box>
      {premiacoes.length === 0 && <Vazio>Nenhuma premiação cadastrada.</Vazio>}
      <Stack spacing={2}>
        {premiacoes.map((p) => (
          <Card key={p.id} variant="outlined">
            <CardContent>
              {editando?.id === p.id ? (
                <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 2 }}>
                  <TextField size="small" label="Nome da premiação" value={editando.nome} onChange={(e) => setEditando({ ...editando, nome: e.target.value })} sx={{ flexGrow: 1 }} />
                  <TextField size="small" label="Descrição (opcional)" value={editando.descricao} onChange={(e) => setEditando({ ...editando, descricao: e.target.value })} sx={{ flexGrow: 1 }} />
                  <Button variant="contained" disabled={!editando.nome.trim()} onClick={async () => {
                    if (await executar(() => api.put(`/admin/premiacoes/${p.id}`, { nome: editando.nome, descricao: editando.descricao }), 'Premiação atualizada.')) setEditando(null);
                  }}>Salvar</Button>
                  <Button onClick={() => setEditando(null)}>Cancelar</Button>
                </Stack>
              ) : (
                <Stack direction="row" justifyContent="space-between" alignItems="flex-start" spacing={1} sx={{ mb: 1 }}>
                  <Box>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <EmojiEventsIcon color="warning" />
                      <Typography fontWeight={700}>{p.nome}</Typography>
                    </Stack>
                    {p.descricao && <Typography variant="body2" color="text.secondary">{p.descricao}</Typography>}
                    <Typography variant="body2" color="text.secondary">{p.votantes} docente(s) votaram</Typography>
                  </Box>
                  {gerenciar && (
                    <Stack direction="row">
                      <Tooltip title="Editar premiação"><IconButton aria-label="Editar premiação" onClick={() => setEditando({ id: p.id, nome: p.nome, descricao: p.descricao })}><EditIcon /></IconButton></Tooltip>
                      <Tooltip title="Remover premiação">
                        <IconButton aria-label="Remover premiação" onClick={async () => {
                          const r = await confirmar({ titulo: 'Remover premiação', mensagem: `Remover "${p.nome}"? Os votos já registrados nesta premiação também serão apagados.`, perigoso: true, confirmar: 'Remover' });
                          if (r.confirmado) void executar(() => api.del(`/admin/premiacoes/${p.id}`), 'Premiação removida.');
                        }}><DeleteIcon /></IconButton>
                      </Tooltip>
                    </Stack>
                  )}
                </Stack>
              )}
              {p.vencedores.length > 0 ? (
                <Alert severity="success" icon={<EmojiEventsIcon />} sx={{ mb: 1 }}>
                  {p.vencedores.length === 1 ? 'Mais votado: ' : `Empate entre os mais votados (${p.vencedores[0].votos} votos): `}
                  {p.vencedores.map((v) => `“${v.titulo}” (${v.aluno})`).join('; ')}
                </Alert>
              ) : <Typography variant="body2" color="text.secondary">Nenhum voto registrado ainda.</Typography>}
              {p.ranking.length > 0 && (
                <Box sx={{ overflowX: 'auto' }}>
                  <Table size="small">
                    <TableHead><TableRow><TableCell>Trabalho</TableCell><TableCell>Aluno</TableCell><TableCell>Orientador</TableCell><TableCell align="right">Votos</TableCell></TableRow></TableHead>
                    <TableBody>
                      {p.ranking.map((t) => (
                        <TableRow key={t.trabalhoId}>
                          <TableCell>{t.titulo}{p.vencedores.some((v) => v.trabalhoId === t.trabalhoId) && <Chip size="small" color="success" label="Premiado" sx={{ ml: 1 }} />}</TableCell>
                          <TableCell>{t.aluno}</TableCell>
                          <TableCell>{t.orientador || '—'}</TableCell>
                          <TableCell align="right"><strong>{t.votos}</strong></TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Box>
              )}
            </CardContent>
          </Card>
        ))}
      </Stack>
      {gerenciar && (
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mt: 2 }}>
          <TextField size="small" label="Nova premiação" value={nome} onChange={(e) => setNome(e.target.value)} sx={{ flexGrow: 1 }} />
          <TextField size="small" label="Descrição (opcional)" value={descricao} onChange={(e) => setDescricao(e.target.value)} sx={{ flexGrow: 1 }} />
          <Button variant="contained" disabled={!nome.trim()} onClick={async () => {
            if (await executar(() => api.post(`/admin/eventos/${eventoId}/premiacoes`, { nome, descricao }), 'Premiação cadastrada.')) { setNome(''); setDescricao(''); }
          }}>Cadastrar premiação</Button>
        </Stack>
      )}
    </Box>
  );
}
