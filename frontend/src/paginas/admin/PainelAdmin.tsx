import { Link as RouterLink } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Card, CardActionArea, CardContent, Stack, Typography } from '@mui/material';
import AddCircleIcon from '@mui/icons-material/AddCircle';
import HowToRegIcon from '@mui/icons-material/HowToReg';
import PeopleIcon from '@mui/icons-material/People';
import ListAltIcon from '@mui/icons-material/ListAlt';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import RuleIcon from '@mui/icons-material/Rule';
import { api } from '../../api/cliente';
import type { Evento } from '../../api/tipos';
import { CartaoResumo, Carregando, ErroCarregamento, EstadoChip, Grade, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { estadosEvento, formatarData, rotulo } from '../../utilitarios/rotulos';

interface Painel {
  proximosEventos: Evento[];
  docentesPendentes: number;
  usuarios: { alunos: number; docentes: number; administradores: number };
  tarefasAtivas: number;
  tarefasFalhas: number;
  criteriosAtivos: number;
  trabalhosSemEvento: number;
}

export function PainelAdmin() {
  const { data, isLoading, error } = useQuery({ queryKey: ['admin', 'painel'], queryFn: () => api.get<Painel>('/admin/painel'), refetchInterval: 15000 });
  return (
    <>
      <Titulo
        titulo="Painel administrativo"
        acoes={<Button variant="contained" size="large" startIcon={<AddCircleIcon />} component={RouterLink} to="/admin/eventos/novo">CRIAR EVENTO</Button>}
      />
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (
        <>
          {data.criteriosAtivos === 0 && (
            <Alert severity="warning" sx={{ mb: 3 }} action={<Button component={RouterLink} to="/admin/criterios">Cadastrar</Button>}>
              Nenhum critério de avaliação ativo. Cadastre o critério geral confirmado pela instituição antes das avaliações.
            </Alert>
          )}
          <Grade>
            <CartaoResumo titulo="Docentes aguardando aprovação" valor={data.docentesPendentes} icone={<HowToRegIcon />} para="/admin/usuarios?perfil=PENDENTE" alerta={data.docentesPendentes > 0} />
            <CartaoResumo titulo="Usuários" valor={data.usuarios.alunos + data.usuarios.docentes} descricao={`${data.usuarios.alunos} alunos · ${data.usuarios.docentes} docentes · ${data.usuarios.administradores} admins`} icone={<PeopleIcon />} para="/admin/usuarios" />
            <CartaoResumo titulo="Documentos em processamento" valor={data.tarefasAtivas} icone={<ListAltIcon />} para="/admin/tarefas" />
            <CartaoResumo titulo="Falhas de geração (30 dias)" valor={data.tarefasFalhas} icone={<ErrorOutlineIcon />} para="/admin/tarefas?estado=falhou" alerta={data.tarefasFalhas > 0} />
            <CartaoResumo titulo="Critérios ativos" valor={data.criteriosAtivos} icone={<RuleIcon />} para="/admin/criterios" alerta={data.criteriosAtivos === 0} />
          </Grade>
          <Secao titulo="Eventos ativos e próximos" acoes={<Button component={RouterLink} to="/admin/eventos">Ver todos</Button>}>
            {data.proximosEventos.length === 0 ? <Vazio>Nenhum evento próximo.</Vazio> : (
              <Stack spacing={1.5}>
                {data.proximosEventos.map((e) => (
                  <Card key={e.id} variant="outlined">
                    <CardActionArea component={RouterLink} to={`/admin/eventos/${e.id}`}>
                      <CardContent>
                        <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" spacing={1}>
                          <div>
                            <Typography fontWeight={700}>{e.nome} {e.edicao}</Typography>
                            <Typography variant="body2" color="text.secondary">{formatarData(e.dataApresentacao)} às {e.horarioApresentacao} — {e.local} · {e.totalTrabalhos} trabalho(s)</Typography>
                          </div>
                          <EstadoChip estado={e.estado} rotulo={rotulo(estadosEvento, e.estado)} />
                        </Stack>
                      </CardContent>
                    </CardActionArea>
                  </Card>
                ))}
              </Stack>
            )}
          </Secao>
          {data.trabalhosSemEvento > 0 && <Alert severity="info">{data.trabalhosSemEvento} trabalho(s) em andamento ainda sem evento associado.</Alert>}
        </>
      )}
    </>
  );
}
