import { useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Alert, Button, Card, CardContent, Link, Stack, TextField, Typography } from '@mui/material';
import { api, ErroApi, mensagemDeErro } from '../../api/cliente';
import { MolduraPublica } from '../../componentes/Publico';
import { aplicarErrosServidor, campo } from '../../utilitarios/formularios';

export const esquemaPrimeiroAcesso = z
  .object({
    nome: z.string().trim().min(3, 'Informe o nome completo, como cadastrado.'),
    email: z.string().trim().email('Informe um e-mail válido.'),
    identificador: z.string().trim().min(1, 'Informe a matrícula (aluno) ou o SIAPE (docente).'),
    senha: z.string().min(8, 'A senha deve ter pelo menos 8 caracteres.'),
    confirmacao: z.string(),
  })
  .refine((d) => d.senha === d.confirmacao, { path: ['confirmacao'], message: 'As senhas não conferem.' });
type Dados = z.infer<typeof esquemaPrimeiroAcesso>;

// Primeiro acesso: a pessoa confirma nome completo, e-mail e matrícula/SIAPE e cria a senha.
// Só vale para contas ainda sem senha; depois disso, somente a administração redefine o acesso.
export function PrimeiroAcesso() {
  const [concluido, setConcluido] = useState(false);
  const [erro, setErro] = useState<{ mensagem: string; jaTemSenha: boolean } | null>(null);
  const form = useForm<Dados>({
    resolver: zodResolver(esquemaPrimeiroAcesso),
    defaultValues: { nome: '', email: '', identificador: '', senha: '', confirmacao: '' },
  });
  const enviar = form.handleSubmit(async (d) => {
    setErro(null);
    try {
      await api.post('/auth/primeiro-acesso', { nome: d.nome, email: d.email, identificador: d.identificador, senha: d.senha });
      setConcluido(true);
    } catch (e) {
      if (!aplicarErrosServidor(form, e)) setErro({ mensagem: mensagemDeErro(e), jaTemSenha: e instanceof ErroApi && e.codigo === 'senha_ja_definida' });
    }
  });
  return (
    <MolduraPublica largura="sm">
      <Card>
        <CardContent sx={{ p: { xs: 3, md: 4 } }}>
          <Typography variant="h1" component="h1" gutterBottom>Primeiro acesso</Typography>
          {concluido ? (
            <Alert severity="success">
              Senha criada com sucesso. <Link component={RouterLink} to="/entrar">Entrar agora</Link>
            </Alert>
          ) : (
            <form onSubmit={enviar} noValidate>
              <Alert severity="info" sx={{ mb: 2 }}>
                Use esta tela para criar sua senha se o seu cadastro foi feito pela administração ou pelo seu orientador.
                Informe os dados <strong>exatamente como cadastrados</strong>. <strong>Esqueceu a senha?</strong> Peça à
                administração para redefinir o seu acesso e depois volte a esta tela.
              </Alert>
              <Stack spacing={2}>
                {erro && <Alert severity={erro.jaTemSenha ? 'warning' : 'error'}>{erro.mensagem}</Alert>}
                <TextField label="Nome completo" autoComplete="name" required {...campo(form, 'nome')} />
                <TextField label="E-mail" type="email" autoComplete="email" required {...campo(form, 'email')} />
                <TextField label="Matrícula (aluno) ou SIAPE (docente)" required {...campo(form, 'identificador')} />
                <TextField label="Nova senha" type="password" autoComplete="new-password" required {...campo(form, 'senha')} />
                <TextField label="Confirme a nova senha" type="password" autoComplete="new-password" required {...campo(form, 'confirmacao')} />
                <Button type="submit" variant="contained" size="large" disabled={form.formState.isSubmitting}>Criar senha</Button>
              </Stack>
            </form>
          )}
        </CardContent>
      </Card>
    </MolduraPublica>
  );
}
