import { createContext, ReactNode, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, definirCsrf, registrarAoSessaoExpirada } from '../api/cliente';
import type { Perfil, Sessao } from '../api/tipos';

const sessaoVazia: Sessao = {
  autenticado: false,
  perfis: [],
  contexto: '',
  precisaEscolherContexto: false,
  situacaoDocente: '',
  ehApresentador: false,
};

interface ValorSessao {
  sessao: Sessao;
  carregando: boolean;
  recarregar: () => Promise<Sessao>;
  aplicar: (s: Sessao) => void;
  entrar: (email: string, senha: string) => Promise<Sessao>;
  sair: () => Promise<void>;
  escolherContexto: (c: Perfil) => Promise<Sessao>;
}

const Contexto = createContext<ValorSessao | null>(null);

export function SessaoProvider({ children }: { children: ReactNode }) {
  const [sessao, setSessao] = useState<Sessao>(sessaoVazia);
  const [carregando, setCarregando] = useState(true);
  const queryClient = useQueryClient();

  const aplicar = useCallback((s: Sessao) => {
    definirCsrf(s.csrf);
    setSessao(s);
  }, []);

  const recarregar = useCallback(async () => {
    try {
      const s = await api.get<Sessao>('/auth/sessao');
      aplicar(s);
      return s;
    } catch {
      aplicar(sessaoVazia);
      return sessaoVazia;
    } finally {
      setCarregando(false);
    }
  }, [aplicar]);

  useEffect(() => {
    registrarAoSessaoExpirada(() => {
      void recarregar();
    });
    void recarregar();
  }, [recarregar]);

  const valor = useMemo<ValorSessao>(
    () => ({
      sessao,
      carregando,
      recarregar,
      aplicar,
      entrar: async (email, senha) => {
        const s = await api.post<Sessao>('/auth/entrar', { email, senha });
        queryClient.clear();
        aplicar(s);
        return s;
      },
      sair: async () => {
        try {
          await api.post('/auth/sair');
        } finally {
          queryClient.clear();
          aplicar(sessaoVazia);
        }
      },
      escolherContexto: async (c) => {
        const s = await api.post<Sessao>('/auth/contexto', { contexto: c });
        queryClient.clear();
        aplicar(s);
        return s;
      },
    }),
    [sessao, carregando, recarregar, aplicar, queryClient],
  );

  return <Contexto.Provider value={valor}>{children}</Contexto.Provider>;
}

export function useSessao() {
  const v = useContext(Contexto);
  if (!v) throw new Error('useSessao fora de SessaoProvider');
  return v;
}

// destinoInicial: para onde levar o usuário após login/recarga.
export function destinoInicial(s: Sessao): string {
  if (!s.autenticado) return '/entrar';
  if (s.precisaEscolherContexto) return '/contexto';
  if (s.contexto === 'ADMIN') return '/admin';
  if (s.contexto === 'DOCENTE') return '/docente';
  if (s.contexto === 'ALUNO') return '/aluno';
  return '/aguardando-aprovacao';
}
