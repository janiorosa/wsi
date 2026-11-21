// Mesma regra do backend (media-simples-v1): média simples, sem pesos, 2 casas, meio-para-cima.
// Usada apenas para prévia na tela; o valor oficial é sempre calculado no servidor.
export function mediaSimples(valores: number[]): number | null {
  if (valores.length === 0) return null;
  const centesimos = valores.map((v) => Math.round(v * 100));
  const soma = centesimos.reduce((a, b) => a + b, 0);
  const n = centesimos.length;
  return Math.floor((2 * soma + n) / (2 * n)) / 100;
}

export function formatarNota(v: number | null | undefined): string {
  if (v === null || v === undefined) return '—';
  return v.toFixed(2).replace('.', ',');
}

export function notaValida(v: number): boolean {
  return Number.isFinite(v) && v >= 0 && v <= 100 && Math.abs(v * 100 - Math.round(v * 100)) < 1e-6;
}
