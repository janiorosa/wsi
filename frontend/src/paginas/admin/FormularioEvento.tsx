import { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { Box, Button, Card, CardContent, MenuItem, Stack, TextField } from '@mui/material';
import { api, mensagemDeErro } from '../../api/cliente';
import type { Evento } from '../../api/tipos';
import { Carregando, Titulo } from '../../componentes/Comuns';
import { useNotificar } from '../../componentes/Notificacoes';
import { aplicarErrosServidor, campo } from '../../utilitarios/formularios';
import { estadosEvento } from '../../utilitarios/rotulos';

const esquema = z.object({
  nome: z.string().trim().min(1, 'Informe o nome do evento.'),
  edicao: z.string().trim(),
  descricao: z.string(),
  dataApresentacao: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Informe a data.'),
  horarioApresentacao: z.string().regex(/^([01]\d|2[0-3]):[0-5]\d$/, 'Informe o horário (HH:MM).'),
  local: z.string().trim().min(1, 'Informe o local.'),
  campus: z.string().trim(),
  estado: z.string(),
  prazoSubmissao: z.string(),
  prazoApresentacao: z.string(),
  informacoesPublicas: z.string(),
});
type Dados = z.infer<typeof esquema>;

const vazio: Dados = {
  nome: '', edicao: String(new Date().getFullYear()), descricao: '', dataApresentacao: '', horarioApresentacao: '19:00',
  local: 'Salão de Convenções do Campus IFMG Ouro Branco', campus: 'Ouro Branco', estado: 'rascunho', prazoSubmissao: '', prazoApresentacao: '', informacoesPublicas: '',
};

export function FormularioEvento() {
  const { id } = useParams();
  const navegar = useNavigate();
  const notificar = useNotificar();
  const cliente = useQueryClient();
  const form = useForm<Dados>({ resolver: zodResolver(esquema), defaultValues: vazio });
  const { data, isLoading } = useQuery({
    queryKey: ['admin', 'evento', id],
    queryFn: () => api.get<{ evento: Evento }>(`/admin/eventos/${id}`),
    enabled: !!id,
  });
  useEffect(() => {
    if (data) {
      const e = data.evento;
      form.reset({ ...e, prazoSubmissao: e.prazoSubmissao ?? '', prazoApresentacao: e.prazoApresentacao ?? '' });
    }
  }, [data, form]);

  const salvar = form.handleSubmit(async (d) => {
    try {
      if (id) {
        await api.put(`/admin/eventos/${id}`, d);
        notificar('Evento atualizado.');
        await cliente.invalidateQueries({ queryKey: ['admin'] });
        navegar(`/admin/eventos/${id}`);
      } else {
        const r = await api.post<{ id: string }>('/admin/eventos', d);
        notificar('Evento criado.');
        await cliente.invalidateQueries({ queryKey: ['admin'] });
        navegar(`/admin/eventos/${r.id}`);
      }
    } catch (e) {
      if (!aplicarErrosServidor(form, e)) notificar(mensagemDeErro(e), 'error');
    }
  });

  if (id && isLoading) return <Carregando />;
  return (
    <>
      <Titulo titulo={id ? 'Editar evento' : 'Criar evento'} subtitulo="A data, o horário e o local cadastrados aqui são os que constarão nas ATAs dos trabalhos deste evento." />
      <Card>
        <CardContent>
          <form onSubmit={salvar} noValidate>
            <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '2fr 1fr' } }}>
              <TextField label="Nome do evento" required {...campo(form, 'nome')} />
              <TextField label="Edição / ano" {...campo(form, 'edicao')} />
              <TextField label="Data da apresentação" type="date" required InputLabelProps={{ shrink: true }} {...campo(form, 'dataApresentacao')} />
              <TextField label="Horário da apresentação" type="time" required InputLabelProps={{ shrink: true }} {...campo(form, 'horarioApresentacao')} />
              <TextField label="Local" required {...campo(form, 'local')} helperText={form.formState.errors.local?.message ?? 'Na ATA, aparece após "na sala".'} />
              <TextField label="Campus" {...campo(form, 'campus')} />
              <TextField label="Prazo de submissão" type="date" InputLabelProps={{ shrink: true }} {...campo(form, 'prazoSubmissao')} />
              <TextField label="Prazo de envio da apresentação" type="date" InputLabelProps={{ shrink: true }} {...campo(form, 'prazoApresentacao')} />
              <TextField select label="Estado" value={form.watch('estado')} onChange={(e) => form.setValue('estado', e.target.value)}>
                {Object.entries(estadosEvento).map(([v, r]) => <MenuItem key={v} value={v}>{r}</MenuItem>)}
              </TextField>
            </Box>
            <Stack spacing={2} sx={{ mt: 2 }}>
              <TextField label="Descrição" multiline minRows={3} {...campo(form, 'descricao')} />
              <TextField label="Informações públicas" multiline minRows={3} {...campo(form, 'informacoesPublicas')} helperText="Exibidas na página pública do evento." />
              <Stack direction="row" spacing={2}>
                <Button type="submit" variant="contained" size="large" disabled={form.formState.isSubmitting}>Salvar evento</Button>
                <Button onClick={() => navegar(-1)}>Cancelar</Button>
              </Stack>
            </Stack>
          </form>
        </CardContent>
      </Card>
    </>
  );
}
