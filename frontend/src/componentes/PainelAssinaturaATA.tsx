import { Alert, Box, Button, Stack, Step, StepLabel, Stepper, Typography } from '@mui/material';
import DownloadIcon from '@mui/icons-material/Download';
import type { ResumoATA } from '../api/tipos';
import { urlApi } from '../api/cliente';
import { EnvioPDF } from './EnvioPDF';

// Fluxo sequencial de assinaturas de UMA ATA (cada trabalho tem o seu).
export function PainelAssinaturaATA({ ata, aoAtualizar }: { ata: ResumoATA; aoAtualizar: () => void }) {
  if (ata.estado === 'gerando') {
    return ata.erroGeracao ? (
      <Alert severity="error">Falha na geração da ATA: {ata.erroGeracao}</Alert>
    ) : (
      <Alert severity="info">A ATA está sendo gerada automaticamente. Esta página será atualizada.</Alert>
    );
  }
  const ativo = ata.estado === 'finalizada' ? ata.signatarios.length : ata.posicaoAtual - 1;
  return (
    <Box>
      <Stepper activeStep={ativo} alternativeLabel sx={{ mb: 3, overflowX: 'auto', pb: 1 }}>
        {ata.signatarios.map((s) => (
          <Step key={s.ordem} completed={s.estado === 'assinada'}>
            <StepLabel optional={<Typography variant="caption">{s.estado === 'assinada' ? `Assinou em ${s.enviadoEm}` : s.ordem === ata.posicaoAtual && ata.estado !== 'finalizada' ? 'Vez de assinar' : 'Aguardando'}</Typography>}>
              {s.rotulo}
              <br />
              <Typography variant="caption" component="span" fontWeight={s.souEu ? 700 : 400}>
                {s.nome}{s.souEu ? ' (você)' : ''}
              </Typography>
            </StepLabel>
          </Step>
        ))}
      </Stepper>
      <Alert severity={ata.minhaVez ? 'warning' : ata.estado === 'finalizada' ? 'success' : 'info'} sx={{ mb: 2 }}>
        <strong>{ata.situacao}</strong>
        {ata.minhaVez && ' — baixe a ATA, assine (ex.: assinador gov.br) e envie o PDF assinado.'}
        {!ata.minhaVez && ata.estado === 'aguardando_assinaturas' && ata.souSignatario && ' — você será avisado no painel quando chegar a sua vez.'}
      </Alert>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
        {ata.podeBaixar && (
          <Button variant="outlined" size="large" startIcon={<DownloadIcon />} href={urlApi(`/atas/${ata.id}/documento`)}>
            {ata.estado === 'finalizada' ? 'Baixar ATA final' : 'Baixar ATA para assinar'}
          </Button>
        )}
        {ata.minhaVez && (
          <EnvioPDF
            size="large"
            caminho={`/atas/${ata.id}/assinatura`}
            campos={{ posicao: String(ata.posicaoAtual) }}
            rotulo="Enviar ATA assinada"
            mensagemSucesso="Assinatura registrada. O próximo signatário foi liberado."
            aoConcluir={aoAtualizar}
          />
        )}
      </Stack>
      <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 2 }}>
        O sistema registra o envio sequencial de PDFs assinados externamente; isso não equivale, por si só, a uma assinatura digital ICP-Brasil.
      </Typography>
    </Box>
  );
}
