import { useNavigate } from 'react-router-dom';
import { Button, Card, CardContent, Stack, Typography } from '@mui/material';
import AdminPanelSettingsIcon from '@mui/icons-material/AdminPanelSettings';
import SchoolIcon from '@mui/icons-material/School';
import type { Perfil } from '../../api/tipos';
import { destinoInicial, useSessao } from '../../sessao/SessaoContexto';
import { MolduraPublica } from '../../componentes/Publico';
import { useNotificar } from '../../componentes/Notificacoes';
import { mensagemDeErro } from '../../api/cliente';

// Usuário ADMIN + DOCENTE escolhe o contexto da sessão. Os perfis armazenados não mudam.
export function EscolherContexto() {
  const { sessao, escolherContexto } = useSessao();
  const navegar = useNavigate();
  const notificar = useNotificar();
  const escolher = async (c: Perfil) => {
    try {
      const s = await escolherContexto(c);
      navegar(destinoInicial(s), { replace: true });
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <MolduraPublica largura="sm">
      <Card>
        <CardContent sx={{ p: 4, textAlign: 'center' }}>
          <Typography variant="h1" component="h1" gutterBottom>Olá, {sessao.usuario?.nome?.split(' ')[0]}!</Typography>
          <Typography color="text.secondary" sx={{ mb: 3 }}>Você possui os perfis de administrador e docente. Como deseja entrar?</Typography>
          <Stack spacing={2}>
            {sessao.perfis.includes('ADMIN') && (
              <Button variant="contained" size="large" startIcon={<AdminPanelSettingsIcon />} onClick={() => void escolher('ADMIN')}>ENTRAR COMO ADMIN</Button>
            )}
            {sessao.perfis.includes('DOCENTE') && (
              <Button variant="contained" color="secondary" size="large" startIcon={<SchoolIcon />} onClick={() => void escolher('DOCENTE')}>ENTRAR COMO DOCENTE</Button>
            )}
          </Stack>
        </CardContent>
      </Card>
    </MolduraPublica>
  );
}
