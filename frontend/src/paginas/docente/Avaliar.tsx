import { Link as RouterLink, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Card, Table, TableBody, TableCell, TableHead, TableRow } from '@mui/material';
import { api } from '../../api/cliente';
import { Carregando, ErroCarregamento, EstadoChip, Titulo, Vazio } from '../../componentes/Comuns';
import { estadosAvaliacao, rotulo } from '../../utilitarios/rotulos';
import type { ItemAvaliacao } from './PainelDocente';

export function Avaliar() {
  const navegar = useNavigate();
  const { data, isLoading, error } = useQuery({ queryKey: ['docente', 'avaliacoes'], queryFn: () => api.get<ItemAvaliacao[]>('/docente/avaliacoes') });
  return (
    <>
      <Titulo titulo="Avaliar" subtitulo="Somente os trabalhos para os quais você foi designado (inclusive os dos seus orientandos)." />
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (data.length === 0 ? <Vazio>Nenhum trabalho destinado a você.</Vazio> : (
        <Card><Box sx={{ overflowX: 'auto' }}>
          <Table>
            <TableHead><TableRow><TableCell>Trabalho</TableCell><TableCell>Aluno</TableCell><TableCell>Evento</TableCell><TableCell>Papel</TableCell><TableCell>Avaliação</TableCell><TableCell>Sua nota</TableCell><TableCell /></TableRow></TableHead>
            <TableBody>
              {data.map((a) => (
                <TableRow key={a.trabalhoId} hover>
                  <TableCell>{a.titulo || <em>sem título</em>}</TableCell>
                  <TableCell>{a.aluno}</TableCell>
                  <TableCell>{a.evento ? `${a.evento} (${a.dataEvento})` : '—'}</TableCell>
                  <TableCell>{a.papel === 'orientador' ? 'Orientador' : 'Examinador'}</TableCell>
                  <TableCell><EstadoChip estado={a.estadoAvaliacao} rotulo={rotulo(estadosAvaliacao, a.estadoAvaliacao)} /></TableCell>
                  <TableCell>{a.nota ?? '—'}</TableCell>
                  <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                    <Button variant={a.estadoAvaliacao === 'finalizada' ? 'outlined' : 'contained'} size="small" onClick={() => navegar(`/docente/avaliar/${a.trabalhoId}`)}>
                      {a.estadoAvaliacao === 'finalizada' ? 'Ver' : 'AVALIAR'}
                    </Button>
                    <Button size="small" component={RouterLink} to={`/docente/trabalhos/${a.trabalhoId}`}>Ficha</Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Box></Card>
      ))}
    </>
  );
}
