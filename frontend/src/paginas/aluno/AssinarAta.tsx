import { useQueryClient } from '@tanstack/react-query';
import { Alert, Card, CardContent } from '@mui/material';
import { Carregando, Titulo } from '../../componentes/Comuns';
import { PainelAssinaturaATA } from '../../componentes/PainelAssinaturaATA';
import { usePainelAluno } from './dados';

// O aluno avaliado é o primeiro signatário e inicia o fluxo da ATA do próprio trabalho.
export function AssinarAta() {
  const { data } = usePainelAluno();
  const cliente = useQueryClient();
  if (!data) return <Carregando />;
  return (
    <>
      <Titulo titulo="ASSINAR ATA" subtitulo="Você vê somente a ATA do seu próprio trabalho." />
      <Card><CardContent>
        {data.ata ? (
          <PainelAssinaturaATA ata={data.ata} aoAtualizar={() => void cliente.invalidateQueries({ queryKey: ['aluno'] })} />
        ) : (
          <Alert severity="info">A ATA ainda não foi gerada. Ela é gerada automaticamente depois que todos os membros da banca concluírem a avaliação.</Alert>
        )}
      </CardContent></Card>
    </>
  );
}
