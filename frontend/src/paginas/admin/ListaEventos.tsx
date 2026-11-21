import { useState } from 'react';
import { Link as RouterLink, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Card, MenuItem, Table, TableBody, TableCell, TableHead, TableRow, TextField } from '@mui/material';
import AddCircleIcon from '@mui/icons-material/AddCircle';
import { api } from '../../api/cliente';
import type { Evento } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip, Titulo, Vazio } from '../../componentes/Comuns';
import { estadosEvento, formatarData, rotulo } from '../../utilitarios/rotulos';

export function ListaEventos() {
  const [estado, setEstado] = useState('');
  const navegar = useNavigate();
  const { data, isLoading, error } = useQuery({ queryKey: ['admin', 'eventos', estado], queryFn: () => api.get<Evento[]>(`/admin/eventos?estado=${estado}`) });
  return (
    <>
      <Titulo titulo="Eventos" acoes={<Button variant="contained" startIcon={<AddCircleIcon />} component={RouterLink} to="/admin/eventos/novo">CRIAR EVENTO</Button>} />
      <TextField select size="small" label="Filtrar por estado" value={estado} onChange={(e) => setEstado(e.target.value)} sx={{ mb: 2, minWidth: 240 }}>
        <MenuItem value="">Todos</MenuItem>
        {Object.entries(estadosEvento).map(([v, r]) => <MenuItem key={v} value={v}>{r}</MenuItem>)}
      </TextField>
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (data.length === 0 ? <Vazio>Nenhum evento cadastrado.</Vazio> : (
        <Card><Box sx={{ overflowX: 'auto' }}>
          <Table>
            <TableHead><TableRow><TableCell>Evento</TableCell><TableCell>Data</TableCell><TableCell>Local</TableCell><TableCell>Trabalhos</TableCell><TableCell>Estado</TableCell></TableRow></TableHead>
            <TableBody>
              {data.map((e) => (
                <TableRow key={e.id} hover sx={{ cursor: 'pointer' }} onClick={() => navegar(`/admin/eventos/${e.id}`)} tabIndex={0} onKeyDown={(k) => k.key === 'Enter' && navegar(`/admin/eventos/${e.id}`)}>
                  <TableCell><strong>{e.nome}</strong> {e.edicao}</TableCell>
                  <TableCell>{formatarData(e.dataApresentacao)} {e.horarioApresentacao}</TableCell>
                  <TableCell>{e.local}</TableCell>
                  <TableCell>{e.totalTrabalhos}</TableCell>
                  <TableCell><EstadoChip estado={e.estado} rotulo={rotulo(estadosEvento, e.estado)} /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box></Card>
      ))}
    </>
  );
}
