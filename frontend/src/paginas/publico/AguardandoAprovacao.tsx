import { useNavigate } from 'react-router-dom';
import { Alert, Button, Card, CardContent, Stack, Typography } from '@mui/material';
import { useSessao } from '../../sessao/SessaoContexto';
import { MolduraPublica } from '../../componentes/Publico';

export function AguardandoAprovacao() {
  const { sessao, sair, recarregar } = useSessao();
  const navegar = useNavigate();
  const rejeitado = sessao.situacaoDocente === 'rejeitado';
  return (
    <MolduraPublica largura="sm">
      <Card>
        <CardContent sx={{ p: 4 }}>
          <Typography variant="h1" component="h1" gutterBottom>Cadastro de docente</Typography>
          <Alert severity={rejeitado ? 'error' : 'info'} sx={{ mb: 3 }}>
            {rejeitado
              ? 'Sua solicitação de perfil docente não foi aprovada. Procure a administração do curso.'
              : 'Sua solicitação está aguardando aprovação da administração. Assim que aprovada, as funções docentes ficarão disponíveis.'}
          </Alert>
          <Stack direction="row" spacing={2}>
            <Button variant="contained" onClick={() => void recarregar().then(() => navegar('/'))}>Verificar novamente</Button>
            <Button onClick={async () => { await sair(); navegar('/'); }}>Sair</Button>
          </Stack>
        </CardContent>
      </Card>
    </MolduraPublica>
  );
}
