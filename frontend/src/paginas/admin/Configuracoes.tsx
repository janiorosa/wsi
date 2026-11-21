import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Box, Button, Card, CardContent, Checkbox, FormControlLabel, IconButton, Stack, TextField, Typography } from '@mui/material';
import DeleteIcon from '@mui/icons-material/Delete';
import AddIcon from '@mui/icons-material/Add';
import { api, mensagemDeErro } from '../../api/cliente';
import { Carregando, Titulo } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';

interface Linha { texto: string; negrito: boolean }

export function Configuracoes() {
  const notificar = useNotificar();
  const { data } = useQuery({ queryKey: ['admin', 'configuracoes'], queryFn: () => api.get<{ cabecalho: Linha[]; cidade: string }>('/admin/configuracoes') });
  const [linhas, setLinhas] = useState<Linha[]>([]);
  const [cidade, setCidade] = useState('');
  useEffect(() => {
    if (data) { setLinhas(data.cabecalho); setCidade(data.cidade); }
  }, [data]);
  if (!data) return <Carregando />;
  const salvar = async () => {
    try {
      await api.put('/admin/configuracoes', { cabecalho: linhas, cidade });
      notificar('Configurações salvas. Os próximos documentos gerados usarão estes dados.');
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  return (
    <>
      <Titulo titulo="Dados institucionais" subtitulo="Cabeçalho impresso em todas as páginas da ATA e da declaração anti-plágio." />
      <Alert severity="warning" sx={{ mb: 3 }}>Altere somente com orientação institucional. O brasão é inserido automaticamente acima destas linhas.</Alert>
      <Card><CardContent>
        <Stack spacing={1.5}>
          {linhas.map((l, i) => (
            <Stack key={i} direction="row" spacing={1} alignItems="center">
              <TextField fullWidth size="small" label={`Linha ${i + 1}`} value={l.texto} onChange={(e) => setLinhas(linhas.map((x, j) => (j === i ? { ...x, texto: e.target.value } : x)))} />
              <FormControlLabel control={<Checkbox checked={l.negrito} onChange={(_, v) => setLinhas(linhas.map((x, j) => (j === i ? { ...x, negrito: v } : x)))} />} label="Negrito" />
              <IconButton aria-label="Remover linha" onClick={() => setLinhas(linhas.filter((_, j) => j !== i))}><DeleteIcon /></IconButton>
            </Stack>
          ))}
          <Button startIcon={<AddIcon />} disabled={linhas.length >= 8} onClick={() => setLinhas([...linhas, { texto: '', negrito: false }])} sx={{ alignSelf: 'flex-start' }}>Adicionar linha</Button>
          <TextField label="Cidade (usada na data da declaração)" value={cidade} onChange={(e) => setCidade(e.target.value)} sx={{ maxWidth: 360 }} />
        </Stack>
        <Typography variant="h3" component="h2" sx={{ mt: 4, mb: 1 }}>Prévia do cabeçalho</Typography>
        <Box sx={{ border: '1px dashed', borderColor: 'divider', p: 2, textAlign: 'center', fontFamily: '"Latin Modern Roman", "Computer Modern", Georgia, serif' }}>
          {linhas.map((l, i) => <div key={i} style={{ fontWeight: l.negrito ? 700 : 400 }}>{l.texto}</div>)}
        </Box>
        <Button variant="contained" size="large" sx={{ mt: 3 }} onClick={() => void salvar()}>Salvar</Button>
      </CardContent></Card>
    </>
  );
}
