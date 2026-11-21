import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { tema } from './tema';
import { SessaoProvider } from './sessao/SessaoContexto';
import { NotificacoesProvider } from './componentes/Notificacoes';
import { ConfirmacaoProvider } from './componentes/Confirmacao';
import { App } from './App';

const clienteConsultas = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: true, staleTime: 5_000 } },
});

ReactDOM.createRoot(document.getElementById('raiz')!).render(
  <React.StrictMode>
    <ThemeProvider theme={tema}>
      <CssBaseline />
      <QueryClientProvider client={clienteConsultas}>
        <BrowserRouter>
          <SessaoProvider>
            <NotificacoesProvider>
              <ConfirmacaoProvider>
                <App />
              </ConfirmacaoProvider>
            </NotificacoesProvider>
          </SessaoProvider>
        </BrowserRouter>
      </QueryClientProvider>
    </ThemeProvider>
  </React.StrictMode>,
);
