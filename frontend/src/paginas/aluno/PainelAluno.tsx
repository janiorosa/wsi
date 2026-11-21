import { Link as RouterLink } from 'react-router-dom';
import { Alert, Box, Button, Card, CardContent, List, ListItem, ListItemIcon, ListItemText, Stack, Typography } from '@mui/material';
import UploadFileIcon from '@mui/icons-material/UploadFile';
import SlideshowIcon from '@mui/icons-material/Slideshow';
import GavelIcon from '@mui/icons-material/Gavel';
import DrawIcon from '@mui/icons-material/Draw';
import ArrowRightIcon from '@mui/icons-material/ArrowRight';
import { Carregando, ErroCarregamento, EstadoChip, Secao, Titulo } from '../../componentes/Comuns';
import { estadosTrabalho, rotulo } from '../../utilitarios/rotulos';
import { usePainelAluno } from './dados';

export function PainelAluno() {
  const { data, isLoading, error } = usePainelAluno();
  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;
  const t = data.trabalho;
  return (
    <>
      <Titulo titulo={`Olá, ${data.aluno.nome.split(' ')[0]}`} subtitulo={`Matrícula ${data.aluno.matricula || '—'} · ${data.aluno.curso}`} />
      {data.ata?.minhaVez && (
        <Alert severity="warning" sx={{ mb: 3 }} action={<Button color="inherit" component={RouterLink} to="/aluno/ata">Assinar</Button>}>
          <strong>Sua ATA está pronta.</strong> Você é o primeiro a assinar: baixe, assine e envie.
        </Alert>
      )}
      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(4, 1fr)' }, mb: 3 }}>
        <Button variant="contained" size="large" startIcon={<UploadFileIcon />} component={RouterLink} to="/aluno/tcc" sx={{ py: 3 }}>ENVIAR MEU TCC</Button>
        <Button variant="contained" color="secondary" size="large" startIcon={<SlideshowIcon />} component={RouterLink} to="/aluno/tcc#apresentacao" sx={{ py: 3 }}>ENVIAR APRESENTAÇÃO</Button>
        <Button variant="outlined" size="large" startIcon={<GavelIcon />} component={RouterLink} to="/aluno/declaracao" sx={{ py: 3 }}>GERAR DECLARAÇÃO ANTIPLÁGIO</Button>
        <Button variant="outlined" size="large" startIcon={<UploadFileIcon />} component={RouterLink} to="/aluno/declaracao#enviar" sx={{ py: 3 }}>ENVIAR DECLARAÇÃO ASSINADA</Button>
      </Box>

      <Box sx={{ display: 'grid', gap: 3, gridTemplateColumns: { xs: '1fr', md: '3fr 2fr' } }}>
        <Box>
          <Secao titulo="Próximos passos">
            <List dense>
              {data.proximosPassos.map((p) => (
                <ListItem key={p} disableGutters><ListItemIcon sx={{ minWidth: 32 }}><ArrowRightIcon color="primary" /></ListItemIcon><ListItemText primary={p} /></ListItem>
              ))}
            </List>
          </Secao>
          <Secao titulo="Meu trabalho">
            {t ? (
              <>
                <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
                  <EstadoChip estado={t.estado} rotulo={rotulo(estadosTrabalho, t.estado)} />
                </Stack>
                <Typography fontWeight={700}>{t.titulo || 'Título ainda não informado'}</Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                  Artigo: {t.arquivos.artigo.length ? `versão ${t.arquivos.artigo[0].versao} (${t.arquivos.artigo[0].enviadoEm})` : 'não enviado'} ·
                  Apresentação: {t.arquivos.apresentacao.length ? `versão ${t.arquivos.apresentacao[0].versao}` : 'não enviada'}
                </Typography>
                <Button sx={{ mt: 2 }} component={RouterLink} to="/aluno/tcc">Ver detalhes e histórico de arquivos</Button>
              </>
            ) : <Typography color="text.secondary">Você ainda não cadastrou seu TCC. Clique em “ENVIAR MEU TCC”.</Typography>}
          </Secao>
        </Box>
        <Box>
          <Card sx={{ mb: 3 }}><CardContent>
            <Typography variant="h3" component="h2" gutterBottom>Orientador</Typography>
            {data.orientador ? <Typography>{data.orientador.nome}<Typography component="span" variant="body2" color="text.secondary" display="block">{data.orientador.email} · desde {data.orientador.desde}</Typography></Typography>
              : <Button variant="contained" component={RouterLink} to="/aluno/orientador">Escolher orientador</Button>}
          </CardContent></Card>
          <Card sx={{ mb: 3 }}><CardContent>
            <Typography variant="h3" component="h2" gutterBottom>Evento</Typography>
            {t?.evento ? <Typography>{t.evento.nome}<Typography component="span" variant="body2" color="text.secondary" display="block">{t.evento.data} às {t.evento.horario} — {t.evento.local}</Typography></Typography>
              : <Typography color="text.secondary">Seu orientador associará o trabalho a um evento.</Typography>}
          </CardContent></Card>
          <Card><CardContent>
            <Typography variant="h3" component="h2" gutterBottom>ATA</Typography>
            {data.ata ? (
              <>
                <Typography>{data.ata.situacao}</Typography>
                <Button sx={{ mt: 1 }} variant={data.ata.minhaVez ? 'contained' : 'outlined'} startIcon={<DrawIcon />} component={RouterLink} to="/aluno/ata">ASSINAR ATA</Button>
              </>
            ) : <Typography color="text.secondary">A ATA é gerada após as avaliações da banca.</Typography>}
          </CardContent></Card>
        </Box>
      </Box>
    </>
  );
}
