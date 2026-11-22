import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Box, Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material';
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents';
import HowToVoteIcon from '@mui/icons-material/HowToVote';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import { api, mensagemDeErro } from '../../api/cliente';
import { Carregando, ErroCarregamento, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';

export interface EventoVotacao {
  id: string;
  nome: string;
  edicao: string;
  data: string;
  trabalhos: { id: string; titulo: string; aluno: string; orientador: string | null }[];
  premiacoes: { id: string; nome: string; descricao: string; meusVotos: string[] }[];
}

const MAXIMO_VOTOS = 2;

// Votação das premiações: cada docente vota em até dois trabalhos por premiação.
// Os votos podem ser retirados ou trocados enquanto a votação estiver aberta.
export function Premiacoes() {
  const notificar = useNotificar();
  const cliente = useQueryClient();
  const { data, isLoading, error, refetch } = useQuery({ queryKey: ['docente', 'premiacoes'], queryFn: () => api.get<EventoVotacao[]>('/docente/premiacoes') });
  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;

  const alternar = async (premiacaoId: string, trabalhoId: string, votado: boolean) => {
    try {
      if (votado) await api.del(`/docente/premiacoes/${premiacaoId}/votos/${trabalhoId}`);
      else await api.post(`/docente/premiacoes/${premiacaoId}/votos`, { trabalhoId });
      notificar(votado ? 'Voto retirado.' : 'Voto registrado.');
      await refetch();
      void cliente.invalidateQueries({ queryKey: ['docente', 'painel'] });
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };

  return (
    <>
      <Titulo titulo="Premiações" subtitulo="Vote em até dois trabalhos por premiação. Você pode retirar ou trocar seus votos enquanto a votação estiver aberta." />
      {data.length === 0 && <Vazio>Nenhuma premiação aberta para votação no momento.</Vazio>}
      {data.map((evento) => (
        <Box key={evento.id} sx={{ mb: 4 }}>
          <Typography variant="h2" component="h2" sx={{ mb: 2 }}>{evento.nome} {evento.edicao} <Typography component="span" color="text.secondary">— {evento.data}</Typography></Typography>
          {evento.premiacoes.map((p) => {
            const restantes = MAXIMO_VOTOS - p.meusVotos.length;
            return (
              <Secao key={p.id} titulo={p.nome}
                acoes={<Chip icon={<HowToVoteIcon />} color={restantes === 0 ? 'success' : 'primary'} variant="outlined"
                  label={restantes === 0 ? 'Seus 2 votos foram registrados' : `${p.meusVotos.length} de ${MAXIMO_VOTOS} votos usados`} />}>
                {p.descricao && <Typography color="text.secondary" sx={{ mb: 2 }}>{p.descricao}</Typography>}
                {evento.trabalhos.length === 0 ? <Alert severity="info">Nenhum trabalho com título neste evento.</Alert> : (
                  <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
                    {evento.trabalhos.map((t) => {
                      const votado = p.meusVotos.includes(t.id);
                      return (
                        <Card key={t.id} variant="outlined" sx={votado ? { borderColor: 'success.main', borderWidth: 2 } : undefined}>
                          <CardContent>
                            <Stack direction="row" spacing={1} alignItems="flex-start" justifyContent="space-between">
                              <Box sx={{ minWidth: 0 }}>
                                <Typography fontWeight={700}>{t.titulo}</Typography>
                                <Typography variant="body2" color="text.secondary">{t.aluno}{t.orientador ? ` · orientação: ${t.orientador}` : ''}</Typography>
                              </Box>
                              {votado && <CheckCircleIcon color="success" aria-label="Seu voto" />}
                            </Stack>
                            <Button sx={{ mt: 1.5 }} size="small" variant={votado ? 'outlined' : 'contained'} startIcon={<EmojiEventsIcon />}
                              disabled={!votado && restantes === 0} onClick={() => void alternar(p.id, t.id, votado)}>
                              {votado ? 'Retirar voto' : 'Votar neste trabalho'}
                            </Button>
                          </CardContent>
                        </Card>
                      );
                    })}
                  </Box>
                )}
              </Secao>
            );
          })}
        </Box>
      ))}
    </>
  );
}
