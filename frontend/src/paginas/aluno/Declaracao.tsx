import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Card, CardContent, Stack, Step, StepContent, StepLabel, Stepper, Typography } from '@mui/material';
import DownloadIcon from '@mui/icons-material/Download';
import GavelIcon from '@mui/icons-material/Gavel';
import { api, mensagemDeErro, urlApi } from '../../api/cliente';
import { Carregando, Titulo } from '../../componentes/Comuns';
import { EnvioPDF } from '../../componentes/EnvioPDF';
import { AcompanharTarefa } from '../../componentes/AcompanharTarefa';
import { useNotificar } from '../../componentes/Notificacoes';
import { usePainelAluno } from './dados';

export function Declaracao() {
  const { data } = usePainelAluno();
  const cliente = useQueryClient();
  const notificar = useNotificar();
  const [tarefa, setTarefa] = useState<string | null>(null);
  if (!data) return <Carregando />;
  const d = data.declaracao;
  const tarefaAtiva = tarefa ?? d.tarefaAtivaId;
  const atualizar = () => void cliente.invalidateQueries({ queryKey: ['aluno'] });
  const passo = d.assinadaId ? 3 : d.geradaId ? 1 : 0;

  const gerar = async () => {
    try {
      const r = await api.post<{ tarefaId: string }>('/aluno/declaracao/gerar');
      setTarefa(r.tarefaId);
    } catch (e) {
      notificar(mensagemDeErro(e), 'error');
    }
  };

  return (
    <>
      <Titulo titulo="Declaração anti-plágio" subtitulo="Gerada automaticamente no modelo institucional, com seus dados e a data de emissão." />
      <Card><CardContent>
        <Stepper activeStep={passo} orientation="vertical">
          <Step>
            <StepLabel>Gerar a declaração</StepLabel>
            <StepContent>
              {tarefaAtiva ? (
                <AcompanharTarefa tarefaId={tarefaAtiva} aoConcluir={() => { setTarefa(null); atualizar(); }} />
              ) : (
                <Button variant="contained" size="large" startIcon={<GavelIcon />} onClick={() => void gerar()}>GERAR DECLARAÇÃO ANTIPLÁGIO</Button>
              )}
            </StepContent>
          </Step>
          <Step>
            <StepLabel>Baixar, assinar e enviar</StepLabel>
            <StepContent>
              <Typography sx={{ mb: 2 }}>Baixe o PDF, assine (por exemplo, com o assinador gov.br) e envie o arquivo assinado.</Typography>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
                {d.geradaId && <Button variant="outlined" size="large" startIcon={<DownloadIcon />} href={urlApi(`/documentos/${d.geradaId}`)}>Baixar declaração ({d.geradaEm})</Button>}
                <span id="enviar"><EnvioPDF size="large" caminho="/aluno/declaracao/assinada" rotulo="ENVIAR DECLARAÇÃO ASSINADA" mensagemSucesso="Declaração assinada recebida." aoConcluir={atualizar} /></span>
              </Stack>
              <Button sx={{ mt: 2 }} onClick={() => void gerar()}>Gerar novamente</Button>
            </StepContent>
          </Step>
          <Step>
            <StepLabel>Concluído</StepLabel>
            <StepContent><span /></StepContent>
          </Step>
        </Stepper>
        {d.assinadaId && (
          <Alert severity="success" sx={{ mt: 2 }}>
            Declaração assinada enviada em {d.assinadaEm} ({d.assinadaNomeOriginal}).{' '}
            <Button size="small" href={urlApi(`/declaracoes-assinadas/${d.assinadaId}`)}>Baixar</Button>
            <span style={{ display: 'inline-block', marginLeft: 8 }}><EnvioPDF size="small" variant="text" caminho="/aluno/declaracao/assinada" rotulo="Substituir" aoConcluir={atualizar} /></span>
          </Alert>
        )}
      </CardContent></Card>
    </>
  );
}
