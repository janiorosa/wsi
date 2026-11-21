import { ReactNode } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { AppBar, Box, Button, Container, Toolbar, Typography } from '@mui/material';
import { useSessao, destinoInicial } from '../sessao/SessaoContexto';
import { cores } from '../tema';

export function MolduraPublica({ children, largura = 'lg' }: { children: ReactNode; largura?: 'sm' | 'md' | 'lg' }) {
  const { sessao } = useSessao();
  return (
    <Box sx={{ minHeight: '100vh' }}>
      <AppBar position="static" color="inherit" elevation={0} sx={{ bgcolor: 'rgba(255,255,255,0.85)', borderBottom: `1px solid ${cores.borda}` }}>
        <Toolbar>
          <Typography component={RouterLink} to="/" variant="h6" sx={{ color: 'primary.dark', fontWeight: 800, textDecoration: 'none', flexGrow: 1 }}>
            WSI · Eventos de TCC — IFMG Ouro Branco
          </Typography>
          {sessao.autenticado ? (
            <Button variant="contained" component={RouterLink} to={destinoInicial(sessao)}>Meu painel</Button>
          ) : (
            <>
              <Button component={RouterLink} to="/cadastro" sx={{ mr: 1 }}>Cadastrar</Button>
              <Button variant="contained" component={RouterLink} to="/entrar">Entrar</Button>
            </>
          )}
        </Toolbar>
      </AppBar>
      <Container maxWidth={largura} sx={{ py: { xs: 3, md: 6 } }}>
        {children}
      </Container>
    </Box>
  );
}
