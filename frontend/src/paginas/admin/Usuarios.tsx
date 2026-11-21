import { useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Box, Button, Card, Chip, Dialog, DialogActions, DialogContent, DialogTitle, FormControlLabel, IconButton, Menu, MenuItem,
  Stack, Switch, Table, TableBody, TableCell, TableHead, TablePagination, TableRow, TextField, Tooltip, Typography,
} from '@mui/material';
import PersonAddIcon from '@mui/icons-material/PersonAdd';
import CheckIcon from '@mui/icons-material/Check';
import CloseIcon from '@mui/icons-material/Close';
import LockResetIcon from '@mui/icons-material/LockReset';
import EditIcon from '@mui/icons-material/Edit';
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts';
import { api, mensagemDeErro } from '../../api/cliente';
import type { Pagina } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip, Titulo, Vazio } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';

interface Usuario {
  id: string; nome: string; email: string; telefone: string; identificador: string; tipoIdentificador: string; perfis: string[];
  situacao: string; situacaoDocente: string; dadosPendentesConferencia: boolean; orientador: string | null; criadoEm: string;
}

const rotulosSituacao: Record<string, string> = { ativo: 'Ativo', bloqueado: 'Bloqueado', aguardando_primeiro_acesso: 'Aguardando primeiro acesso' };

