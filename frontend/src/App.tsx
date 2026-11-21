import { Navigate, Route, Routes } from 'react-router-dom';
import { Layout } from './componentes/Layout';
import { Protegida } from './componentes/Protegida';
import { Inicio } from './paginas/publico/Inicio';
import { EventoPublico } from './paginas/publico/EventoPublico';
import { Entrar } from './paginas/publico/Entrar';
import { Cadastro } from './paginas/publico/Cadastro';
import { PrimeiroAcesso } from './paginas/publico/PrimeiroAcesso';
import { EscolherContexto } from './paginas/publico/EscolherContexto';
import { AguardandoAprovacao } from './paginas/publico/AguardandoAprovacao';
import { MinhaConta } from './paginas/publico/MinhaConta';
import { PainelAdmin } from './paginas/admin/PainelAdmin';
import { ListaEventos } from './paginas/admin/ListaEventos';
import { FormularioEvento } from './paginas/admin/FormularioEvento';
import { DetalheEvento } from './paginas/admin/DetalheEvento';
import { Usuarios } from './paginas/admin/Usuarios';
import { Orientacoes } from './paginas/admin/Orientacoes';
import { Criterios } from './paginas/admin/Criterios';
import { Configuracoes } from './paginas/admin/Configuracoes';
import { Tarefas } from './paginas/admin/Tarefas';
import { Auditoria } from './paginas/admin/Auditoria';
import { PainelDocente } from './paginas/docente/PainelDocente';
import { Avaliar } from './paginas/docente/Avaliar';
import { TelaAvaliacao } from './paginas/docente/TelaAvaliacao';
import { MeusOrientandos } from './paginas/docente/MeusOrientandos';
import { FichaTrabalho } from './paginas/docente/FichaTrabalho';
import { AtasParaAssinar } from './paginas/docente/AtasParaAssinar';
import { EventosApresentador } from './paginas/docente/EventosApresentador';
import { PainelApresentador } from './paginas/docente/PainelApresentador';
import { PainelAluno } from './paginas/aluno/PainelAluno';
import { MeuTcc } from './paginas/aluno/MeuTcc';
import { Orientador } from './paginas/aluno/Orientador';
import { Declaracao } from './paginas/aluno/Declaracao';
import { AssinarAta } from './paginas/aluno/AssinarAta';

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Inicio />} />
      <Route path="/eventos/:slug" element={<EventoPublico />} />
      <Route path="/entrar" element={<Entrar />} />
      <Route path="/cadastro" element={<Cadastro />} />
      <Route path="/primeiro-acesso" element={<PrimeiroAcesso />} />
      <Route path="/contexto" element={<Protegida><EscolherContexto /></Protegida>} />
      <Route path="/aguardando-aprovacao" element={<Protegida><AguardandoAprovacao /></Protegida>} />

      <Route element={<Protegida><Layout /></Protegida>}>
        <Route path="/conta" element={<MinhaConta />} />
      </Route>

      <Route path="/admin" element={<Protegida contexto="ADMIN"><Layout /></Protegida>}>
        <Route index element={<PainelAdmin />} />
        <Route path="eventos" element={<ListaEventos />} />
        <Route path="eventos/novo" element={<FormularioEvento />} />
        <Route path="eventos/:id" element={<DetalheEvento />} />
        <Route path="eventos/:id/editar" element={<FormularioEvento />} />
        <Route path="usuarios" element={<Usuarios />} />
        <Route path="orientacoes" element={<Orientacoes />} />
        <Route path="criterios" element={<Criterios />} />
        <Route path="configuracoes" element={<Configuracoes />} />
        <Route path="tarefas" element={<Tarefas />} />
        <Route path="auditoria" element={<Auditoria />} />
      </Route>

      <Route path="/docente" element={<Protegida contexto="DOCENTE"><Layout /></Protegida>}>
        <Route index element={<PainelDocente />} />
        <Route path="avaliar" element={<Avaliar />} />
        <Route path="avaliar/:trabalhoId" element={<TelaAvaliacao />} />
        <Route path="orientandos" element={<MeusOrientandos />} />
        <Route path="trabalhos/:id" element={<FichaTrabalho />} />
        <Route path="atas" element={<AtasParaAssinar />} />
        <Route path="apresentador" element={<EventosApresentador />} />
        <Route path="apresentador/:id" element={<PainelApresentador />} />
      </Route>

      <Route path="/aluno" element={<Protegida contexto="ALUNO"><Layout /></Protegida>}>
        <Route index element={<PainelAluno />} />
        <Route path="tcc" element={<MeuTcc />} />
        <Route path="orientador" element={<Orientador />} />
        <Route path="declaracao" element={<Declaracao />} />
        <Route path="ata" element={<AssinarAta />} />
      </Route>

      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
