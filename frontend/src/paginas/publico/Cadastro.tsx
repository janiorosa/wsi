import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Alert, Button, Card, CardContent, Stack, Tab, Tabs, TextField, Typography } from '@mui/material';
import { api, mensagemDeErro } from '../../api/cliente';
import type { Sessao } from '../../api/tipos';
import { destinoInicial, useSessao } from '../../sessao/SessaoContexto';
import { MolduraPublica } from '../../componentes/Publico';
import { aplicarErrosServidor, campo } from '../../utilitarios/formularios';

export const esquemaCadastro = z
  .object({
    tipo: z.enum(['aluno', 'docente']),
    nome: z.string().trim().min(3, 'Informe o nome completo.'),
    email: z.string().trim().email('Informe um e-mail válido.'),
    telefone: z.string().trim().max(30, 'Telefone muito longo.'),
    identificador: z.string().trim(),
    senha: z.string().min(8, 'A senha deve ter pelo menos 8 caracteres.'),
    confirmacao: z.string(),
  })
  .refine((d) => d.senha === d.confirmacao, { path: ['confirmacao'], message: 'As senhas não conferem.' })
  .refine((d) => d.tipo === 'docente' || d.identificador.length > 0, { path: ['identificador'], message: 'Informe a matrícula.' });
type Dados = z.infer<typeof esquemaCadastro>;

export function Cadastro() {
  const { aplicar } = useSessao();
  const navegar = useNavigate();
  const [erro, setErro] = useState('');
  const form = useForm<Dados>({
    resolver: zodResolver(esquemaCadastro),
    defaultValues: { tipo: 'aluno', nome: '', email: '', telefone: '', identificador: '', senha: '', confirmacao: '' },
  });
  const tipo = form.watch('tipo');

  const enviar = form.handleSubmit(async (d) => {
    setErro('');
    try {
      const { tipo: t, confirmacao: _c, ...corpo } = d;
      void _c;
      const s = await api.post<Sessao>(`/auth/cadastro/${t}`, corpo);
      aplicar(s);
      navegar(destinoInicial(s), { replace: true });
    } catch (e) {
      if (!aplicarErrosServidor(form, e)) setErro(mensagemDeErro(e));
    }
  });

  return (
    <MolduraPublica largura="sm">
      <Card>
        <CardContent sx={{ p: { xs: 3, md: 4 } }}>
          <Typography variant="h1" component="h1" gutterBottom>Criar conta</Typography>
          <Tabs value={tipo} onChange={(_, v) => form.setValue('tipo', v)} sx={{ mb: 2 }} aria-label="Tipo de conta">
            <Tab value="aluno" label="Sou aluno(a)" />
            <Tab value="docente" label="Sou docente" />
          </Tabs>
          <Alert severity="info" sx={{ mb: 2 }}>
            {tipo === 'aluno'
              ? 'Sua conta de aluno fica ativa imediatamente.'
              : 'O acesso às funções docentes é liberado após aprovação da administração.'}
          </Alert>
          <form onSubmit={enviar} noValidate>
            <Stack spacing={2}>
              {erro && <Alert severity="error">{erro}</Alert>}
              <TextField label="Nome completo" autoComplete="name" required {...campo(form, 'nome')} />
              <TextField label="E-mail" type="email" autoComplete="email" required {...campo(form, 'email')} />
              <TextField label="Telefone" autoComplete="tel" {...campo(form, 'telefone')} />
              <TextField label={tipo === 'aluno' ? 'Matrícula' : 'SIAPE (opcional)'} required={tipo === 'aluno'} {...campo(form, 'identificador')} />
              <TextField label="Senha" type="password" autoComplete="new-password" required {...campo(form, 'senha')} />
              <TextField label="Confirme a senha" type="password" autoComplete="new-password" required {...campo(form, 'confirmacao')} />
              <Button type="submit" variant="contained" size="large" disabled={form.formState.isSubmitting}>
                {tipo === 'aluno' ? 'Criar conta de aluno' : 'Solicitar cadastro de docente'}
              </Button>
            </Stack>
          </form>
        </CardContent>
      </Card>
    </MolduraPublica>
  );
}
