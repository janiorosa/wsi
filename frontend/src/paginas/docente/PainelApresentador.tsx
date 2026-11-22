import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  Alert, Box, Button, Card, CardContent, Chip, IconButton, Stack, Table, TableBody, TableCell, TableHead, TableRow, Typography,
} from '@mui/material';
import MenuBookIcon from '@mui/icons-material/MenuBook';
import SlideshowIcon from '@mui/icons-material/Slideshow';
import PublishIcon from '@mui/icons-material/Publish';
import VisibilityIcon from '@mui/icons-material/Visibility';
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward';
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward';
import { api, mensagemDeErro, urlApi } from '../../api/cliente';
import type { Evento, Publicacao, ResultadoPremiacao } from '../../api/tipos';
import { Carregando, ErroCarregamento, EstadoChip, Secao, Titulo, Vazio } from '../../componentes/Comuns';
import { AcompanharTarefa } from '../../componentes/AcompanharTarefa';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { ResultadoPremiacoes } from '../../componentes/ResultadoPremiacoes';
import { estadosPublicacao, formatarData, rotulo } from '../../utilitarios/rotulos';

interface TrabalhoApresentador { id: string; titulo: string; aluno: string; ordem: number | null; sessao: string | null; artigoId: string | null; artigoVersao: number | null; apresentacaoId: string | null; apresentacaoVersao: number | null }
interface PainelDados { evento: Evento; trabalhos: TrabalhoApresentador[]; publicacoes: Publicacao[]; tarefasAtivas: { id: string; tipo: string }[]; podePublicar: boolean; premiacoes: ResultadoPremiacao[] }

