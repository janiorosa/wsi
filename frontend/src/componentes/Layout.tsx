import { ReactNode, useState } from 'react';
import { Link as RouterLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  AppBar, Box, Button, Chip, Divider, Drawer, IconButton, List, ListItemButton, ListItemIcon, ListItemText,
  ListSubheader, Stack, Toolbar, Tooltip, Typography, useMediaQuery, useTheme,
} from '@mui/material';
import MenuIcon from '@mui/icons-material/Menu';
import DashboardIcon from '@mui/icons-material/Dashboard';
import EventIcon from '@mui/icons-material/Event';
import AddCircleIcon from '@mui/icons-material/AddCircle';
import PeopleIcon from '@mui/icons-material/People';
import HowToRegIcon from '@mui/icons-material/HowToReg';
import GroupsIcon from '@mui/icons-material/Groups';
import RuleIcon from '@mui/icons-material/Rule';
import SettingsIcon from '@mui/icons-material/Settings';
import ListAltIcon from '@mui/icons-material/ListAlt';
import HistoryIcon from '@mui/icons-material/History';
import FactCheckIcon from '@mui/icons-material/FactCheck';
import SchoolIcon from '@mui/icons-material/School';
import DrawIcon from '@mui/icons-material/Draw';
import CampaignIcon from '@mui/icons-material/Campaign';
import UploadFileIcon from '@mui/icons-material/UploadFile';
import PersonSearchIcon from '@mui/icons-material/PersonSearch';
import GavelIcon from '@mui/icons-material/Gavel';
import AccountCircleIcon from '@mui/icons-material/AccountCircle';
import LogoutIcon from '@mui/icons-material/Logout';
import SwapHorizIcon from '@mui/icons-material/SwapHoriz';
import { useSessao } from '../sessao/SessaoContexto';
import { cores } from '../tema';

interface ItemMenu {
  rotulo: string;
  para: string;
  icone: ReactNode;
  destaque?: boolean;
}

function itensDoContexto(contexto: string, ehApresentador: boolean): { grupo: string; itens: ItemMenu[] }[] {
  if (contexto === 'ADMIN') {
    return [
      { grupo: 'Administração', itens: [
        { rotulo: 'Painel', para: '/admin', icone: <DashboardIcon /> },
        { rotulo: 'CRIAR EVENTO', para: '/admin/eventos/novo', icone: <AddCircleIcon />, destaque: true },
        { rotulo: 'Eventos', para: '/admin/eventos', icone: <EventIcon /> },
        { rotulo: 'Usuários', para: '/admin/usuarios', icone: <PeopleIcon /> },
        { rotulo: 'Docentes pendentes', para: '/admin/usuarios?perfil=PENDENTE', icone: <HowToRegIcon /> },
        { rotulo: 'Orientações', para: '/admin/orientacoes', icone: <GroupsIcon /> },
      ] },
      { grupo: 'Configuração', itens: [
        { rotulo: 'Critérios de avaliação', para: '/admin/criterios', icone: <RuleIcon /> },
        { rotulo: 'Dados institucionais', para: '/admin/configuracoes', icone: <SettingsIcon /> },
        { rotulo: 'Fila de documentos', para: '/admin/tarefas', icone: <ListAltIcon /> },
        { rotulo: 'Auditoria', para: '/admin/auditoria', icone: <HistoryIcon /> },
      ] },
    ];
  }
  if (contexto === 'DOCENTE') {
    const grupos = [
      { grupo: 'Docente', itens: [
        { rotulo: 'Painel', para: '/docente', icone: <DashboardIcon /> },
        { rotulo: 'AVALIAR', para: '/docente/avaliar', icone: <FactCheckIcon />, destaque: true },
        { rotulo: 'MEUS ORIENTANDOS', para: '/docente/orientandos', icone: <SchoolIcon /> },
        { rotulo: 'ATAs para assinar', para: '/docente/atas', icone: <DrawIcon /> },
      ] },
    ];
    if (ehApresentador) grupos.push({ grupo: 'Apresentador', itens: [{ rotulo: 'Meus eventos (apresentador)', para: '/docente/apresentador', icone: <CampaignIcon /> }] });
    return grupos;
  }
  if (contexto === 'ALUNO') {
    return [
      { grupo: 'Aluno', itens: [
        { rotulo: 'Painel', para: '/aluno', icone: <DashboardIcon /> },
        { rotulo: 'ENVIAR MEU TCC', para: '/aluno/tcc', icone: <UploadFileIcon />, destaque: true },
        { rotulo: 'Orientador', para: '/aluno/orientador', icone: <PersonSearchIcon /> },
        { rotulo: 'Declaração anti-plágio', para: '/aluno/declaracao', icone: <GavelIcon /> },
        { rotulo: 'ASSINAR ATA', para: '/aluno/ata', icone: <DrawIcon /> },
      ] },
    ];
  }
  return [];
}

