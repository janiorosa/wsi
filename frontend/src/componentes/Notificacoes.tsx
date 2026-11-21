import { createContext, ReactNode, useCallback, useContext, useState } from 'react';
import { Alert, Snackbar } from '@mui/material';

type Tipo = 'success' | 'error' | 'info' | 'warning';
const Contexto = createContext<(mensagem: string, tipo?: Tipo) => void>(() => undefined);

export function NotificacoesProvider({ children }: { children: ReactNode }) {
  const [atual, setAtual] = useState<{ mensagem: string; tipo: Tipo; chave: number } | null>(null);
  const notificar = useCallback((mensagem: string, tipo: Tipo = 'success') => {
    setAtual({ mensagem, tipo, chave: Date.now() });
  }, []);
  return (
    <Contexto.Provider value={notificar}>
      {children}
      <Snackbar
        key={atual?.chave}
        open={!!atual}
        autoHideDuration={atual?.tipo === 'error' ? 8000 : 4000}
        onClose={() => setAtual(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity={atual?.tipo ?? 'info'} variant="filled" onClose={() => setAtual(null)} sx={{ maxWidth: 560 }}>
          {atual?.mensagem}
        </Alert>
      </Snackbar>
    </Contexto.Provider>
  );
}

export function useNotificar() {
  return useContext(Contexto);
}
