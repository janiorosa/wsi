import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Card, CardContent, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography } from '@mui/material';
import MenuBookIcon from '@mui/icons-material/MenuBook';
import SlideshowIcon from '@mui/icons-material/Slideshow';
import { api, urlApi } from '../../api/cliente';
import type { Publicacao, SessaoEvento } from '../../api/tipos';
import { MolduraPublica } from '../../componentes/Publico';
import { Carregando, ErroCarregamento, Secao, Vazio } from '../../componentes/Comuns';
import { formatarData } from '../../utilitarios/rotulos';

interface EventoPublicoDados {
  evento: { nome: string; edicao: string; descricao: string; dataApresentacao: string; horarioApresentacao: string; local: string; campus: string; informacoesPublicas: string; slug: string; temCapa: boolean };
  sessoes: SessaoEvento[];
  programacao: { titulo: string; aluno: string; orientador: string | null; ordem: number | null; sessao: string | null }[];
  publicacoes: Publicacao[];
}

// Página pública: nunca exibe ATAs, notas, avaliações ou declarações.
export function EventoPublico() {
  const { slug } = useParams();
  const { data, isLoading, error } = useQuery({ queryKey: ['publico', 'evento', slug], queryFn: () => api.get<EventoPublicoDados>(`/publico/eventos/${slug}`) });
  return (
    <MolduraPublica>
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (
        <>
          {data.evento.temCapa && <Box component="img" src={urlApi(`/publico/eventos/${slug}/capa`)} alt="" sx={{ width: '100%', maxHeight: 260, objectFit: 'cover', borderRadius: 3, mb: 3 }} />}
          <Typography variant="h1" component="h1">{data.evento.nome} {data.evento.edicao}</Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }}>
            {formatarData(data.evento.dataApresentacao)} às {data.evento.horarioApresentacao} — {data.evento.local} (Campus {data.evento.campus})
          </Typography>
          {data.evento.descricao && <Typography sx={{ mt: 2, whiteSpace: 'pre-line' }}>{data.evento.descricao}</Typography>}
          {data.evento.informacoesPublicas && <Card sx={{ mt: 3 }}><CardContent><Typography sx={{ whiteSpace: 'pre-line' }}>{data.evento.informacoesPublicas}</Typography></CardContent></Card>}

          <Box sx={{ mt: 4 }}>
            <Secao titulo="Anais e apresentações">
              {data.publicacoes.length === 0 ? (
                <Vazio>Os documentos do evento ainda não foram publicados.</Vazio>
              ) : (
                <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
                  {data.publicacoes.map((p) => (
                    <Button key={p.id} variant="contained" size="large" startIcon={p.tipo === 'anais' ? <MenuBookIcon /> : <SlideshowIcon />}
                      href={urlApi(`/publico/publicacoes/${p.id}/arquivo?visualizar=1`)} target="_blank" rel="noopener">
                      {p.tipo === 'anais' ? 'Anais do evento' : 'Apresentações'} ({p.totalPaginas} págs.)
                    </Button>
                  ))}
                </Stack>
              )}
            </Secao>
            {data.sessoes.length > 0 && (
              <Secao titulo="Sessões">
                {data.sessoes.map((s) => (
                  <Typography key={s.id}>{s.titulo}: {formatarData(s.data)} às {s.horario} — {s.local}</Typography>
                ))}
              </Secao>
            )}
            <Secao titulo="Programação">
              {data.programacao.length === 0 ? <Vazio>Programação ainda não divulgada.</Vazio> : (
                <Box sx={{ overflowX: 'auto' }}>
                  <Table size="small">
                    <TableHead><TableRow><TableCell>#</TableCell><TableCell>Trabalho</TableCell><TableCell>Aluno(a)</TableCell><TableCell>Orientador(a)</TableCell>{data.sessoes.length > 0 && <TableCell>Sessão</TableCell>}</TableRow></TableHead>
                    <TableBody>
                      {data.programacao.map((t, i) => (
                        <TableRow key={i}>
                          <TableCell>{i + 1}</TableCell><TableCell>{t.titulo}</TableCell><TableCell>{t.aluno}</TableCell><TableCell>{t.orientador ?? '—'}</TableCell>
                          {data.sessoes.length > 0 && <TableCell>{t.sessao ?? '—'}</TableCell>}
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </Box>
              )}
            </Secao>
          </Box>
        </>
      )}
    </MolduraPublica>
  );
}
