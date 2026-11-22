import { useEffect, useMemo, useRef, useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Box, Button, Card, CardContent, Chip, Divider, InputAdornment, Stack, TextField, Typography } from '@mui/material';
import PictureAsPdfIcon from '@mui/icons-material/PictureAsPdf';
import CloudDoneIcon from '@mui/icons-material/CloudDone';
import SyncIcon from '@mui/icons-material/Sync';
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline';
import SendIcon from '@mui/icons-material/Send';
import { api, mensagemDeErro, urlApi } from '../../api/cliente';
import type { EventoTrabalho } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { formatarNota, mediaSimples, notaValida } from '../../utilitarios/notas';
import { estadosAvaliacao, rotulo } from '../../utilitarios/rotulos';

interface CriterioNota { id: string; nome: string; descricao: string; notaMinima: number; notaMaxima: number; origem: 'geral' | 'extra'; ordem: number; nota: number | null; comentario: string }
export interface DadosAvaliacao {
  trabalho: { id: string; titulo: string; resumo: string; aluno: string; artigoId: string | null };
  evento: EventoTrabalho | null;
  papel: string;
  criterios: CriterioNota[];
  estado: string;
  notaCalculada: string | null;
  podeEditar: boolean;
  regra: string;
  parecer: string;
  parecerAtualizadoEm: string | null;
  podeEditarParecer: boolean;
}

type Situacao = 'ocioso' | 'salvando' | 'salvo' | 'erro';
const ATRASO_MS = 800;

function paraNumero(v: string): number | null {
  const t = v.trim().replace(',', '.');
  return t === '' ? null : Number(t);
}

function agora() {
  return new Date().toLocaleTimeString('pt-BR', { hour: '2-digit', minute: '2-digit' });
}

function IndicadorSalvamento({ situacao, rotuloSalvo }: { situacao: Situacao; rotuloSalvo: string }) {
  if (situacao === 'salvando') return <Chip size="small" icon={<SyncIcon />} label="Salvando..." variant="outlined" />;
  if (situacao === 'erro') return <Chip size="small" color="error" icon={<ErrorOutlineIcon />} label="Não foi possível salvar" />;
  if (situacao === 'salvo') return <Chip size="small" color="success" variant="outlined" icon={<CloudDoneIcon />} label={rotuloSalvo} />;
  return null;
}

