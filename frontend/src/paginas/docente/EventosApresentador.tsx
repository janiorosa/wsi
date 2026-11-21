import { Link as RouterLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { api } from '../../api/cliente';
import type { Evento } from '../../api/tipos';
import { Carregando, EstadoChip, Grade, Titulo, Vazio } from '../../componentes/Comuns';
import { estadosEvento, formatarData, rotulo } from '../../utilitarios/rotulos';

export function EventosApresentador() {
  const { data, isLoading } = useQuery({ queryKey: ['apresentador', 'eventos'], queryFn: () => api.get<Evento[]>('/apresentador/eventos') });
  return (
    <>
      <Titulo titulo="Meus eventos como apresentador" subtitulo="Gere o PDF consolidado das apresentações e os anais, confira e publique." />
      {isLoading && <Carregando />}
      {data && data.length === 0 && <Vazio>Você não foi designado como apresentador de nenhum evento.</Vazio>}
      <Grade min={320}>
        {data?.map((e) => (
          <Card key={e.id}>
            <CardContent>
              <EstadoChip estado={e.estado} rotulo={rotulo(estadosEvento, e.estado)} />
              <Typography variant="h3" component="h2" sx={{ mt: 1 }}>{e.nome} {e.edicao}</Typography>
              <Typography variant="body2" color="text.secondary">{formatarData(e.dataApresentacao)} · {e.totalTrabalhos} trabalho(s)</Typography>
              <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
                <Button variant="contained" component={RouterLink} to={`/docente/apresentador/${e.id}`}>Abrir painel do evento</Button>
              </Stack>
            </CardContent>
          </Card>
        ))}
      </Grade>
    </>
  );
}
