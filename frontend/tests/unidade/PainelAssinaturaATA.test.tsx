import { render, screen } from '@testing-library/react';
import { ThemeProvider } from '@mui/material';
import { tema } from '../../src/tema';
import { PainelAssinaturaATA } from '../../src/componentes/PainelAssinaturaATA';
import type { ResumoATA } from '../../src/api/tipos';

const base: ResumoATA = {
  id: 'ata-1', versao: 1, estado: 'aguardando_assinaturas', posicaoAtual: 1, minhaVez: true, souSignatario: true,
  situacao: 'Aguardando sua assinatura', erroGeracao: '', geradaEm: '14/07/2026 20:00', podeBaixar: true,
  signatarios: [
    { ordem: 1, rotulo: 'Aluno', nome: 'Aluna Teste', estado: 'pendente', enviadoEm: null, souEu: true },
    { ordem: 2, rotulo: 'Professor Orientador', nome: 'Orientador Teste', estado: 'pendente', enviadoEm: null, souEu: false },
    { ordem: 3, rotulo: 'Examinador 1', nome: 'Examinador Teste', estado: 'pendente', enviadoEm: null, souEu: false },
  ],
};

function renderizar(ata: ResumoATA) {
  return render(<ThemeProvider theme={tema}><PainelAssinaturaATA ata={ata} aoAtualizar={() => undefined} /></ThemeProvider>);
}

describe('PainelAssinaturaATA', () => {
  it('na vez do signatário, mostra baixar e enviar', () => {
    renderizar(base);
    expect(screen.getByText('Baixar ATA para assinar')).toBeInTheDocument();
    expect(screen.getByText('Enviar ATA assinada')).toBeInTheDocument();
    expect(screen.getByText('Orientador Teste')).toBeInTheDocument();
  });
  it('fora da vez, não permite baixar nem enviar', () => {
    renderizar({ ...base, minhaVez: false, podeBaixar: false, posicaoAtual: 2, situacao: 'Aguardando assinaturas dos docentes' });
    expect(screen.queryByText('Baixar ATA para assinar')).not.toBeInTheDocument();
    expect(screen.queryByText('Enviar ATA assinada')).not.toBeInTheDocument();
  });
  it('finalizada: oferece somente a ATA final', () => {
    renderizar({ ...base, estado: 'finalizada', minhaVez: false, podeBaixar: true, posicaoAtual: 4, situacao: 'ATA finalizada' });
    expect(screen.getByText('Baixar ATA final')).toBeInTheDocument();
    expect(screen.queryByText('Enviar ATA assinada')).not.toBeInTheDocument();
  });
});
