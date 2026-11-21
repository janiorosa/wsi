import { useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Box, Button, Card, CardContent, Chip, Collapse, Stack, Typography } from '@mui/material';
import DrawIcon from '@mui/icons-material/Draw';
import { api } from '../../api/cliente';
import type { ResumoATA } from '../../api/tipos';
import { Carregando, ErroCarregamento, Titulo, Vazio } from '../../componentes/Comuns';
import { PainelAssinaturaATA } from '../../componentes/PainelAssinaturaATA';
import type { ItemATA } from './PainelDocente';

// Cada linha é um processo independente (um por trabalho).
export function AtasParaAssinar() {
  const { data, isLoading, error, refetch } = useQuery({ queryKey: ['docente', 'atas'], queryFn: () => api.get<ItemATA[]>('/docente/atas') });
  const [aberta, setAberta] = useState<string | null>(null);
  const pendentes = data?.filter((a) => a.estado === 'aguardando_assinaturas') ?? [];
  const finalizadas = data?.filter((a) => a.estado === 'finalizada') ?? [];
  return (
    <>
      <Titulo titulo="ATAs para assinar" subtitulo="Você só pode baixar e enviar uma ATA quando chegar a sua vez naquele trabalho." />
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && pendentes.length === 0 && <Vazio>Nenhuma ATA com assinatura pendente.</Vazio>}
      <Stack spacing={2}>
        {pendentes.map((a) => <LinhaATA key={a.ataId} ata={a} aberta={aberta === a.ataId} alternar={() => setAberta(aberta === a.ataId ? null : a.ataId)} aoAtualizar={() => void refetch()} />)}
      </Stack>
      {finalizadas.length > 0 && (
        <>
          <Typography variant="h2" component="h2" sx={{ mt: 5, mb: 2 }}>ATAs finalizadas</Typography>
          <Stack spacing={2}>
            {finalizadas.map((a) => <LinhaATA key={a.ataId} ata={a} aberta={aberta === a.ataId} alternar={() => setAberta(aberta === a.ataId ? null : a.ataId)} aoAtualizar={() => void refetch()} />)}
          </Stack>
        </>
      )}
    </>
  );
}

function LinhaATA({ ata, aberta, alternar, aoAtualizar }: { ata: ItemATA; aberta: boolean; alternar: () => void; aoAtualizar: () => void }) {
  const { data: detalhe, refetch } = useQuery({ queryKey: ['ata', ata.ataId], queryFn: () => api.get<ResumoATA>(`/atas/${ata.ataId}`), enabled: aberta });
  return (
    <Card sx={{ borderColor: ata.minhaVez ? 'warning.main' : undefined, borderWidth: ata.minhaVez ? 2 : 1 }}>
      <CardContent>
        <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={2}>
          <Box>
            <Typography fontWeight={700}>{ata.titulo}</Typography>
            <Typography variant="body2" color="text.secondary">{ata.aluno} · {ata.evento} · você assina como {ata.meuRotulo} ({ata.minhaOrdem}º de {ata.totalSignatarios})</Typography>
            <Stack direction="row" spacing={1} sx={{ mt: 1 }}>
              {ata.estado === 'finalizada' ? <Chip size="small" color="success" label="Finalizada" /> :
                ata.minhaVez ? <Chip size="small" color="warning" icon={<DrawIcon />} label="Sua vez de assinar" /> :
                  ata.jaAssinei ? <Chip size="small" color="success" variant="outlined" label="Você já assinou" /> :
                    <Chip size="small" variant="outlined" label={`Aguardando: ${ata.aguardando}`} />}
            </Stack>
          </Box>
          <Stack direction="row" spacing={1} alignItems="center">
            <Button variant={ata.minhaVez ? 'contained' : 'outlined'} onClick={alternar}>{aberta ? 'Fechar' : ata.minhaVez ? 'Assinar' : 'Detalhes'}</Button>
            <Button component={RouterLink} to={`/docente/trabalhos/${ata.trabalhoId}`}>Ficha</Button>
          </Stack>
        </Stack>
        <Collapse in={aberta} unmountOnExit>
          <Box sx={{ mt: 3 }}>
            {detalhe ? <PainelAssinaturaATA ata={detalhe} aoAtualizar={() => { void refetch(); aoAtualizar(); }} /> : <Carregando />}
          </Box>
        </Collapse>
      </CardContent>
    </Card>
  );
}