export function Usuarios() {
  const [parametros, setParametros] = useSearchParams();
  const perfil = parametros.get('perfil') ?? '';
  const [busca, setBusca] = useState('');
  const [pagina, setPagina] = useState(0);
  const [editando, setEditando] = useState<Usuario | null>(null);
  const [novo, setNovo] = useState(false);
  const [menuPerfis, setMenuPerfis] = useState<{ el: HTMLElement; u: Usuario } | null>(null);
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['admin', 'usuarios', perfil, busca, pagina],
    queryFn: () => api.get<Pagina<Usuario>>(`/admin/usuarios?perfil=${perfil}&busca=${encodeURIComponent(busca)}&pagina=${pagina + 1}&tamanho=20`),
  });

  const executar = async (acao: () => Promise<unknown>, sucesso: string) => {
    try {
      await acao();
      notificar(sucesso);
      await refetch();
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };

  return (
    <>
      <Titulo titulo="Usuários" acoes={<Button variant="contained" startIcon={<PersonAddIcon />} onClick={() => setNovo(true)}>Cadastrar usuário</Button>} />
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ mb: 2 }}>
        <TextField size="small" label="Buscar por nome, e-mail, matrícula ou SIAPE" value={busca} onChange={(e) => { setBusca(e.target.value); setPagina(0); }} sx={{ minWidth: 320 }} />
        <TextField select size="small" label="Perfil" value={perfil} onChange={(e) => { setParametros(e.target.value ? { perfil: e.target.value } : {}); setPagina(0); }} sx={{ minWidth: 220 }}>
          <MenuItem value="">Todos</MenuItem>
          <MenuItem value="PENDENTE">Docentes aguardando aprovação</MenuItem>
          <MenuItem value="ADMIN">Administradores</MenuItem>
          <MenuItem value="DOCENTE">Docentes</MenuItem>
          <MenuItem value="ALUNO">Alunos</MenuItem>
        </TextField>
      </Stack>
      {isLoading && <Carregando />}
      {error && <ErroCarregamento erro={error} />}
      {data && (data.itens.length === 0 ? <Vazio>Nenhum usuário encontrado.</Vazio> : (
        <Card>
          <Box sx={{ overflowX: 'auto' }}>
            <Table size="small">
              <TableHead><TableRow><TableCell>Nome</TableCell><TableCell>E-mail</TableCell><TableCell>Identificador</TableCell><TableCell>Perfis</TableCell><TableCell>Situação</TableCell><TableCell align="right">Ações</TableCell></TableRow></TableHead>
              <TableBody>
                {data.itens.map((u) => (
                  <TableRow key={u.id} hover>
                    <TableCell>
                      {u.nome}
                      {u.orientador && <Typography variant="caption" display="block" color="text.secondary">Orientador: {u.orientador}</Typography>}
                      {u.dadosPendentesConferencia && <Typography variant="caption" display="block" color="warning.main">Dados pendentes de conferência</Typography>}
                    </TableCell>
                    <TableCell>{u.email}</TableCell>
                    <TableCell>{u.identificador || '—'}</TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                        {u.perfis.map((p) => <Chip key={p} size="small" label={p} color={p === 'ADMIN' ? 'secondary' : 'primary'} />)}
                        {u.situacaoDocente === 'pendente' && <Chip size="small" color="warning" label="DOCENTE pendente" />}
                        {u.situacaoDocente === 'rejeitado' && <Chip size="small" label="DOCENTE rejeitado" variant="outlined" />}
                      </Stack>
                    </TableCell>
                    <TableCell><EstadoChip estado={u.situacao} rotulo={rotulosSituacao[u.situacao] ?? u.situacao} /></TableCell>
                    <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                      {u.situacaoDocente === 'pendente' && (
                        <>
                          <Button size="small" variant="contained" color="success" startIcon={<CheckIcon />} onClick={() => void executar(() => api.post(`/admin/docentes/${u.id}/aprovar`), 'Docente aprovado.')}>Aprovar</Button>
                          <Button size="small" color="error" startIcon={<CloseIcon />} onClick={async () => {
                            const r = await confirmar({ titulo: 'Rejeitar solicitação', mensagem: `Rejeitar o perfil docente de ${u.nome}?`, perigoso: true, confirmar: 'Rejeitar', pedirMotivo: 'Motivo' });
                            if (r.confirmado) void executar(() => api.post(`/admin/docentes/${u.id}/rejeitar`, { motivo: r.motivo }), 'Solicitação rejeitada.');
                          }}>Rejeitar</Button>
                        </>
                      )}
                      {!u.perfis.includes('ALUNO') && (
                        <Tooltip title="Perfis ADMIN / DOCENTE"><IconButton aria-label="Gerenciar perfis" onClick={(e) => setMenuPerfis({ el: e.currentTarget, u })}><ManageAccountsIcon /></IconButton></Tooltip>
                      )}
                      <Tooltip title="Redefinir acesso (apaga a senha; a pessoa refaz o Primeiro acesso)">
                        <IconButton aria-label="Redefinir acesso" onClick={async () => {
                          const r = await confirmar({
                            titulo: 'Redefinir acesso',
                            mensagem: <>A senha de <strong>{u.nome}</strong> será apagada e as sessões abertas serão encerradas. A pessoa deverá fazer o <strong>Primeiro acesso</strong> informando nome completo, e-mail ({u.email}) e matrícula/SIAPE ({u.identificador || 'não cadastrado — complete antes em Editar'}).</>,
                            confirmar: 'Redefinir acesso', perigoso: true,
                          });
                          if (r.confirmado) void executar(() => api.post(`/admin/usuarios/${u.id}/redefinir-acesso`), 'Acesso redefinido. A pessoa já pode fazer o Primeiro acesso.');
                        }}><LockResetIcon /></IconButton>
                      </Tooltip>
                      <Tooltip title="Editar"><IconButton aria-label="Editar usuário" onClick={() => setEditando(u)}><EditIcon /></IconButton></Tooltip>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
          <TablePagination component="div" count={data.total} page={pagina} rowsPerPage={20} rowsPerPageOptions={[20]} onPageChange={(_, p) => setPagina(p)} labelDisplayedRows={({ from, to, count }) => `${from}–${to} de ${count}`} />
        </Card>
      ))}

      <Menu anchorEl={menuPerfis?.el} open={!!menuPerfis} onClose={() => setMenuPerfis(null)}>
        {menuPerfis && ['ADMIN', 'DOCENTE'].map((p) => {
          const tem = menuPerfis.u.perfis.includes(p);
          return (
            <MenuItem key={p} onClick={async () => {
              const u = menuPerfis.u;
              setMenuPerfis(null);
              const r = await confirmar({ titulo: tem ? `Remover perfil ${p}` : `Atribuir perfil ${p}`, mensagem: `${tem ? 'Remover' : 'Atribuir'} o perfil ${p} ${tem ? 'de' : 'a'} ${u.nome}?`, perigoso: tem });
              if (r.confirmado) void executar(() => tem ? api.del(`/admin/usuarios/${u.id}/perfis/${p}`) : api.post(`/admin/usuarios/${u.id}/perfis`, { perfil: p }), 'Perfis atualizados.');
            }}>
              {tem ? `Remover ${p}` : `Atribuir ${p}`}
            </MenuItem>
          );
        })}
      </Menu>
      <DialogoEditarUsuario usuario={editando} aoFechar={(salvou) => { setEditando(null); if (salvou) void refetch(); }} />
      <DialogoNovoUsuario aberto={novo} aoFechar={(criado) => { setNovo(false); if (criado) void refetch(); }} />
    </>
  );
}

function DialogoEditarUsuario({ usuario, aoFechar }: { usuario: Usuario | null; aoFechar: (salvou: boolean) => void }) {
  const notificar = useNotificar();
  const [d, setD] = useState({ nome: '', email: '', telefone: '', identificador: '', situacao: 'ativo' });
  const [carregado, setCarregado] = useState<string | null>(null);
  if (usuario && carregado !== usuario.id) {
    setCarregado(usuario.id);
    setD({ nome: usuario.nome, email: usuario.email, telefone: usuario.telefone, identificador: usuario.identificador, situacao: usuario.situacao });
  }
  const salvar = async () => {
    try {
      await api.patch(`/admin/usuarios/${usuario!.id}`, d);
      notificar('Usuário atualizado.');
      setCarregado(null);
      aoFechar(true);
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <Dialog open={!!usuario} onClose={() => { setCarregado(null); aoFechar(false); }} maxWidth="sm" fullWidth>
      <DialogTitle>Editar usuário</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField label="Nome completo" value={d.nome} onChange={(e) => setD({ ...d, nome: e.target.value })} />
          <TextField label="E-mail" value={d.email} onChange={(e) => setD({ ...d, email: e.target.value })} />
          <TextField label="Telefone" value={d.telefone} onChange={(e) => setD({ ...d, telefone: e.target.value })} />
          <TextField label="Matrícula / SIAPE" value={d.identificador} onChange={(e) => setD({ ...d, identificador: e.target.value })} />
          <TextField select label="Situação" value={d.situacao} onChange={(e) => setD({ ...d, situacao: e.target.value })}>
            {Object.entries(rotulosSituacao).map(([v, r]) => <MenuItem key={v} value={v}>{r}</MenuItem>)}
          </TextField>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => { setCarregado(null); aoFechar(false); }}>Cancelar</Button>
        <Button variant="contained" onClick={() => void salvar()}>Salvar</Button>
      </DialogActions>
    </Dialog>
  );
}

function DialogoNovoUsuario({ aberto, aoFechar }: { aberto: boolean; aoFechar: (criado: boolean) => void }) {
  const notificar = useNotificar();
  const inicial = { tipo: 'DOCENTE', nome: '', email: '', telefone: '', identificador: '', tambemAdmin: false };
  const [d, setD] = useState(inicial);
  const [erros, setErros] = useState<Record<string, string>>({});
  const salvar = async () => {
    setErros({});
    try {
      await api.post('/admin/usuarios', d);
      notificar('Usuário cadastrado. A pessoa cria a senha em “Primeiro acesso” com nome completo, e-mail e matrícula/SIAPE.');
      setD(inicial);
      aoFechar(true);
    } catch (e) {
      const campos = (e as { campos?: Record<string, string> }).campos;
      if (campos && Object.keys(campos).length) setErros(campos);
      else notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <Dialog open={aberto} onClose={() => aoFechar(false)} maxWidth="sm" fullWidth>
      <DialogTitle>Cadastrar usuário</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ mt: 1 }}>
          <TextField select label="Tipo" value={d.tipo} onChange={(e) => setD({ ...d, tipo: e.target.value })}>
            <MenuItem value="DOCENTE">Docente (já aprovado)</MenuItem>
            <MenuItem value="ALUNO">Aluno</MenuItem>
            <MenuItem value="ADMIN">Somente administrador</MenuItem>
          </TextField>
          <TextField label="Nome completo" value={d.nome} error={!!erros.nome} helperText={erros.nome} onChange={(e) => setD({ ...d, nome: e.target.value })} />
          <TextField label="E-mail" value={d.email} error={!!erros.email} helperText={erros.email} onChange={(e) => setD({ ...d, email: e.target.value })} />
          <TextField label="Telefone" value={d.telefone} onChange={(e) => setD({ ...d, telefone: e.target.value })} />
          <TextField required label={d.tipo === 'ALUNO' ? 'Matrícula' : 'SIAPE'} value={d.identificador} error={!!erros.identificador} helperText={erros.identificador} onChange={(e) => setD({ ...d, identificador: e.target.value })} />
          {d.tipo === 'DOCENTE' && <FormControlLabel control={<Switch checked={d.tambemAdmin} onChange={(_, v) => setD({ ...d, tambemAdmin: v })} />} label="Também é administrador" />}
          <Typography variant="caption" color="text.secondary">A pessoa criará a própria senha na tela “Primeiro acesso”, informando nome completo, e-mail e matrícula/SIAPE exatamente como cadastrados aqui.</Typography>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => aoFechar(false)}>Cancelar</Button>
        <Button variant="contained" onClick={() => void salvar()}>Cadastrar</Button>
      </DialogActions>
    </Dialog>
  );
}