// Avaliação de um trabalho: notas e parecer são salvos automaticamente enquanto o docente digita.
// A nota só fica definitiva ao clicar em "Enviar avaliação"; até lá pode ser alterada a qualquer momento.
// O parecer (texto para o aluno) é independente da nota e pode ser alterado até a geração da ATA.
export function FormularioAvaliacao({ trabalhoId, compacto = false, aoEnviar }: { trabalhoId: string; compacto?: boolean; aoEnviar?: () => void }) {
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const cliente = useQueryClient();
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['docente', 'avaliacao', trabalhoId],
    queryFn: () => api.get<DadosAvaliacao>(`/docente/avaliacoes/${trabalhoId}`),
  });
  const [valores, setValores] = useState<Record<string, string>>({});
  const [comentarios, setComentarios] = useState<Record<string, string>>({});
  const [parecer, setParecer] = useState('');
  const [situacaoNotas, setSituacaoNotas] = useState<Situacao>('ocioso');
  const [situacaoParecer, setSituacaoParecer] = useState<Situacao>('ocioso');
  const [salvoNotasEm, setSalvoNotasEm] = useState('');
  const [salvoParecerEm, setSalvoParecerEm] = useState('');
  const [errosServidor, setErrosServidor] = useState<Record<string, string>>({});
  const [enviando, setEnviando] = useState(false);
  // Valores carregados do servidor: só reinicializa o formulário na carga inicial ou quando o estado muda
  // (assim o salvamento automático nunca apaga o que o docente está digitando).
  const carregado = useRef<string>('');
  const alteradoNotas = useRef(false);
  const alteradoParecer = useRef(false);

  useEffect(() => {
    if (!data) return;
    const chave = `${trabalhoId}:${data.estado}:${data.podeEditar}`;
    if (carregado.current === chave) return;
    carregado.current = chave;
    alteradoNotas.current = false;
    alteradoParecer.current = false;
    setValores(Object.fromEntries(data.criterios.map((c) => [c.id, c.nota === null ? '' : String(c.nota).replace('.', ',')])));
    setComentarios(Object.fromEntries(data.criterios.map((c) => [c.id, c.comentario])));
    setParecer(data.parecer ?? '');
  }, [data, trabalhoId]);

  const numeros = useMemo(() => {
    const r: Record<string, number | null> = {};
    Object.entries(valores).forEach(([k, v]) => { r[k] = paraNumero(v); });
    return r;
  }, [valores]);

  // Salvamento automático das notas (somente valores válidos; vazio = sem nota).
  useEffect(() => {
    if (!data || !data.podeEditar || !alteradoNotas.current) return;
    const id = setTimeout(async () => {
      const notas = data.criterios
        .filter((c) => numeros[c.id] === null || numeros[c.id] === undefined || notaValida(numeros[c.id] as number))
        .map((c) => ({ criterioId: c.id, nota: numeros[c.id] ?? null, comentario: comentarios[c.id] ?? '' }));
      setSituacaoNotas('salvando');
      try {
        await api.put(`/docente/avaliacoes/${trabalhoId}`, { notas, finalizar: false });
        setErrosServidor({});
        setSituacaoNotas('salvo');
        setSalvoNotasEm(agora());
      } catch (e) {
        setSituacaoNotas('erro');
        notificar(mensagemDeErro(e), 'error');
      }
    }, ATRASO_MS);
    return () => clearTimeout(id);
  }, [valores, comentarios]);

  // Salvamento automático do parecer.
  useEffect(() => {
    if (!data || !data.podeEditarParecer || !alteradoParecer.current) return;
    const id = setTimeout(async () => {
      setSituacaoParecer('salvando');
      try {
        await api.put(`/docente/avaliacoes/${trabalhoId}/parecer`, { parecer });
        setSituacaoParecer('salvo');
        setSalvoParecerEm(agora());
      } catch (e) {
        setSituacaoParecer('erro');
        notificar(mensagemDeErro(e), 'error');
      }
    }, ATRASO_MS + 400);
    return () => clearTimeout(id);
  }, [parecer]);

  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;

  const preenchidos = data.criterios.filter((c) => numeros[c.id] !== null && numeros[c.id] !== undefined && notaValida(numeros[c.id] as number));
  const completo = preenchidos.length === data.criterios.length && data.criterios.length > 0;
  const previa = completo ? mediaSimples(preenchidos.map((c) => numeros[c.id] as number)) : null;
  const finalizada = data.estado === 'finalizada';

  const enviar = async () => {
    const r = await confirmar({
      titulo: 'Enviar avaliação',
      mensagem: <>Sua nota para este trabalho será <strong>{formatarNota(previa)}</strong>. Depois de enviada, a nota não pode mais ser alterada (somente o orientador pode reabri-la, com registro em auditoria). O parecer continua editável até a geração da ATA.</>,
      confirmar: 'Enviar avaliação',
    });
    if (!r.confirmado) return;
    setEnviando(true);
    try {
      const notas = data.criterios.map((c) => ({ criterioId: c.id, nota: numeros[c.id] ?? null, comentario: comentarios[c.id] ?? '' }));
      const resp = await api.put<{ estado: string; ataId?: string }>(`/docente/avaliacoes/${trabalhoId}`, { notas, finalizar: true });
      notificar(resp.ataId ? 'Avaliação enviada. Todas as avaliações foram concluídas: a ATA está sendo gerada.' : 'Avaliação enviada.');
      await cliente.invalidateQueries({ queryKey: ['docente'] });
      await refetch();
      aoEnviar?.();
    } catch (e) {
      const campos = (e as { campos?: Record<string, string> }).campos;
      if (campos && Object.keys(campos).length) setErrosServidor(campos);
      notificar(mensagemDeErro(e), 'error');
    } finally {
      setEnviando(false);
    }
  };

  const cabecalho = (
    <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} justifyContent="space-between" alignItems={{ sm: 'flex-start' }} sx={{ mb: 2 }}>
      <Box sx={{ minWidth: 0 }}>
        <Typography variant={compacto ? 'h3' : 'h2'} component="h2">{data.trabalho.titulo || 'Trabalho sem título'}</Typography>
        <Typography color="text.secondary" variant="body2">
          {data.trabalho.aluno}{data.evento ? ` · ${data.evento.nome} — ${data.evento.data} às ${data.evento.horario}` : ''} · {data.papel === 'orientador' ? 'você avalia como orientador' : 'você avalia como examinador'}
        </Typography>
      </Box>
      <Stack direction="row" spacing={1} alignItems="center" flexShrink={0}>
        <EstadoChip estado={data.estado} rotulo={rotulo(estadosAvaliacao, data.estado)} />
        {data.trabalho.artigoId && (
          <Button size="small" variant="outlined" startIcon={<PictureAsPdfIcon />} href={urlApi(`/arquivos/${data.trabalho.artigoId}?visualizar=1`)} target="_blank" rel="noopener">Abrir artigo</Button>
        )}
      </Stack>
    </Stack>
  );

  return (
    <Card variant={compacto ? 'outlined' : 'elevation'} sx={{ mb: compacto ? 2 : 3 }}>
      <CardContent>
        {cabecalho}
        {!compacto && <Typography sx={{ whiteSpace: 'pre-line', mb: 2 }}>{data.trabalho.resumo}</Typography>}
        {!data.trabalho.artigoId && <Alert severity="info" sx={{ mb: 2 }}>O aluno ainda não enviou o PDF do artigo.</Alert>}
        {data.criterios.length === 0 && <Alert severity="warning" sx={{ mb: 2 }}>Nenhum critério configurado para este trabalho. Procure a administração.</Alert>}

        <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
          <Typography fontWeight={700}>{data.criterios.length > 1 ? 'Notas por critério' : 'Nota'}</Typography>
          {data.podeEditar && <IndicadorSalvamento situacao={situacaoNotas} rotuloSalvo={`Rascunho salvo às ${salvoNotasEm}`} />}
        </Stack>
        {!compacto && <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>{data.regra}</Typography>}
        <Stack spacing={2} divider={compacto ? undefined : <Divider flexItem />}>
          {data.criterios.map((c) => {
            const n = numeros[c.id];
            const erroLocal = n !== null && n !== undefined && !notaValida(n) ? 'Use um valor de 0 a 100, com até duas casas decimais.' : undefined;
            const erro = erroLocal ?? errosServidor[c.id];
            return (
              <Box key={c.id} sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: compacto ? '1fr 200px' : '1fr 180px' }, alignItems: 'start' }}>
                <Box>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography fontWeight={600}>{c.nome}</Typography>
                    {data.criterios.length > 1 && <Chip size="small" label={c.origem === 'extra' ? 'Critério extra' : 'Critério geral'} variant="outlined" />}
                  </Stack>
                  {c.descricao && <Typography variant="body2" color="text.secondary">{c.descricao}</Typography>}
                  {!compacto && (
                    <TextField size="small" fullWidth sx={{ mt: 1 }} label="Comentário sobre o critério (opcional)" disabled={!data.podeEditar}
                      value={comentarios[c.id] ?? ''} onChange={(e) => { alteradoNotas.current = true; setComentarios({ ...comentarios, [c.id]: e.target.value }); }} />
                  )}
                </Box>
                <TextField
                  label="Nota"
                  inputMode="decimal"
                  disabled={!data.podeEditar}
                  value={valores[c.id] ?? ''}
                  onChange={(e) => { alteradoNotas.current = true; setValores({ ...valores, [c.id]: e.target.value }); }}
                  error={!!erro}
                  helperText={erro ?? `Escala: ${c.notaMinima} a ${c.notaMaxima}`}
                  InputProps={{ endAdornment: <InputAdornment position="end">/ 100</InputAdornment> }}
                  inputProps={{ 'aria-label': `Nota para ${c.nome}` }}
                />
              </Box>
            );
          })}
        </Stack>

        <Box sx={{ mt: 3 }}>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
            <Typography fontWeight={700}>Parecer para o aluno</Typography>
            {data.podeEditarParecer && <IndicadorSalvamento situacao={situacaoParecer} rotuloSalvo={`Parecer salvo às ${salvoParecerEm}`} />}
          </Stack>
          <TextField
            fullWidth
            multiline
            minRows={compacto ? 3 : 6}
            placeholder="Comentários, sugestões e correções para o aluno melhorar o artigo (apenas informativo; não altera a nota)."
            disabled={!data.podeEditarParecer}
            value={parecer}
            onChange={(e) => { alteradoParecer.current = true; setParecer(e.target.value); }}
            helperText={data.podeEditarParecer
              ? 'Salvo automaticamente. Pode ser escrito antes da nota e alterado até a geração da ATA. O aluno vê este parecer.'
              : 'A ATA já foi gerada: o parecer não pode mais ser alterado.'}
            inputProps={{ 'aria-label': 'Parecer para o aluno', maxLength: 20000 }}
          />
        </Box>

        <Divider sx={{ my: 3 }} />
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="space-between" alignItems={{ sm: 'center' }}>
          <Box>
            <Typography color="text.secondary" variant="body2">{finalizada ? 'Sua nota enviada' : 'Resultado calculado (média simples dos critérios)'}</Typography>
            <Typography variant={compacto ? 'h2' : 'h1'} component="p">{finalizada ? data.notaCalculada : formatarNota(previa)}</Typography>
          </Box>
          <Stack direction="row" spacing={1} alignItems="center">
            {compacto && <Button component={RouterLink} to={`/docente/avaliar/${trabalhoId}`}>Abrir tela completa</Button>}
            {data.podeEditar && (
              <Button size="large" variant="contained" startIcon={<SendIcon />} disabled={enviando || !completo} onClick={() => void enviar()}>
                Enviar avaliação
              </Button>
            )}
          </Stack>
        </Stack>
        {data.podeEditar && !completo && data.criterios.length > 0 && (
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
            A nota é salva automaticamente como rascunho e pode ser alterada a qualquer momento. Preencha todas as notas para enviar.
          </Typography>
        )}
      </CardContent>
    </Card>
  );
}
