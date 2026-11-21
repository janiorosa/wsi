import { useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Alert, Box, LinearProgress, Typography } from '@mui/material';
import { api } from '../api/cliente';
import type { Tarefa } from '../api/tipos';

// Acompanha uma tarefa da fila até a conclusão, sem manter o usuário preso a uma requisição longa.
export function AcompanharTarefa({ tarefaId, aoConcluir, texto = 'O documento está sendo preparado...' }: {
  tarefaId: string; aoConcluir?: (t: Tarefa) => void; texto?: string;
}) {
  const avisado = useRef(false);
  const { data } = useQuery({
    queryKey: ['tarefa', tarefaId],
    queryFn: () => api.get<Tarefa>(`/tarefas/${tarefaId}`),
    refetchInterval: (q) => {
      const estado = q.state.data?.estado;
      return estado === 'concluida' || estado === 'falhou' ? false : 1500;
    },
  });
  useEffect(() => {
    if (data && (data.estado === 'concluida' || data.estado === 'falhou') && !avisado.current) {
      avisado.current = true;
      aoConcluir?.(data);
    }
  }, [data, aoConcluir]);

  if (data?.estado === 'falhou') {
    return <Alert severity="error">{data.erro || 'Não foi possível gerar o documento.'}</Alert>;
  }
  if (data?.estado === 'concluida') {
    return <Alert severity="success">Documento pronto.</Alert>;
  }
  return (
    <Box role="status" aria-live="polite" sx={{ my: 1 }}>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        {data?.estado === 'pendente' ? 'Na fila de geração...' : texto}
      </Typography>
      <LinearProgress variant={data && data.progresso > 5 ? 'determinate' : 'indeterminate'} value={data?.progresso ?? 0} />
    </Box>
  );
}
