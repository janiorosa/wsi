import { useEffect, useState } from 'react';
import { Link as RouterLink, useNavigate, useSearchParams } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Alert, Button, Card, CardContent, Link, Stack, TextField, Typography } from '@mui/material';
import { mensagemDeErro, ErroApi } from '../../api/cliente';
import { destinoInicial, useSessao } from '../../sessao/SessaoContexto';
import { MolduraPublica } from '../../componentes/Publico';
import { campo } from '../../utilitarios/formularios';

const esquema = z.object({
  email: z.string().trim().email('Informe um e-mail válido.'),
  senha: z.string().min(1, 'Informe a senha.'),
});
type Dados = z.infer<typeof esquema>;

export function Entrar() {
  const { sessao, entrar } = useSessao();
  const navegar = useNavigate();
  const [parametros] = useSearchParams();
  const [erro, setErro] = useState<{ mensagem: string; codigo: string } | null>(null);
  const form = useForm<Dados>({ resolver: zodResolver(esquema), defaultValues: { email: '', senha: '' } });

  useEffect(() => {
    if (sessao.autenticado) navegar(destinoInicial(sessao), { replace: true });
  }, [sessao, navegar]);

  const enviar = form.handleSubmit(async (d) => {
    setErro(null);
    try {
      const s = await entrar(d.email, d.senha);
      const voltar = parametros.get('voltar');
      navegar(voltar && !s.precisaEscolherContexto ? voltar : destinoInicial(s), { replace: true });
    } catch (e) {
      setErro({ mensagem: mensagemDeErro(e), codigo: e instanceof ErroApi ? e.codigo : '' });
    }
  });

  return (
    <MolduraPublica largura="sm">
      <Card>
        <CardContent sx={{ p: { xs: 3, md: 4 } }}>
          <Typography variant="h1" component="h1" gutterBottom>Entrar</Typography>
          <form onSubmit={enviar} noValidate>
            <Stack spacing={2}>
              {erro && (
                <Alert severity="error">
                  {erro.mensagem}{' '}
                  {erro.codigo === 'primeiro_acesso_pendente' && <Link component={RouterLink} to="/primeiro-acesso">Ir para o primeiro acesso</Link>}
                </Alert>
              )}
              <TextField label="E-mail" type="email" autoComplete="email" autoFocus {...campo(form, 'email')} />
              <TextField label="Senha" type="password" autoComplete="current-password" {...campo(form, 'senha')} />
              <Button type="submit" variant="contained" size="large" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting ? 'Entrando...' : 'Entrar'}
              </Button>
              <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" spacing={1}>
                <Link component={RouterLink} to="/cadastro">Ainda não tenho conta</Link>
                <Link component={RouterLink} to="/primeiro-acesso">Primeiro acesso / esqueci a senha</Link>
              </Stack>
            </Stack>
          </form>
        </CardContent>
      </Card>
    </MolduraPublica>
  );
}
