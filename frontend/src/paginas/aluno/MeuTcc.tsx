import { useEffect, useRef, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Alert, Box, Button, Card, CardContent, Chip, List, ListItem, ListItemText, Stack, TextField, Typography } from '@mui/material';
import PictureAsPdfIcon from '@mui/icons-material/PictureAsPdf';
import CloudDoneIcon from '@mui/icons-material/CloudDone';
import SyncIcon from '@mui/icons-material/Sync';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import { api, mensagemDeErro, urlApi } from '../../api/cliente';
import { Carregando, ErroCarregamento, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { EnvioPDF } from '../../componentes/EnvioPDF';
import { useNotificar } from '../../componentes/Notificacoes';
import { aplicarErrosServidor, campo } from '../../utilitarios/formularios';
import { formatarTamanho } from '../../utilitarios/rotulos';
import { usePainelAluno } from './dados';

// Salvamento automático: aceita campos ainda vazios (o aluno pode escrever aos poucos).
const esquema = z.object({
  titulo: z.string().max(400, 'Máximo de 400 caracteres.'),
  resumo: z.string().max(6000, 'Máximo de 6000 caracteres.'),
  linkOverleaf: z.string().trim().refine((v) => v === '' || /^https?:\/\/\S+$/.test(v), 'Informe um endereço válido (https://...).'),
});
type Dados = z.infer<typeof esquema>;
type Situacao = 'ocioso' | 'salvando' | 'salvo' | 'erro';

// Título/resumo, artigo e apresentação podem ser enviados em momentos diferentes.
export function MeuTcc() {
  const { data, isLoading, error } = usePainelAluno();
  const cliente = useQueryClient();
  const notificar = useNotificar();
  const local = useLocation();
  const form = useForm<Dados>({ resolver: zodResolver(esquema), mode: 'onChange', defaultValues: { titulo: '', resumo: '', linkOverleaf: '' } });
  const valores = useWatch({ control: form.control });
  const [situacao, setSituacao] = useState<Situacao>('ocioso');
  const [salvoEm, setSalvoEm] = useState('');
  const carregado = useRef(false);
  const ultimoSalvo = useRef('');

  // Preenche o formulário somente na primeira carga: o salvamento automático não sobrescreve o que está sendo digitado.
  useEffect(() => {
    if (data && !carregado.current) {
      carregado.current = true;
      const inicial = { titulo: data.trabalho?.titulo ?? '', resumo: data.trabalho?.resumo ?? '', linkOverleaf: data.trabalho?.linkOverleaf ?? '' };
      ultimoSalvo.current = JSON.stringify(inicial);
      form.reset(inicial);
    }
  }, [data, form]);
  useEffect(() => {
    if (local.hash) document.getElementById(local.hash.slice(1))?.scrollIntoView({ behavior: 'smooth' });
  }, [local.hash, data]);

  const editavel = data?.trabalho?.editavel ?? true;
  useEffect(() => {
    if (!carregado.current || !editavel) return;
    const atual = { titulo: valores.titulo ?? '', resumo: valores.resumo ?? '', linkOverleaf: valores.linkOverleaf ?? '' };
    const serializado = JSON.stringify(atual);
    if (serializado === ultimoSalvo.current) return;
    const id = setTimeout(async () => {
      if (!(await form.trigger())) return;
      setSituacao('salvando');
      try {
        await api.put('/aluno/trabalho', atual);
        ultimoSalvo.current = serializado;
        setSituacao('salvo');
        setSalvoEm(new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' }));
        void cliente.invalidateQueries({ queryKey: ['aluno'] });
      } catch (e) {
        setSituacao('erro');
        if (!aplicarErrosServidor(form, e)) notificar(mensagemDeErro(e), 'error');
      }
    }, 1000);
    return () => clearTimeout(id);
  }, [valores.titulo, valores.resumo, valores.linkOverleaf, editavel]); // salvamento com atraso de 1 s após a última digitação

  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;
  const atualizar = () => void cliente.invalidateQueries({ queryKey: ['aluno'] });

  return (
    <>
      <Titulo titulo="ENVIAR MEU TCC" subtitulo="O título e o resumo são salvos automaticamente enquanto você digita. O artigo e a apresentação podem ser enviados em momentos diferentes." />
      {!data.orientador && <Alert severity="info" sx={{ mb: 3 }}>Você ainda não tem orientador. Escolha um em “Orientador”.</Alert>}
      <Secao titulo="Título e resumo" acoes={editavel ? (
        situacao === 'salvando' ? <Chip size="small" icon={<SyncIcon />} label="Salvando..." variant="outlined" />
          : situacao === 'erro' ? <Chip size="small" color="error" icon={<ErrorOutlineIcon />} label="Não foi possível salvar" />
            : situacao === 'salvo' ? <Chip size="small" color="success" variant="outlined" icon={<CloudDoneIcon />} label={`Salvo automaticamente às ${salvoEm}`} />
              : <Chip size="small" variant="outlined" icon={<CloudDoneIcon />} label="Salvamento automático" />
      ) : undefined}>
        {!editavel && <Alert severity="info" sx={{ mb: 2 }}>A ATA já foi gerada: título e resumo não podem mais ser alterados.</Alert>}
        <Stack spacing={2}>
          <TextField label="Título do trabalho" required disabled={!editavel} {...campo(form, 'titulo')} />
          <TextField label="Resumo" required multiline minRows={6} disabled={!editavel} {...campo(form, 'resumo')} />
          <TextField label="Link do Overleaf (opcional)" disabled={!editavel} {...campo(form, 'linkOverleaf')} />
        </Stack>
      </Secao>

      {data.pareceres.length > 0 && (
        <Secao titulo="Pareceres da banca" dica="Comentários dos avaliadores para você melhorar e corrigir o artigo. São informativos e não alteram a nota.">
          <Stack spacing={2}>
            {data.pareceres.map((p, i) => (
              <Card key={i} variant="outlined">
                <CardContent>
                  <Typography fontWeight={700}>{p.avaliador} <Typography component="span" variant="body2" color="text.secondary">· {p.papel === 'orientador' ? 'Orientador' : 'Examinador'}{p.atualizadoEm ? ` · atualizado em ${p.atualizadoEm}` : ''}</Typography></Typography>
                  <Typography sx={{ whiteSpace: 'pre-line', mt: 1 }}>{p.parecer}</Typography>
                </CardContent>
              </Card>
            ))}
          </Stack>
        </Secao>
      )}

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