export function PainelApresentador() {
  const { id } = useParams();
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const { data, isLoading, error, refetch } = useQuery({ queryKey: ['apresentador', 'evento', id], queryFn: () => api.get<PainelDados>(`/apresentador/eventos/${id}`) });
  const [ordem, setOrdem] = useState<TrabalhoApresentador[]>([]);
  const [tarefas, setTarefas] = useState<Record<string, string>>({});
  useEffect(() => {
    if (data) {
      setOrdem(data.trabalhos);
      setTarefas(Object.fromEntries(data.tarefasAtivas.map((t) => [t.tipo === 'gerar_anais' ? 'anais' : 'apresentacoes', t.id])));
    }
  }, [data]);
  if (isLoading) return <Carregando />;
  if (error || !data) return <ErroCarregamento erro={error} />;

  const gerar = async (tipo: 'anais' | 'apresentacoes') => {
    try {
      const r = await api.post<{ tarefaId: string }>(`/apresentador/eventos/${id}/publicacoes`, { tipo });
      setTarefas((t) => ({ ...t, [tipo]: r.tarefaId }));
      notificar('Geração iniciada. O PDF ficará disponível para conferência.');
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };
  const mover = (i: number, d: number) => {
    const n = [...ordem];
    const j = i + d;
    if (j < 0 || j >= n.length) return;
    [n[i], n[j]] = [n[j], n[i]];
    setOrdem(n);
  };
  const ordemAlterada = ordem.map((t) => t.id).join() !== data.trabalhos.map((t) => t.id).join();
  const semArtigo = data.trabalhos.filter((t) => !t.artigoId).length;
  const semApresentacao = data.trabalhos.filter((t) => !t.apresentacaoId).length;

  return (
    <>
      <Titulo titulo={`${data.evento.nome} ${data.evento.edicao}`} subtitulo={`Painel do apresentador · ${formatarData(data.evento.dataApresentacao)} — ${data.evento.local}`} />
      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, mb: 3 }}>
        {(['apresentacoes', 'anais'] as const).map((tipo) => (
          <Card key={tipo}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
                {tipo === 'anais' ? <MenuBookIcon color="primary" /> : <SlideshowIcon color="primary" />}
                <Typography variant="h3" component="h2">{tipo === 'anais' ? 'Anais do evento (artigos)' : 'PDF com todas as apresentações'}</Typography>
              </Stack>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                {tipo === 'anais' ? `Capa, folha de rosto e editorial (com os premiados e a maior nota) seguidos dos artigos, na ordem da programação. ${semArtigo} trabalho(s) sem artigo. Gere após o fim das avaliações e da votação.` : `Junta as apresentações na ordem da programação. ${semApresentacao} trabalho(s) sem apresentação.`}
                {' '}Nunca inclui ATAs, notas ou declarações.
              </Typography>
              {tipo === 'anais' && !data.evento.numeroEdicao && (
                <Alert severity="warning" sx={{ mb: 2 }}>O número da edição do evento não foi informado. Peça à administração para preenchê-lo antes de gerar os anais.</Alert>
              )}
              {tarefas[tipo] ? (
                <AcompanharTarefa tarefaId={tarefas[tipo]} aoConcluir={() => { setTarefas((t) => { const n = { ...t }; delete n[tipo]; return n; }); void refetch(); }} />
              ) : (
                <Button variant="contained" size="large" onClick={() => void gerar(tipo)}>
                  {tipo === 'anais' ? 'GERAR ANAIS DO EVENTO' : 'GERAR PDF COM TODAS AS APRESENTAÇÕES'}
                </Button>
              )}
            </CardContent>
          </Card>
        ))}
      </Box>

      <Secao titulo="Premiações (votos dos docentes)" dica="O trabalho mais votado de cada premiação (ou os empatados) é citado no editorial dos anais.">
        <ResultadoPremiacoes premiacoes={data.premiacoes} />
      </Secao>

      <Secao titulo="Documentos gerados" dica="Confira o PDF e o relatório de inclusão antes de publicar. A publicação substitui a versão publicada anterior.">
        {!data.podePublicar && <Alert severity="info" sx={{ mb: 2 }}>Você pode gerar e conferir, mas não recebeu permissão para publicar.</Alert>}
        {data.publicacoes.length === 0 ? <Vazio>Nenhum documento gerado ainda.</Vazio> : (
          <Stack spacing={2}>
            {data.publicacoes.map((p) => (
              <Card key={p.id} variant="outlined">
                <CardContent>
                  <Stack direction={{ xs: 'column', md: 'row' }} justifyContent="space-between" spacing={2}>
                    <Box>
                      <Typography fontWeight={700}>{p.tipo === 'anais' ? 'Anais' : 'Apresentações'} — versão {p.versao}</Typography>
                      <Typography variant="body2" color="text.secondary">
                        {p.geradoEm ? `Gerado em ${p.geradoEm} por ${p.geradoPor}` : 'Em preparação'}{p.totalPaginas ? ` · ${p.totalPaginas} páginas` : ''}
                        {p.publicadoEm ? ` · publicado em ${p.publicadoEm}` : ''}
                      </Typography>
                      {p.erro && <Alert severity="error" sx={{ mt: 1 }}>{p.erro}</Alert>}
                    </Box>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <EstadoChip estado={p.estado} rotulo={rotulo(estadosPublicacao, p.estado)} />
                      {p.totalPaginas && <Button startIcon={<VisibilityIcon />} href={urlApi(`/apresentador/publicacoes/${p.id}/arquivo?visualizar=1`)} target="_blank" rel="noopener">Conferir PDF</Button>}
                      {p.estado === 'aguardando_revisao' && data.podePublicar && (
                        <Button variant="contained" color="success" startIcon={<PublishIcon />} onClick={async () => {
                          const r = await confirmar({ titulo: 'Publicar documento', mensagem: 'O PDF ficará público na página do evento. Confirma que conferiu o conteúdo?', confirmar: 'Publicar' });
                          if (!r.confirmado) return;
                          try { await api.post(`/apresentador/publicacoes/${p.id}/publicar`); notificar('Publicado na página pública do evento.'); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
                        }}>Publicar</Button>
                      )}
                    </Stack>
                  </Stack>
                  {p.itens && p.itens.length > 0 && (
                    <Box sx={{ mt: 2, overflowX: 'auto' }}>
                      <Table size="small">
                        <TableHead><TableRow><TableCell>#</TableCell><TableCell>Trabalho</TableCell><TableCell>Aluno</TableCell><TableCell>Resultado</TableCell><TableCell>Páginas</TableCell></TableRow></TableHead>
                        <TableBody>
                          {p.itens.map((it) => (
                            <TableRow key={it.posicao}>
                              <TableCell>{it.posicao}</TableCell><TableCell>{it.titulo}</TableCell><TableCell>{it.aluno}</TableCell>
                              <TableCell><Chip size="small" color={it.resultado === 'incluido' ? 'success' : it.resultado === 'ausente' ? 'warning' : 'error'} label={it.resultado === 'incluido' ? `Incluído (v${it.versaoArquivo})` : it.resultado === 'ausente' ? 'Pendente: não enviado' : `Inválido: ${it.detalhe}`} /></TableCell>
                              <TableCell>{it.paginas ?? '—'}</TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </Box>
                  )}
                </CardContent>
              </Card>
            ))}
          </Stack>
        )}
      </Secao>

      <Secao titulo="Programação e arquivos" acoes={<Button variant="contained" disabled={!ordemAlterada} onClick={async () => {
        try { await api.put(`/apresentador/eventos/${id}/programacao`, { trabalhoIds: ordem.map((t) => t.id) }); notificar('Ordem salva.'); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
      }}>Salvar ordem</Button>}>
        {ordem.length === 0 ? <Vazio>Nenhum trabalho associado ao evento.</Vazio> : (
          <Box sx={{ overflowX: 'auto' }}>
            <Table size="small">
              <TableHead><TableRow><TableCell>#</TableCell><TableCell>Trabalho</TableCell><TableCell>Aluno</TableCell><TableCell>Artigo</TableCell><TableCell>Apresentação</TableCell><TableCell align="right">Ordem</TableCell></TableRow></TableHead>
              <TableBody>
                {ordem.map((t, i) => (
                  <TableRow key={t.id}>
                    <TableCell>{i + 1}</TableCell>
                    <TableCell>{t.titulo || <em>sem título</em>}</TableCell>
                    <TableCell>{t.aluno}</TableCell>
                    <TableCell>{t.artigoId ? <Button size="small" href={urlApi(`/arquivos/${t.artigoId}?visualizar=1`)} target="_blank">v{t.artigoVersao}</Button> : <Chip size="small" color="warning" label="Pendente" />}</TableCell>
                    <TableCell>{t.apresentacaoId ? <Button size="small" href={urlApi(`/arquivos/${t.apresentacaoId}?visualizar=1`)} target="_blank">v{t.apresentacaoVersao}</Button> : <Chip size="small" color="warning" label="Pendente" />}</TableCell>
                    <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                      <IconButton size="small" aria-label="Mover para cima" disabled={i === 0} onClick={() => mover(i, -1)}><ArrowUpwardIcon fontSize="small" /></IconButton>
                      <IconButton size="small" aria-label="Mover para baixo" disabled={i === ordem.length - 1} onClick={() => mover(i, 1)}><ArrowDownwardIcon fontSize="small" /></IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box>
        )}
      </Secao>
    </>
  );
}
