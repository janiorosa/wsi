import { Link as RouterLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Chip, List, ListItemButton, ListItemText, Stack, Typography } from '@mui/material';
import FactCheckIcon from '@mui/icons-material/FactCheck';
import SchoolIcon from '@mui/icons-material/School';
import DrawIcon from '@mui/icons-material/Draw';
import CampaignIcon from '@mui/icons-material/Campaign';
import { api } from '../../api/cliente';
import { CartaoResumo, Carregando, ErroCarregamento, EstadoChip, Grade, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { useSessao } from '../../sessao/SessaoContexto';
import { estadosTrabalho, rotulo } from '../../utilitarios/rotulos';

export interface ItemAvaliacao { trabalhoId: string; titulo: string; aluno: string; evento: string | null; dataEvento: string | null; papel: string; estadoAvaliacao: string; nota: string | null; estadoTrabalho: string }
export interface ItemATA { ataId: string; trabalhoId: string; titulo: string; aluno: string; evento: string; estado: string; posicaoAtual: number; minhaOrdem: number; meuRotulo: string; jaAssinei: boolean; minhaVez: boolean; aguardando: string; totalSignatarios: number }
export interface Orientando { vinculoId: string; alunoId: string; nome: string; email: string; matricula: string; desde: string; trabalhoId: string | null; titulo: string | null; estadoTrabalho: string | null; evento: string | null; temArtigo: boolean; temApresentacao: boolean; declaracaoAssinada: boolean; pendencias: string[] }
interface Painel { avaliacoesPendentes: ItemAvaliacao[]; avaliacoesConcluidas: number; atasMinhaVez: ItemATA[]; atasAguardandoOutros: number; orientandos: Orientando[]; eventosComoApresentador: number; criteriosConfigurados: boolean }

export function PainelDocente() {
  const { sessao } = useSessao();
  const { data, isLoading, error } = useQuery({ queryKey: ['docente', 'painel'], queryFn: () => api.get<Painel>('/docente/painel'), refetchInterval: 20000 });
  return (
    <>
      <Titulo
        titulo={`Olá, ${sessao.usuario?.nome.split(' ')[0] ?? ''}`}
        subtitulo="Avaliação e orientação no mesmo lugar."
        acoes={
          <>
            <Button variant="contained" size="large" startIcon={<FactCheckIcon />} component={RouterLink} to="/docente/avaliar">AVALIAR</Button>
            <Button variant="outlined" size="large" startIcon={<SchoolIcon />} component={RouterLink} to="/docente/orientandos">MEUS ORIENTANDOS</Button>
          </>
        }
      />
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (
        <>
          {data.atasMinhaVez.length > 0 && (
            <Alert severity="warning" sx={{ mb: 3 }} action={<Button color="inherit" component={RouterLink} to="/docente/atas">Assinar</Button>}>
              <strong>É a sua vez de assinar {data.atasMinhaVez.length} ATA(s).</strong>
            </Alert>
          )}
          <Grade>
            <CartaoResumo titulo="Avaliações pendentes" valor={data.avaliacoesPendentes.length} icone={<FactCheckIcon />} para="/docente/avaliar" alerta={data.avaliacoesPendentes.length > 0} />
            <CartaoResumo titulo="Avaliações concluídas" valor={data.avaliacoesConcluidas} icone={<FactCheckIcon />} para="/docente/avaliar" />
            <CartaoResumo titulo="ATAs aguardando sua assinatura" valor={data.atasMinhaVez.length} descricao={`${data.atasAguardandoOutros} aguardando outros signatários`} icone={<DrawIcon />} para="/docente/atas" alerta={data.atasMinhaVez.length > 0} />
            <CartaoResumo titulo="Orientandos" valor={data.orientandos.length} icone={<SchoolIcon />} para="/docente/orientandos" />
            {data.eventosComoApresentador > 0 && <CartaoResumo titulo="Eventos como apresentador" valor={data.eventosComoApresentador} icone={<CampaignIcon />} para="/docente/apresentador" />}
          </Grade>
          {!data.criteriosConfigurados && <Alert severity="info" sx={{ mb: 3 }}>A administração ainda não cadastrou o critério geral de avaliação.</Alert>}
          <Secao titulo="Avaliações pendentes" acoes={<Button component={RouterLink} to="/docente/avaliar">AVALIAR</Button>}>
            {data.avaliacoesPendentes.length === 0 ? <Vazio>Nenhuma avaliação pendente.</Vazio> : (
              <List>
                {data.avaliacoesPendentes.map((a) => (
                  <ListItemButton key={a.trabalhoId} component={RouterLink} to={`/docente/avaliar/${a.trabalhoId}`}>
                    <ListItemText primary={a.titulo || 'Trabalho sem título'} secondary={`${a.aluno}${a.evento ? ` · ${a.evento} (${a.dataEvento})` : ''} · ${a.papel === 'orientador' ? 'como orientador' : 'como examinador'}`} />
                    <Button variant="contained" size="small">Avaliar</Button>
                  </ListItemButton>
                ))}
              </List>
            )}
          </Secao>
          <Secao titulo="Meus orientandos" acoes={<Button component={RouterLink} to="/docente/orientandos">Ver todos</Button>}>
            {data.orientandos.length === 0 ? (
              <Vazio>Você ainda não tem orientandos. <Button component={RouterLink} to="/docente/orientandos">Vincular aluno</Button></Vazio>
            ) : (
              <List>
                {data.orientandos.map((o) => (
                  <ListItemButton key={o.vinculoId} component={RouterLink} to={o.trabalhoId ? `/docente/trabalhos/${o.trabalhoId}` : '/docente/orientandos'}>
                    <ListItemText
                      primary={<Stack direction="row" spacing={1} alignItems="center"><span>{o.nome}</span>{o.estadoTrabalho && <EstadoChip estado={o.estadoTrabalho} rotulo={rotulo(estadosTrabalho, o.estadoTrabalho)} />}</Stack>}
                      secondary={o.titulo || 'Trabalho ainda não cadastrado'}
                    />
                    <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap sx={{ maxWidth: 420, justifyContent: 'flex-end' }}>
                      {o.pendencias.slice(0, 3).map((p) => <Chip key={p} size="small" color="warning" variant="outlined" label={p} />)}
                      {o.pendencias.length > 3 && <Typography variant="caption">+{o.pendencias.length - 3}</Typography>}
                    </Stack>
                  </ListItemButton>
                ))}
              </List>
            )}
          </Secao>
        </>
      )}
    </>
  );
}
