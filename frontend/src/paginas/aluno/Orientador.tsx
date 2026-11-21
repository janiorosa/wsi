import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Alert, Autocomplete, Button, Card, CardContent, Stack, TextField, Typography } from '@mui/material';
import { api, mensagemDeErro } from '../../api/cliente';
import type { DocenteResumo } from '../../api/tipos';
import { Carregando, Titulo } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { useConfirmar } from '../../componentes/Confirmacao';
import { usePainelAluno } from './dados';

export function Orientador() {
  const { data } = usePainelAluno();
  const { data: docentes } = useQuery({ queryKey: ['aluno', 'docentes'], queryFn: () => api.get<DocenteResumo[]>('/aluno/docentes') });
  const [escolhido, setEscolhido] = useState<DocenteResumo | null>(null);
  const cliente = useQueryClient();
  const notificar = useNotificar();
  const confirmar = useConfirmar();
  if (!data) return <Carregando />;
  const atual = data.orientador;

  const salvar = async () => {
    if (!escolhido) return;
    let confirmarTroca = false;
    if (atual) {
      const r = await confirmar({
        titulo: 'Trocar de orientador',
        mensagem: `Seu vínculo com ${atual.nome} será encerrado e ${escolhido.nome} passará a ser seu orientador. A troca fica registrada.`,
        confirmar: 'Confirmar troca', perigoso: true,
      });
      if (!r.confirmado) return;
      confirmarTroca = true;
    }
    try {
      await api.post('/aluno/orientador', { docenteId: escolhido.id, confirmarTroca });
      notificar('Orientador definido.');
      setEscolhido(null);
      void cliente.invalidateQueries({ queryKey: ['aluno'] });
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };

  return (
    <>
      <Titulo titulo="Orientador" />
      <Card><CardContent>
        {atual ? (
          <Alert severity="success" sx={{ mb: 3 }}>Seu orientador é <strong>{atual.nome}</strong> ({atual.email}) desde {atual.desde}.</Alert>
        ) : <Alert severity="info" sx={{ mb: 3 }}>Você ainda não tem orientador. Escolha um docente abaixo.</Alert>}
        <Typography fontWeight={700} gutterBottom>{atual ? 'Solicitar troca de orientador' : 'Escolher orientador'}</Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
          <Autocomplete sx={{ minWidth: 320 }} options={(docentes ?? []).filter((d) => d.id !== atual?.id)} value={escolhido} onChange={(_, v) => setEscolhido(v)}
            getOptionLabel={(o) => o.nome} renderInput={(p) => <TextField {...p} label="Docente" />} />
          <Button variant="contained" size="large" disabled={!escolhido} onClick={() => void salvar()}>{atual ? 'Trocar orientador' : 'Confirmar orientador'}</Button>
        </Stack>
      </CardContent></Card>
    </>
  );
}
