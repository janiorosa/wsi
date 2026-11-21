import { createTheme } from '@mui/material/styles';

// Tokens de cor (seção 16.1 da especificação): tema claro em tons de azul.
export const cores = {
  fundoSuperior: '#EAF4FF',
  fundoInferior: '#F7FBFF',
  superficie: '#FFFFFF',
  superficieSecundaria: '#F2F8FF',
  primario: '#2367A8',
  azulEscuro: '#17466F',
  destaque: '#78B7E8',
  texto: '#18324A',
  textoSecundario: '#587086',
  borda: '#D8E7F5',
};

export const tema = createTheme({
  palette: {
    mode: 'light',
    primary: { main: cores.primario, dark: cores.azulEscuro, light: cores.destaque, contrastText: '#FFFFFF' },
    secondary: { main: cores.azulEscuro },
    background: { default: cores.fundoInferior, paper: cores.superficie },
    text: { primary: cores.texto, secondary: cores.textoSecundario },
    divider: cores.borda,
    success: { main: '#2E7D4F' },
    warning: { main: '#B26A00' },
    error: { main: '#C62828' },
    info: { main: cores.primario },
  },
  shape: { borderRadius: 10 },
  typography: {
    fontFamily: '"Inter", "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
    h1: { fontSize: '1.9rem', fontWeight: 700, color: cores.azulEscuro },
    h2: { fontSize: '1.5rem', fontWeight: 700, color: cores.azulEscuro },
    h3: { fontSize: '1.2rem', fontWeight: 600, color: cores.azulEscuro },
    button: { fontWeight: 600 },
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          minHeight: '100vh',
          backgroundColor: cores.fundoInferior,
          backgroundImage: `linear-gradient(180deg, ${cores.fundoSuperior} 0%, ${cores.fundoInferior} 55%, #FFFFFF 100%)`,
          backgroundAttachment: 'fixed',
        },
        // Foco visível para navegação por teclado.
        '*:focus-visible': { outline: `3px solid ${cores.destaque}`, outlineOffset: 2 },
      },
    },
    MuiPaper: {
      styleOverrides: { root: { border: `1px solid ${cores.borda}` } },
      defaultProps: { elevation: 0 },
    },
    MuiCard: {
      styleOverrides: { root: { boxShadow: '0 2px 10px rgba(23, 70, 111, 0.06)' } },
    },
    MuiButton: {
      styleOverrides: { root: { textTransform: 'none', borderRadius: 10 }, sizeLarge: { padding: '12px 22px', fontSize: '1rem' } },
    },
    MuiTableCell: { styleOverrides: { head: { fontWeight: 700, color: cores.azulEscuro, backgroundColor: cores.superficieSecundaria } } },
    MuiTooltip: { defaultProps: { arrow: true, enterTouchDelay: 0 } },
  },
});
