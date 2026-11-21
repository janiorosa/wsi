// Cliente HTTP da API. Autenticação por cookie HttpOnly; CSRF via cabeçalho X-CSRF-Token.
const BASE = '/api/v1';
let tokenCsrf = '';

export function definirCsrf(token: string | undefined) {
  tokenCsrf = token ?? '';
}

export class ErroApi extends Error {
  status: number;
  codigo: string;
  campos: Record<string, string>;
  constructor(status: number, codigo: string, mensagem: string, campos?: Record<string, string>) {
    super(mensagem);
    this.status = status;
    this.codigo = codigo;
    this.campos = campos ?? {};
  }
}

let aoSessaoExpirada: (() => void) | null = null;
export function registrarAoSessaoExpirada(f: () => void) {
  aoSessaoExpirada = f;
}

async function tratarResposta<T>(resp: Response): Promise<T> {
  if (!resp.ok) {
    let dados: { erro?: { codigo?: string; mensagem?: string; campos?: Record<string, string> } } | null = null;
    try {
      dados = await resp.json();
    } catch {
      dados = null;
    }
    const erro = dados?.erro;
    if (resp.status === 401 || erro?.codigo === 'csrf') aoSessaoExpirada?.();
    throw new ErroApi(
      resp.status,
      erro?.codigo ?? 'erro',
      erro?.mensagem ?? (resp.status === 413 ? 'Arquivo grande demais.' : 'Não foi possível concluir a operação.'),
      erro?.campos,
    );
  }
  if (resp.status === 204) return undefined as T;
  return (await resp.json()) as T;
}

async function requisitar<T>(metodo: string, caminho: string, corpo?: unknown): Promise<T> {
  const cabecalhos: Record<string, string> = { Accept: 'application/json' };
  if (corpo !== undefined) cabecalhos['Content-Type'] = 'application/json';
  if (metodo !== 'GET' && tokenCsrf) cabecalhos['X-CSRF-Token'] = tokenCsrf;
  let resp: Response;
  try {
    resp = await fetch(BASE + caminho, {
      method: metodo,
      credentials: 'same-origin',
      headers: cabecalhos,
      body: corpo !== undefined ? JSON.stringify(corpo) : undefined,
    });
  } catch {
    throw new ErroApi(0, 'rede', 'Sem conexão com o servidor. Verifique sua internet e tente novamente.');
  }
  return tratarResposta<T>(resp);
}

export const api = {
  get: <T>(caminho: string) => requisitar<T>('GET', caminho),
  post: <T>(caminho: string, corpo?: unknown) => requisitar<T>('POST', caminho, corpo ?? {}),
  put: <T>(caminho: string, corpo?: unknown) => requisitar<T>('PUT', caminho, corpo ?? {}),
  patch: <T>(caminho: string, corpo?: unknown) => requisitar<T>('PATCH', caminho, corpo ?? {}),
  del: <T>(caminho: string) => requisitar<T>('DELETE', caminho),
};

// enviarArquivo envia multipart/form-data com o campo "arquivo".
export async function enviarArquivo<T>(caminho: string, arquivo: File, campos?: Record<string, string>): Promise<T> {
  const dados = new FormData();
  Object.entries(campos ?? {}).forEach(([k, v]) => dados.append(k, v));
  dados.append('arquivo', arquivo);
  let resp: Response;
  try {
    resp = await fetch(BASE + caminho, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-CSRF-Token': tokenCsrf, Accept: 'application/json' },
      body: dados,
    });
  } catch {
    throw new ErroApi(0, 'rede', 'Falha de conexão durante o envio. Tente novamente.');
  }
  return tratarResposta<T>(resp);
}

export function urlApi(caminho: string) {
  return BASE + caminho;
}

export function mensagemDeErro(erro: unknown): string {
  if (erro instanceof ErroApi) return erro.message;
  if (erro instanceof Error) return erro.message;
  return 'Erro inesperado.';
}
