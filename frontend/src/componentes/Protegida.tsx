import { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import type { Perfil } from '../api/tipos';
import { destinoInicial, useSessao } from '../sessao/SessaoContexto';
import { Carregando } from './Comuns';

// Proteção de rotas no frontend (conveniência). A autorização real é sempre feita no backend.
export function Protegida({ contexto, children }: { contexto?: Perfil; children: ReactNode }) {
  const { sessao, carregando } = useSessao();
  const local = useLocation();
  if (carregando) return <Carregando />;
  if (!sessao.autenticado) return <Navigate to={`/entrar?voltar=${encodeURIComponent(local.pathname + local.search)}`} replace />;
  if (contexto && sessao.contexto !== contexto) return <Navigate to={destinoInicial(sessao)} replace />;
  return <>{children}</>;
}