const LARGURA = 268;

export function Layout() {
  const { sessao, sair } = useSessao();
  const navegar = useNavigate();
  const local = useLocation();
  const theme = useTheme();
  const telaGrande = useMediaQuery(theme.breakpoints.up('md'));
  const [aberto, setAberto] = useState(false);
  const grupos = itensDoContexto(sessao.contexto, sessao.ehApresentador);
  const ambosPerfis = sessao.perfis.includes('ADMIN') && sessao.perfis.includes('DOCENTE');
  const rotuloContexto = { ADMIN: 'Administrador', DOCENTE: 'Docente', ALUNO: 'Aluno' }[sessao.contexto || 'ALUNO'];

  const caminhoAtual = local.pathname + local.search;
  const menu = (
    <Box sx={{ width: LARGURA, pt: 1 }} role="navigation" aria-label="Menu principal">
      {grupos.map((g) => (
        <List key={g.grupo} subheader={<ListSubheader sx={{ bgcolor: 'transparent' }}>{g.grupo}</ListSubheader>} dense>
          {g.itens.map((i) => {
            const selecionado = caminhoAtual === i.para || (i.para.split('/').length > 2 && local.pathname.startsWith(i.para) && !i.para.includes('?'));
            return (
              <ListItemButton
                key={i.para}
                component={RouterLink}
                to={i.para}
                selected={selecionado}
                onClick={() => setAberto(false)}
                sx={{ mx: 1, borderRadius: 2, ...(i.destaque ? { bgcolor: cores.superficieSecundaria, border: `1px solid ${cores.borda}` } : {}) }}
              >
                <ListItemIcon sx={{ minWidth: 38, color: 'primary.main' }}>{i.icone}</ListItemIcon>
                <ListItemText primary={i.rotulo} primaryTypographyProps={{ fontWeight: i.destaque ? 700 : 500 }} />
              </ListItemButton>
            );
          })}
        </List>
      ))}
      <Divider sx={{ my: 1 }} />
      <List dense>
        <ListItemButton component={RouterLink} to="/conta" sx={{ mx: 1, borderRadius: 2 }} onClick={() => setAberto(false)}>
          <ListItemIcon sx={{ minWidth: 38 }}><AccountCircleIcon /></ListItemIcon>
          <ListItemText primary="Minha conta" />
        </ListItemButton>
      </List>
    </Box>
  );

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh' }}>
      <AppBar position="fixed" color="inherit" sx={{ zIndex: (t) => t.zIndex.drawer + 1, bgcolor: 'rgba(255,255,255,0.92)', backdropFilter: 'blur(6px)', borderBottom: `1px solid ${cores.borda}` }} elevation={0}>
        <Toolbar sx={{ gap: 1 }}>
          {!telaGrande && (
            <IconButton edge="start" aria-label="Abrir menu" onClick={() => setAberto(true)}>
              <MenuIcon />
            </IconButton>
          )}
          <Typography component={RouterLink} to="/" variant="h6" sx={{ color: 'primary.dark', fontWeight: 800, textDecoration: 'none', mr: 2 }}>
            WSI · TCC
          </Typography>
          <Box sx={{ flexGrow: 1 }} />
          <Stack direction="row" spacing={1} alignItems="center">
            {telaGrande && <Typography variant="body2" color="text.secondary">{sessao.usuario?.nome}</Typography>}
            <Chip size="small" color="primary" label={rotuloContexto} aria-label={`Contexto atual: ${rotuloContexto}`} />
            {ambosPerfis && (
              <Tooltip title="Alternar entre ADMIN e DOCENTE">
                <Button size="small" startIcon={<SwapHorizIcon />} onClick={() => navegar('/contexto')}>
                  {telaGrande ? 'Trocar contexto' : ''}
                </Button>
              </Tooltip>
            )}
            <Tooltip title="Sair do sistema">
              <IconButton aria-label="Sair" onClick={async () => { await sair(); navegar('/entrar'); }}>
                <LogoutIcon />
              </IconButton>
            </Tooltip>
          </Stack>
        </Toolbar>
      </AppBar>
      <Drawer
        variant={telaGrande ? 'permanent' : 'temporary'}
        open={telaGrande || aberto}
        onClose={() => setAberto(false)}
        sx={{ width: LARGURA, flexShrink: 0, '& .MuiDrawer-paper': { width: LARGURA, boxSizing: 'border-box', bgcolor: 'rgba(255,255,255,0.85)', borderRight: `1px solid ${cores.borda}` } }}
      >
        <Toolbar />
        {menu}
      </Drawer>
      <Box component="main" sx={{ flexGrow: 1, p: { xs: 2, md: 4 }, minWidth: 0 }}>
        <Toolbar />
        <Box sx={{ maxWidth: 1200, mx: 'auto' }}>
          <Outlet />
        </Box>
      </Box>
    </Box>
  );
}
