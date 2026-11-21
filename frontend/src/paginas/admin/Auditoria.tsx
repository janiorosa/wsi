import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Box, Card, Table, TableBody, TableCell, TableHead, TablePagination, TableRow, TextField, Typography } from '@mui/material';
import { api } from '../../api/cliente';
import type { Pagina } from '../../api/tipos';
import { Carregando, Titulo } from '../../componentes/Comuns';

interface Registro { id: number; ator: string; acao: string; entidade: string; entidadeId: string; dados: unknown; ip: string; data: string }

export function Auditoria() {
  const [busca, setBusca] = useState('');
  const [pagina, setPagina] = useState(0);
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'auditoria', busca, pagina],
    queryFn: () => api.get<Pagina<Registro>>(`/admin/auditoria?busca=${encodeURIComponent(busca)}&pagina=${pagina + 1}&tamanho=50`),
  });
  return (
    <>
      <Titulo titulo="Auditoria" subtitulo="Operações sensíveis registradas (sem senhas ou segredos)." />
      <TextField size="small" label="Buscar por ação, entidade ou usuário" value={busca} onChange={(e) => { setBusca(e.target.value); setPagina(0); }} sx={{ mb: 2, minWidth: 340 }} />
      {isLoading ? <Carregando /> : (
        <Card>
          <Box sx={{ overflowX: 'auto' }}>
            <Table size="small">
              <TableHead><TableRow><TableCell>Data</TableCell><TableCell>Usuário</TableCell><TableCell>Ação</TableCell><TableCell>Entidade</TableCell><TableCell>Detalhes</TableCell></TableRow></TableHead>
              <TableBody>
                {data?.itens.map((r) => (
                  <TableRow key={r.id}>
                    <TableCell sx={{ whiteSpace: 'nowrap' }}>{r.data}</TableCell>
                    <TableCell>{r.ator}</TableCell>
                    <TableCell>{r.acao}</TableCell>
                    <TableCell>{r.entidade}<Typography variant="caption" display="block" color="text.secondary">{r.entidadeId}</Typography></TableCell>
                    <TableCell><Box component="code" sx={{ fontSize: 12, wordBreak: 'break-all' }}>{JSON.stringify(r.dados)}</Box></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
          {data && <TablePagination component="div" count={data.total} page={pagina} rowsPerPage={50} rowsPerPageOptions={[50]} onPageChange={(_, p) => setPagina(p)} labelDisplayedRows={({ from, to, count }) => `${from}–${to} de ${count}`} />}
        </Card>
      )}
    </>
  );
}
