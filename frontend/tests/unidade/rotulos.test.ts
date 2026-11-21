import { estadosTrabalho, formatarData, rotulo, tomDoEstado } from '../../src/utilitarios/rotulos';
import { esquemaCadastro } from '../../src/paginas/publico/Cadastro';

describe('rótulos e formatação', () => {
  it('traduz estados do trabalho', () => {
    expect(rotulo(estadosTrabalho, 'assinaturas_em_andamento')).toBe('Assinaturas em andamento');
    expect(rotulo(estadosTrabalho, null)).toBe('—');
  });
  it('formata data ISO no padrão brasileiro', () => {
    expect(formatarData('2026-07-14')).toBe('14/07/2026');
  });
  it('associa tons semânticos', () => {
    expect(tomDoEstado('concluido')).toBe('success');
    expect(tomDoEstado('falhou')).toBe('error');
  });
});

describe('validação do cadastro', () => {
  const base = { tipo: 'aluno' as const, nome: 'Fulana de Tal', email: 'fulana@exemplo.com', telefone: '', identificador: '0123', senha: 'senha-forte', confirmacao: 'senha-forte' };
  it('aceita cadastro válido de aluno', () => {
    expect(esquemaCadastro.safeParse(base).success).toBe(true);
  });
  it('exige matrícula do aluno', () => {
    expect(esquemaCadastro.safeParse({ ...base, identificador: '' }).success).toBe(false);
  });
  it('SIAPE é opcional para docente', () => {
    expect(esquemaCadastro.safeParse({ ...base, tipo: 'docente', identificador: '' }).success).toBe(true);
  });
  it('rejeita e-mail inválido e senhas diferentes', () => {
    expect(esquemaCadastro.safeParse({ ...base, email: 'invalido' }).success).toBe(false);
    expect(esquemaCadastro.safeParse({ ...base, confirmacao: 'outra-senha' }).success).toBe(false);
  });
});

import { esquemaPrimeiroAcesso } from '../../src/paginas/publico/PrimeiroAcesso';

describe('validação do primeiro acesso', () => {
  const base = { nome: 'Maria José da Silva', email: 'maria@exemplo.com', identificador: '0012345', senha: 'senha-forte', confirmacao: 'senha-forte' };
  it('exige nome, e-mail, matrícula/SIAPE e senha', () => {
    expect(esquemaPrimeiroAcesso.safeParse(base).success).toBe(true);
    expect(esquemaPrimeiroAcesso.safeParse({ ...base, identificador: '' }).success).toBe(false);
    expect(esquemaPrimeiroAcesso.safeParse({ ...base, nome: '' }).success).toBe(false);
  });
});
