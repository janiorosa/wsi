import { formatarNota, mediaSimples, notaValida } from '../../src/utilitarios/notas';

describe('regra de notas (mesma do backend)', () => {
  it('critério único é a própria nota', () => {
    expect(mediaSimples([87.5])).toBe(87.5);
  });
  it('média simples, sem pesos e sem soma', () => {
    expect(mediaSimples([80, 90, 100])).toBe(90);
  });
  it('arredonda meio para cima com 2 casas', () => {
    expect(mediaSimples([90, 85, 85.01])).toBe(86.67);
    expect(mediaSimples([0.01, 0])).toBe(0.01);
    expect(mediaSimples([10, 20, 20])).toBe(16.67);
  });
  it('valida escala 0 a 100 com até 2 casas', () => {
    expect(notaValida(100)).toBe(true);
    expect(notaValida(0)).toBe(true);
    expect(notaValida(100.01)).toBe(false);
    expect(notaValida(-1)).toBe(false);
    expect(notaValida(87.555)).toBe(false);
  });
  it('formata com vírgula', () => {
    expect(formatarNota(86.6666)).toBe('86,67');
    expect(formatarNota(null)).toBe('—');
  });
});
