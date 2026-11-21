import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Autocomplete, Box, Button, Card, FormControlLabel, Stack, Switch, Table, TableBody, TableCell, TableHead, TableRow, TextField } from '@mui/material';
import { api, ErroApi, mensagemDeErro } from '../../api/cliente';
import type { Pagina } from '../../api/tipos';
import { Carregando, EstadoChip, Secao, Titulo } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';

interface Vinculo { id: string; alunoNome: string; docenteNome: string; situacao: string; inicio: string; fim: string | null; criadoPor: string; motivoEncerramento: string }
interface Pessoa { id: string; nome: string; email: string }

export function Orientacoes() {
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  const [aluno, setAluno] = useState<Pessoa | null>(null);
  const [docente, setDocente] = useState<Pessoa | null>(null);
  const [troca, setTroca] = useState(false);
  const { data, isLoading, refetch } = useQuery({ queryKey: ['admin', 'vinculos'], queryFn: () => api.get<Vinculo[]>('/admin/vinculos') });
  const { data: alunos } = useQuery({ queryKey: ['admin', 'usuarios', 'ALUNO-todos'], queryFn: () => api.get<Pagina<Pessoa>>('/admin/usuarios?perfil=ALUNO&tamanho=100') });
  const { data: docentes } = useQuery({ queryKey: ['admin', 'usuarios', 'DOCENTE-todos'], queryFn: () => api.get<Pagina<Pessoa>>('/admin/usuarios?perfil=DOCENTE&tamanho=100') });

  const vincular = async () => {
    try {
      await api.post('/admin/vinculos', { alunoId: aluno!.id, docenteId: docente!.id, confirmarTroca: troca });
      notificar('Vínculo de orientação criado.');
      setAluno(null); setDocente(null); setTroca(false);
      void refetch();
    } catch (e) {
      if (e instanceof ErroApi && e.codigo === 'aluno_com_orientador') {
        notificar('O aluno já possui orientador. Marque "Confirmar troca" para substituir explicitamente.', 'warning');
      } else notificar(mensagemDeErro(e), 'error');
    }
  };

  return (
    <>
      <Titulo titulo="Orientações" subtitulo="Cada aluno tem um orientador principal ativo. Trocas e encerramentos preservam o histórico." />
      <Secao titulo="Novo vínculo">
        <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems={{ md: 'center' }}>
          <Autocomplete sx={{ minWidth: 280 }} options={alunos?.itens ?? []} getOptionLabel={(o) => `${o.nome} (${o.email})`} value={aluno} onChange={(_, v) => setAluno(v)} renderInput={(p) => <TextField {...p} label="Aluno" size="small" />} />
          <Autocomplete sx={{ minWidth: 280 }} options={docentes?.itens ?? []} getOptionLabel={(o) => o.nome} value={docente} onChange={(_, v) => setDocente(v)} renderInput={(p) => <TextField {...p} label="Docente orientador" size="small" />} />
          <FormControlLabel control={<Switch checked={troca} onChange={(_, v) => setTroca(v)} />} label="Confirmar troca" />
          <Button variant="contained" disabled={!aluno || !docente} onClick={() => void vincular()}>Vincular</Button>
        </Stack>
      </Secao>
      <Secao titulo="Vínculos">
        {isLoading ? <Carregando /> : (
          <Card variant="outlined"><Box sx={{ overflowX: 'auto' }}>
            <Table size="small">
              <TableHead><TableRow><TableCell>Aluno</TableCell><TableCell>Orientador</TableCell><TableCell>Período</TableCell><TableCell>Situação</TableCell><TableCell /></TableRow></TableHead>
              <TableBody>
                {data?.map((v) => (
                  <TableRow key={v.id}>
                    <TableCell>{v.alunoNome}</TableCell>
                    <TableCell>{v.docenteNome}</TableCell>
                    <TableCell>{v.inicio}{v.fim ? ` → ${v.fim}` : ''}{v.motivoEncerramento && <Box component="span" sx={{ display: 'block', fontSize: 12, color: 'text.secondary' }}>{v.motivoEncerramento}</Box>}</TableCell>
                    <TableCell><EstadoChip estado={v.situacao} rotulo={v.situacao === 'ativo' ? 'Ativo' : 'Encerrado'} /></TableCell>
                    <TableCell align="right">
                      {v.situacao === 'ativo' && <Button size="small" color="error" onClick={async () => {
                        const r = await confirmar({ titulo: 'Encerrar orientação', mensagem: `Encerrar a orientação de ${v.alunoNome} por ${v.docenteNome}? O histórico e os trabalhos são preservados.`, perigoso: true, confirmar: 'Encerrar', pedirMotivo: 'Motivo' });
                        if (r.confirmado) {
                          try { await api.post(`/admin/vinculos/${v.id}/encerrar`, { motivo: r.motivo }); notificar('Vínculo encerrado.'); void refetch(); } catch (e) { notificar(mensagemDeErro(e), 'error'); }
                        }
                      }}>Encerrar</Button>}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Box></Card>
        )}
      </Secao>
    </>
  );
}
