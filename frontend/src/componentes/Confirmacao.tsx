import { createContext, ReactNode, useCallback, useContext, useRef, useState } from 'react';
import { Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle, TextField } from '@mui/material';

interface Opcoes {
  titulo: string;
  mensagem: ReactNode;
  confirmar?: string;
  perigoso?: boolean;
  pedirMotivo?: string; // rótulo do campo de motivo (obrigatório quando informado)
}
type Resultado = { confirmado: boolean; motivo: string };
const Contexto = createContext<(o: Opcoes) => Promise<Resultado>>(async () => ({ confirmado: false, motivo: '' }));

// Diálogo de confirmação para ações destrutivas ou irreversíveis.
export function ConfirmacaoProvider({ children }: { children: ReactNode }) {
  const [opcoes, setOpcoes] = useState<Opcoes | null>(null);
  const [motivo, setMotivo] = useState('');
  const resolver = useRef<(r: Resultado) => void>();
  const confirmar = useCallback((o: Opcoes) => {
    setOpcoes(o);
    setMotivo('');
    return new Promise<Resultado>((resolve) => {
      resolver.current = resolve;
    });
  }, []);
  const fechar = (confirmado: boolean) => {
    resolver.current?.({ confirmado, motivo: motivo.trim() });
    setOpcoes(null);
  };
  const motivoInvalido = !!opcoes?.pedirMotivo && motivo.trim().length < 5;
  return (
    <Contexto.Provider value={confirmar}>
      {children}
      <Dialog open={!!opcoes} onClose={() => fechar(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{opcoes?.titulo}</DialogTitle>
        <DialogContent>
          <DialogContentText component="div">{opcoes?.mensagem}</DialogContentText>
          {opcoes?.pedirMotivo && (
            <TextField
              autoFocus
              fullWidth
              multiline
              minRows={2}
              margin="normal"
              label={opcoes.pedirMotivo}
              value={motivo}
              onChange={(e) => setMotivo(e.target.value)}
              helperText="Mínimo de 5 caracteres. Fica registrado na auditoria."
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => fechar(false)}>Cancelar</Button>
          <Button
            variant="contained"
            color={opcoes?.perigoso ? 'error' : 'primary'}
            disabled={motivoInvalido}
            onClick={() => fechar(true)}
          >
            {opcoes?.confirmar ?? 'Confirmar'}
          </Button>
        </DialogActions>
      </Dialog>
    </Contexto.Provider>
  );
}

export function useConfirmar() {
  return useContext(Contexto);
}
