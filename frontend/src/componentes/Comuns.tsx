import { ReactNode } from 'react';
import { Alert, Box, Card, CardActionArea, CardContent, Chip, CircularProgress, Stack, Tooltip, Typography } from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import HourglassTopIcon from '@mui/icons-material/HourglassTop';
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined';
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked';
import { Link as RouterLink } from 'react-router-dom';
import { mensagemDeErro } from '../api/cliente';
import { tomDoEstado } from '../utilitarios/rotulos';

export function Carregando({ texto = 'Carregando...' }: { texto?: string }) {
  return (
    <Stack direction="row" spacing={2} alignItems="center" sx={{ py: 6, justifyContent: 'center' }} role="status">
      <CircularProgress size={28} />
      <Typography color="text.secondary">{texto}</Typography>
    </Stack>
  );
}

export function ErroCarregamento({ erro }: { erro: unknown }) {
  return <Alert severity="error">{mensagemDeErro(erro)}</Alert>;
}

export function Titulo({ titulo, subtitulo, acoes }: { titulo: string; subtitulo?: ReactNode; acoes?: ReactNode }) {
  return (
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ sm: 'center' }} sx={{ mb: 3 }}>
      <Box>
        <Typography variant="h1" component="h1">
          {titulo}
        </Typography>
        {subtitulo && (
          <Typography color="text.secondary" sx={{ mt: 0.5 }}>
            {subtitulo}
          </Typography>
        )}
      </Box>
      {acoes && (
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          {acoes}
        </Stack>
      )}
    </Stack>
  );
}

// Indicador de estado com ícone e texto (não depende apenas de cor).
export function EstadoChip({ estado, rotulo }: { estado: string; rotulo: string }) {
  const tom = tomDoEstado(estado);
  const icone =
    tom === 'success' ? <CheckCircleIcon /> : tom === 'error' ? <ErrorOutlineIcon /> : tom === 'warning' ? <HourglassTopIcon /> : tom === 'default' ? <RadioButtonUncheckedIcon /> : <InfoOutlinedIcon />;
  return <Chip size="small" icon={icone} label={rotulo} color={tom === 'default' ? 'default' : tom} variant={tom === 'default' ? 'outlined' : 'filled'} />;
}

export function Secao({ titulo, children, acoes, dica }: { titulo: string; children: ReactNode; acoes?: ReactNode; dica?: string }) {
  return (
    <Card sx={{ mb: 3 }}>
      <CardContent>
        <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 2 }} spacing={2}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Typography variant="h3" component="h2">
              {titulo}
            </Typography>
            {dica && (
              <Tooltip title={dica}>
                <InfoOutlinedIcon fontSize="small" color="action" tabIndex={0} aria-label={dica} />
              </Tooltip>
            )}
          </Stack>
          {acoes && <Stack direction="row" spacing={1}>{acoes}</Stack>}
        </Stack>
        {children}
      </CardContent>
    </Card>
  );
}

// Cartão de resumo clicável usado nos painéis.
export function CartaoResumo({ titulo, valor, descricao, para, icone, alerta }: {
  titulo: string; valor: ReactNode; descricao?: string; para?: string; icone?: ReactNode; alerta?: boolean;
}) {
  const conteudo = (
    <CardContent>
      <Stack direction="row" spacing={2} alignItems="center">
        {icone && <Box sx={{ color: alerta ? 'warning.main' : 'primary.main', display: 'flex', '& svg': { fontSize: 36 } }}>{icone}</Box>}
        <Box>
          <Typography variant="body2" color="text.secondary">
            {titulo}
          </Typography>
          <Typography variant="h2" component="p" sx={{ color: alerta ? 'warning.main' : undefined }}>
            {valor}
          </Typography>
          {descricao && (
            <Typography variant="caption" color="text.secondary">
              {descricao}
            </Typography>
          )}
        </Box>
      </Stack>
    </CardContent>
  );
  return <Card>{para ? <CardActionArea component={RouterLink} to={para}>{conteudo}</CardActionArea> : conteudo}</Card>;
}

export function Grade({ children, min = 240 }: { children: ReactNode; min?: number }) {
  return <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: `repeat(auto-fill, minmax(${min}px, 1fr))`, mb: 3 }}>{children}</Box>;
}

export function Vazio({ children }: { children: ReactNode }) {
  return (
    <Typography color="text.secondary" sx={{ py: 2 }}>
      {children}
    </Typography>
  );
}
