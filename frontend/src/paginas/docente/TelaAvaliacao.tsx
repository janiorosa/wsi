import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Box, Button, Card, CardContent, Chip, Divider, InputAdornment, Stack, TextField, Typography } from '@mui/material';
import PictureAsPdfIcon from '@mui/icons-material/PictureAsPdf';
import { api, ErroApi, mensagemDeErro, urlApi } from '../../api/cliente';
import type { EventoTrabalho } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip, Titulo } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { formatarNota, mediaSimples, notaValida } from '../../utilitarios/notas';
import { estadosAvaliacao, rotulo } from '../../utilitarios/rotulos';

interface CriterioNota { id: string; nome: string; descricao: string; notaMinima: number; notaMaxima: number; origem: 'geral' | 'extra'; ordem: number; nota: number | null; comentario: string }
interface TelaDados {
  trabalho: { id: string; titulo: string; resumo: string; aluno: string; artigoId: string | null };
  evento: EventoTrabalho | null;
  papel: string;
  criterios: CriterioNota[];
  estado: string;
  notaCalculada: string | null;
  podeEditar: boolean;
  regra: string;
}

// Tela única: todas as notas preenchidas de uma vez, com conferência do resultado antes de confirmar.
export function TelaAvaliacao() {
  const { trabalhoId } = useParams();
  const navegar = useNavigate();
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const cliente = useQueryClient();
  const { data, isLoading, error, refetch } = useQuery({ queryKey: ['docente', 'avaliacao', trabalhoId], queryFn: () => api.get<TelaDados>(`/docente/avaliacoes/${trabalhoId}`) });
  const [valores, setValores] = useState<Record<string, string>>({});
  const [comentarios, setComentarios] = useState<Record<string, string>>({});
  const [errosServidor, setErrosServidor] = useState<Record<string, string>>({});
  const [salvando, setSalvando] = useState(false);

  useEffect(() => {
    if (data) {
      setValores(Object.fromEntries(data.criterios.map((c) => [c.id, c.nota === null ? '' : String(c.nota).replace('.', ',')])));
      setComentarios(Object.fromEntries(data.criterios.map((c) => [c.id, c.comentario])));
    }
  }, [data]);

  const numeros = useMemo(() => {
    const r: Record<string, number | null> = {};
    Object.entries(valores).forEach(([k, v]) => {
      const t = v.trim().replace(',', '.');
      r[k] = t === '' ? null : Number(t);
    });
    return r;
  }, [valores]);

  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;

  const invalidos = data.criterios.filter((c) => numeros[c.id] !== null && numeros[c.id] !== undefined && !notaValida(numeros[c.id] as number));
  const preenchidos = data.criterios.filter((c) => numeros[c.id] !== null && numeros[c.id] !== undefined && notaValida(numeros[c.id] as number));
  const completo = preenchidos.length === data.criterios.length && data.criterios.length > 0;
  const previa = completo ? mediaSimples(preenchidos.map((c) => numeros[c.id] as number)) : null;

  const enviar = async (finalizar: boolean) => {
    if (finalizar) {
      const r = await confirmar({
        titulo: 'Confirmar avaliação',
        mensagem: <>Sua nota para este trabalho será <strong>{formatarNota(previa)}</strong>. Após confirmar, a avaliação só poderá ser reaberta pelo orientador, com registro em auditoria.</>,
        confirmar: 'Confirmar avaliação',
      });
      if (!r.confirmado) return;
    }
    setSalvando(true);
    setErrosServidor({});
    try {
      const notas = data.criterios.map((c) => ({ criterioId: c.id, nota: numeros[c.id] ?? null, comentario: comentarios[c.id] ?? '' }));
      const resp = await api.put<{ estado: string; ataId?: string }>(`/docente/avaliacoes/${trabalhoId}`, { notas, finalizar });
      notificar(finalizar ? (resp.ataId ? 'Avaliação confirmada. Todas as avaliações foram concluídas: a ATA está sendo gerada.' : 'Avaliação confirmada.') : 'Rascunho salvo.');
      await cliente.invalidateQueries({ queryKey: ['docente'] });
      if (finalizar) navegar('/docente/avaliar');
      else void refetch();
    } catch (e) {
      if (e instanceof ErroApi && Object.keys(e.campos).length) setErrosServidor(e.campos);
      notificar(mensagemDeErro(e), 'error');
    } finally {
      setSalvando(false);
    }
  };

  return (
    <>
      <Titulo titulo="Avaliação do trabalho" subtitulo={data.papel === 'orientador' ? 'Você avalia como orientador (integrante da banca).' : 'Você avalia como examinador.'} acoes={<EstadoChip estado={data.estado} rotulo={rotulo(estadosAvaliacao, data.estado)} />} />
      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h2" component="h2">{data.trabalho.titulo}</Typography>
          <Typography color="text.secondary" sx={{ mb: 2 }}>{data.trabalho.aluno}{data.evento ? ` · ${data.evento.nome} — ${data.evento.data} às ${data.evento.horario}` : ''}</Typography>
          <Typography sx={{ whiteSpace: 'pre-line' }}>{data.trabalho.resumo}</Typography>
          {data.trabalho.artigoId ? (
            <Button sx={{ mt: 2 }} variant="outlined" startIcon={<PictureAsPdfIcon />} href={urlApi(`/arquivos/${data.trabalho.artigoId}?visualizar=1`)} target="_blank" rel="noopener">Abrir artigo</Button>
          ) : <Alert severity="info" sx={{ mt: 2 }}>O aluno ainda não enviou o PDF do artigo.</Alert>}
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Typography variant="h3" component="h2" gutterBottom>Notas por critério</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>{data.regra}</Typography>
          {data.criterios.length === 0 && <Alert severity="warning">Nenhum critério configurado para este trabalho. Procure a administração.</Alert>}
          <Stack spacing={2} divider={<Divider flexItem />}>
            {data.criterios.map((c) => {
              const n = numeros[c.id];
              const erroLocal = n !== null && n !== undefined && !notaValida(n) ? 'Use um valor de 0 a 100, com até duas casas decimais.' : undefined;
              const erro = erroLocal ?? errosServidor[c.id];
              return (
                <Box key={c.id} sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 180px' }, alignItems: 'start' }}>
                  <Box>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography fontWeight={700}>{c.nome}</Typography>
                      <Chip size="small" label={c.origem === 'extra' ? 'Critério extra' : 'Critério geral'} variant="outlined" />
                    </Stack>
                    {c.descricao && <Typography variant="body2" color="text.secondary">{c.descricao}</Typography>}
                    <TextField size="small" fullWidth sx={{ mt: 1 }} label="Comentário (opcional)" disabled={!data.podeEditar}
                      value={comentarios[c.id] ?? ''} onChange={(e) => setComentarios({ ...comentarios, [c.id]: e.target.value })} />
                  </Box>
                  <TextField
                    label="Nota"
                    inputMode="decimal"
                    disabled={!data.podeEditar}
                    value={valores[c.id] ?? ''}
                    onChange={(e) => setValores({ ...valores, [c.id]: e.target.value })}
                    error={!!erro}
                    helperText={erro ?? `Escala: ${c.notaMinima} a ${c.notaMaxima}`}
                    InputProps={{ endAdornment: <InputAdornment position="end">/ 100</InputAdornment> }}
                    inputProps={{ 'aria-label': `Nota para ${c.nome}` }}
                  />
                </Box>
              );
            })}
          </Stack>
          <Divider sx={{ my: 3 }} />
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ sm: 'center' }}>
            <Box>
              <Typography color="text.secondary">Resultado calculado (média simples dos critérios)</Typography>
              <Typography variant="h1" component="p">{data.estado === 'finalizada' ? data.notaCalculada : formatarNota(previa)}</Typography>
            </Box>
            {data.podeEditar && (
              <Stack direction="row" spacing={2}>
                <Button size="large" disabled={salvando || invalidos.length > 0} onClick={() => void enviar(false)}>Salvar rascunho</Button>
                <Button size="large" variant="contained" disabled={salvando || !completo} onClick={() => void enviar(true)}>Confirmar avaliação</Button>
              </Stack>
            )}
          </Stack>
        </CardContent>
      </Card>
    </>
  );
}
