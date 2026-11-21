import { Link as RouterLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Card, CardActionArea, CardContent, Stack, Typography } from '@mui/material';
import EventIcon from '@mui/icons-material/Event';
import PlaceIcon from '@mui/icons-material/Place';
import { api } from '../../api/cliente';
import type { Evento } from '../../api/tipos';
import { MolduraPublica } from '../../componentes/Publico';
import { Carregando, ErroCarregamento, EstadoChip, Grade, Vazio } from '../../componentes/Comuns';
import { estadosEvento, formatarData, rotulo } from '../../utilitarios/rotulos';

export function Inicio() {
  const { data, isLoading, error } = useQuery({ queryKey: ['publico', 'eventos'], queryFn: () => api.get<Evento[]>('/publico/eventos') });
  return (
    <MolduraPublica>
      <Box sx={{ textAlign: 'center', mb: 5 }}>
        <Typography variant="h1" component="h1" sx={{ fontSize: { xs: '1.7rem', md: '2.4rem' } }}>
          Eventos de apresentação de TCC
        </Typography>
        <Typography color="text.secondary" sx={{ mt: 1, maxWidth: 680, mx: 'auto' }}>
          Bacharelado em Sistemas de Informação — IFMG Campus Ouro Branco. Consulte a programação, os anais e as apresentações dos eventos.
        </Typography>
        <Stack direction="row" spacing={2} justifyContent="center" sx={{ mt: 3 }}>
          <Button variant="contained" size="large" component={RouterLink} to="/entrar">Entrar no sistema</Button>
          <Button variant="outlined" size="large" component={RouterLink} to="/cadastro">Criar conta</Button>
        </Stack>
      </Box>
      <Typography variant="h2" component="h2" sx={{ mb: 2 }}>Eventos</Typography>
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && data.length === 0 && <Vazio>Nenhum evento divulgado ainda.</Vazio>}
      <Grade min={300}>
        {data?.map((e) => (
          <Card key={e.slug}>
            <CardActionArea component={RouterLink} to={`/eventos/${e.slug}`} sx={{ height: '100%' }}>
              {e.temCapa && <Box component="img" src={`/api/v1/publico/eventos/${e.slug}/capa`} alt="" sx={{ width: '100%', height: 140, objectFit: 'cover' }} />}
              <CardContent>
                <EstadoChip estado={e.estado} rotulo={rotulo(estadosEvento, e.estado)} />
                <Typography variant="h3" component="h3" sx={{ mt: 1 }}>{e.nome} {e.edicao}</Typography>
                <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 1, color: 'text.secondary' }}>
                  <EventIcon fontSize="small" /><Typography variant="body2">{formatarData(e.dataApresentacao)} às {e.horarioApresentacao}</Typography>
                </Stack>
                <Stack direction="row" spacing={1} alignItems="center" sx={{ color: 'text.secondary' }}>
                  <PlaceIcon fontSize="small" /><Typography variant="body2">{e.local}</Typography>
                </Stack>
              </CardContent>
            </CardActionArea>
          </Card>
        ))}
      </Grade>
    </MolduraPublica>
  );
}
