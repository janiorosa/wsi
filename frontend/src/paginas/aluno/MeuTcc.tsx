import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Alert, Box, Button, Card, CardContent, List, ListItem, ListItemText, Stack, TextField, Typography } from '@mui/material';
import PictureAsPdfIcon from '@mui/icons-material/PictureAsPdf';
import { api, mensagemDeErro, urlApi } from '../../api/cliente';
import { Carregando, ErroCarregamento, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { EnvioPDF } from '../../componentes/EnvioPDF';
import { useNotificar } from '../../componentes/Notificacoes';
import { aplicarErrosServidor, campo } from '../../utilitarios/formularios';
import { formatarTamanho } from '../../utilitarios/rotulos';
import { usePainelAluno } from './dados';

const esquema = z.object({
  titulo: z.string().trim().min(1, 'Informe o título.').max(400, 'Máximo de 400 caracteres.'),
  resumo: z.string().trim().min(1, 'Informe o resumo.').max(6000, 'Máximo de 6000 caracteres.'),
  linkOverleaf: z.string().trim().refine((v) => v === '' || /^https?:\/\/\S+$/.test(v), 'Informe um endereço válido (https://...).'),
});
type Dados = z.infer<typeof esquema>;

// Título/resumo, artigo e apresentação podem ser enviados em momentos diferentes.
export function MeuTcc() {
  const { data, isLoading, error } = usePainelAluno();
  const cliente = useQueryClient();
  const notificar = useNotificar();
  const local = useLocation();
  const form = useForm<Dados>({ resolver: zodResolver(esquema), defaultValues: { titulo: '', resumo: '', linkOverleaf: '' } });
  useEffect(() => {
    if (data?.trabalho) form.reset({ titulo: data.trabalho.titulo, resumo: data.trabalho.resumo, linkOverleaf: data.trabalho.linkOverleaf });
  }, [data, form]);
  useEffect(() => {
    if (local.hash) document.getElementById(local.hash.slice(1))?.scrollIntoView({ behavior: 'smooth' });
  }, [local.hash, data]);
  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;
  const atualizar = () => void cliente.invalidateQueries({ queryKey: ['aluno'] });
  const editavel = data.trabalho?.editavel ?? true;

  const salvar = form.handleSubmit(async (d) => {
    try {
      await api.put('/aluno/trabalho', d);
      notificar('Dados do TCC salvos.');
      atualizar();
    } catch (e) {
      if (!aplicarErrosServidor(form, e)) notificar(mensagemDeErro(e), 'error');
    }
  });

  return (
    <>
      <Titulo titulo="ENVIAR MEU TCC" subtitulo="Você pode salvar o título e o resumo e enviar o artigo e a apresentação em momentos diferentes." />
      {!data.orientador && <Alert severity="info" sx={{ mb: 3 }}>Você ainda não tem orientador. Escolha um em “Orientador”.</Alert>}
      <Secao titulo="Título e resumo">
        {!editavel && <Alert severity="info" sx={{ mb: 2 }}>A ATA já foi gerada: título e resumo não podem mais ser alterados.</Alert>}
        <form onSubmit={salvar} noValidate>
          <Stack spacing={2}>
            <TextField label="Título do trabalho" required disabled={!editavel} {...campo(form, 'titulo')} />
            <TextField label="Resumo" required multiline minRows={6} disabled={!editavel} {...campo(form, 'resumo')} />
            <TextField label="Link do Overleaf (opcional)" disabled={!editavel} {...campo(form, 'linkOverleaf')} />
            {editavel && <Button type="submit" variant="contained" size="large" sx={{ alignSelf: 'flex-start' }} disabled={form.formState.isSubmitting}>Salvar título e resumo</Button>}
          </Stack>
        </form>
      </Secao>
      <Box sx={{ display: 'grid', gap: 3, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
        {(['artigo', 'apresentacao'] as const).map((tipo) => {
          const arquivos = data.trabalho?.arquivos[tipo] ?? [];
          return (
            <Card key={tipo} id={tipo}>
              <CardContent>
                <Typography variant="h3" component="h2" gutterBottom>{tipo === 'artigo' ? 'PDF do artigo' : 'PDF da apresentação'}</Typography>
                <EnvioPDF size="large" caminho={`/aluno/trabalho/arquivos/${tipo}`} rotulo={arquivos.length ? 'Enviar nova versão' : tipo === 'artigo' ? 'Enviar artigo' : 'ENVIAR APRESENTAÇÃO'} aoConcluir={atualizar} />
                <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>Somente PDF, até 30 MB. Versões anteriores ficam no histórico.</Typography>
                {arquivos.length === 0 ? <Vazio>Nenhum arquivo enviado.</Vazio> : (
                  <List dense>
                    {arquivos.map((a, i) => (
                      <ListItem key={a.id} disableGutters secondaryAction={<Button size="small" startIcon={<PictureAsPdfIcon />} href={urlApi(`/arquivos/${a.id}?visualizar=1`)} target="_blank" rel="noopener">Abrir</Button>}>
                        <ListItemText primary={`Versão ${a.versao}${i === 0 ? ' (atual)' : ''} — ${a.nomeOriginal}`} secondary={`${a.enviadoEm} · ${formatarTamanho(a.tamanho)}`} />
                      </ListItem>
                    ))}
                  </List>
                )}
              </CardContent>
            </Card>
          );
        })}
      </Box>
    </>
  );
}
