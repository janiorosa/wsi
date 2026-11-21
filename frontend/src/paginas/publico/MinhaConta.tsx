import { useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { Button, Stack, TextField } from '@mui/material';
import { api, mensagemDeErro } from '../../api/cliente';
import { Carregando, Secao, Titulo } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { aplicarErrosServidor, campo } from '../../utilitarios/formularios';

interface Conta { nome: string; email: string; telefone: string; matricula: string; curso: string; siape: string; ehAluno: boolean; ehDocente: boolean }

export function MinhaConta() {
  const notificar = useNotificar();
  const { data, refetch } = useQuery({ queryKey: ['conta'], queryFn: () => api.get<Conta>('/conta') });
  const form = useForm<Conta>();
  const senha = useForm<{ senhaAtual: string; novaSenha: string }>({ defaultValues: { senhaAtual: '', novaSenha: '' } });
  useEffect(() => {
    if (data) form.reset(data);
  }, [data, form]);
  if (!data) return <Carregando />;

  const salvar = form.handleSubmit(async (d) => {
    try {
      const r = await api.put<{ aviso?: string }>('/conta', d);
      notificar(r.aviso ?? 'Dados atualizados.', r.aviso ? 'warning' : 'success');
      void refetch();
    } catch (e) {
      if (!aplicarErrosServidor(form, e)) notificar(mensagemDeErro(e), 'error');
    }
  });
  const trocarSenha = senha.handleSubmit(async (d) => {
    try {
      await api.post('/conta/senha', d);
      notificar('Senha alterada.');
      senha.reset();
    } catch (e) {
      if (!aplicarErrosServidor(senha, e)) notificar(mensagemDeErro(e), 'error');
    }
  });

  return (
    <>
      <Titulo titulo="Minha conta" subtitulo={data.email} />
      <Secao titulo="Dados cadastrais">
        <form onSubmit={salvar} noValidate>
          <Stack spacing={2} sx={{ maxWidth: 520 }}>
            <TextField label="Nome completo" {...campo(form, 'nome')} />
            <TextField label="Telefone" {...campo(form, 'telefone')} />
            {data.ehAluno && <TextField label="Matrícula" {...campo(form, 'matricula')} />}
            {data.ehAluno && <TextField label="Curso" {...campo(form, 'curso')} />}
            {data.ehDocente && <TextField label="SIAPE" {...campo(form, 'siape')} />}
            <Button type="submit" variant="contained" sx={{ alignSelf: 'flex-start' }}>Salvar</Button>
          </Stack>
        </form>
      </Secao>
      <Secao titulo="Alterar senha">
        <form onSubmit={trocarSenha} noValidate>
          <Stack spacing={2} sx={{ maxWidth: 520 }}>
            <TextField label="Senha atual" type="password" autoComplete="current-password" {...campo(senha, 'senhaAtual')} />
            <TextField label="Nova senha (mínimo 8 caracteres)" type="password" autoComplete="new-password" {...campo(senha, 'novaSenha')} />
            <Button type="submit" variant="outlined" sx={{ alignSelf: 'flex-start' }}>Alterar senha</Button>
          </Stack>
        </form>
      </Secao>
    </>
  );
}
