import { Alert, Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField, Typography } from '@mui/material';
import type { CodigoAcesso } from '../api/tipos';
import { useNotificar } from './Notificacoes';

// Exibe UMA vez o código de primeiro acesso/redefinição (o sistema não envia e-mails).
export function DialogoCodigoAcesso({ codigo, aoFechar }: { codigo: CodigoAcesso | null; aoFechar: () => void }) {
  const notificar = useNotificar();
  const link = codigo ? `${window.location.origin}${codigo.caminho}` : '';
  const copiar = async () => {
    try {
      await navigator.clipboard.writeText(link);
      notificar('Link copiado.');
    } catch {
      notificar('Não foi possível copiar automaticamente; selecione e copie o link.', 'warning');
    }
  };
  return (
    <Dialog open={!!codigo} onClose={aoFechar} maxWidth="sm" fullWidth>
      <DialogTitle>Código de acesso gerado</DialogTitle>
      <DialogContent>
        <Alert severity="warning" sx={{ mb: 2 }}>
          Este código é exibido somente agora. Entregue-o à pessoa por um canal institucional. Validade até{' '}
          {codigo ? new Date(codigo.expiraEm).toLocaleString('pt-BR') : ''}.
        </Alert>
        <Typography variant="body2" gutterBottom>Link de primeiro acesso / redefinição de senha:</Typography>
        <TextField fullWidth value={link} InputProps={{ readOnly: true }} onFocus={(e) => e.target.select()} multiline />
        <Typography variant="body2" sx={{ mt: 2 }}>Código: <strong style={{ wordBreak: 'break-all' }}>{codigo?.token}</strong></Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => void copiar()}>Copiar link</Button>
        <Button variant="contained" onClick={aoFechar}>Concluir</Button>
      </DialogActions>
    </Dialog>
  );
}
